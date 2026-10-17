package core

import (
	"regexp"
	"strings"
)

// 签到结果与页面证据校验，禁止用登录状态或缺失按钮推断签到完成。
// 移植自 MoviePilot autosignin 3.0.3 result.py 与 SiteUtils.is_logged_in。

var (
	negEvidenceRe = regexp.MustCompile(`(?:签到|簽到)(?:失败|失敗|未成功)|(?:未|没有|沒有)(?:签到|簽到)成功`)
	loginWordRe   = regexp.MustCompile(`(?i)\b(?:logout|logoff|signout)\b`)
	pxRe          = regexp.MustCompile(`\d+px`)
	hashRefRe     = regexp.MustCompile(`#\d+`)

	// signin evidence 的正面/反面模式
	signinButtonLabels = map[string]bool{
		"签到": true, "簽到": true, "立即签到": true, "立即簽到": true,
		"每日签到": true, "打卡": true, "checkin": true, "checkinnow": true,
	}
	signinDoneLabels = map[string]bool{
		"签到成功": true, "簽到成功": true, "今日已签到": true, "今天已签到": true,
		"今日已簽到": true, "今天已簽到": true, "已经签到": true, "已經簽到": true,
		"已经打卡": true,
	}
	signinNotYetRe = regexp.MustCompile(`(?:您|你)?(?:今天|今日)?(?:尚未|还未|還未|未|没有|沒有)(?:签到|簽到)[！!。.]?`)
	// NexusPHP 今日签到结果 / 学校一次性到账提示 / U2 重复英文提示
	signinBonusRe = regexp.MustCompile(
		`(?:您|你)(?:今天|今日)(?:已(?:经)?签(?:到|过到)|已經簽到)(?:过|過|了)|` +
			`(?:本次|此次)(?:签到|簽到)(?:您)?(?:获得|獲得)(?:了)?\d|(?:签到|簽到)已得\d|` +
			`(?:今天|今日)(?:签到|簽到)(?:您|你)?(?:获得|獲得)(?:了)?\d+(?:点|點)魔力值|` +
			`You have already attend, no refresh please\.|` +
			`You have already attended \d+ days, Continuous \d+ days, this time you will get \d+ bonus\.`,
	)
)

// visibleHTML 移除脚本、注释和显式隐藏内容，并清理尺寸噪声。
func visibleHTML(source string) string {
	doc := ParseVisible(source)
	out := hashRefRe.ReplaceAllString(pxRe.ReplaceAllString(doc.Visible, ""), "")
	return out
}

// HasLoginForm 仅在可见页面确实要求输入密码时判定 Cookie 登录已失效。
func HasLoginForm(source string) bool {
	doc := ParseVisible(visibleHTML(source))
	for _, e := range doc.Elements {
		if e.Tag == "input" && strings.EqualFold(e.Attrs["type"], "password") {
			return true
		}
	}
	return false
}

// IsLoggedIn 移植 MoviePilot SiteUtils.is_logged_in：存在登出/用户面板链接判定已登录。
func IsLoggedIn(source string) bool {
	doc := ParseDoc(source)
	if doc.Empty {
		return false
	}
	for _, e := range doc.Elements {
		if e.Tag == "input" && strings.EqualFold(e.Attrs["type"], "password") {
			return false
		}
	}
	attrContains := func(tag, attr string, keywords ...string) bool {
		for _, e := range doc.Elements {
			if e.Tag != tag {
				continue
			}
			val := strings.ToLower(e.Attrs[attr])
			for _, kw := range keywords {
				if strings.Contains(val, kw) {
					return true
				}
			}
		}
		return false
	}
	if attrContains("a", "href", "logout", "mybonus", "usercp") ||
		attrContains("a", "data-url", "logout") ||
		attrContains("a", "onclick", "logout") ||
		attrContains("a", "lay-on", "logout") ||
		attrContains("form", "action", "logout") {
		return true
	}
	for _, e := range doc.Elements {
		if e.Tag == "div" && strings.Contains(strings.ToLower(e.Attrs["class"]), "user-info-side") {
			return true
		}
		if e.Tag == "a" && e.Attrs["id"] == "myitem" {
			return true
		}
	}
	return false
}

// HasLoginEvidence 识别可见的用户导航或退出控件，兼容新模板且排除隐藏模板和登录表单。
func HasLoginEvidence(source string) bool {
	doc := ParseVisible(visibleHTML(source))
	if doc.Empty {
		return false
	}
	for _, e := range doc.Elements {
		if e.Tag == "input" && strings.EqualFold(e.Attrs["type"], "password") {
			return false
		}
	}
	if IsLoggedIn(doc.Visible) {
		return true
	}
	for _, e := range doc.Elements {
		roleButton := strings.EqualFold(e.Attrs["role"], "button")
		switch {
		case e.Tag == "a", e.Tag == "button", e.Tag == "form", e.Tag == "shark-icon-button", roleButton:
			for _, attr := range []string{"href", "action", "formaction", "data-url", "onclick", "lay-on"} {
				if loginWordRe.MatchString(e.Attrs[attr]) {
					return true
				}
			}
			if e.Tag == "form" {
				continue
			}
			label := strings.ToLower(CompactText(e.Text))
			switch label {
			case "退出登录", "退出登錄", "登出", "logout", "signout":
				return true
			}
		}
	}
	return false
}

// HasSigninEvidence 只接受可见的完成状态或本次奖励，未完成表单和否定提示优先拒绝。
func HasSigninEvidence(source string) bool {
	doc := ParseVisible(visibleHTML(source))
	if doc.Empty {
		return false
	}
	for _, e := range doc.Elements {
		if e.Tag == "input" && strings.EqualFold(e.Attrs["type"], "password") {
			return false
		}
	}
	text := strings.TrimSpace(doc.Text)
	compact := CompactText(text)
	if negEvidenceRe.MatchString(compact) {
		return false
	}
	// 带验证码或提交按钮的签到表单仍等待用户操作，页面上的说明不构成成功证据。
	for _, e := range doc.Elements {
		if e.Tag == "input" && strings.EqualFold(e.Attrs["type"], "submit") {
			if signinButtonLabels[strings.ToLower(CompactText(e.Attrs["value"]))] {
				return false
			}
		}
		if e.Tag == "button" && signinButtonLabels[strings.ToLower(CompactText(e.Text))] {
			return false
		}
	}
	for _, e := range doc.Elements {
		if signinNotYetRe.MatchString(CompactText(e.Text)) {
			return false
		}
	}
	for _, e := range doc.Elements {
		if e.Tag == "input" {
			t := strings.ToLower(e.Attrs["type"])
			if t != "" && t != "button" && t != "submit" {
				continue
			}
			if signinDoneLabels[CompactText(e.Attrs["value"])] {
				return true
			}
		}
		if e.Tag == "button" && signinDoneLabels[CompactText(e.Text)] {
			return true
		}
	}
	return signinBonusRe.MatchString(compact) || signinBonusRe.MatchString(text)
}

// SignInResult 仅匹配可见页面中的签到结果，脚本和隐藏模板里的文案不代表已执行。
// regexs 中含 "<" 或 "value=" 时匹配可见 HTML，否则匹配纯文本。
func SignInResult(source string, regexs []string) bool {
	htmlText := visibleHTML(source)
	doc := ParseVisible(htmlText)
	text := strings.TrimSpace(doc.Text)
	if negEvidenceRe.MatchString(CompactText(text)) {
		return false
	}
	for _, pattern := range regexs {
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}
		target := text
		if strings.Contains(pattern, "<") || strings.Contains(pattern, "value=") {
			target = htmlText
		}
		if re.MatchString(target) {
			return true
		}
	}
	return false
}

// UnderChallenge 检测 Cloudflare 挑战页。
func UnderChallenge(source string) bool {
	lower := strings.ToLower(source)
	return strings.Contains(lower, "just a moment") ||
		strings.Contains(lower, "challenge-platform") ||
		strings.Contains(lower, "cf-chl-") ||
		strings.Contains(lower, "cf_chl_opt") ||
		strings.Contains(lower, "/cdn-cgi/challenge-platform/")
}

// HasLoginPhp 大多数站点以 login.php 出现在页面中判定 Cookie 失效。
func HasLoginPhp(source string) bool { return strings.Contains(source, "login.php") }
