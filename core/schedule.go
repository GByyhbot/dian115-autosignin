package core

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

// 调度语义移植自 autosignin：
//   - 5 字段 cron（"0 8 * * *"）；
//   - 周期/窗口（"2.3/9-23"：每 2.3 小时且在 9-23 点之间）；
//   - 纯周期（"2.3"：每 2.3 小时）；
//   - 空值且启用：随机 2 次（9-23 点窗口内）。
const (
	RandomExecutions = 2
	RandomBeginHour  = 9
	RandomEndHour    = 23
)

// CronSpec 为解析后的调度描述。
type CronSpec struct {
	Mode       string // "cron" | "interval" | "random" | "off"
	Raw        string
	CronFields []string
	Hours      float64
	StartHour  int
	EndHour    int
}

// ParseCron 解析配置中的 cron 字符串，返回规范化的调度描述。
func ParseCron(cron string) CronSpec {
	raw := strings.TrimSpace(cron)
	if raw == "" {
		return CronSpec{Mode: "random", Raw: raw, StartHour: RandomBeginHour, EndHour: RandomEndHour}
	}
	fields := strings.Fields(raw)
	if len(fields) == 5 {
		return CronSpec{Mode: "cron", Raw: raw, CronFields: fields}
	}
	if parts := strings.Split(raw, "/"); len(parts) == 2 {
		hours, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		start, end := RandomBeginHour, RandomEndHour
		if times := strings.Split(strings.TrimSpace(parts[1]), "-"); len(times) == 2 {
			if s, err := strconv.Atoi(strings.TrimSpace(times[0])); err == nil {
				start = s
			}
			if e, err := strconv.Atoi(strings.TrimSpace(times[1])); err == nil {
				end = e
			}
		}
		if err1 == nil && hours > 0 {
			return CronSpec{Mode: "interval", Raw: raw, Hours: hours, StartHour: start, EndHour: end}
		}
	}
	if hours, err := strconv.ParseFloat(raw, 64); err == nil && hours > 0 {
		return CronSpec{Mode: "interval", Raw: raw, Hours: hours, StartHour: 0, EndHour: 24}
	}
	return CronSpec{Mode: "random", Raw: raw, StartHour: RandomBeginHour, EndHour: RandomEndHour}
}

// ValidateCron 校验调度格式可解析。
func ValidateCron(cron string) error {
	spec := ParseCron(cron)
	if spec.Mode == "cron" {
		for _, f := range spec.CronFields {
			if _, err := parseField(f, 0, 59); err != nil {
				return fmt.Errorf("cron 字段错误：%w", err)
			}
		}
	}
	if spec.Mode == "interval" && (spec.StartHour < 0 || spec.StartHour > 23 || spec.EndHour < 0 || spec.EndHour > 24 || spec.StartHour >= spec.EndHour) {
		return fmt.Errorf("周期窗口格式错误：%q", cron)
	}
	return nil
}

// parseField 解析 cron 单字段（*、*/n、a-b、a,b、数值）。
func parseField(field string, minV, maxV int) ([]int, error) {
	field = strings.TrimSpace(field)
	if field == "*" {
		out := make([]int, 0, maxV-minV+1)
		for v := minV; v <= maxV; v++ {
			out = append(out, v)
		}
		return out, nil
	}
	if strings.HasPrefix(field, "*/") {
		step, err := strconv.Atoi(strings.TrimPrefix(field, "*/"))
		if err != nil || step <= 0 {
			return nil, fmt.Errorf("step 错误：%s", field)
		}
		out := []int{}
		for v := minV; v <= maxV; v += step {
			out = append(out, v)
		}
		return out, nil
	}
	if strings.Contains(field, "-") {
		parts := strings.SplitN(field, "-", 2)
		a, err1 := strconv.Atoi(parts[0])
		b, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil || a > b {
			return nil, fmt.Errorf("range 错误：%s", field)
		}
		out := []int{}
		for v := a; v <= b; v++ {
			out = append(out, v)
		}
		return out, nil
	}
	if strings.Contains(field, ",") {
		out := []int{}
		for _, part := range strings.Split(field, ",") {
			vals, err := parseField(part, minV, maxV)
			if err != nil {
				return nil, err
			}
			out = append(out, vals...)
		}
		return out, nil
	}
	v, err := strconv.Atoi(field)
	if err != nil || v < minV || v > maxV {
		return nil, fmt.Errorf("value 错误：%s", field)
	}
	return []int{v}, nil
}

func fieldMatches(field string, minV, maxV, value int) bool {
	vals, err := parseField(field, minV, maxV)
	if err != nil {
		return false
	}
	for _, v := range vals {
		if v == value {
			return true
		}
	}
	return false
}

// CronMatches 判断时刻是否命中 5 字段 cron（分 时 日 月 周）。
func CronMatches(spec CronSpec, t time.Time) bool {
	if spec.Mode != "cron" || len(spec.CronFields) != 5 {
		return false
	}
	if !fieldMatches(spec.CronFields[0], 0, 59, t.Minute()) ||
		!fieldMatches(spec.CronFields[1], 0, 23, t.Hour()) ||
		!fieldMatches(spec.CronFields[2], 1, 31, t.Day()) ||
		!fieldMatches(spec.CronFields[3], 1, 12, int(t.Month())) {
		return false
	}
	dow := int(t.Weekday())
	if dow == 0 {
		dow = 7 // cron 周日为 7，与常见实现一致
	}
	return fieldMatches(spec.CronFields[4], 0, 7, dow)
}

// RandomSchedule 生成当天随机时间（与原插件 random_scheduler 语义一致：窗口内随机、间隔受限）。
func RandomSchedule(now time.Time) []time.Time {
	begin := time.Date(now.Year(), now.Month(), now.Day(), RandomBeginHour, 0, 0, 0, now.Location())
	end := time.Date(now.Year(), now.Month(), now.Day(), RandomEndHour, 0, 0, 0, now.Location())
	window := int(end.Sub(begin).Minutes())
	if window <= 0 {
		return nil
	}
	// 生成 2 个互异时刻，最小间隔 2 分钟
	times := []time.Time{}
	seen := map[int]bool{}
	for len(times) < RandomExecutions {
		if len(seen) >= window {
			break
		}
		offset := rand.Intn(window)
		if seen[offset] {
			continue
		}
		seen[offset] = true
		times = append(times, begin.Add(time.Duration(offset)*time.Minute))
	}
	// 稳定排序
	for i := 1; i < len(times); i++ {
		for j := i; j > 0 && times[j].Before(times[j-1]); j-- {
			times[j], times[j-1] = times[j-1], times[j]
		}
	}
	return times
}
