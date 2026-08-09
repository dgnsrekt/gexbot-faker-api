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
// run handles it. Results are capped to the maxDays most recent days (the
// individual /hist endpoint only serves a ~90-day look-back), and the returned
// bool reports whether older days were dropped by that cap.
func (s *Scheduler) MissedMarketDays(lastDate string, maxDays int) ([]string, bool) {
	last, err := time.ParseInLocation("2006-01-02", lastDate, s.location)
	if err != nil {
		return nil, false
	}
	todayStr := s.now().In(s.location).Format("2006-01-02")

	var days []string
	// Bound the scan so a corrupt/ancient lastDate can never spin forever.
	d := last.AddDate(0, 0, 1)
	for i := 0; i < 366 && d.Format("2006-01-02") < todayStr; i, d = i+1, d.AddDate(0, 0, 1) {
		ds := d.Format("2006-01-02")
		if s.IsMarketDay(ds) {
			days = append(days, ds)
		}
	}

	capped := false
	if maxDays > 0 && len(days) > maxDays {
		days = days[len(days)-maxDays:]
		capped = true
	}
	return days, capped
}
