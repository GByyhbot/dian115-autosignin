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
