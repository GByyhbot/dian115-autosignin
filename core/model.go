// Package core 定义插件配置、站点与调度模型，适配自 MoviePilot autosignin 3.0.3。
package core

import (
	"dian115-autosignin/pluginjson"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// SchemaVersion 为配置结构版本。
const SchemaVersion = 1

// AllSites 全选标记，执行时展开为配置中的全部站点。
const AllSites = "all"

// HistoryDays 保留含当天的最近天数。
const HistoryDays = 14

// Site 为一个待签到/登录的站点实例。
// 凭据三选一（优先会话）：Cookie 直填进受管会话（Session）、宿主“托管凭据”
// 界面创建的绑定引用（CredentialRef）、或直接填入配置（Cookie）。三者都为空
// 时任务会报告未配置凭据。
type Site struct {
	ID            string `json:"id"`
	Key           string `json:"key"` // 内置处理器预设键，"" 表示通用 NexusPHP attendance
	Name          string `json:"name"`
	URL           string `json:"url"`
	Cookie        string `json:"cookie"`
	CredentialRef string `json:"credential_ref"`
	Session       string `json:"session"` // 受管 Cookie 会话名（http-sessions-v1）；空则按站点自动生成
	UA            string `json:"ua"`
	Timeout       int    `json:"timeout"` // 秒，0 使用默认值
	Apikey        string `json:"apikey"`  // Rousi Pro 个人 API Key
	Token         string `json:"token"`   // mTeam 浏览器登录态 / Rousi 兼容 Authorization
	Custom        bool   `json:"custom"`  // 是否为自定义站点
}

// Config 为插件全局配置。
type Config struct {
	SchemaVersion int    `json:"schema_version"`
	Enabled       bool   `json:"enabled"`
	Cron          string `json:"cron"` // "0 8 * * *" | "2.3/9-23" | "2.3" | ""（随机）
	Notify        bool   `json:"notify"`
	QueueCnt      int    `json:"queue_cnt"`
	SignSites     []string `json:"sign_sites"`  // 站点 ID 或 "all"
	LoginSites    []string `json:"login_sites"` // 站点 ID 或 "all"
	RetryKeyword  string   `json:"retry_keyword"`
	Sites         []Site   `json:"sites"`
}

// DefaultConfig 返回空配置，保证可序列化且字段齐全。
func DefaultConfig() Config {
	return Config{
		SchemaVersion: SchemaVersion,
		QueueCnt:      5,
		SignSites:     []string{},
		LoginSites:    []string{},
		Sites:         []Site{},
	}
}

// Validate 校验配置：站点 ID 唯一、重试关键词必须是合法正则、站点数量受限。
func (c Config) Validate() error {
	if c.SchemaVersion != SchemaVersion {
		return errors.New("unsupported config schema_version")
	}
	if c.QueueCnt <= 0 {
		return errors.New("queue_cnt 必须大于 0")
	}
	seen := map[string]bool{}
	for i := range c.Sites {
		s := &c.Sites[i]
		if strings.TrimSpace(s.ID) == "" {
			return fmt.Errorf("站点 %d 缺少 ID", i)
		}
		if seen[s.ID] {
			return fmt.Errorf("站点 ID 重复：%s", s.ID)
		}
		seen[s.ID] = true
		if s.Timeout < 0 || s.Timeout > 300 {
			return errors.New("站点超时必须在 0-300 秒之间")
		}
	}
	if len(c.Sites) > 100 {
		return errors.New("站点数量超过 100 个上限")
	}
	if c.RetryKeyword != "" {
		if _, err := regexp.Compile(c.RetryKeyword); err != nil {
			return fmt.Errorf("重试关键词不是有效正则：%w", err)
		}
	}
	if err := ValidateCron(c.Cron); err != nil {
		return err
	}
	return nil
}

// SiteByID 返回站点副本与是否存在标记。
func (c Config) SiteByID(id string) (Site, bool) {
	for _, s := range c.Sites {
		if s.ID == id {
			return s, true
		}
	}
	return Site{}, false
}

// ResolveSiteIDs 展开全选标记并过滤不存在的站点。
func (c Config) ResolveSiteIDs(selected []string) []string {
	ids := []string{}
	for _, id := range selected {
		if id == AllSites {
			for _, s := range c.Sites {
				ids = append(ids, s.ID)
			}
			continue
		}
		if _, ok := c.SiteByID(id); ok {
			ids = append(ids, id)
		}
	}
	// 去重并保持顺序
	seen := map[string]bool{}
	out := ids[:0]
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// SessionName 返回站点的受管 Cookie 会话名；未配置时按站点 ID 自动生成。
// 会话名规则与 http-sessions-v1 一致：字母/数字开头，可含 . _ -。
func (s Site) SessionName() string {
	if s.Session != "" {
		return s.Session
	}
	name := s.ID
	if name == "" {
		name = "default"
	}
	for i, c := range name {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' {
			continue
		}
		if i == 0 || c != '.' && c != '_' && c != '-' {
			name = name[:i] + "x" + name[i+1:]
		}
	}
	if len(name) > 64 {
		name = name[:64]
	}
	return name
}

// siteToMap 将站点转为与 UI/存储一致的 JSON 对象（字段名对应原 struct tag）。
func siteToMap(s Site) map[string]any {
	return map[string]any{
		"id": s.ID, "key": s.Key, "name": s.Name, "url": s.URL,
		"cookie": s.Cookie, "credential_ref": s.CredentialRef, "session": s.Session,
		"ua": s.UA, "timeout": s.Timeout, "apikey": s.Apikey, "token": s.Token,
		"custom": s.Custom,
	}
}

// ConfigToMap 将配置转为与 UI/存储一致的 JSON 对象。
func ConfigToMap(c Config) map[string]any {
	sites := make([]any, 0, len(c.Sites))
	for _, s := range c.Sites {
		sites = append(sites, siteToMap(s))
	}
	return map[string]any{
		"schema_version": c.SchemaVersion, "enabled": c.Enabled, "cron": c.Cron,
		"notify": c.Notify, "queue_cnt": c.QueueCnt, "sign_sites": stringSliceToAny(c.SignSites),
		"login_sites": stringSliceToAny(c.LoginSites), "retry_keyword": c.RetryKeyword,
		"sites": sites,
	}
}

// EncodeConfig 将配置编码为 JSON 文档。
func EncodeConfig(c Config) (pluginjson.Raw, error) {
	b, err := pluginjson.Encode(ConfigToMap(c))
	if err != nil {
		return nil, err
	}
	return pluginjson.Raw(b), nil
}

// DecodeConfig 从 JSON 文档解码配置；缺失字段回退默认值。
func DecodeConfig(raw pluginjson.Raw) (Config, error) {
	c := DefaultConfig()
	if raw == nil {
		return c, nil
	}
	if pluginjson.Field(raw, "schema_version") != nil {
		c.SchemaVersion = pluginjson.Int(pluginjson.Field(raw, "schema_version"))
	}
	c.Enabled = pluginjson.Bool(pluginjson.Field(raw, "enabled"))
	c.Cron = pluginjson.Text(pluginjson.Field(raw, "cron"))
	c.Notify = pluginjson.Bool(pluginjson.Field(raw, "notify"))
	if q := pluginjson.Int(pluginjson.Field(raw, "queue_cnt")); q > 0 {
		c.QueueCnt = q
	}
	c.SignSites = StringList(pluginjson.Field(raw, "sign_sites"))
	c.LoginSites = StringList(pluginjson.Field(raw, "login_sites"))
	c.RetryKeyword = pluginjson.Text(pluginjson.Field(raw, "retry_keyword"))
	sites := pluginjson.Field(raw, "sites")
	for position := 0; ; position++ {
		entry := pluginjson.Index(sites, position)
		if entry == nil {
			break
		}
		c.Sites = append(c.Sites, Site{
			ID:            pluginjson.Text(pluginjson.Field(entry, "id")),
			Key:           pluginjson.Text(pluginjson.Field(entry, "key")),
			Name:          pluginjson.Text(pluginjson.Field(entry, "name")),
			URL:           pluginjson.Text(pluginjson.Field(entry, "url")),
			Cookie:        pluginjson.Text(pluginjson.Field(entry, "cookie")),
			CredentialRef: pluginjson.Text(pluginjson.Field(entry, "credential_ref")),
			Session:       pluginjson.Text(pluginjson.Field(entry, "session")),
			UA:            pluginjson.Text(pluginjson.Field(entry, "ua")),
			Timeout:       pluginjson.Int(pluginjson.Field(entry, "timeout")),
			Apikey:        pluginjson.Text(pluginjson.Field(entry, "apikey")),
			Token:         pluginjson.Text(pluginjson.Field(entry, "token")),
			Custom:        pluginjson.Bool(pluginjson.Field(entry, "custom")),
		})
	}
	return c, nil
}

func stringSliceToAny(in []string) []any {
	out := make([]any, 0, len(in))
	for _, s := range in {
		out = append(out, s)
	}
	return out
}

// StringList 从 JSON 数组中读取字符串列表。
func StringList(raw pluginjson.Raw) []string {
	out := []string{}
	for position := 0; ; position++ {
		entry := pluginjson.Index(raw, position)
		if entry == nil {
			break
		}
		if text := pluginjson.Text(entry); text != "" {
			out = append(out, text)
		}
	}
	return out
}
