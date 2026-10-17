package core

import "testing"

func TestVisibleHTMLStripsScriptsAndHidden(t *testing.T) {
	html := `<html><head><title>T</title></head><body>
<script>var x = "签到成功";</script>
<div style="display:none">已签到</div>
<input type="hidden" value="签到成功">
<div class="main"><a href="logout.php">退出登录</a> 欢迎回来</div>
</body></html>`
	doc := ParseVisible(html)
	if doc.Text == "" {
		t.Fatal("expected visible text")
	}
	if contains(doc.Text, "var x") || contains(doc.Text, "已签到") || contains(doc.Text, "签到成功") {
		t.Fatalf("hidden content leaked into visible text: %q", doc.Text)
	}
	if !contains(doc.Text, "欢迎回来") {
		t.Fatalf("expected visible text kept: %q", doc.Text)
	}
}

func TestHasLoginForm(t *testing.T) {
	if !HasLoginForm(`<input type="text"><input type="password" name="password">`) {
		t.Fatal("password input must be detected")
	}
	if HasLoginForm(`<input type="text" name="user">`) {
		t.Fatal("no password input must not be detected")
	}
}

func TestIsLoggedIn(t *testing.T) {
	if !IsLoggedIn(`<html><body><a href="logout.php">退出</a></body></html>`) {
		t.Fatal("logout link must imply logged in")
	}
	if !IsLoggedIn(`<html><body><form action="logout"><input type="submit"></form></body></html>`) {
		t.Fatal("logout form must imply logged in")
	}
	if IsLoggedIn(`<html><body><input type="password"></body></html>`) {
		t.Fatal("password form must not imply logged in")
	}
	if IsLoggedIn(`<html><body>hello</body></html>`) {
		t.Fatal("plain page must not imply logged in")
	}
}

func TestHasLoginEvidence(t *testing.T) {
	if !HasLoginEvidence(`<a href="logout.php">退出登录</a>`) {
		t.Fatal("logout control must be evidence")
	}
	if !HasLoginEvidence(`<button>退出登录</button>`) {
		t.Fatal("logout button label must be evidence")
	}
	if HasLoginEvidence(`<input type="password">`) {
		t.Fatal("password form is not login evidence")
	}
}

func TestHasSigninEvidence(t *testing.T) {
	cases := []struct {
		html string
		want bool
	}{
		{`<input type="submit" value="签到">`, false},
		{`<input type="submit" value="签到成功">`, true},
		{`<span>您今天尚未签到</span><input type="submit" value="签到">`, false},
		{`<span class="tips">今日签到获得5点魔力值</span>`, true},
		{`<div>你已经签到过了，请明天再来</div>`, false}, // 无“今天/今日”提示词不算当天签到凭据（与 autosignin 一致）
		{`<button>打卡</button>`, false},
		{`<button>已签到</button>`, false}, // 仅按钮文案不足以证明完成
		{`<script>签到成功</script>`, false},
	}
	for _, c := range cases {
		if got := HasSigninEvidence(c.html); got != c.want {
			t.Errorf("HasSigninEvidence(%q) = %v, want %v", c.html, got, c.want)
		}
	}
}

func TestSignInResult(t *testing.T) {
	if !SignInResult(`<html><body><div>您已连续签到 10 天</div></body></html>`, []string{"您已连续签到"}) {
		t.Fatal("text regex must match")
	}
	if SignInResult(`<script>您已连续签到</script><div>请登录</div>`, []string{"您已连续签到"}) {
		t.Fatal("script content must not match")
	}
	if !SignInResult(`<a class="label label-default" href="#">已签到</a>`, []string{`<a class="label label-default" href="#">已签到</a>`}) {
		t.Fatal("markup regex must match raw visible html")
	}
	if SignInResult(`<div>签到失败</div>`, []string{"签到成功"}) {
		t.Fatal("negative phrase must reject")
	}
}

func TestUnderChallenge(t *testing.T) {
	if !UnderChallenge(`<title>Just a moment...</title><script src="/cdn-cgi/challenge-platform/scripts/js/main.js">`) {
		t.Fatal("cloudflare challenge must be detected")
	}
	if UnderChallenge(`<html><body>normal</body></html>`) {
		t.Fatal("normal page must not be challenge")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
