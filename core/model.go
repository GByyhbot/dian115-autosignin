// Package core 定义插件配置、站点与调度模型，适配自 MoviePilot autosignin 3.0.3。
package core

import (
	"encoding/json"
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
// 凭据二选一：Cookie 直接填入配置，或在宿主“托管凭据”界面创建绑定后填入
// CredentialRef 由 Broker 注入，二者都为空时任务会报告未配置凭据。
type Site struct {
	ID            string `json:"id"`
	Key           string `json:"key"` // 内置处理器预设键，"" 表示通用 NexusPHP attendance
	Name          string `json:"name"`
	URL           string `json:"url"`
	Cookie        string `json:"cookie"`
	CredentialRef string `json:"credential_ref"`
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

// JSONRoundTrip 验证值可被 JSON 序列化（宿主 Bridge 要求）。
func JSONRoundTrip(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var out any
	return json.Unmarshal(b, &out)
}
