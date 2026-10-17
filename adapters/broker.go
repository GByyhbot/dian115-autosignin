// Package adapters 实现站点请求客户端与全部内置签到处理器。
package adapters

import (
	"bytes"
	"context"
	"dian115-autosignin/core"
	"dian115-autosignin/host"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/url"
	"strings"
	"time"
)

// Broker 封装宿主 host.call 网络通道。
type Broker interface {
	Do(host.Request) (host.Response, error)
}

// Store 供处理器读写少量持久化缓存（如 tjupt 答案缓存）。
type Store interface {
	Read(key string, out any) (string, bool, error)
	Write(key string, value any, etag, idempotency string) error
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

// request 构造并发送请求：凭据优先使用托管凭据引用，否则直传 Cookie。
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
	req := host.Request{Method: method, Path: rawURL, Headers: h}
	if s.CredentialRef != "" {
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
func (c *Client) PostJSON(ctx context.Context, rawURL string, s core.Site, timeout int, payload any, headers map[string]string) (int, []byte, error) {
	b, err := json.Marshal(payload)
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

// ParseJSON 解析 JSON 正文为通用对象。
func ParseJSON(body []byte) (map[string]any, error) {
	var v map[string]any
	dec := json.NewDecoder(bytes.NewReader(body))
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
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

// JSONInt 安全读取 map 中的数字值（支持 float64 / json.Number）。
func JSONInt(v map[string]any, keys ...string) int {
	for _, key := range keys {
		switch n := v[key].(type) {
		case float64:
			return int(n)
		case int:
			return n
		case json.Number:
			if i, err := n.Int64(); err == nil {
				return int(i)
			}
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

// ReadCloser 帮助函数，供需要流式读取正文的实现使用。
func readAll(r io.Reader) ([]byte, error) { return io.ReadAll(r) }

// randInt 返回 [0,n) 的随机整数，供随机选择答案使用。
func randInt(n int) int {
	if n <= 0 {
		return 0
	}
	return rand.IntN(n)
}
