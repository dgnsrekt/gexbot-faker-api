package main

import (
	"time"

	"github.com/scmhub/calendar"
)

// Scheduler handles time-based scheduling and market day validation
type Scheduler struct {
	hour     int
	minute   int
	location *time.Location
	nyse     *calendar.Calendar
	now      func() time.Time
}

// NewScheduler creates a new scheduler with the given schedule time and timezone
func NewScheduler(hour, minute int, timezone string) *Scheduler {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	return &Scheduler{
		hour:     hour,
		minute:   minute,
		location: loc,
		nyse:     calendar.XNYS(),
		now:      time.Now,
	}
}

// IsScheduledTime checks if current time matches the schedule (within the same minute)
func (s *Scheduler) IsScheduledTime() bool {
	now := s.now().In(s.location)
	return now.Hour() == s.hour && now.Minute() == s.minute
}

// IsScheduledOrLater supports catch-up after restarts and retrying failed EOD reports.
func (s *Scheduler) IsScheduledOrLater() bool {
	now := s.now().In(s.location)
	return now.Hour() > s.hour || (now.Hour() == s.hour && now.Minute() >= s.minute)
}

func (s *Scheduler) IsFallbackTime() bool {
	return s.now().In(s.location).Hour() >= 20
}

// TodayDate returns today's date in YYYY-MM-DD format in the configured timezone
func (s *Scheduler) TodayDate() string {
	return s.now().In(s.location).Format("2006-01-02")
}

// IsMarketDay checks if the given date is a trading day (not weekend/holiday)
func (s *Scheduler) IsMarketDay(dateStr string) bool {
	// Parse as noon in the configured timezone to ensure correct date matching
	t, err := time.ParseInLocation("2006-01-02 15:04:05", dateStr+" 12:00:00", s.location)
	if err != nil {
		return false
	}
	return s.nyse.IsBusinessDay(t)
}

// Location returns the scheduler's timezone location
func (s *Scheduler) Location() *time.Location {
	return s.location
}

// MissedMarketDays returns trading days strictly after lastDate and strictly
// before today (oldest first). Today is excluded because the normal scheduled
// run handles it.
//
// The scan is anchored to TODAY and clamped to the endpoint's look-back window
// (lookbackDays calendar days): when lastDate is older than the window, the scan
// starts at today-lookbackDays so the result is the most recent, still-fetchable
// market days — never a stale year that all falls outside the window. The
// returned bool reports whether older, now-unfetchable days were dropped.
func (s *Scheduler) MissedMarketDays(lastDate string, lookbackDays int) ([]string, bool) {
	last, err := time.ParseInLocation("2006-01-02", lastDate, s.location)
	if err != nil {
		return nil, false
	}
	now := s.now().In(s.location)
	todayStr := now.Format("2006-01-02")

	start := last.AddDate(0, 0, 1)
	dropped := false
	if lookbackDays > 0 {
		if earliest := now.AddDate(0, 0, -lookbackDays); start.Before(earliest) {
			start = earliest
			dropped = true
		}
	}

	var days []string
	// Loop is bounded by (today - start) <= lookbackDays, so it always terminates.
	for d := start; d.Format("2006-01-02") < todayStr; d = d.AddDate(0, 0, 1) {
		ds := d.Format("2006-01-02")
		if s.IsMarketDay(ds) {
			days = append(days, ds)
		}
	}
	return days, dropped
}
