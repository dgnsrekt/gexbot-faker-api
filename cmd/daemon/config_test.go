package main

import "testing"

func TestDefaultSchedule(t *testing.T) {
	t.Setenv("DAEMON_SCHEDULE_HOUR", "")
	t.Setenv("DAEMON_SCHEDULE_MINUTE", "")
	cfg := LoadDaemonConfig()
	if cfg.ScheduleHour != 17 || cfg.ScheduleMinute != 5 {
		t.Fatalf("got %02d:%02d, want 17:05", cfg.ScheduleHour, cfg.ScheduleMinute)
	}
}
