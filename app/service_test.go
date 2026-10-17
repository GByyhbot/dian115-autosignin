package app

import (
	"dian115-autosignin/core"
	"dian115-autosignin/host"
	"dian115-autosignin/pluginjson"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// memStore 为测试用内存存储。
type memStore struct {
	docs   map[string]string
	etags  map[string]string
	nextID int
}

func newMemStore() *memStore {
	return &memStore{docs: map[string]string{}, etags: map[string]string{}}
}

func (m *memStore) Read(key string) (pluginjson.Raw, string, bool, error) {
	raw, ok := m.docs[key]
	if !ok {
		return nil, "", false, nil
	}
	return pluginjson.Raw(raw), m.etags[key], true, nil
}

func (m *memStore) Write(key string, value pluginjson.Raw, etag, idempotency string) error {
	if etag != "" && m.etags[key] != etag {
		return host.ErrConflict
	}
	m.nextID++
	m.docs[key] = string(value)
	m.etags[key] = `"mem_` + string(rune('0'+m.nextID%10)) + `"`
	return nil
}

// stubBroker 返回固定页面。
type stubBroker struct{ body string }

func (b *stubBroker) Do(req host.Request) (host.Response, error) {
	return host.Response{Status: 200, Headers: map[string][]string{"content-type": {"text/html"}}, BodyBase64: host.EncodeBody([]byte(b.body))}, nil
}

func svc(store *memStore, body string) Service {
	return Service{Store: store, Broker: &stubBroker{body: body}}
}

func invoke(t *testing.T, s Service, op, id string, input any) map[string]any {
	payload, _ := json.Marshal(input)
	res, err := s.Invoke(op, "test-"+id, pluginjson.Raw(payload))
	if err != nil {
		t.Fatalf("invoke %s/%s failed: %v", op, id, err)
	}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("invoke returned %T", res)
	}
	return m
}

func configWithSite() core.Config {
	c := core.DefaultConfig()
	c.Enabled = true
	c.Cron = "0 8 * * *"
	c.Sites = []core.Site{{ID: "s1", Key: "pttime", Name: "PTTime", URL: "https://www.pttime.org", Cookie: "uid=1", Timeout: 10}}
	c.SignSites = []string{"s1"}
	return c
}

func TestStateAndSaveConfig(t *testing.T) {
	store := newMemStore()
	s := svc(store, "")
	st := invoke(t, s, "state", "1", map[string]any{})
	state := st["state"].(map[string]any)
	if state["live_execution"] != true {
		t.Fatalf("state = %v", state)
	}
	cfg := state["config"].(map[string]any)
	if cfg["schema_version"] != float64(core.SchemaVersion) && cfg["schema_version"] != core.SchemaVersion {
		t.Fatalf("config = %+v", cfg)
	}
	rev := state["config_revision"].(string)
	out := invoke(t, s, "action", "2", map[string]any{"id": "save-config", "input": map[string]any{"config": core.ConfigToMap(configWithSite()), "revision": rev}})
	if out["status"] != "succeeded" {
		t.Fatalf("save = %v", out)
	}
	// 冲突：旧 revision
	out2 := invoke(t, s, "action", "3", map[string]any{"id": "save-config", "input": map[string]any{"config": core.ConfigToMap(configWithSite()), "revision": rev}})
	if out2["status"] != "failed" || !strings.Contains(out2["message"].(string), "refresh") {
		t.Fatalf("conflict save = %v", out2)
	}
	// 校验失败
	out3 := invoke(t, s, "action", "4", map[string]any{"id": "save-config", "input": map[string]any{"config": map[string]any{"sites": []any{map[string]any{"id": ""}}}, "revision": ""}})
	if out3["status"] != "failed" {
		t.Fatalf("invalid save = %v", out3)
	}
}

func TestSignNowNoSites(t *testing.T) {
	store := newMemStore()
	s := svc(store, "")
	out := invoke(t, s, "action", "5", map[string]any{"id": "sign-now", "input": map[string]any{"type": "sign"}})
	if out["status"] != "succeeded" {
		t.Fatalf("sign-now = %v", out)
	}
	report := out["report"].(map[string]any)
	if report["all"] != float64(0) && report["all"] != 0 {
		t.Fatalf("report = %+v", report)
	}
}

func TestSignNowWithSite(t *testing.T) {
	store := newMemStore()
	c := configWithSite()
	cfgRaw, _ := core.EncodeConfig(c)
	_ = store.Write("config", cfgRaw, "", "init")
	page := `<html><body><a href="logout.php">退出登录</a><div>签到成功，获得魔力值</div></body></html>`
	s := svc(store, page)
	out := invoke(t, s, "action", "6", map[string]any{"id": "sign-now", "input": map[string]any{"type": "sign"}})
	if out["status"] != "succeeded" {
		t.Fatalf("sign-now = %v", out)
	}
	report := out["report"].(map[string]any)
	if report["all"] != float64(1) && report["all"] != 1 {
		t.Fatalf("report = %+v", report)
	}
	// 历史已记录
	st, err := s.runtimeState()
	if err != nil {
		t.Fatal(err)
	}
	today := time.Now().Format("2006-01-02")
	entry := st.History.Sign[today]
	if entry == nil || len(entry.Results) != 1 {
		t.Fatalf("history entry = %+v", entry)
	}
}

func TestTickCronDue(t *testing.T) {
	store := newMemStore()
	c := configWithSite()
	cfgRaw, _ := core.EncodeConfig(c)
	_ = store.Write("config", cfgRaw, "", "init")
	page := `<html><body><a href="logout.php">退出</a><span>签到成功</span></body></html>`
	s := svc(store, page)
	now := time.Now()
	// 将 Cron 改为当前分钟匹配（Go 布局：04=分钟、15=小时）
	nowCfg := c
	nowCfg.Cron = now.Format("04 15 * * *")
	nowRaw, _ := core.EncodeConfig(nowCfg)
	_ = store.Write("config", nowRaw, "", "init2")
	if err := s.Tick(now); err != nil {
		t.Fatalf("tick: %v", err)
	}
	st, _ := s.runtimeState()
	if st.LastRun == "" {
		t.Fatal("tick must set last_run when due")
	}
	// 同一分钟再次 tick 不应重复执行（last_cron_key 去重）
	stBefore := st.LastRun
	if err := s.Tick(now); err != nil {
		t.Fatal(err)
	}
	st2, _ := s.runtimeState()
	if st2.LastRun != stBefore {
		t.Fatal("duplicate run within same minute")
	}
}

func TestClearHistory(t *testing.T) {
	store := newMemStore()
	s := svc(store, "")
	out := invoke(t, s, "action", "7", map[string]any{"id": "clear-history", "input": map[string]any{}})
	if out["status"] != "succeeded" {
		t.Fatalf("clear-history = %v", out)
	}
}
