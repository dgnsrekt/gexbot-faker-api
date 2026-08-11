package server

import (
	"context"
	"sort"

	"github.com/dgnsrekt/gexbot-downloader/internal/api/generated"
	"github.com/dgnsrekt/gexbot-downloader/internal/eod"
)

// loadStatusToAPI maps an internal job to the generated wire type.
func loadStatusToAPI(j rangeLoadJob) generated.LoadStatus {
	st := generated.LoadStatusState(j.State)
	out := generated.LoadStatus{
		JobId: ptr(j.ID),
		Dates: ptr(j.Dates),
		State: &st,
		Done:  ptr(j.Done),
		Total: ptr(j.Total),
	}
	if j.Error != "" {
		out.Error = ptr(j.Error)
	}
	if j.LoadedRange != nil {
		out.LoadedRange = &generated.LoadedRange{
			From:  ptr(j.LoadedRange.From),
			To:    ptr(j.LoadedRange.To),
			Dates: ptr(j.LoadedRange.Dates),
		}
	}
	return out
}

// Load starts an asynchronous load of one day or a contiguous span. Provide exactly one of: date
// (a single day), from+to (the available archived days in that inclusive span are loaded), or an
// explicit dates list. Single-day and span both run through the same job. Returns 202 + a job to poll.
func (s *Server) Load(ctx context.Context, request generated.LoadRequestObject) (generated.LoadResponseObject, error) {
	if request.Body == nil {
		return generated.Load400JSONResponse{Error: ptr("missing request body")}, nil
	}
	b := request.Body

	// Enforce the documented exclusive selector: exactly one of date | from+to | dates.
	selectors := 0
	if b.Date != nil && *b.Date != "" {
		selectors++
	}
	if b.Dates != nil && len(*b.Dates) > 0 {
		selectors++
	}
	if (b.From != nil && *b.From != "") || (b.To != nil && *b.To != "") {
		selectors++
	}
	if selectors > 1 {
		return generated.Load400JSONResponse{Error: ptr("provide exactly one of: date, from+to, or dates")}, nil
	}

	var dates []string
	switch {
	case b.Date != nil && *b.Date != "":
		if !isValidDateFormat(*b.Date) {
			return generated.Load400JSONResponse{Error: ptr("invalid date format (YYYY-MM-DD)")}, nil
		}
		dates = []string{*b.Date}
	case b.Dates != nil && len(*b.Dates) > 0:
		dates = *b.Dates
	case b.From != nil && b.To != nil && *b.From != "" && *b.To != "":
		from, to := *b.From, *b.To
		if !isValidDateFormat(from) || !isValidDateFormat(to) {
			return generated.Load400JSONResponse{Error: ptr("invalid from/to date format (YYYY-MM-DD)")}, nil
		}
		if from > to {
			from, to = to, from
		}
		archives, err := eod.ListArchives(s.config.DataDir)
		if err != nil {
			return generated.Load400JSONResponse{Error: ptr("failed to list archives: " + err.Error())}, nil
		}
		for _, a := range archives {
			if a.Date >= from && a.Date <= to {
				dates = append(dates, a.Date)
			}
		}
		if len(dates) == 0 {
			return generated.Load400JSONResponse{Error: ptr("no archived dates in range " + from + " to " + to)}, nil
		}
	default:
		return generated.Load400JSONResponse{Error: ptr("provide date, from+to, or a non-empty dates list")}, nil
	}

	dates = normalizeDates(dates)
	for _, d := range dates {
		if !isValidDateFormat(d) {
			return generated.Load400JSONResponse{Error: ptr("invalid date format: " + d)}, nil
		}
	}
	if len(dates) == 0 {
		return generated.Load400JSONResponse{Error: ptr("no dates to load")}, nil
	}

	job := s.rangeLoad.start(dates)
	return generated.Load202JSONResponse(loadStatusToAPI(job)), nil
}

// GetLoadStatus polls a load job.
func (s *Server) GetLoadStatus(ctx context.Context, request generated.GetLoadStatusRequestObject) (generated.GetLoadStatusResponseObject, error) {
	job, ok := s.rangeLoad.status(request.JobId)
	if !ok {
		return generated.GetLoadStatus404JSONResponse{Error: ptr("unknown job id: " + request.JobId)}, nil
	}
	return generated.GetLoadStatus200JSONResponse(loadStatusToAPI(job)), nil
}

// GetCoverage reports, from the archive inventory (no load needed), which tickers each day in
// [from, to] covers, plus the union and intersection across the span.
func (s *Server) GetCoverage(ctx context.Context, request generated.GetCoverageRequestObject) (generated.GetCoverageResponseObject, error) {
	from, to := request.Params.From, request.Params.To
	if !isValidDateFormat(from) || !isValidDateFormat(to) {
		return generated.GetCoverage400JSONResponse{Error: ptr("invalid from/to date format (YYYY-MM-DD)")}, nil
	}
	if from > to {
		from, to = to, from
	}
	archives, err := eod.ListArchives(s.config.DataDir)
	if err != nil {
		return generated.GetCoverage400JSONResponse{Error: ptr("failed to list archives: " + err.Error())}, nil
	}

	type dayCov struct {
		date    string
		tickers []string
	}
	var sel []dayCov
	for _, a := range archives {
		if a.Date >= from && a.Date <= to {
			tks := append([]string(nil), a.Tickers...)
			sort.Strings(tks)
			sel = append(sel, dayCov{date: a.Date, tickers: tks})
		}
	}
	sort.Slice(sel, func(i, j int) bool { return sel[i].date < sel[j].date })

	days := make([]generated.CoverageDay, 0, len(sel))
	unionSet := map[string]struct{}{}
	perDay := make([][]string, 0, len(sel))
	for _, d := range sel {
		days = append(days, generated.CoverageDay{Date: ptr(d.date), Tickers: ptr(d.tickers)})
		for _, t := range d.tickers {
			unionSet[t] = struct{}{}
		}
		perDay = append(perDay, d.tickers)
	}

	return generated.GetCoverage200JSONResponse{
		From:         ptr(from),
		To:           ptr(to),
		Days:         &days,
		Union:        ptr(sortedSetKeys(unionSet)),
		Intersection: ptr(intersectAll(perDay)),
	}, nil
}

// GetCurrentLoad reports the currently loaded span (one day for a single-day load).
func (s *Server) GetCurrentLoad(ctx context.Context, request generated.GetCurrentLoadRequestObject) (generated.GetCurrentLoadResponseObject, error) {
	dates := s.reloadManager.LoadedDates()
	filesLoaded := len(s.loader.GetLoadedKeys())
	loadedAt := s.reloadManager.LoadedAt()

	resp := generated.GetCurrentLoad200JSONResponse{
		Dates:       ptr(dates),
		FilesLoaded: ptr(filesLoaded),
		LoadedAt:    ptr(loadedAt),
	}
	if len(dates) > 0 {
		resp.From = ptr(dates[0])
		resp.To = ptr(dates[len(dates)-1])
	}
	return resp, nil
}

func sortedSetKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// intersectAll returns the tickers present on every day (empty when there are no days).
func intersectAll(sets [][]string) []string {
	if len(sets) == 0 {
		return []string{}
	}
	counts := map[string]int{}
	for _, s := range sets {
		seen := map[string]struct{}{}
		for _, t := range s {
			if _, ok := seen[t]; ok {
				continue
			}
			seen[t] = struct{}{}
			counts[t]++
		}
	}
	out := make([]string, 0)
	for t, c := range counts {
		if c == len(sets) {
			out = append(out, t)
		}
	}
	sort.Strings(out)
	return out
}
