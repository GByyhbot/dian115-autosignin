package adapters

import (
	"context"
	"dian115-autosignin/core"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Handler 为一个站点专用处理器。
type Handler struct {
	Key          string
	Match        func(rawURL string) bool
	Signin       func(ctx context.Context, s core.Site, c *Client) Result
	Login        func(ctx context.Context, s core.Site, c *Client) Result // nil 时用通用登录
	SigninIsLogin bool
}

// handlers 为内置处理器表，键与 core.BuiltinPresets 对应。
var handlers = map[string]Handler{}

// LookupHandler 按预设键返回处理器；不存在返回 nil。
func LookupHandler(key string) *Handler {
	h, ok := handlers[key]
	if !ok {
		return nil
	}
	return &h
}

// MatchHandler 按 URL 匹配内置处理器键；无匹配返回 ""（走通用签到）。
func MatchHandler(rawURL string) string { return core.MatchPreset(rawURL) }

// exactMatcher 生成精确域名匹配器（忽略 www 前缀）。
func exactMatcher(domain string) func(string) bool {
	return func(raw string) bool {
		host := core.HostOf(raw)
		return host == domain || host == strings.TrimPrefix(domain, "www.")
	}
}

// containsMatcher 生成包含匹配器。
func containsMatcher(domain string) func(string) bool {
	return func(raw string) bool {
		return strings.Contains(core.HostOf(raw), domain)
	}
}

// docOf 解析可见 HTML 供站点逻辑使用。
func docOf(body []byte) *core.Doc { return core.ParseDoc(string(body)) }

// jsonOf 解析 JSON 正文。
func jsonOf(body []byte) (map[string]any, error) { return ParseJSON(body) }

// hasLoginPhp 常见 Cookie 失效判定。
func hasLoginPhp(page string) bool { return core.HasLoginPhp(page) }

// connectFail 统一连通性失败文案。
func connectFail() Result { return Fail("签到失败，请检查站点连通性") }

var numericRe = regexp.MustCompile(`^\d+(?:\.\d+)?$`)

// ---------------------------------------------------------------------------
// 52PT：bakatest 答题签到（随机答案）
// ---------------------------------------------------------------------------
func sign52(ctx context.Context, s core.Site, c *Client) Result {
	const site = "https://52pt.site"
	return bakaTestSignin(ctx, s, c, site)
}

func bakaTestSignin(ctx context.Context, s core.Site, c *Client, base string) Result {
	pageURL := base + "/bakatest.php"
	status, body, err := c.Get(ctx, pageURL, s, s.Timeout, nil)
	if err != nil {
		return connectFail()
	}
	page := string(body)
	if len(page) == 0 || hasLoginPhp(page) {
		if hasLoginPhp(page) {
			return Fail("签到失败，Cookie已失效")
		}
		return connectFail()
	}
	if core.SignInResult(page, []string{"今天已经签过到了"}) {
		return SignOK("今日已签到")
	}
	doc := docOf(body)
	questionEl, ok := doc.ElementAttr("input", "name", "questionid")
	questionID := ""
	if ok {
		questionID = questionEl["value"]
	}
	if questionID == "" {
		return Fail("签到失败，未获取到签到问题")
	}
	var options []string
	for _, e := range doc.Elements {
		if e.Tag == "input" && e.Attrs["name"] == "choice[]" {
			if v := e.Attrs["value"]; v != "" {
				options = append(options, v)
			}
		}
	}
	if len(options) == 0 {
		return Fail("签到失败，未获取到签到问题")
	}
	// 随机选择一个答案（原插件支持 OpenAI，本移植默认随机）
	choice := options[randInt(len(options))]
	form := url.Values{}
	form.Set("questionid", questionID)
	form.Set("choice[]", choice)
	form.Set("usercomment", "太难了！")
	form.Set("wantskip", "不会")
	status, body, err = c.PostForm(ctx, pageURL, s, s.Timeout, form, nil)
	if err != nil || status != 200 {
		return Fail("签到失败，签到接口请求失败")
	}
	page = string(body)
	if core.SignInResult(page, []string{`\d+点魔力值`}) {
		// 答题页面的奖励说明也包含“点魔力值”，回读已签到提示才能确认完成。
		_, checkBody, checkErr := c.Get(ctx, pageURL, s, s.Timeout, nil)
		if checkErr == nil && core.SignInResult(string(checkBody), []string{"今天已经签过到了"}) {
			return SignOK("签到成功")
		}
		return Fail("签到失败，未确认签到结果")
	}
	if core.SignInResult(page, []string{"今天已经签过到了"}) {
		return SignOK("今日已签到")
	}
	return Fail("签到失败，请到页面查看")
}

// ---------------------------------------------------------------------------
// 学校（BTSchool）
// ---------------------------------------------------------------------------
func signBTSchool(ctx context.Context, s core.Site, c *Client) Result {
	status, body, err := c.Get(ctx, "https://pt.btschool.club", s, s.Timeout, nil)
	if err != nil || len(body) == 0 {
		return connectFail()
	}
	_ = status
	page := string(body)
	if !core.IsLoggedIn(page) {
		return Fail("签到失败，Cookie已失效")
	}
	if core.HasSigninEvidence(page) {
		return SignOK("今日已签到")
	}
	status, body, err = c.Get(ctx, "https://pt.btschool.club/index.php?action=addbonus", s, s.Timeout, nil)
	if err != nil || len(body) == 0 {
		return Fail("签到失败，签到接口请求失败")
	}
	page = string(body)
	if core.IsLoggedIn(page) && core.HasSigninEvidence(page) {
		return SignOK("签到成功")
	}
	return Fail("签到失败，未确认签到结果")
}

// ---------------------------------------------------------------------------
// 白兔（Hares）
// ---------------------------------------------------------------------------
func signHares(ctx context.Context, s core.Site, c *Client) Result {
	status, body, err := c.Get(ctx, "https://club.hares.top", s, s.Timeout, nil)
	if err != nil || len(body) == 0 {
		return Fail("模拟访问失败，请检查站点连通性")
	}
	_ = status
	if hasLoginPhp(string(body)) {
		return Fail("模拟访问失败，Cookie已失效")
	}
	status, body, err = c.Get(ctx, "https://club.hares.top/attendance.php?action=sign", s, s.Timeout,
		map[string]string{"accept": "application/json"})
	if err != nil || status != 200 {
		return Fail("签到失败，签到接口请求失败")
	}
	v, jErr := jsonOf(body)
	if jErr != nil {
		return Fail("签到失败，签到接口返回数据异常")
	}
	code := JSONInt(v, "code")
	if code == 0 {
		return SignOK("签到成功")
	}
	if code == 1 && core.HasSigninEvidence(JSONString(v, "msg", "message")) {
		return SignOK("今日已签到")
	}
	return Fail("签到失败，签到接口未返回明确的成功或重复签到结果")
}

// ---------------------------------------------------------------------------
// 好大（HDArea）
// ---------------------------------------------------------------------------
func signHDArea(ctx context.Context, s core.Site, c *Client) Result {
	form := url.Values{"action": {"sign_in"}}
	status, body, err := c.PostForm(ctx, "https://hdarea.club/sign_in.php", s, s.Timeout, form, nil)
	if err != nil || status != 200 {
		return connectFail()
	}
	page := string(body)
	if hasLoginPhp(page) {
		return Fail("签到失败，Cookie已失效")
	}
	if core.SignInResult(page, []string{"此次签到您获得"}) {
		return SignOK("签到成功")
	}
	if core.SignInResult(page, []string{"请不要重复签到哦"}) {
		return SignOK("今日已签到")
	}
	return Fail("签到失败")
}

// ---------------------------------------------------------------------------
// 城市（HDCity）
// ---------------------------------------------------------------------------
func signHDCity(ctx context.Context, s core.Site, c *Client) Result {
	status, body, err := c.Get(ctx, "https://hdcity.city/sign", s, s.Timeout, nil)
	if err != nil || len(body) == 0 {
		return connectFail()
	}
	_ = status
	page := string(body)
	if strings.Contains(page, "login") {
		return Fail("签到失败，Cookie已失效")
	}
	if core.SignInResult(page, []string{"本次签到获得魅力"}) {
		return SignOK("签到成功")
	}
	if core.SignInResult(page, []string{"已签到"}) {
		return SignOK("今日已签到")
	}
	return Fail("签到失败")
}

// ---------------------------------------------------------------------------
// HDUPT
// ---------------------------------------------------------------------------
func signHDUpt(ctx context.Context, s core.Site, c *Client) Result {
	const home = "https://pt.hdupt.com"
	status, body, err := c.Get(ctx, home, s, s.Timeout, nil)
	if err != nil || len(body) == 0 {
		return connectFail()
	}
	_ = status
	page := string(body)
	if hasLoginPhp(page) {
		return Fail("签到失败，Cookie已失效")
	}
	if core.SignInResult(page, []string{`<span id="yiqiandao">`}) {
		return SignOK("今日已签到")
	}
	status, body, err = c.Get(ctx, home+"/added.php?action=qiandao", s, s.Timeout, nil)
	if err != nil || len(body) == 0 {
		return connectFail()
	}
	reward := strings.TrimSpace(string(body))
	if numericRe.MatchString(reward) {
		_, checkBody, checkErr := c.Get(ctx, home, s, s.Timeout, nil)
		if checkErr == nil && core.SignInResult(string(checkBody), []string{`<span id="yiqiandao">`}) {
			return SignOK("签到成功")
		}
	}
	return Fail("签到失败")
}

// ---------------------------------------------------------------------------
// NexusHD
// ---------------------------------------------------------------------------
func signNexusHD(ctx context.Context, s core.Site, c *Client) Result {
	form := url.Values{"action": {"post"}, "content": {""}}
	status, body, err := c.PostForm(ctx, "https://v6.nexushd.org/signin.php", s, s.Timeout, form, nil)
	if err != nil || status != 200 {
		return connectFail()
	}
	page := string(body)
	if hasLoginPhp(page) {
		return Fail("签到失败，Cookie已失效")
	}
	if core.SignInResult(page, []string{"本次签到获得"}) {
		return SignOK("签到成功")
	}
	if core.SignInResult(page, []string{"你今天已经签到过了"}) {
		return SignOK("今日已签到")
	}
	return Fail("签到失败")
}

// ---------------------------------------------------------------------------
// 猫（PTerClub）
// ---------------------------------------------------------------------------
func signPterClub(ctx context.Context, s core.Site, c *Client) Result {
	status, body, err := c.Get(ctx, "https://pterclub.com/attendance-ajax.php", s, s.Timeout, nil)
	if err != nil || len(body) == 0 {
		return connectFail()
	}
	_ = status
	page := string(body)
	if hasLoginPhp(page) {
		return Fail("签到失败，Cookie已失效")
	}
	v, jErr := jsonOf(body)
	if jErr != nil {
		return Fail("签到失败，签到接口返回数据异常")
	}
	st := JSONString(v, "status")
	if st == "1" {
		return SignOK("签到成功")
	}
	if st == "0" && core.HasSigninEvidence(JSONString(v, "message", "msg", "data")) {
		return SignOK("今日已签到")
	}
	return Fail("签到失败，签到接口未返回明确的成功或重复签到结果")
}

// ---------------------------------------------------------------------------
// PT时间（PTTime）
// ---------------------------------------------------------------------------
func signPTTime(ctx context.Context, s core.Site, c *Client) Result {
	status, body, err := c.Get(ctx, "https://www.pttime.org/attendance.php", s, s.Timeout, nil)
	if err != nil || len(body) == 0 {
		return connectFail()
	}
	_ = status
	page := string(body)
	if hasLoginPhp(page) {
		return Fail("签到失败，Cookie已失效")
	}
	if core.SignInResult(page, []string{"签到成功"}) {
		return SignOK("签到成功")
	}
	return Fail("签到失败")
}

// ---------------------------------------------------------------------------
// PTZONE：POST 提交签到
// ---------------------------------------------------------------------------
func signPTZone(ctx context.Context, s core.Site, c *Client) Result {
	if strings.TrimSpace(s.URL) == "" || (strings.TrimSpace(s.Cookie) == "" && strings.TrimSpace(s.CredentialRef) == "") {
		return Fail("签到失败，未配置站点地址或Cookie")
	}
	checkinURL := Resolve(s.URL, "attendance.php")
	status, body, err := c.PostForm(ctx, checkinURL, s, s.Timeout, url.Values{},
		map[string]string{"referer": checkinURL})
	if err != nil {
		return Fail("签到失败，无法打开网站")
	}
	page := string(body)
	if core.UnderChallenge(page) {
		return Fail("签到失败，站点被Cloudflare防护")
	}
	if status != 200 {
		return Fail(fmt.Sprintf("签到失败，状态码：%d", status))
	}
	if len(page) == 0 {
		return Fail("签到失败，站点返回空页面")
	}
	if core.HasLoginForm(page) {
		return Fail("签到失败，Cookie已失效")
	}
	if !core.HasLoginEvidence(page) {
		return Fail("签到失败，未识别到登录状态")
	}
	if core.HasSigninEvidence(page) {
		return SignOK("签到成功")
	}
	return Fail("签到失败，未确认签到结果")
}

// ---------------------------------------------------------------------------
// 天空（HDSky）：验证码 OCR
// ---------------------------------------------------------------------------
func signHDSky(ctx context.Context, s core.Site, c *Client) Result {
	status, body, err := c.Get(ctx, "https://hdsky.me", s, s.Timeout, nil)
	if err != nil || len(body) == 0 {
		return connectFail()
	}
	_ = status
	page := string(body)
	if hasLoginPhp(page) {
		return Fail("签到失败，Cookie已失效")
	}
	if core.SignInResult(page, []string{"已签到"}) {
		return SignOK("今日已签到")
	}
	// 获取验证码，最多尝试 4 次
	var imgHash string
	for attempt := 0; attempt < 4 && imgHash == ""; attempt++ {
		f := url.Values{"action": {"new"}}
		_, codeBody, codeErr := c.PostForm(ctx, "https://hdsky.me/image_code_ajax.php", s, s.Timeout, f,
			map[string]string{"content-type": "application/x-www-form-urlencoded; charset=UTF-8", "referer": "https://hdsky.me/index.php", "accept": "*/*"})
		if codeErr != nil {
			continue
		}
		if v, jErr := jsonOf(codeBody); jErr == nil && JSONBool(v, "success") {
			imgHash = JSONString(v, "code")
			break
		}
	}
	if imgHash == "" {
		return Fail("签到失败：未获取到验证码")
	}
	imgURL := "https://hdsky.me/image.php?action=regimage&imagehash=" + imgHash
	if DefaultCaptchaReader == nil {
		return Fail("签到失败：未获取到验证码")
	}
	code, ocrErr := DefaultCaptchaReader.Read(ctx, imgURL, s, c)
	if ocrErr != nil || len(code) != 6 {
		return Fail("签到失败：未获取到验证码")
	}
	form := url.Values{"action": {"showup"}, "imagehash": {imgHash}, "imagestring": {code}}
	status, body, err = c.PostForm(ctx, "https://hdsky.me/showup.php", s, s.Timeout, form,
		map[string]string{"referer": s.URL})
	if err != nil || status != 200 {
		return Fail("签到失败：未获取到验证码")
	}
	v, jErr := jsonOf(body)
	if jErr != nil {
		return Fail("签到失败：未获取到验证码")
	}
	if JSONBool(v, "success") {
		return SignOK("签到成功")
	}
	switch JSONString(v, "message", "msg") {
	case "date_unmatch":
		return SignOK("今日已签到")
	case "invalid_imagehash":
		return Fail("签到失败：验证码错误")
	}
	return Fail("签到失败：未获取到验证码")
}

// ---------------------------------------------------------------------------
// 皇后（OpenCD）：验证码 OCR
// ---------------------------------------------------------------------------
func signOpenCD(ctx context.Context, s core.Site, c *Client) Result {
	status, body, err := c.Get(ctx, "https://www.open.cd", s, s.Timeout, nil)
	if err != nil || len(body) == 0 {
		return connectFail()
	}
	_ = status
	page := string(body)
	if hasLoginPhp(page) {
		return Fail("签到失败，Cookie已失效")
	}
	if core.HasSigninEvidence(page) {
		return SignOK("今日已签到")
	}
	status, body, err = c.Get(ctx, "https://www.open.cd/plugin_sign-in.php", s, s.Timeout, nil)
	if err != nil || len(body) == 0 {
		return connectFail()
	}
	doc := docOf(body)
	var imgHash string
	var imgRel string
	for _, e := range doc.Elements {
		if e.Tag == "form" && e.Attrs["id"] == "frmSignin" {
			for _, inner := range doc.Elements {
				if inner.ParentIdx >= 0 && doc.Elements[inner.ParentIdx].Tag == "form" && doc.Elements[inner.ParentIdx].Attrs["id"] == "frmSignin" {
					if inner.Tag == "input" && inner.Attrs["name"] == "imagehash" {
						imgHash = inner.Attrs["value"]
					}
					if inner.Tag == "img" && imgRel == "" {
						imgRel = inner.Attrs["src"]
					}
				}
			}
		}
	}
	if imgRel == "" || imgHash == "" {
		return Fail("签到失败，获取签到参数失败")
	}
	imgURL := "https://www.open.cd/" + strings.TrimPrefix(imgRel, "/")
	if DefaultCaptchaReader == nil {
		return Fail("签到失败：未获取到验证码")
	}
	code, ocrErr := DefaultCaptchaReader.Read(ctx, imgURL, s, c)
	if ocrErr != nil || len(code) != 6 {
		return Fail("签到失败：未获取到验证码")
	}
	form := url.Values{"imagehash": {imgHash}, "imagestring": {code}}
	status, body, err = c.PostForm(ctx, "https://www.open.cd/plugin_sign-in.php?cmd=signin", s, s.Timeout, form, nil)
	if err != nil || status != 200 {
		return Fail("签到失败")
	}
	v, jErr := jsonOf(body)
	if jErr != nil {
		return Fail("签到失败")
	}
	if JSONString(v, "state") == "success" {
		return SignOK("签到成功")
	}
	return Fail("签到失败")
}
