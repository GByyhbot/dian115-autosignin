// Package adapters 实现站点请求客户端与全部内置签到处理器。
package adapters

import (
	"context"
	"dian115-autosignin/core"
	"dian115-autosignin/host"
	"dian115-autosignin/pluginjson"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Broker 封装宿主 host.call 网络通道。
type Broker interface {
	Do(host.Request) (host.Response, error)
}

// Store 供处理器读写少量持久化缓存（如 tjupt 答案缓存）。
type Store interface {
	Read(key string) (pluginjson.Raw, string, bool, error)
	Write(key string, value pluginjson.Raw, etag, idempotency string) error
}

// CaptchaReader 读取验证码图片并返回识别文本；DIAN115 宿主不提供 OCR，
// 默认为空实现，相关站点会报告“未获取到验证码”。可接入外部 OCR 服务。
type CaptchaReader interface {
	Read(ctx context.Context, imgURL string, s core.Site, c *Client) (string, error)
}

// DefaultCaptchaReader 可被宿主侧替换（当前为空实现）。
var DefaultCaptchaReader CaptchaReader

// Client 通过宿主 Broker 发起站点 HTTP 请求。
type Client struct {
	Broker Broker
	Store  Store
	// HostClient 提供 http-sessions-v1 会话管理（CookieCloud 导入、直填 Cookie、
	// 自定义站点）。为 nil 时会话相关方法返回明确错误，不影响普通请求。
	HostClient *host.Client
}

func (c *Client) do(ctx context.Context, req host.Request) (*host.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	res, err := c.Broker.Do(req)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// request 构造并发送请求。凭据优先级（http-sessions-v1）：
// 受管 Cookie 会话（Session） > 托管凭据引用（CredentialRef） > 直传 Cookie。
func (c *Client) request(ctx context.Context, method, rawURL string, s core.Site, timeout int, headers map[string]string, body []byte, contentType string) (*host.Response, error) {
	if timeout <= 0 {
		timeout = 20
	}
	if timeout > 120 {
		timeout = 120
	}
	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()
	h := map[string]string{}
	for k, v := range headers {
		h[k] = v
	}
	if _, ok := h["user-agent"]; !ok && s.UA != "" {
		h["user-agent"] = s.UA
	}
	req := host.Request{
		Method:    method,
		Path:      rawURL,
		Headers:   h,
		TimeoutMS: timeout * 1000,
	}
	if s.SessionName() != "" && s.CredentialRef == "" {
		req.Session = s.SessionName()
	} else if s.CredentialRef != "" {
		req.CredentialRef = s.CredentialRef
	} else if s.Cookie != "" {
		h["cookie"] = s.Cookie
	}
	if contentType != "" {
		h["content-type"] = contentType
	}
	if len(body) > 0 {
		req.BodyBase64 = host.EncodeBody(body)
	}
	return c.do(reqCtx, req)
}

// Get 发起 GET 请求。
func (c *Client) Get(ctx context.Context, rawURL string, s core.Site, timeout int, headers map[string]string) (int, []byte, error) {
	res, err := c.request(ctx, "GET", rawURL, s, timeout, headers, nil, "")
	if err != nil {
		return 0, nil, err
	}
	body, err := res.BodyBytes()
	if err != nil {
		return res.Status, nil, err
	}
	return res.Status, body, nil
}

// PostForm 发起表单 POST。
func (c *Client) PostForm(ctx context.Context, rawURL string, s core.Site, timeout int, data url.Values, headers map[string]string) (int, []byte, error) {
	body := []byte(data.Encode())
	res, err := c.request(ctx, "POST", rawURL, s, timeout, headers, body, "application/x-www-form-urlencoded")
	if err != nil {
		return 0, nil, err
	}
	b, err := res.BodyBytes()
	if err != nil {
		return res.Status, nil, err
	}
	return res.Status, b, nil
}

// PostJSON 发起 JSON POST。
func (c *Client) PostJSON(ctx context.Context, rawURL string, s core.Site, timeout int, payload map[string]any, headers map[string]string) (int, []byte, error) {
	b, err := pluginjson.Encode(payload)
	if err != nil {
		return 0, nil, err
	}
	res, err := c.request(ctx, "POST", rawURL, s, timeout, headers, b, "application/json")
	if err != nil {
		return 0, nil, err
	}
	body, err := res.BodyBytes()
	if err != nil {
		return res.Status, nil, err
	}
	return res.Status, body, nil
}

// Resolve 将相对路径解析为绝对地址（等价 Python urljoin）。
func Resolve(base, ref string) string {
	u, err := url.Parse(strings.TrimSpace(base))
	if err != nil {
		return ref
	}
	r, err := url.Parse(strings.TrimSpace(ref))
	if err != nil {
		return ref
	}
	return u.ResolveReference(r).String()
}

// ParseJSON 用 pluginjson 解析 JSON 正文为通用对象（不链接 encoding/json）。
func ParseJSON(body []byte) (map[string]any, error) {
	return convertObject(pluginjson.Raw(body))
}

func convertObject(raw pluginjson.Raw) (map[string]any, error) {
	fields := pluginjson.Fields(raw)
	if fields == nil {
		return nil, errors.New("json: not an object")
	}
	out := make(map[string]any, len(fields))
	for key, value := range fields {
		converted, err := convertValue(value)
		if err != nil {
			return nil, err
		}
		out[key] = converted
	}
	return out, nil
}

func convertArray(raw pluginjson.Raw) ([]any, error) {
	out := []any{}
	for position := 0; ; position++ {
		entry := pluginjson.Index(raw, position)
		if entry == nil {
			break
		}
		converted, err := convertValue(entry)
		if err != nil {
			return nil, err
		}
		out = append(out, converted)
	}
	return out, nil
}

func convertValue(raw pluginjson.Raw) (any, error) {
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return nil, errors.New("json: empty value")
	}
	switch text[0] {
	case '{':
		return convertObject(raw)
	case '[':
		return convertArray(raw)
	case '"':
		return pluginjson.Text(raw), nil
	case 't', 'f':
		return pluginjson.Bool(raw), nil
	case 'n':
		return nil, nil
	default:
		if number, err := strconv.Atoi(text); err == nil {
			return number, nil
		}
		if number, err := strconv.ParseFloat(text, 64); err == nil {
			return number, nil
		}
		return nil, errors.New("json: invalid number " + text)
	}
}

// JSONString 安全读取 map 中的字符串值。
func JSONString(v map[string]any, keys ...string) string {
	for _, key := range keys {
		if s, ok := v[key].(string); ok {
			return s
		}
	}
	return ""
}

// JSONInt 安全读取 map 中的数字值（支持 float64 / int / 数字字符串）。
func JSONInt(v map[string]any, keys ...string) int {
	for _, key := range keys {
		switch n := v[key].(type) {
		case float64:
			return int(n)
		case int:
			return n
		case string:
			var i int
			if _, err := fmt.Sscanf(n, "%d", &i); err == nil {
				return i
			}
		}
	}
	return -1
}

// JSONBool 安全读取布尔值。
func JSONBool(v map[string]any, keys ...string) bool {
	for _, key := range keys {
		if b, ok := v[key].(bool); ok {
			return b
		}
		if s, ok := v[key].(string); ok {
			return strings.EqualFold(s, "true")
		}
	}
	return false
}

// Result 为一次站点动作的结果，语义与 SiteResult 一致。
type Result struct {
	Success  bool
	LoggedIn bool
	Message  string
}

// Completed 已签到或确认登录均完成本轮访问任务。
func (r Result) Completed() bool { return r.Success || r.LoggedIn }

// Fail 构造失败结果。
func Fail(msg string) Result { return Result{Message: msg} }

// SignOK 构造确认签到成功。
func SignOK(msg string) Result { return Result{Success: true, LoggedIn: true, Message: msg} }

// LoginOK 构造仅确认登录。
func LoginOK(msg string) Result { return Result{LoggedIn: true, Message: msg} }

// readAll 帮助函数，供需要流式读取正文的实现使用。
func readAll(r io.Reader) ([]byte, error) { return io.ReadAll(r) }

// randInt 返回 [0,n) 的随机整数，供随机选择答案使用。
func randInt(n int) int {
	if n <= 0 {
		return 0
	}
	return rand.Intn(n)
}

// ---------------------------------------------------------------------------
// http-sessions-v1 会话管理（直填 Cookie / CookieCloud 导入 / 自定义站点）
// ---------------------------------------------------------------------------

// hasSession 返回会话能力是否可用。
func (c *Client) hasSession() (*host.Client, error) {
	if c.HostClient == nil {
		return nil, errors.New("宿主未启用 http-sessions-v1 会话能力（需 DIAN115 >= 5.0.54），请升级宿主或改用托管凭据/直填 Cookie")
	}
	return c.HostClient, nil
}

// SetSiteCookie 把用户直填的 Cookie header 持久化到该站受管会话。
// 幂等键由调用方提供（重试复用同一键）。
func (c *Client) SetSiteCookie(s core.Site, cookie, idempotency string) (pluginjson.Raw, error) {
	hc, err := c.hasSession()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(s.URL) == "" {
		return nil, errors.New("站点缺少 URL，无法确定 Cookie 作用域")
	}
	return hc.SetCookieHeader(s.URL, s.SessionName(), cookie, idempotency)
}

// ImportCookieCloud 从本地 CookieCloud 按站点域名导入 Cookie 到该站受管会话。
func (c *Client) ImportCookieCloud(s core.Site, idempotency string) (pluginjson.Raw, error) {
	hc, err := c.hasSession()
	if err != nil {
		return nil, err
	}
	domain := siteDomain(s)
	if domain == "" {
		return nil, errors.New("无法从站点 URL 提取 CookieCloud 域名")
	}
	return hc.ImportCookieCloud(s.SessionName(), []string{domain}, idempotency)
}

// CookieCloudOverview 读取本地 CookieCloud 概况（域名、数量、revision）。
func (c *Client) CookieCloudOverview() (pluginjson.Raw, error) {
	hc, err := c.hasSession()
	if err != nil {
		return nil, err
	}
	return hc.CookieCloud()
}

// SiteCookieCount 读取该站会话已保存的 Cookie 数量；会话不存在时返回 0。
func (c *Client) SiteCookieCount(s core.Site) (int, error) {
	hc, err := c.hasSession()
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(s.URL) == "" {
		return 0, nil
	}
	raw, err := hc.SiteCookies(s.URL, s.SessionName())
	if err != nil {
		return 0, err
	}
	return pluginjson.Int(pluginjson.Field(raw, "total_cookies")), nil
}

// siteDomain 提取站点 URL 的注册主域名（host 去 www.），供 CookieCloud 选择。
func siteDomain(s core.Site) string {
	host := core.HostOf(s.URL)
	if host == "" {
		return ""
	}
	return host
}

// SaveCustomSite 把自定义站点地址登记到宿主 sites 接口（纳入 CookieCloud 范围）。
func (c *Client) SaveCustomSite(id, url, label, idempotency string) (pluginjson.Raw, error) {
	hc, err := c.hasSession()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(id) == "" || strings.TrimSpace(url) == "" {
		return nil, errors.New("自定义站点需要 id 与 url")
	}
	return hc.SaveSite(id, url, label, idempotency)
}

// DeleteCustomSite 删除宿主 sites 接口中的自定义站点地址。
func (c *Client) DeleteCustomSite(id, idempotency string) (pluginjson.Raw, error) {
	hc, err := c.hasSession()
	if err != nil {
		return nil, err
	}
	return hc.DeleteSite(id, idempotency)
}

// ListSites 读取宿主当前保存的站点地址列表。
func (c *Client) ListSites() (pluginjson.Raw, error) {
	hc, err := c.hasSession()
	if err != nil {
		return nil, err
	}
	return hc.Sites()
}
