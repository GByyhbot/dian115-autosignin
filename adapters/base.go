package adapters

import (
	"context"
	"dian115-autosignin/core"
	"fmt"
	"strings"
)

// BaseSignin 通用签到：访问 attendance.php（或站点配置地址），确认完成状态。
func BaseSignin(ctx context.Context, s core.Site, c *Client) Result {
	if strings.TrimSpace(s.URL) == "" || (strings.TrimSpace(s.Cookie) == "" && strings.TrimSpace(s.CredentialRef) == "") {
		return Fail("签到失败，未配置站点地址或Cookie")
	}
	timeout := s.Timeout
	checkinURL := s.URL
	if !strings.Contains(checkinURL, "attendance.php") {
		checkinURL = Resolve(s.URL, "attendance.php")
	}
	status, body, err := c.Get(ctx, checkinURL, s, timeout, nil)
	if err != nil {
		return Fail(fmt.Sprintf("签到失败：%s！", err.Error()))
	}
	page := string(body)
	if len(page) == 0 {
		return Fail("签到失败，站点返回空页面")
	}
	if core.UnderChallenge(page) {
		return Fail("签到失败，无法通过Cloudflare！")
	}
	if status != 200 {
		return Fail(fmt.Sprintf("签到失败，状态码：%d！", status))
	}
	if !core.HasLoginEvidence(page) {
		return Fail("签到失败，未识别到登录状态")
	}
	if core.HasSigninEvidence(page) {
		return SignOK("签到成功，已确认站点返回的签到状态")
	}
	return LoginOK("登录成功，签到未确认")
}

// BaseLogin 通用模拟登录：访问站点首页，确认登录状态。
func BaseLogin(ctx context.Context, s core.Site, c *Client) Result {
	if strings.TrimSpace(s.URL) == "" || (strings.TrimSpace(s.Cookie) == "" && strings.TrimSpace(s.CredentialRef) == "") {
		return Fail("模拟登录失败，未配置站点地址或Cookie")
	}
	pageURL := s.URL
	if strings.Contains(pageURL, "attendance.php") {
		pageURL = Resolve(s.URL, "./")
	}
	status, body, err := c.Get(ctx, pageURL, s, s.Timeout, nil)
	if err != nil {
		return Fail(fmt.Sprintf("模拟登录失败：%s！", err.Error()))
	}
	page := string(body)
	if len(page) == 0 {
		return Fail("模拟登录失败，站点返回空页面")
	}
	if core.UnderChallenge(page) {
		return Fail("模拟登录失败，无法通过Cloudflare！")
	}
	if status != 200 {
		return Fail(fmt.Sprintf("模拟登录失败，状态码：%d！", status))
	}
	if !core.HasLoginEvidence(page) {
		if core.HasLoginForm(page) {
			return Fail("模拟登录失败，Cookie已失效")
		}
		return Fail("模拟登录失败，未识别到登录状态")
	}
	return Result{Success: true, LoggedIn: true, Message: "模拟登录成功"}
}
