package host

import (
	"dian115-autosignin/pluginjson"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// Caller 封装对宿主的一次 JSON-RPC host.* 调用：params 以 map 形式给出，
// 返回 result 的原始 JSON（未解码）。实现位于 runtime 包（wasmimport）。
type Caller interface {
	Call(method string, params map[string]any) (pluginjson.Raw, error)
}

// Request 为一次宿主 Broker 网络请求。
type Request struct {
	Method        string
	Path          string
	Headers       map[string]string
	BodyBase64    string
	CredentialRef string
	// Session 对应 http-sessions-v1 的 cookie_session：受管 Cookie 会话名，
	// 空表示不指定；"none" 关闭受管 Cookie；其他名称隔离账号会话。
	Session string
	// TimeoutMS 对应 timeout_ms（1-300000）；0 使用宿主默认。
	TimeoutMS int
	// CacheMode 对应 cache_mode：default / no-store / reload。
	CacheMode string
	// CacheTTLMS 对应 cache_ttl_ms（0-86400000）；0 不额外指定。
	CacheTTLMS int
}

// Response 为宿主 Broker 返回。
type Response struct {
	Status     int
	Headers    map[string][]string
	BodyBase64 string
}

func (r Response) Header(name string) string {
	for k, v := range r.Headers {
		if strings.EqualFold(k, name) && len(v) > 0 {
			return v[0]
		}
	}
	return ""
}

func (r Response) BodyBytes() ([]byte, error) {
	b, err := base64.RawStdEncoding.DecodeString(r.BodyBase64)
	if err != nil {
		b, err = base64.StdEncoding.DecodeString(r.BodyBase64)
	}
	if err != nil {
		return nil, errors.New("invalid broker body encoding")
	}
	return b, nil
}

type Client struct{ Caller Caller }

func EncodeBody(value []byte) string { return base64.RawStdEncoding.EncodeToString(value) }

func (c Client) Do(req Request) (Response, error) {
	params := map[string]any{"method": req.Method, "path": req.Path}
	if len(req.Headers) > 0 {
		params["headers"] = req.Headers
	}
	if req.BodyBase64 != "" {
		params["body_base64"] = req.BodyBase64
	}
	if req.CredentialRef != "" {
		params["credential_ref"] = req.CredentialRef
	}
	if req.Session != "" {
		params["cookie_session"] = req.Session
	}
	if req.TimeoutMS > 0 {
		params["timeout_ms"] = req.TimeoutMS
	}
	if req.CacheMode != "" {
		params["cache_mode"] = req.CacheMode
	}
	if req.CacheTTLMS > 0 {
		params["cache_ttl_ms"] = req.CacheTTLMS
	}
	result, err := c.Caller.Call("host.call", params)
	if err != nil {
		return Response{}, err
	}
	res := Response{
		Status:     pluginjson.Int(pluginjson.Field(result, "status")),
		BodyBase64: pluginjson.Text(pluginjson.Field(result, "body_base64")),
		Headers:    map[string][]string{},
	}
	headers := pluginjson.Field(result, "headers")
	for key, raw := range pluginjson.Fields(headers) {
		var values []string
		for position := 0; ; position++ {
			entry := pluginjson.Index(raw, position)
			if entry == nil {
				break
			}
			values = append(values, pluginjson.Text(entry))
		}
		res.Headers[key] = values
	}
	if res.Header("x-dian115-body-truncated") == "true" {
		return res, errors.New("broker response truncated")
	}
	return res, nil
}

var ErrConflict = errors.New("configuration changed; refresh before saving")

// Read 读取 Host Storage 中 key 的原始 JSON 文档值。
// 返回 (valueRaw, etag, found, err)。
func (c Client) Read(key string) (pluginjson.Raw, string, bool, error) {
	res, err := c.Do(Request{Method: "GET", Path: "/api/plugin-runtime/storage/" + key})
	if err != nil {
		return nil, "", false, err
	}
	if res.Status == 404 {
		return nil, "", false, nil
	}
	if res.Status != 200 {
		return nil, "", false, fmt.Errorf("storage read HTTP %d", res.Status)
	}
	body, err := res.BodyBytes()
	if err != nil {
		return nil, "", false, err
	}
	value := pluginjson.Field(pluginjson.Field(body, "data"), "value")
	if value == nil {
		return nil, "", false, errors.New("storage value missing")
	}
	etag := res.Header("etag")
	if etag == "" {
		return nil, "", false, errors.New("storage ETag missing")
	}
	return value, etag, true, nil
}

// Write 将已编码的 JSON 文档写入 Host Storage，支持 ETag 乐观锁。
func (c Client) Write(key string, value pluginjson.Raw, etag, idempotency string) error {
	body, err := pluginjson.Encode(map[string]any{"value": pluginjson.Raw(value)})
	if err != nil {
		return err
	}
	if len(body) > 64<<10 {
		return errors.New("storage document exceeds 64 KiB")
	}
	headers := map[string]string{"content-type": "application/json", "idempotency-key": idempotency}
	if etag != "" {
		headers["if-match"] = etag
	}
	res, err := c.Do(Request{Method: "PUT", Path: "/api/plugin-runtime/storage/" + key, Headers: headers, BodyBase64: base64.RawStdEncoding.EncodeToString(body)})
	if err != nil {
		return err
	}
	if res.Status == 409 || res.Status == 412 {
		return ErrConflict
	}
	if res.Status < 200 || res.Status >= 300 {
		return fmt.Errorf("storage write HTTP %d", res.Status)
	}
	return nil
}

// ---------------------------------------------------------------------------
// 宿主能力探测（http-sessions-v1 / http-sites-v1 / credential-fields-v1）
// ---------------------------------------------------------------------------

// Capabilities 探测宿主能力。返回 features 的 JSON（如
// {"http_sessions_v1":true,"http_sites_v1":true,"credential_fields_v1":true}）。
func (c Client) Capabilities() (pluginjson.Raw, error) {
	if c.Caller == nil {
		return nil, errors.New("host call unavailable")
	}
	raw, err := c.Caller.Call("host.capabilities", map[string]any{})
	if err != nil {
		return nil, err
	}
	features := pluginjson.Field(raw, "features")
	if features == nil {
		return nil, errors.New("capabilities features missing")
	}
	return features, nil
}

// Feature reports whether a named capability feature is true.
func Feature(features pluginjson.Raw, name string) bool {
	return pluginjson.Bool(pluginjson.Field(features, name))
}

// ---------------------------------------------------------------------------
// http-sessions-v1 会话管理 API（均经 host.call 授权调用）
// ---------------------------------------------------------------------------

// validSessionName 校验受管会话名：1-64 ASCII，字母/数字开头，后续允许 . _ -；
// 保留名 "none" 仅用于外部请求关闭受管 Cookie，不能作为会话接口的 session。
func validSessionName(value string) bool {
	if value == "" || value == "none" || len(value) > 64 {
		return false
	}
	for i, c := range value {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' {
			continue
		}
		if i == 0 || c != '.' && c != '_' && c != '-' {
			return false
		}
	}
	return true
}

// validIdempotencyKey 校验幂等键格式（16-128 个可打印 ASCII 字符）。
func validIdempotencyKey(key string) bool {
	if len(key) < 16 || len(key) > 128 {
		return false
	}
	for i := 0; i < len(key); i++ {
		c := key[i]
		if c < 0x21 || c > 0x7e {
			return false
		}
	}
	return true
}

func escapeQuery(value string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		v := value[i]
		if v >= 'a' && v <= 'z' || v >= 'A' && v <= 'Z' || v >= '0' && v <= '9' || v == '-' || v == '_' || v == '.' || v == '~' {
			b.WriteByte(v)
		} else {
			b.WriteByte('%')
			b.WriteByte(hex[v>>4])
			b.WriteByte(hex[v&15])
		}
	}
	return b.String()
}

// sessionRequest 调 host.call 访问 /api/plugin-runtime/http/* 会话接口。
// 返回成功响应的 data 字段（原始 JSON）。
func (c Client) sessionRequest(method, path string, input any, idempotency string) (pluginjson.Raw, error) {
	if c.Caller == nil {
		return nil, errors.New("HTTP session client is unavailable")
	}
	headers := map[string]string{"accept": "application/json"}
	if method != "GET" {
		if !validIdempotencyKey(idempotency) {
			return nil, errors.New("invalid idempotency key")
		}
		headers["idempotency-key"] = idempotency
	}
	params := map[string]any{"method": method, "path": path, "headers": headers}
	if input != nil {
		body, err := pluginjson.Encode(input)
		if err != nil {
			return nil, err
		}
		headers["content-type"] = "application/json"
		params["body_base64"] = base64.RawStdEncoding.EncodeToString(body)
	}
	result, err := c.Caller.Call("host.call", params)
	if err != nil {
		return nil, err
	}
	if pluginjson.Int(pluginjson.Field(result, "status")) != 200 {
		return nil, fmt.Errorf("HTTP session %s %s failed with HTTP %d", method, path, pluginjson.Int(pluginjson.Field(result, "status")))
	}
	data := pluginjson.Field(result, "data")
	if data == nil {
		return nil, errors.New("HTTP session response data missing")
	}
	return data, nil
}

// Sites 读取当前插件保存的站点地址列表。
func (c Client) Sites() (pluginjson.Raw, error) {
	return c.sessionRequest("GET", "/api/plugin-runtime/http/sites", nil, "")
}

// SaveSite 安装后添加或修改一个站点地址。
func (c Client) SaveSite(id, url, label, idempotency string) (pluginjson.Raw, error) {
	return c.sessionRequest("PUT", "/api/plugin-runtime/http/sites", map[string]any{"id": id, "url": url, "label": label}, idempotency)
}

// DeleteSite 删除一个站点地址（空 id 清空全部地址配置）。
func (c Client) DeleteSite(id, idempotency string) (pluginjson.Raw, error) {
	path := "/api/plugin-runtime/http/sites"
	if id != "" {
		path += "?id=" + escapeQuery(id)
	}
	return c.sessionRequest("DELETE", path, nil, idempotency)
}

// SiteCookies 读取当前插件会话中匹配 targetURL 的 Cookie（含 HttpOnly）。
// 仅在用户明确查看自己凭据时展示，不自动复制进整页 state 或日志。
func (c Client) SiteCookies(targetURL, session string) (pluginjson.Raw, error) {
	if !validSessionName(session) {
		return nil, errors.New("invalid cookie session")
	}
	return c.sessionRequest("GET", "/api/plugin-runtime/http/cookies?url="+escapeQuery(targetURL)+"&session="+escapeQuery(session), nil, "")
}

// SetCookieHeader 把用户手动填写的 Cookie header 持久化到指定账号会话。
func (c Client) SetCookieHeader(targetURL, session, cookie, idempotency string) (pluginjson.Raw, error) {
	if !validSessionName(session) {
		return nil, errors.New("invalid cookie session")
	}
	if strings.TrimSpace(cookie) == "" || len(cookie) > 8192 {
		return nil, errors.New("invalid Cookie header")
	}
	return c.sessionRequest("PUT", "/api/plugin-runtime/http/cookies", map[string]any{"url": targetURL, "session": session, "cookie": cookie}, idempotency)
}

// ClearCookies 清空指定会话所有域名的 Cookie 与 User-Agent。
func (c Client) ClearCookies(session, idempotency string) (pluginjson.Raw, error) {
	if !validSessionName(session) {
		return nil, errors.New("invalid cookie session")
	}
	return c.sessionRequest("DELETE", "/api/plugin-runtime/http/cookies?session="+escapeQuery(session), nil, idempotency)
}

// CookieCloud 读取本地 CookieCloud 概况（域名、数量、revision，不含全局密码）。
func (c Client) CookieCloud() (pluginjson.Raw, error) {
	return c.sessionRequest("GET", "/api/plugin-runtime/http/cookiecloud", nil, "")
}

// ImportCookieCloud 从本地 CookieCloud 按域名导入 Cookie 到指定会话。
func (c Client) ImportCookieCloud(session string, domains []string, idempotency string) (pluginjson.Raw, error) {
	if !validSessionName(session) || len(domains) == 0 || len(domains) > 16 {
		return nil, errors.New("invalid CookieCloud import")
	}
	return c.sessionRequest("POST", "/api/plugin-runtime/http/cookiecloud/import", map[string]any{"session": session, "domains": domains}, idempotency)
}
