package adapters

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"dian115-autosignin/core"
	"dian115-autosignin/pluginjson"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// 瓷器（HDChina）
// ---------------------------------------------------------------------------
func signHDChina(ctx context.Context, s core.Site, c *Client) Result {
	cookie := filterHDChinaCookie(s.Cookie)
	if cookie == "" {
		return Fail("签到失败，Cookie已失效")
	}
	s.Cookie = cookie
	status, body, err := c.Get(ctx, "https://hdchina.org/index.php", s, s.Timeout, nil)
	if err != nil || len(body) == 0 {
		return connectFail()
	}
	_ = status
	page := string(body)
	if hasLoginPhp(page) || strings.Contains(page, "阻断页面") {
		return Fail("签到失败，Cookie已失效")
	}
	if core.SignInResult(page, []string{`<a class="label label-default" href="#">已签到</a>`}) {
		return SignOK("今日已签到")
	}
	doc := docOf(body)
	csrfEl, ok := doc.ElementAttr("meta", "name", "x-csrf")
	if !ok {
		return Fail("签到失败，获取x-csrf失败")
	}
	csrf := csrfEl["content"]
	if csrf == "" {
		return Fail("签到失败，获取x-csrf失败")
	}
	form := url.Values{"csrf": {csrf}}
	status, body, err = c.PostForm(ctx, "https://hdchina.org/plugin_sign-in.php?cmd=signin", s, s.Timeout, form, nil)
	if err != nil || status != 200 {
		return Fail("签到失败，签到接口请求失败")
	}
	v, jErr := jsonOf(body)
	if jErr != nil {
		return Fail("签到失败，签到接口返回数据异常")
	}
	if JSONString(v, "state") == "success" {
		return SignOK("签到成功")
	}
	return Fail("签到失败")
}

// filterHDChinaCookie 仅保留包含 hdchina= 的 Cookie 段（原插件语义）。
func filterHDChinaCookie(cookie string) string {
	if strings.TrimSpace(cookie) == "" {
		return ""
	}
	parts := strings.Split(cookie, ";")
	kept := []string{}
	for _, p := range parts {
		if strings.Contains(strings.TrimSpace(p), "hdchina=") {
			kept = append(kept, strings.TrimSpace(p))
		}
	}
	if len(kept) == 0 {
		return ""
	}
	return strings.Join(kept, "; ")
}

// ---------------------------------------------------------------------------
// 海胆之家（HaiDan）
// ---------------------------------------------------------------------------
func signHaiDan(ctx context.Context, s core.Site, c *Client) Result {
	_, _, err := c.Get(ctx, "https://haidan.cc/signin.php", s, s.Timeout, nil)
	if err != nil {
		return connectFail()
	}
	status, body, err := c.Get(ctx, "https://haidan.cc/index.php", s, s.Timeout, nil)
	if err != nil || len(body) == 0 {
		return connectFail()
	}
	_ = status
	page := string(body)
	if hasLoginPhp(page) {
		return Fail("签到失败，Cookie已失效")
	}
	// 原插件使用 lookbehind，RE2 改写为直接匹配 value 属性
	if core.SignInResult(page, []string{`value="已经打卡"`}) {
		return SignOK("签到成功")
	}
	return Fail("签到失败")
}

// ---------------------------------------------------------------------------
// 彩虹岛（CHDBits）：bakatest 答题签到
// ---------------------------------------------------------------------------
func signCHDBits(ctx context.Context, s core.Site, c *Client) Result {
	return bakaTestSignin(ctx, s, c, "https://ptchdbits.co")
}

// ---------------------------------------------------------------------------
// 馒头（M-Team）：带签名 API 模拟登录（login 即签到）
// ---------------------------------------------------------------------------
func signMTeam(ctx context.Context, s core.Site, c *Client) Result {
	if strings.TrimSpace(s.URL) == "" {
		return Fail("签到失败，未配置站点地址")
	}
	token := strings.TrimSpace(s.Token)
	if token == "" {
		return Fail("签到失败，未配置 mTeam Token")
	}
	host := core.HostOf(s.URL)
	base := "https://" + host
	if !strings.HasPrefix(strings.ToLower(s.URL), "http") {
		base = s.URL
	} else {
		base = strings.TrimRight(s.URL, "/")
	}
	ts := time.Now().UnixMilli()
	msg := fmt.Sprintf("POST&/api/member/updateLastBrowse&%d", ts)
	sign := base64.StdEncoding.EncodeToString(hmacSHA1(token, msg))
	did := sha256Hex(token)
	headers := map[string]string{
		"content-type": "application/x-www-form-urlencoded",
		"accept":       "application/json",
		"authorization": token,
		"origin":        base,
		"referer":       base + "/index",
		"ts":            strconv.FormatInt(ts/1000, 10),
		"webversion":    "2000",
		"did":           did[:32],
		"visitorid":     did[32:],
	}
	form := url.Values{"_timestamp": {strconv.FormatInt(ts, 10)}, "_sgin": {sign}}
	status, body, err := c.PostForm(ctx, base+"/api/member/updateLastBrowse", s, s.Timeout, form, headers)
	if err != nil || status != 200 {
		return Fail("签到失败：网络请求失败")
	}
	v, jErr := jsonOf(body)
	if jErr != nil {
		return Fail("签到失败：接口返回异常")
	}
	code := JSONString(v, "code")
	message := JSONString(v, "message", "msg")
	if JSONBool(v, "success") || code == "0" || strings.EqualFold(message, "SUCCESS") ||
		(code == "1" && strings.EqualFold(message, "FAIL")) {
		return LoginOK("模拟登录成功")
	}
	if code == "401" || strings.Contains(strings.ToLower(message), "full authentication") {
		return Fail("签到失败：Token已过期")
	}
	return Fail(fmt.Sprintf("签到失败：%s", message))
}

func hmacSHA1(secret, message string) []byte {
	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write([]byte(message))
	return mac.Sum(nil)
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// ---------------------------------------------------------------------------
// TTG：签名时间戳签到
// ---------------------------------------------------------------------------
var (
	ttgTimestampRe = regexp.MustCompile(`signed_timestamp:\s*"(\d{10})"`)
	ttgTokenRe     = regexp.MustCompile(`signed_token:\s*"([^"]*)"`)
)

func signTTG(ctx context.Context, s core.Site, c *Client) Result {
	status, body, err := c.Get(ctx, "https://totheglory.im", s, s.Timeout, nil)
	if err != nil || len(body) == 0 {
		return connectFail()
	}
	_ = status
	page := string(body)
	if hasLoginPhp(page) {
		return Fail("签到失败，Cookie已失效")
	}
	if core.SignInResult(page, []string{`<b style="color:green;">已签到</b>`}) {
		return SignOK("今日已签到")
	}
	mTs := ttgTimestampRe.FindStringSubmatch(page)
	mToken := ttgTokenRe.FindStringSubmatch(page)
	if len(mTs) < 2 || len(mToken) < 2 {
		return Fail("签到失败，获取签名参数失败")
	}
	form := url.Values{"signed_timestamp": {mTs[1]}, "signed_token": {mToken[1]}}
	status, body, err = c.PostForm(ctx, "https://totheglory.im/signed.php", s, s.Timeout, form, nil)
	if err != nil || status != 200 {
		return Fail("签到失败，签到接口请求失败")
	}
	page = string(body)
	if core.SignInResult(page, []string{"您已连续签到"}) {
		return SignOK("签到成功")
	}
	if core.SignInResult(page, []string{"亲，您今天已签到过，不要太贪哦"}) {
		return SignOK("今日已签到")
	}
	return Fail("签到失败")
}

// ---------------------------------------------------------------------------
// U2：表单答题签到（9 点后）
// ---------------------------------------------------------------------------
var redirectRe = regexp.MustCompile(`window\s*\.\s*location\s*\.\s*href\s*=\s*['"]([^'"]+)['"]`)

func signU2(ctx context.Context, s core.Site, c *Client) Result {
	if time.Now().Hour() < 9 {
		return Fail("签到失败，9点前不签到")
	}
	pageURL := "https://u2.dmhy.org/showup.php"
	status, body, err := c.Get(ctx, pageURL, s, s.Timeout, nil)
	if err != nil || len(body) == 0 {
		return connectFail()
	}
	_ = status
	page := string(body)
	if !core.IsLoggedIn(page) {
		return Fail("签到失败，Cookie已失效")
	}
	if hasSignedState(page) {
		return SignOK("今日已签到")
	}
	doc := docOf(body)
	var req, hash, form, csrf string
	var submits [][2]string
	for _, e := range doc.Elements {
		if e.Tag != "input" {
			continue
		}
		switch e.Attrs["name"] {
		case "req":
			req = e.Attrs["value"]
		case "hash":
			hash = e.Attrs["value"]
		case "form":
			form = e.Attrs["value"]
		case "_csrf":
			csrf = e.Attrs["value"]
		}
	}
	for _, e := range doc.Elements {
		if e.Tag == "input" && strings.EqualFold(e.Attrs["type"], "submit") {
			submits = append(submits, [2]string{e.Attrs["name"], e.Attrs["value"]})
		}
	}
	if req == "" || hash == "" || form == "" || csrf == "" || len(submits) == 0 {
		return Fail("签到失败，获取签到参数失败")
	}
	pick := submits[randInt(len(submits))]
	values := url.Values{
		"req":     {req},
		"hash":    {hash},
		"form":    {form},
		"_csrf":   {csrf},
		"message": {"一切随缘~"},
		"usercomment": {"一切随缘~"},
		pick[0]:   {pick[1]},
	}
	status, body, err = c.PostForm(ctx, pageURL+"?action=show", s, s.Timeout, values, nil)
	if err != nil || status != 200 {
		return Fail("签到失败，签到接口请求失败")
	}
	m := redirectRe.FindSubmatch(body)
	if m != nil {
		_, checkBody, checkErr := c.Get(ctx, pageURL, s, s.Timeout, nil)
		if checkErr == nil && hasSignedState(string(checkBody)) {
			return SignOK("签到成功")
		}
		return Fail("签到失败，未确认签到结果")
	}
	if hasSignedState(string(body)) {
		return SignOK("签到成功")
	}
	return Fail("签到失败，未确认签到结果")
}

// hasSignedState U2 签到完成状态：不再出现 req 输入框且命中签到文案。
func hasSignedState(page string) bool {
	doc := core.ParseDoc(page)
	for _, e := range doc.Elements {
		if e.Tag == "input" && e.Attrs["name"] == "req" {
			return false
		}
		if e.Tag == "form" && strings.Contains(strings.ToLower(e.Attrs["action"]), "showup.php") {
			return false
		}
	}
	return core.SignInResult(page, []string{`<a href="showup.php">已签到</a>`})
}

// ---------------------------------------------------------------------------
// Rousi Pro：PeerGo API 签到/登录
// ---------------------------------------------------------------------------
func bearer(v string) string {
	if strings.HasPrefix(strings.ToLower(v), "bearer ") {
		return v
	}
	return "Bearer " + v
}

func rousiSignin(ctx context.Context, s core.Site, c *Client) Result {
	if strings.TrimSpace(s.Apikey) == "" && strings.TrimSpace(s.Token) == "" {
		return Fail("签到失败，未配置 Rousi API Key 或 Token")
	}
	payload := map[string]any{"mode": "fixed"}
	status, body, err := c.PostJSON(ctx, "https://rousi.pro/api/points/attendance", s, s.Timeout, payload, map[string]string{
		"api-token": s.Apikey, "accept": "application/json"})
	if err != nil {
		return Fail("签到失败：网络请求失败")
	}
	if status == 200 {
		if v, jErr := jsonOf(body); jErr == nil && JSONInt(v, "code") == 0 {
			return SignOK("签到成功")
		}
	}
	if status == 400 {
		if v, jErr := jsonOf(body); jErr == nil && JSONInt(v, "code") == 1 && core.HasSigninEvidence(JSONString(v, "message", "msg")) {
			return SignOK("今日已签到")
		}
	}
	if s.Token != "" {
		status, body, err = c.PostJSON(ctx, "https://rousi.pro/api/points/attendance", s, s.Timeout, payload,
			map[string]string{"authorization": bearer(s.Token), "accept": "application/json"})
		if err != nil {
			return Fail("签到失败：网络请求失败")
		}
		if status == 200 {
			if v, jErr := jsonOf(body); jErr == nil && JSONInt(v, "code") == 0 {
				return SignOK("签到成功")
			}
		}
		if status == 400 {
			if v, jErr := jsonOf(body); jErr == nil && JSONInt(v, "code") == 1 && core.HasSigninEvidence(JSONString(v, "message", "msg")) {
				return SignOK("今日已签到")
			}
		}
	}
	return Fail("签到失败：接口返回异常")
}

func rousiLogin(ctx context.Context, s core.Site, c *Client) Result {
	if strings.TrimSpace(s.Apikey) != "" {
		status, body, err := c.Get(ctx, "https://rousi.pro/api/v1/profile", s, s.Timeout,
			map[string]string{"authorization": bearer(s.Apikey), "accept": "application/json"})
		if err != nil {
			return Fail("模拟登录失败：网络请求失败")
		}
		if status == 200 {
			if v, jErr := jsonOf(body); jErr == nil && JSONInt(v, "code") == 0 {
				return Result{Success: true, LoggedIn: true, Message: "模拟登录成功"}
			}
		}
	}
	if strings.TrimSpace(s.Token) != "" {
		status, body, err := c.Get(ctx, "https://rousi.pro/api/points/attendance/stats", s, s.Timeout,
			map[string]string{"authorization": bearer(s.Token), "accept": "application/json"})
		if err != nil {
			return Fail("模拟登录失败：网络请求失败")
		}
		if status == 200 {
			if v, jErr := jsonOf(body); jErr == nil && JSONInt(v, "code") == 0 {
				return Result{Success: true, LoggedIn: true, Message: "模拟登录成功"}
			}
		}
	}
	return Fail("模拟登录失败：Token或API Key已失效")
}

// ---------------------------------------------------------------------------
// YemaPT：JSON 接口
// ---------------------------------------------------------------------------
func signYema(ctx context.Context, s core.Site, c *Client) Result {
	if strings.TrimSpace(s.URL) == "" {
		return Fail("签到失败，未配置站点地址")
	}
	status, body, err := c.Get(ctx, Resolve(s.URL, "/api/consumer/checkIn"), s, s.Timeout,
		map[string]string{"accept": "application/json"})
	if err != nil {
		return Fail("签到失败：网络请求失败")
	}
	_ = status
	v, jErr := jsonOf(body)
	if jErr != nil {
		return Fail("签到失败：接口返回异常")
	}
	if JSONBool(v, "success") {
		return SignOK("签到成功")
	}
	return Fail("签到失败")
}

func loginYema(ctx context.Context, s core.Site, c *Client) Result {
	status, body, err := c.Get(ctx, Resolve(s.URL, "/api/user/profile"), s, s.Timeout,
		map[string]string{"accept": "application/json"})
	if err != nil {
		return Fail("模拟登录失败：网络请求失败")
	}
	_ = status
	v, jErr := jsonOf(body)
	if jErr != nil {
		return Fail("模拟登录失败：接口返回异常")
	}
	if JSONBool(v, "success") {
		return Result{Success: true, LoggedIn: true, Message: "模拟登录成功"}
	}
	return Fail("模拟登录失败：Cookie已失效")
}

// ---------------------------------------------------------------------------
// 朱雀（ZHUQUE）：释放技能确认登录
// ---------------------------------------------------------------------------
func loginZhuque(ctx context.Context, s core.Site, c *Client) Result {
	status, body, err := c.Get(ctx, "https://zhuque.in", s, s.Timeout, nil)
	if err != nil || len(body) == 0 {
		return Fail("模拟登录失败：网络请求失败")
	}
	_ = status
	page := string(body)
	if hasLoginPhp(page) {
		return Fail("模拟登录失败，Cookie已失效")
	}
	doc := docOf(body)
	csrfEl, ok := doc.ElementAttr("meta", "name", "x-csrf-token")
	csrf := ""
	if ok {
		csrf = csrfEl["content"]
	}
	if csrf == "" {
		return Result{Success: true, LoggedIn: true, Message: "模拟登录成功"}
	}
	status, body, err = c.PostJSON(ctx, "https://zhuque.in/api/gaming/fireGenshinCharacterMagic", s, s.Timeout,
		map[string]any{"all": 1, "resetModal": "true"},
		map[string]string{"x-csrf-token": csrf, "accept": "application/json"})
	msg := "失败"
	if err == nil && status == 200 {
		if v, jErr := jsonOf(body); jErr == nil && JSONInt(v, "status") == 200 {
			msg = "成功，获得" + JSONString(v, "data", "msg", "message") + "魔力"
		}
	}
	return Result{Success: true, LoggedIn: true, Message: "模拟登录成功，技能释放" + msg}
}

// ---------------------------------------------------------------------------
// 北洋（TJUPT）：验证码图片匹配（douban 辅助）
// ---------------------------------------------------------------------------
var tjuptOptionRe = regexp.MustCompile(`(?is)<input[^>]*name=["']ban_robot["'][^>]*value=["']([^"']*)["'][^>]*>`)

func signTJUPT(ctx context.Context, s core.Site, c *Client) Result {
	status, body, err := c.Get(ctx, "https://tjupt.org/attendance.php", s, s.Timeout, nil)
	if err != nil || len(body) == 0 {
		return connectFail()
	}
	_ = status
	page := string(body)
	if hasLoginPhp(page) {
		return Fail("签到失败，Cookie已失效")
	}
	if core.SignInResult(page, []string{`<a href="attendance.php">今日已签到</a>`}) {
		return SignOK("今日已签到")
	}
	doc := docOf(body)
	imgs := doc.DescendantsWithAncestorClass("img", "captcha")
	if len(imgs) == 0 {
		return Fail("签到失败，获取验证码失败")
	}
	imgRel := imgs[0].Attrs["src"]
	if imgRel == "" {
		return Fail("签到失败，获取验证码失败")
	}
	imgURL := "https://tjupt.org/" + strings.TrimPrefix(imgRel, "/")
	_, imgBody, imgErr := c.Get(ctx, imgURL, s, s.Timeout, nil)
	if imgErr != nil || len(imgBody) == 0 {
		return Fail("签到失败，获取验证码失败")
	}
	hash, hashErr := imageHash(imgBody)
	if hashErr != nil {
		return Fail("签到失败，验证码图像处理失败")
	}
	// 答案缓存
	answers := map[string]string{}
	answersKey := "tjupt_answers"
	if c.Store != nil {
		if raw, _, found, rErr := c.Store.Read(answersKey); rErr == nil && found {
			for k, v := range pluginjson.Fields(raw) {
				answers[k] = pluginjson.Text(v)
			}
			if ans, ok := answers[hash]; ok && tjuptHasOption(page, ans) {
				return tjuptSubmit(ctx, s, c, ans, page)
			}
		}
	}
	// 逐个选项去豆瓣搜索匹配
	options := tjuptOptions(page)
	answer := ""
	for _, opt := range options {
		if answer != "" {
			break
		}
		if matched := tjuptMatchDouban(ctx, c, s, opt, hash); matched {
			answer = opt
		}
	}
	if answer == "" {
		return Fail("签到失败，未找到验证码匹配答案")
	}
	if c.Store != nil {
		answers[hash] = answer
		cached, err := pluginjson.Encode(answers)
		if err == nil {
			_ = c.Store.Write(answersKey, pluginjson.Raw(cached), "", "tjupt-answers-cache")
		}
	}
	return tjuptSubmit(ctx, s, c, answer, page)
}

func tjuptOptions(page string) []string {
	out := []string{}
	for _, m := range tjuptOptionRe.FindAllStringSubmatchIndex(page, -1) {
		val := page[m[2]:m[3]]
		start := m[1]
		end := strings.IndexByte(page[start:], '<')
		if end < 0 {
			continue
		}
		text := strings.TrimSpace(page[start : start+end])
		if text != "" {
			out = append(out, text)
		}
		_ = val
	}
	return out
}

func tjuptHasOption(page, ans string) bool {
	for _, opt := range tjuptOptions(page) {
		if opt == ans {
			return true
		}
	}
	return false
}

func tjuptSubmit(ctx context.Context, s core.Site, c *Client, answer, _ string) Result {
	form := url.Values{"ban_robot": {answer}}
	status, body, err := c.PostForm(ctx, "https://tjupt.org/attendance.php", s, s.Timeout, form, nil)
	if err != nil || status != 200 {
		return Fail("签到失败，签到接口请求失败")
	}
	page := string(body)
	if core.SignInResult(page, []string{`<a href="attendance.php">今日已签到</a>`}) {
		return SignOK("签到成功")
	}
	return Fail("签到失败，签到结果未确认")
}

// tjuptMatchDouban 在豆瓣 sug 接口中检索选项文本对应海报图并比较感知哈希。
func tjuptMatchDouban(ctx context.Context, c *Client, s core.Site, opt, targetHash string) bool {
	q := url.Values{"q": {opt}}
	sugURL := "https://movie.douban.com/j/subject_suggest?" + q.Encode()
	status, body, err := c.Get(ctx, sugURL, s, 20, map[string]string{"accept": "application/json"})
	if err != nil || status != 200 {
		return false
	}
	list := pluginjson.Raw(body)
	for position := 0; ; position++ {
		item := pluginjson.Index(list, position)
		if item == nil {
			break
		}
		imgURL := pluginjson.Text(pluginjson.Field(item, "img"))
		if imgURL == "" {
			continue
		}
		_, imgBody, imgErr := c.Get(ctx, imgURL, s, 20, nil)
		if imgErr != nil {
			continue
		}
		h, hErr := imageHash(imgBody)
		if hErr != nil {
			continue
		}
		if compareHashes(targetHash, h) > 0.9 {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// 注册表
// ---------------------------------------------------------------------------
func init() {
	reg := func(h Handler) { handlers[h.Key] = h }
	reg(Handler{Key: "52pt", Match: exactMatcher("52pt.site"), Signin: sign52})
	reg(Handler{Key: "btschool", Match: exactMatcher("pt.btschool.club"), Signin: signBTSchool})
	reg(Handler{Key: "chdbits", Match: exactMatcher("ptchdbits.co"), Signin: signCHDBits})
	reg(Handler{Key: "haidan", Match: exactMatcher("haidan.cc"), Signin: signHaiDan})
	reg(Handler{Key: "hares", Match: exactMatcher("club.hares.top"), Signin: signHares})
	reg(Handler{Key: "hdarea", Match: exactMatcher("hdarea.club"), Signin: signHDArea})
	reg(Handler{Key: "hdchina", Match: exactMatcher("hdchina.org"), Signin: signHDChina})
	reg(Handler{Key: "hdcity", Match: exactMatcher("hdcity.city"), Signin: signHDCity})
	reg(Handler{Key: "hdsky", Match: exactMatcher("hdsky.me"), Signin: signHDSky})
	reg(Handler{Key: "hdupt", Match: exactMatcher("pt.hdupt.com"), Signin: signHDUpt})
	reg(Handler{Key: "mteam", Match: containsMatcher("m-team"), Signin: signMTeam, SigninIsLogin: true})
	reg(Handler{Key: "nexushd", Match: exactMatcher("v6.nexushd.org"), Signin: signNexusHD})
	reg(Handler{Key: "opencd", Match: exactMatcher("open.cd"), Signin: signOpenCD})
	reg(Handler{Key: "pterclub", Match: exactMatcher("pterclub.com"), Signin: signPterClub})
	reg(Handler{Key: "pttime", Match: exactMatcher("pttime.org"), Signin: signPTTime})
	reg(Handler{Key: "ptzone", Match: exactMatcher("ptzone.xyz"), Signin: signPTZone})
	reg(Handler{Key: "rousi", Match: exactMatcher("rousi.pro"), Signin: rousiSignin, Login: rousiLogin})
	reg(Handler{Key: "tjupt", Match: exactMatcher("tjupt.org"), Signin: signTJUPT})
	reg(Handler{Key: "ttg", Match: exactMatcher("totheglory.im"), Signin: signTTG})
	reg(Handler{Key: "u2", Match: exactMatcher("u2.dmhy.org"), Signin: signU2})
	reg(Handler{Key: "yema", Match: exactMatcher("yemapt.org"), Signin: signYema, Login: loginYema})
	reg(Handler{Key: "zhuque", Match: exactMatcher("zhuque.in"), Login: loginZhuque, SigninIsLogin: true})
}
