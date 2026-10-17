// Package app 实现插件状态机、动作、常驻调度、历史记录与通知。
// 语义适配自 MoviePilot autosignin 3.0.3，宿主为 DIAN115 Plugin API v2。
package app

import (
	"context"
	"crypto/sha256"
	"dian115-autosignin/adapters"
	"dian115-autosignin/core"
	"dian115-autosignin/host"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type Store interface {
	Read(string, any) (string, bool, error)
	Write(string, any, string, string) error
}

// Service 为插件主服务。
type Service struct {
	Store  Store
	Broker adapters.Broker
}

// Input 为 runtime.invoke 载荷。
type Input struct {
	Envelope struct {
		Op           string          `json:"op"`
		InvocationID string          `json:"invocation_id"`
		Payload      json.RawMessage `json:"payload"`
	} `json:"envelope"`
}

// Heartbeat 心跳。
type Heartbeat struct {
	At     string `json:"at"`
	Enabled int    `json:"enabled_sites"`
	Status string `json:"status"`
}

// SiteResult 单站点一次动作的结果记录。
type SiteResult struct {
	Site     string `json:"site"`
	SiteID   string `json:"site_id"`
	Success  bool   `json:"success"`
	LoggedIn bool   `json:"logged_in"`
	Message  string `json:"message"`
}

// Completed 已签到或确认登录均完成本轮访问任务。
func (r SiteResult) Completed() bool { return r.Success || r.LoggedIn }

// DayEntry 一天内某类型（sign/login）的执行记录。
type DayEntry struct {
	Do      []string     `json:"do"`
	Retry   []string     `json:"retry"`
	Results []SiteResult `json:"results"`
}

// History 签到与登录的 14 天历史，按天索引。
type History struct {
	Sign  map[string]*DayEntry `json:"sign"`
	Login map[string]*DayEntry `json:"login"`
}

// RuntimeState 运行状态（含历史、随机调度与上次运行标记）。
type RuntimeState struct {
	LastRun     string   `json:"last_run"`
	LastCronKey string   `json:"last_cron_key"`
	LastError   string   `json:"last_error"`
	RandomDate  string   `json:"random_date"`
	RandomTimes []string `json:"random_times"`
	History     History  `json:"history"`
}

const (
	keyConfig  = "config"
	keyRuntime = "runtime"
	keyBeat    = "heartbeat"
	version    = "1.0.0"
)

// RunReport 动作返回的摘要。
type RunReport struct {
	Type     string       `json:"type"`
	All      int          `json:"all"`
	Done     int          `json:"done"`
	Next     int          `json:"next"`
	Success  int          `json:"success"`
	LoggedIn int          `json:"logged_in"`
	Failed   int          `json:"failed"`
	Message  string       `json:"message"`
	Results  []SiteResult `json:"results"`
}

func (s Service) config() (core.Config, string, error) {
	c := core.DefaultConfig()
	rev, found, err := s.Store.Read(keyConfig, &c)
	if err != nil {
		return c, "", err
	}
	if !found {
		return core.DefaultConfig(), "", nil
	}
	if err = c.Validate(); err != nil {
		return c, "", err
	}
	return c, rev, nil
}

func (s Service) runtimeState() (RuntimeState, error) {
	var st RuntimeState
	_, found, err := s.Store.Read(keyRuntime, &st)
	if err != nil {
		return st, err
	}
	if !found {
		return RuntimeState{History: History{Sign: map[string]*DayEntry{}, Login: map[string]*DayEntry{}}}, nil
	}
	if st.History.Sign == nil {
		st.History.Sign = map[string]*DayEntry{}
	}
	if st.History.Login == nil {
		st.History.Login = map[string]*DayEntry{}
	}
	return st, nil
}

// Invoke 分发运行时调用。
func (s Service) Invoke(in Input) (any, error) {
	if in.Envelope.InvocationID == "" {
		return nil, errors.New("invocation_id required")
	}
	switch in.Envelope.Op {
	case "state":
		return s.state(in.Envelope.Payload)
	case "action":
		return s.action(in.Envelope.InvocationID, in.Envelope.Payload)
	case "job":
		return map[string]any{"status": "skipped", "message": "No jobs declared; resident tick drives scheduling"}, nil
	case "event":
		return map[string]any{"handled": false}, nil
	default:
		return nil, errors.New("unsupported operation")
	}
}

func (s Service) state(raw json.RawMessage) (any, error) {
	var p struct {
		IfNoneMatch string `json:"if_none_match"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	c, rev, err := s.config()
	if err != nil {
		return nil, err
	}
	var beat Heartbeat
	_, _, _ = s.Store.Read(keyBeat, &beat)
	runtimeState, err := s.runtimeState()
	if err != nil {
		return nil, err
	}
	state := map[string]any{
		"config":          c,
		"config_revision": rev,
		"heartbeat":       beat,
		"runtime":         runtimeState,
		"mode":            "live",
		"live_execution":  true,
		"version":         version,
	}
	b, _ := json.Marshal(state)
	digest := sha256.Sum256(b)
	etag := `"` + hex.EncodeToString(digest[:]) + `"`
	if p.IfNoneMatch == etag {
		return map[string]any{"not_modified": true, "etag": etag}, nil
	}
	return map[string]any{"state_version": hex.EncodeToString(digest[:]), "etag": etag, "state": state}, nil
}

func failed(err error) map[string]any {
	return map[string]any{"status": "failed", "message": err.Error()}
}

func (s Service) action(id string, raw json.RawMessage) (any, error) {
	if len(raw) > 64<<10 {
		return failed(errors.New("action input exceeds 64 KiB")), nil
	}
	var p struct {
		ID    string          `json:"id"`
		Input json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	switch p.ID {
	case "save-config":
		var v struct {
			Config   core.Config `json:"config"`
			Revision string      `json:"revision"`
		}
		if err := json.Unmarshal(p.Input, &v); err != nil {
			return failed(err), nil
		}
		if err := v.Config.Validate(); err != nil {
			return failed(err), nil
		}
		_, rev, err := s.config()
		if err != nil {
			return failed(err), nil
		}
		if rev != v.Revision {
			return failed(host.ErrConflict), nil
		}
		digest := sha256.Sum256([]byte(id))
		key := "save-config-" + hex.EncodeToString(digest[:])
		if err = s.Store.Write(keyConfig, v.Config, rev, key); err != nil {
			return failed(err), nil
		}
		return map[string]any{"status": "succeeded", "message": "配置已保存"}, nil
	case "sign-now":
		var v struct {
			Type string `json:"type"` // "sign" | "login"
		}
		if err := json.Unmarshal(p.Input, &v); err != nil {
			return failed(err), nil
		}
		if v.Type == "" {
			v.Type = "sign"
		}
		if s.Broker == nil {
			return failed(errors.New("network broker unavailable")), nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 110*time.Second)
		defer cancel()
		report, execErr := s.runType(ctx, v.Type, id)
		if execErr != nil {
			return failed(execErr), nil
		}
		return map[string]any{"status": "succeeded", "message": fmt.Sprintf("%s：全部 %d，完成 %d，失败 %d", typeLabel(v.Type), report.All, report.Done, report.Failed), "report": report}, nil
	case "clear-history":
		st, err := s.runtimeState()
		if err != nil {
			return failed(err), nil
		}
		st.History = History{Sign: map[string]*DayEntry{}, Login: map[string]*DayEntry{}}
		if err := s.saveRuntime(st, "clear-history-"+time.Now().Format("20060102150405")); err != nil {
			return failed(err), nil
		}
		return map[string]any{"status": "succeeded", "message": "历史记录已清空"}, nil
	default:
		return failed(errors.New("unknown action")), nil
	}
}

// Tick 供 resident 循环每分钟调用，按调度执行签到/登录并刷新心跳。
func (s Service) Tick(now time.Time) error {
	c, _, err := s.config()
	if err != nil {
		return err
	}
	enabled := countEnabled(c)
	status := "ok"
	runErr := s.runDue(context.Background(), now)
	if runErr != nil {
		status = runErr.Error()
	}
	stamp := now.UTC().Format(time.RFC3339Nano)
	if err := s.Store.Write(keyBeat, Heartbeat{stamp, enabled, status}, "", fmt.Sprintf("heartbeat-%s", stamp)); err != nil {
		return err
	}
	return runErr
}

func countEnabled(c core.Config) int {
	seen := map[string]bool{}
	for _, id := range append(append([]string{}, c.SignSites...), c.LoginSites...) {
		if id == core.AllSites {
			return len(c.Sites)
		}
		if _, ok := c.SiteByID(id); ok {
			seen[id] = true
		}
	}
	return len(seen)
}

// runDue 判断当前时刻是否触发调度。
func (s Service) runDue(ctx context.Context, now time.Time) error {
	c, _, err := s.config()
	if err != nil {
		return err
	}
	if !c.Enabled {
		return nil
	}
	spec := core.ParseCron(c.Cron)
	st, err := s.runtimeState()
	if err != nil {
		return err
	}
	due := false
	switch spec.Mode {
	case "cron":
		key := now.Format("2006-01-02 15:04")
		due = core.CronMatches(spec, now) && st.LastCronKey != key
		if due {
			st.LastCronKey = key
		}
	case "interval":
		last := parseTime(st.LastRun)
		if last.IsZero() || now.Sub(last) >= time.Duration(spec.Hours*3600)*time.Second {
			if spec.StartHour == 0 && spec.EndHour == 24 || now.Hour() >= spec.StartHour && now.Hour() < spec.EndHour {
				due = true
			}
		}
	case "random":
		today := now.Format("2006-01-02")
		if st.RandomDate != today {
			times := core.RandomSchedule(now)
			st.RandomDate = today
			st.RandomTimes = nil
			for _, t := range times {
				st.RandomTimes = append(st.RandomTimes, t.Format("15:04"))
			}
		}
		lastDate := parseTime(st.LastRun).Format("2006-01-02")
		if lastDate != today {
			for _, rt := range st.RandomTimes {
				if rt <= now.Format("15:04") {
					due = true
					break
				}
			}
		}
	}
	if !due {
		return nil
	}
	_, signErr := s.runType(ctx, "sign", "tick-sign")
	_, loginErr := s.runType(ctx, "login", "tick-login")
	st.LastRun = now.UTC().Format(time.RFC3339Nano)
	if err := s.saveRuntime(st, "runtime-"+st.LastRun); err != nil {
		return err
	}
	if signErr != nil {
		return signErr
	}
	return loginErr
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// runType 执行一类站点动作（sign/login）并更新历史与通知。
func (s Service) runType(ctx context.Context, typ string, invocation string) (RunReport, error) {
	report := RunReport{Type: typ}
	c, _, err := s.config()
	if err != nil {
		return report, err
	}
	selected := c.SignSites
	if typ == "login" {
		selected = c.LoginSites
	}
	ids := c.ResolveSiteIDs(selected)
	report.All = len(ids)
	if len(ids) == 0 {
		report.Message = "没有需要" + typeLabel(typ) + "的站点"
		return report, nil
	}
	now := time.Now()
	day := now.Format("2006-01-02")
	st, err := s.runtimeState()
	if err != nil {
		return report, err
	}
	entry := st.History.Sign[day]
	if typ == "login" {
		entry = st.History.Login[day]
	}
	if entry == nil {
		entry = &DayEntry{}
	}
	doneSet := map[string]bool{}
	for _, id := range entry.Do {
		doneSet[id] = true
	}
	// 本次执行 = 昨日重试失败 + 今日未执行
	do := []string{}
	for _, id := range ids {
		if !doneSet[id] {
			do = append(do, id)
		}
	}
	for _, id := range entry.Retry {
		if !doneSet[id] {
			do = append(do, id)
		}
	}
	do = dedupe(do)
	if len(do) == 0 {
		report.Message = "本次无需" + typeLabel(typ)
		return report, nil
	}
	report.Done = len(do)
	results := map[string]SiteResult{}
	queue := c.QueueCnt
	if queue <= 0 {
		queue = 5
	}
	sem := make(chan struct{}, queue)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, id := range do {
		wg.Add(1)
		sem <- struct{}{}
		go func(id string) {
			defer wg.Done()
			defer func() { <-sem }()
			res := s.runSite(ctx, c, id, typ)
			mu.Lock()
			results[id] = res
			mu.Unlock()
		}(id)
	}
	wg.Wait()
	// 结果落历史
	ordered := []string{}
	for _, id := range do {
		ordered = append(ordered, id)
	}
	sort.Strings(ordered)
	newRetry := []string{}
	for _, id := range ordered {
		res := results[id]
		report.Results = append(report.Results, res)
		if res.Completed() {
			if res.Success {
				report.Success++
			} else {
				report.LoggedIn++
			}
		} else {
			report.Failed++
			newRetry = append(newRetry, id)
		}
	}
	report.Next = len(newRetry)
	entry.Do = dedupe(append(entry.Do, do...))
	entry.Retry = newRetry
	entry.Results = mergeResults(entry.Results, report.Results)
	if typ == "sign" {
		st.History.Sign[day] = entry
	} else {
		st.History.Login[day] = entry
	}
	pruneHistory(&st.History)
	st.LastRun = now.UTC().Format(time.RFC3339Nano)
	if err := s.saveRuntime(st, "run-"+typ+"-"+day+"-"+invocation); err != nil {
		return report, err
	}
	// 通知
	if c.Notify {
		_ = s.notify(typ, c, report)
	}
	report.Message = fmt.Sprintf("%s：全部 %d，完成 %d，失败 %d", typeLabel(typ), report.All, report.Done, report.Failed)
	return report, nil
}

func typeLabel(typ string) string {
	if typ == "login" {
		return "登录"
	}
	return "签到"
}

// runSite 执行单个站点动作，返回结果记录。
func (s Service) runSite(ctx context.Context, c core.Config, id, typ string) SiteResult {
	site, ok := c.SiteByID(id)
	if !ok {
		return SiteResult{Site: id, SiteID: id, Message: "站点不存在"}
	}
	client := &adapters.Client{Broker: s.Broker, Store: s.Store}
	handler := adapters.LookupHandler(site.Key)
	if handler == nil {
		handler = genericHandler()
	}
	name := site.Name
	if name == "" {
		name = id
	}
	res := adapters.Result{Message: "任务未执行"}
	if typ == "sign" {
		res = handler.Signin(ctx, site, client)
		if !res.Completed() && !handler.SigninIsLogin {
			login := handler.Login
			if login == nil {
				login = adapters.BaseLogin
			}
			if lr := login(ctx, site, client); lr.LoggedIn {
				if !strings.Contains(res.Message, lr.Message) {
					res.Message = fmt.Sprintf("%s，%s", res.Message, lr.Message)
				}
				res.LoggedIn = true
			}
		}
	} else {
		login := handler.Login
		if login == nil {
			login = adapters.BaseLogin
		}
		res = login(ctx, site, client)
	}
	// 重试关键词命中失败文案时视为失败
	if c.RetryKeyword != "" && !res.Completed() && retryMatch(c.RetryKeyword, res.Message) {
		res.Message = "签到失败，" + res.Message
	}
	return SiteResult{Site: name, SiteID: id, Success: res.Success, LoggedIn: res.LoggedIn, Message: truncate(res.Message, 200)}
}

func retryMatch(keyword, message string) bool {
	re, err := regexp.Compile(keyword)
	if err != nil {
		return false
	}
	return re.MatchString(message)
}

func genericHandler() *adapters.Handler {
	return &adapters.Handler{Signin: adapters.BaseSignin, Login: adapters.BaseLogin}
}

func mergeResults(existing, fresh []SiteResult) []SiteResult {
	byID := map[string]SiteResult{}
	for _, r := range existing {
		byID[r.SiteID] = r
	}
	for _, r := range fresh {
		byID[r.SiteID] = r
	}
	out := []SiteResult{}
	for _, r := range byID {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SiteID < out[j].SiteID })
	return out
}

// pruneHistory 只保留最近 14 天且总量受限，避免超出宿主存储单文档 64 KiB 限制。
func pruneHistory(h *History) {
	cut := time.Now().AddDate(0, 0, -(core.HistoryDays - 1)).Format("2006-01-02")
	pruneOne := func(m map[string]*DayEntry) {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if k < cut {
				delete(m, k)
			}
		}
	}
	pruneOne(h.Sign)
	pruneOne(h.Login)
	// 总量上限：每天最多 40 条结果，若仍超 120 条则丢弃最旧日期
	for totalEntries(h) > 120 {
		dropOldest(h)
	}
}

func totalEntries(h *History) int {
	n := 0
	for _, m := range []map[string]*DayEntry{h.Sign, h.Login} {
		for _, e := range m {
			n += len(e.Results)
		}
	}
	return n
}

func dropOldest(h *History) {
	oldest := ""
	for _, m := range []map[string]*DayEntry{h.Sign, h.Login} {
		for k := range m {
			if oldest == "" || k < oldest {
				oldest = k
			}
		}
	}
	if oldest == "" {
		return
	}
	delete(h.Sign, oldest)
	delete(h.Login, oldest)
}

func (s Service) saveRuntime(st RuntimeState, idem string) error {
	return s.Store.Write(keyRuntime, st, "", idem)
}

// notify 通过宿主通知 API 发送结果摘要。
func (s Service) notify(typ string, c core.Config, report RunReport) error {
	title := "【站点自动签到】"
	if typ == "login" {
		title = "【站点自动登录】"
	}
	level := "success"
	if report.Failed > 0 {
		level = "warning"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("全部%s数量: %d\n", typeLabel(typ), report.All))
	b.WriteString(fmt.Sprintf("本次%s数量: %d\n", typeLabel(typ), report.Done))
	b.WriteString(fmt.Sprintf("下次%s数量: %d\n", typeLabel(typ), report.Next))
	b.WriteString(fmt.Sprintf("确认%s成功: %d\n", typeLabel(typ), report.Success))
	b.WriteString(fmt.Sprintf("仅确认登录: %d\n", report.LoggedIn))
	b.WriteString(fmt.Sprintf("%s或访问失败: %d\n\n", typeLabel(typ), report.Failed))
	for _, r := range report.Results {
		b.WriteString(fmt.Sprintf("【%s】%s\n", r.Site, r.Message))
	}
	body := truncate(b.String(), 1900)
	payload, _ := json.Marshal(map[string]any{"level": level, "title": title, "body": body})
	res, err := s.Broker.Do(host.Request{
		Method:  "POST",
		Path:    "/api/notifications/plugin",
		Headers: map[string]string{"content-type": "application/json", "idempotency-key": "notify-" + report.Type + "-" + reportTime()},
		BodyBase64: host.EncodeBody(payload),
	})
	if err != nil {
		return err
	}
	if res.Status < 200 || res.Status > 299 {
		return fmt.Errorf("notification HTTP %d", res.Status)
	}
	return nil
}

func reportTime() string {
	return time.Now().Format("2006-01-02-15-04-05")
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := in[:0]
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
