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
	}
}

// IsScheduledTime checks if current time matches the schedule (within the same minute)
func (s *Scheduler) IsScheduledTime() bool {
	now := time.Now().In(s.location)
	return now.Hour() == s.hour && now.Minute() == s.minute
}

// TodayDate returns today's date in YYYY-MM-DD format in the configured timezone
func (s *Scheduler) TodayDate() string {
	return time.Now().In(s.location).Format("2006-01-02")
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
