package core

import (
	"testing"
	"time"
)

func TestCronMatches(t *testing.T) {
	spec := ParseCron("0 8 * * *")
	if spec.Mode != "cron" {
		t.Fatalf("mode = %s", spec.Mode)
	}
	at := time.Date(2026, 10, 17, 8, 0, 0, 0, time.Local)
	if !CronMatches(spec, at) {
		t.Fatal("08:00 must match 0 8 * * *")
	}
	if CronMatches(spec, at.Add(time.Minute)) {
		t.Fatal("08:01 must not match")
	}
	// 分 时 日 月 周
	weekly := ParseCron("30 9 * * 1")
	sun := time.Date(2026, 10, 18, 9, 30, 0, 0, time.Local) // Sunday
	mon := time.Date(2026, 10, 19, 9, 30, 0, 0, time.Local) // Monday
	if CronMatches(weekly, sun) {
		t.Fatal("Sunday must not match * * 1")
	}
	if !CronMatches(weekly, mon) {
		t.Fatal("Monday must match * * 1")
	}
	step := ParseCron("*/15 * * * *")
	if !CronMatches(step, time.Date(2026, 10, 17, 8, 15, 0, 0, time.Local)) {
		t.Fatal("08:15 must match */15")
	}
	if CronMatches(step, time.Date(2026, 10, 17, 8, 20, 0, 0, time.Local)) {
		t.Fatal("08:20 must not match */15")
	}
}

func TestParseCronModes(t *testing.T) {
	if ParseCron("").Mode != "random" {
		t.Fatal("empty must be random")
	}
	if ParseCron("2.3/9-23").Mode != "interval" {
		t.Fatal("2.3/9-23 must be interval")
	}
	if ParseCron("2.3").Mode != "interval" {
		t.Fatal("2.3 must be interval")
	}
	if ParseCron("abc").Mode != "random" {
		t.Fatal("invalid must fall back to random")
	}
	if err := ValidateCron("0 8 * * *"); err != nil {
		t.Fatalf("valid cron rejected: %v", err)
	}
	if err := ValidateCron("61 * * * *"); err == nil {
		t.Fatal("invalid cron accepted")
	}
}

func TestRandomSchedule(t *testing.T) {
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.Local)
	times := RandomSchedule(now)
	if len(times) != RandomExecutions {
		t.Fatalf("random times = %d", len(times))
	}
	for _, tt := range times {
		if tt.Hour() < RandomBeginHour || tt.Hour() >= RandomEndHour {
			t.Fatalf("random time out of window: %v", tt)
		}
	}
	if times[1].Before(times[0]) {
		t.Fatal("random times must be sorted")
	}
}
