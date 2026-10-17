package adapters

import (
	"context"
	"dian115-autosignin/core"
	"dian115-autosignin/host"
	"strings"
	"testing"
	"time"
)

func timeNow() time.Time { return time.Now() }

// mockBroker 按路径前缀返回预设响应；支持按调用次数轮换 bodySeq。
type mockBroker struct {
	routes []route
}

type route struct {
	contains string
	method   string
	status   int
	body     string
	bodySeq  []string
	calls    int
}

func (m *mockBroker) Do(req host.Request) (host.Response, error) {
	best := -1
	bestLen := -1
	for i, r := range m.routes {
		if r.contains != "" && !strings.Contains(req.Path, r.contains) {
			continue
		}
		if r.method != "" && req.Method != r.method {
			continue
		}
		if len(r.contains) > bestLen {
			best = i
			bestLen = len(r.contains)
		}
	}
	if best < 0 {
		return host.Response{Status: 404}, nil
	}
	r := &m.routes[best]
	body := r.body
	if len(r.bodySeq) > 0 {
		idx := r.calls
		if idx >= len(r.bodySeq) {
			idx = len(r.bodySeq) - 1
		}
		body = r.bodySeq[idx]
	}
	r.calls++
	return host.Response{Status: r.status, Headers: map[string][]string{"content-type": {"text/html"}}, BodyBase64: host.EncodeBody([]byte(body))}, nil
}

func newClient(routes []route) *Client {
	return &Client{Broker: &mockBroker{routes: routes}}
}

func site(key, cookie, token string) core.Site {
	s := core.Site{ID: key, Key: key, URL: "https://example.org", Cookie: cookie, Token: token}
	if p, ok := core.PresetByKey(key); ok {
		s.URL = "https://" + p.Domain
	}
	return s
}

func TestBaseSigninSuccess(t *testing.T) {
	page := `<html><body><a href="logout.php">退出登录</a>
<input type="submit" value="签到成功">
</body></html>`
	c := newClient([]route{
		{contains: "attendance.php", method: "GET", status: 200, body: page},
	})
	res := BaseSignin(context.Background(), site("custom", "sid=1", ""), c)
	if !res.Completed() || !res.Success {
		t.Fatalf("expected success, got %+v", res)
	}
}

func TestBaseSigninLoginOnly(t *testing.T) {
	page := `<html><body><a href="logout.php">退出登录</a></body></html>`
	c := newClient([]route{{contains: "attendance.php", method: "GET", status: 200, body: page}})
	res := BaseSignin(context.Background(), site("custom", "sid=1", ""), c)
	if !res.LoggedIn || res.Success {
		t.Fatalf("expected login-only, got %+v", res)
	}
}

func TestBaseSigninCookieExpired(t *testing.T) {
	page := `<html><body><input type="password" name="password"></body></html>`
	c := newClient([]route{{contains: "attendance.php", method: "GET", status: 200, body: page}})
	res := BaseSignin(context.Background(), site("custom", "sid=1", ""), c)
	if res.Completed() {
		t.Fatalf("expected failure, got %+v", res)
	}
	if !strings.Contains(res.Message, "未识别到登录状态") {
		t.Fatalf("message = %q", res.Message)
	}
}

func TestSignPTZone(t *testing.T) {
	page := `<html><body><a href="logout.php">退出登录</a><span>今天签到您获得5点魔力值</span></body></html>`
	c := newClient([]route{
		{contains: "attendance.php", method: "POST", status: 200, body: page},
	})
	res := signPTZone(context.Background(), site("ptzone", "uid=1", ""), c)
	if !res.Completed() || !res.Success {
		t.Fatalf("expected success, got %+v", res)
	}
}

func TestSignHares(t *testing.T) {
	c := newClient([]route{
		{contains: "club.hares.top", method: "GET", status: 200, body: `<a href="logout.php">退出</a>`},
		{contains: "attendance.php?action=sign", method: "GET", status: 200, body: `{"code":0,"msg":"签到成功"}`},
	})
	res := signHares(context.Background(), site("hares", "uid=1", ""), c)
	if !res.Success {
		t.Fatalf("expected success, got %+v", res)
	}
}

func TestSignHDAreaRepeat(t *testing.T) {
	c := newClient([]route{
		{contains: "sign_in.php", method: "POST", status: 200, body: `<div class="text">请不要重复签到哦</div>`},
	})
	res := signHDArea(context.Background(), site("hdarea", "uid=1", ""), c)
	if !res.Success {
		t.Fatalf("expected already-signed success, got %+v", res)
	}
}

func TestSignNexusHD(t *testing.T) {
	c := newClient([]route{
		{contains: "signin.php", method: "POST", status: 200, body: `<span>本次签到获得 30 魔力</span>`},
	})
	res := signNexusHD(context.Background(), site("nexushd", "uid=1", ""), c)
	if !res.Success {
		t.Fatalf("expected success, got %+v", res)
	}
}

func TestSignPterClub(t *testing.T) {
	c := newClient([]route{
		{contains: "attendance-ajax.php", method: "GET", status: 200, body: `{"status":"1","message":"success"}`},
	})
	res := signPterClub(context.Background(), site("pterclub", "uid=1", ""), c)
	if !res.Success {
		t.Fatalf("expected success, got %+v", res)
	}
}

func TestSignPTTime(t *testing.T) {
	c := newClient([]route{
		{contains: "attendance.php", method: "GET", status: 200, body: `<div>签到成功，获得魔力值</div>`},
	})
	res := signPTTime(context.Background(), site("pttime", "uid=1", ""), c)
	if !res.Success {
		t.Fatalf("expected success, got %+v", res)
	}
}

func TestSignHDChina(t *testing.T) {
	index := `<html><head><meta name="x-csrf" content="csrf123"></head>
<body><a href="logout.php">退出</a></body></html>`
	c := newClient([]route{
		{contains: "hdchina.org/index.php", method: "GET", status: 200, body: index},
		{contains: "plugin_sign-in.php", method: "POST", status: 200, body: `{"state":"success"}`},
	})
	s := site("hdchina", "hdchina=abc; other=x", "")
	res := signHDChina(context.Background(), s, c)
	if !res.Success {
		t.Fatalf("expected success, got %+v", res)
	}
}

func TestSignHDChinaCookieFilter(t *testing.T) {
	c := newClient(nil)
	res := signHDChina(context.Background(), site("hdchina", "other=x", ""), c)
	if res.Completed() {
		t.Fatalf("expected failure for filtered cookie, got %+v", res)
	}
}

func TestSignHaiDan(t *testing.T) {
	c := newClient([]route{
		{contains: "signin.php", method: "GET", status: 200, body: `ok`},
		{contains: "index.php", method: "GET", status: 200, body: `<input type="text" value="已经打卡">`},
	})
	res := signHaiDan(context.Background(), site("haidan", "uid=1", ""), c)
	if !res.Success {
		t.Fatalf("expected success, got %+v", res)
	}
}

func TestSignTTG(t *testing.T) {
	home := `<html><body><a href="logout.php">退出</a>
<script>var signed_timestamp: "1720000000"; var signed_token: "tok123";</script></body></html>`
	c := newClient([]route{
		{contains: "totheglory.im", method: "GET", status: 200, body: home},
		{contains: "signed.php", method: "POST", status: 200, body: `<b>您已连续签到 30 天</b>`},
	})
	res := signTTG(context.Background(), site("ttg", "uid=1", ""), c)
	if !res.Success {
		t.Fatalf("expected success, got %+v", res)
	}
}

func TestSignMTeam(t *testing.T) {
	c := newClient([]route{
		{contains: "updateLastBrowse", method: "POST", status: 200, body: `{"success":true,"code":"0","message":"SUCCESS"}`},
	})
	s := site("mteam", "", "dummy-token")
	res := signMTeam(context.Background(), s, c)
	if !res.LoggedIn {
		t.Fatalf("expected login success, got %+v", res)
	}
}

func TestSign52Baka(t *testing.T) {
	baka := `<html><body><a href="logout.php">退出</a>
<form method="post"><input name="questionid" value="7">
<input name="choice[]" value="A"><input name="choice[]" value="B">
</form></body></html>`
	signed := `<html><body><a href="logout.php">退出</a><span>今天已经签过到了</span></body></html>`
	c := newClient([]route{
		{contains: "52pt.site/bakatest.php", method: "GET", status: 200, bodySeq: []string{baka, signed}},
		{contains: "bakatest.php", method: "POST", status: 200, body: `<div>答对啦，获得5点魔力值</div>`},
	})
	res := bakaTestSignin(context.Background(), site("52pt", "uid=1", ""), c, "https://52pt.site")
	if !res.Success {
		t.Fatalf("expected success, got %+v", res)
	}
}

func TestSignYema(t *testing.T) {
	c := newClient([]route{
		{contains: "api/consumer/checkIn", method: "GET", status: 200, body: `{"success":true}`},
	})
	res := signYema(context.Background(), site("yema", "uid=1", ""), c)
	if !res.Success {
		t.Fatalf("expected success, got %+v", res)
	}
}

func TestRousiSignin(t *testing.T) {
	c := newClient([]route{
		{contains: "api/points/attendance", method: "POST", status: 200, body: `{"code":0,"message":"success"}`},
	})
	s := site("rousi", "", "")
	s.Apikey = "peer-go-key"
	res := rousiSignin(context.Background(), s, c)
	if !res.Success {
		t.Fatalf("expected success, got %+v", res)
	}
}

func TestSignU2(t *testing.T) {
	if timeNow().Hour() < 9 {
		t.Skip("9点前不签到")
	}
	form := `<html><body><a href="logout.php">退出</a>
<form action="showup.php"><input type="hidden" name="req" value="r1">
<input type="hidden" name="hash" value="h1"><input type="hidden" name="form" value="f1">
<input type="hidden" name="_csrf" value="c1">
<input type="submit" name="submit" value="签到">
</form></body></html>`
	signed := `<html><body><a href="logout.php">退出</a><a href="showup.php">已签到</a></body></html>`
	c := newClient([]route{
		{contains: "showup.php", method: "GET", status: 200, bodySeq: []string{form, signed}},
		{contains: "showup.php?action=show", method: "POST", status: 200, body: `<script>window.location.href = 'showup.php';</script>`},
	})
	res := signU2(context.Background(), site("u2", "uid=1", ""), c)
	if !res.Success {
		t.Fatalf("expected success, got %+v", res)
	}
}

func TestSignHDCity(t *testing.T) {
	c := newClient([]route{
		{contains: "hdcity.city/sign", method: "GET", status: 200, body: `<div>本次签到获得魅力 5</div>`},
	})
	res := signHDCity(context.Background(), site("hdcity", "uid=1", ""), c)
	if !res.Success {
		t.Fatalf("expected success, got %+v", res)
	}
}

func TestSignBTSchool(t *testing.T) {
	home := `<html><body><a href="logout.php">退出</a><span>今天签到您获得5点魔力值</span></body></html>`
	c := newClient([]route{
		{contains: "pt.btschool.club", method: "GET", status: 200, body: home},
	})
	res := signBTSchool(context.Background(), site("btschool", "uid=1", ""), c)
	if !res.Success {
		t.Fatalf("expected success, got %+v", res)
	}
}

func TestSignHDUpt(t *testing.T) {
	home := `<html><body><a href="logout.php">退出</a><span id="yiqiandao">1.5</span></body></html>`
	c := newClient([]route{
		{contains: "pt.hdupt.com", method: "GET", status: 200, body: home},
	})
	res := signHDUpt(context.Background(), site("hdupt", "uid=1", ""), c)
	if !res.Success {
		t.Fatalf("expected success, got %+v", res)
	}
}

func TestLookupHandler(t *testing.T) {
	for _, p := range core.BuiltinPresets {
		if LookupHandler(p.Key) == nil {
			t.Errorf("handler missing for %s", p.Key)
		}
	}
}
