package main

import (
	"fmt"
	"time"

	"github.com/dgnsrekt/gexbot-downloader/internal/config"
	"github.com/dgnsrekt/gexbot-downloader/internal/download"
	"github.com/scmhub/calendar"
	"go.uber.org/zap"
)

// parseDates parses date arguments and returns a list of dates
func parseDates(args []string) ([]string, error) {
	const layout = "2006-01-02"

	start, err := time.Parse(layout, args[0])
	if err != nil {
		return nil, fmt.Errorf("invalid start date format (use YYYY-MM-DD): %w", err)
	}

	if len(args) == 1 {
		return []string{args[0]}, nil
	}

	end, err := time.Parse(layout, args[1])
	if err != nil {
		return nil, fmt.Errorf("invalid end date format (use YYYY-MM-DD): %w", err)
	}

	if end.Before(start) {
		return nil, fmt.Errorf("end date must be after start date")
	}

	var dates []string
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		dates = append(dates, d.Format(layout))
	}

	return dates, nil
}

// generateTasks creates download tasks based on config and overrides
func generateTasks(cfg *config.Config, dates []string, tickerOverride, packageOverride []string) []download.Task {
	var tasks []download.Task

	// Determine tickers
	tickers := cfg.Tickers
	if len(tickerOverride) > 0 {
		tickers = tickerOverride
	}
	if len(tickers) == 0 {
		tickers = config.DefaultTickers()
	}

	// Build package/category map
	pkgCategories := make(map[string][]string)

	// Check for package overrides
	if len(packageOverride) > 0 {
		for _, pkg := range packageOverride {
			switch pkg {
			case "state":
				if cfg.Packages.State.Enabled || len(packageOverride) > 0 {
					cats := cfg.Packages.State.Categories
					if len(cats) == 0 {
						cats = config.ValidCategories[config.PackageState]
					}
					pkgCategories["state"] = cats
				}
			case "classic":
				if cfg.Packages.Classic.Enabled || len(packageOverride) > 0 {
					cats := cfg.Packages.Classic.Categories
					if len(cats) == 0 {
						cats = config.ValidCategories[config.PackageClassic]
					}
					pkgCategories["classic"] = cats
				}
			case "orderflow":
				if cfg.Packages.Orderflow.Enabled || len(packageOverride) > 0 {
					cats := cfg.Packages.Orderflow.Categories
					if len(cats) == 0 {
						cats = config.ValidCategories[config.PackageOrderflow]
					}
					pkgCategories["orderflow"] = cats
				}
			}
		}
	} else {
		// Use config settings
		if cfg.Packages.State.Enabled {
			cats := cfg.Packages.State.Categories
			if len(cats) == 0 {
				cats = config.ValidCategories[config.PackageState]
			}
			pkgCategories["state"] = cats
		}
		if cfg.Packages.Classic.Enabled {
			cats := cfg.Packages.Classic.Categories
			if len(cats) == 0 {
				cats = config.ValidCategories[config.PackageClassic]
			}
			pkgCategories["classic"] = cats
		}
		if cfg.Packages.Orderflow.Enabled {
			cats := cfg.Packages.Orderflow.Categories
			if len(cats) == 0 {
				cats = config.ValidCategories[config.PackageOrderflow]
			}
			pkgCategories["orderflow"] = cats
		}
	}

	// Generate tasks for all combinations
	for _, date := range dates {
		for _, ticker := range tickers {
			for pkg, categories := range pkgCategories {
				for _, category := range categories {
					tasks = append(tasks, download.Task{
						Ticker:   ticker,
						Package:  pkg,
						Category: category,
						Date:     date,
					})
				}
			}
		}
	}

	return tasks
}

// filterMarketDays filters out non-trading days (weekends and NYSE holidays)
// and logs warnings for skipped dates
func filterMarketDays(dates []string, logger *zap.Logger) []string {
	nyse := calendar.XNYS()
	const layout = "2006-01-02 15:04:05"

	// NYSE operates in Eastern time
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		logger.Warn("failed to load America/New_York timezone, using UTC", zap.Error(err))
		loc = time.UTC
	}

	var marketDays []string
	for _, dateStr := range dates {
		// Parse as noon in NYC timezone to ensure correct date matching
		t, _ := time.ParseInLocation(layout, dateStr+" 12:00:00", loc)
		if nyse.IsBusinessDay(t) {
			marketDays = append(marketDays, dateStr)
		} else {
			logger.Warn("skipping non-market day", zap.String("date", dateStr))
		}
	}
	return marketDays
}
