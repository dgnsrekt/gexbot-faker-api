package data

import (
	"context"
	"fmt"
	"sort"

	"go.uber.org/zap"
)

// RangeLoader serves a contiguous span of trading days as one continuous dataset by composing
// one single-day DataLoader (StreamLoader/MemoryLoader) per date. Per key (ticker/pkg/category)
// the days' rows are concatenated in chronological order, so a global index addresses the whole
// span. Each day keeps only its own offset slices + file handles, so RAM stays bounded regardless
// of how many days are loaded.
//
// Days are chronological and each day's rows precede the next day's, so the concatenated per-key
// sequence stays timestamp-ascending — cross-day seek reuses each day's binary search and only
// adds a day-selection + gap/clamp layer on top (see SeekResolve).
type RangeLoader struct {
	dates []string     // sorted chronologically; aligned with days
	days  []DataLoader // one loader per date
}

// Compile-time interface verification.
var (
	_ DataLoader  = (*RangeLoader)(nil)
	_ RangeSeeker = (*RangeLoader)(nil)
)

// SeekResult is the range-aware resolution of a timestamp seek.
type SeekResult struct {
	Index      int    // global index into the concatenated span for this key
	ResolvedTs int64  // timestamp of the row the cursor landed on
	Date       string // which loaded day the resolved row belongs to
	InGap      bool   // target fell in an inter-session gap; clamped forward to the next open
	Clamped    string // "none" | "start" (before span) | "end" (after span, clamp policy)
}

// RangeSeeker resolves a timestamp across a loaded span, honoring an end-of-range policy
// ("clamp" → last row, "error" → ErrTimestampOutOfRange). Single-day loaders can be treated as a
// span of one via the ReloadableLoader fallback.
type RangeSeeker interface {
	SeekResolve(ctx context.Context, ticker, pkg, category string, target int64, endPolicy string) (SeekResult, error)
}

// NewRangeLoader builds a RangeLoader over the given dates (deduped + sorted chronologically),
// one per-day loader each per the data mode. On any per-day failure it closes what it built.
func NewRangeLoader(dataDir string, dates []string, mode string, logger *zap.Logger) (*RangeLoader, error) {
	sorted := dedupeSortedDates(dates)
	if len(sorted) == 0 {
		return nil, fmt.Errorf("no dates provided")
	}
	rl := &RangeLoader{dates: sorted}
	for _, d := range sorted {
		var day DataLoader
		var err error
		switch mode {
		case "memory":
			day, err = NewMemoryLoader(dataDir, d, logger)
		case "stream":
			day, err = NewStreamLoader(dataDir, d, logger)
		default:
			err = fmt.Errorf("unknown data mode: %s", mode)
		}
		if err != nil {
			_ = rl.Close()
			return nil, fmt.Errorf("loading %s: %w", d, err)
		}
		rl.days = append(rl.days, day)
	}
	return rl, nil
}

func dedupeSortedDates(dates []string) []string {
	seen := make(map[string]struct{}, len(dates))
	out := make([]string, 0, len(dates))
	for _, d := range dates {
		if d == "" {
			continue
		}
		if _, ok := seen[d]; ok {
			continue
		}
		seen[d] = struct{}{}
		out = append(out, d)
	}
	sort.Strings(out) // "YYYY-MM-DD" sorts chronologically
	return out
}

// Dates returns the loaded span in chronological order.
func (r *RangeLoader) Dates() []string {
	out := make([]string, len(r.dates))
	copy(out, r.dates)
	return out
}

// resolve maps a global index for a key to the owning day loader + local index. Days lacking the
// key contribute nothing. Returns ErrNotFound if no loaded day has the key, ErrIndexOutOfBounds if
// the global index is past the concatenated length.
func (r *RangeLoader) resolve(ticker, pkg, category string, global int) (DataLoader, int, error) {
	if global < 0 {
		return nil, 0, ErrIndexOutOfBounds
	}
	found := false
	g := global
	for _, day := range r.days {
		n, err := day.GetLength(ticker, pkg, category)
		if err != nil { // ErrNotFound: key not present on this day
			continue
		}
		found = true
		if g < n {
			return day, g, nil
		}
		g -= n
	}
	if !found {
		return nil, 0, ErrNotFound
	}
	return nil, 0, ErrIndexOutOfBounds
}

func (r *RangeLoader) GetAtIndex(ctx context.Context, ticker, pkg, category string, index int) (*GexData, error) {
	day, local, err := r.resolve(ticker, pkg, category, index)
	if err != nil {
		return nil, err
	}
	return day.GetAtIndex(ctx, ticker, pkg, category, local)
}

func (r *RangeLoader) GetRawAtIndex(ctx context.Context, ticker, pkg, category string, index int) ([]byte, error) {
	day, local, err := r.resolve(ticker, pkg, category, index)
	if err != nil {
		return nil, err
	}
	return day.GetRawAtIndex(ctx, ticker, pkg, category, local)
}

// GetLength returns the concatenated length across all days that carry the key.
func (r *RangeLoader) GetLength(ticker, pkg, category string) (int, error) {
	total := 0
	found := false
	for _, day := range r.days {
		n, err := day.GetLength(ticker, pkg, category)
		if err != nil {
			continue
		}
		found = true
		total += n
	}
	if !found {
		return 0, ErrNotFound
	}
	return total, nil
}

func (r *RangeLoader) Exists(ticker, pkg, category string) bool {
	for _, day := range r.days {
		if day.Exists(ticker, pkg, category) {
			return true
		}
	}
	return false
}

func (r *RangeLoader) GetLoadedKeys() []string {
	seen := make(map[string]struct{})
	keys := make([]string, 0)
	for _, day := range r.days {
		for _, k := range day.GetLoadedKeys() {
			if _, ok := seen[k]; ok {
				continue
			}
			seen[k] = struct{}{}
			keys = append(keys, k)
		}
	}
	return keys
}

func (r *RangeLoader) Close() error {
	var firstErr error
	for _, day := range r.days {
		if err := day.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	r.days = nil
	return firstErr
}

// FindIndexByTimestamp satisfies DataLoader with default (strict) end-of-range semantics: before
// the span clamps to index 0, after the span returns ErrTimestampOutOfRange — matching single-day
// behavior. Callers wanting gap/clamp policy detail use SeekResolve.
func (r *RangeLoader) FindIndexByTimestamp(ctx context.Context, ticker, pkg, category string, target int64) (int, int64, error) {
	res, err := r.SeekResolve(ctx, ticker, pkg, category, target, "error")
	if err != nil {
		return 0, 0, err
	}
	return res.Index, res.ResolvedTs, nil
}

// segment describes one day's contribution to a key's concatenated sequence.
type segment struct {
	loader        DataLoader
	base, length  int
	firstTs, last int64
	date          string
}

// SeekResolve resolves target across the whole span for one key. Days are chronological, so it
// walks them once: before the first row → clamp to global 0 (Clamped="start"); inside a day →
// that day's binary search; target between day N's last and day N+1's first row → clamp to N+1's
// open with InGap=true; past the last row → endPolicy ("clamp" → last row / "error" → out-of-range).
func (r *RangeLoader) SeekResolve(ctx context.Context, ticker, pkg, category string, target int64, endPolicy string) (SeekResult, error) {
	var segs []segment
	base := 0
	for i, day := range r.days {
		n, err := day.GetLength(ticker, pkg, category)
		if err != nil || n == 0 {
			continue
		}
		firstTs, err := tsAt(ctx, day, ticker, pkg, category, 0)
		if err != nil {
			return SeekResult{}, err
		}
		lastTs, err := tsAt(ctx, day, ticker, pkg, category, n-1)
		if err != nil {
			return SeekResult{}, err
		}
		segs = append(segs, segment{loader: day, base: base, length: n, firstTs: firstTs, last: lastTs, date: r.dates[i]})
		base += n
	}
	if len(segs) == 0 {
		return SeekResult{}, ErrNotFound
	}

	// Before the span (or exactly at the first row).
	if target <= segs[0].firstTs {
		clamped := "none"
		if target < segs[0].firstTs {
			clamped = "start"
		}
		return SeekResult{Index: segs[0].base, ResolvedTs: segs[0].firstTs, Date: segs[0].date, Clamped: clamped}, nil
	}

	// Inside the span. The first segment whose last row is at/after target owns it (all earlier
	// segments ended before target).
	for _, s := range segs {
		if target <= s.last {
			localIdx, actualTs, err := s.loader.FindIndexByTimestamp(ctx, ticker, pkg, category, target)
			if err != nil {
				return SeekResult{}, err
			}
			inGap := target < s.firstTs // fell in the gap before this day → clamped forward to its open
			return SeekResult{Index: s.base + localIdx, ResolvedTs: actualTs, Date: s.date, InGap: inGap, Clamped: "none"}, nil
		}
	}

	// After the span.
	last := segs[len(segs)-1]
	if endPolicy == "error" {
		return SeekResult{}, ErrTimestampOutOfRange
	}
	return SeekResult{Index: last.base + last.length - 1, ResolvedTs: last.last, Date: last.date, Clamped: "end"}, nil
}

// tsAt reads the timestamp of a single row from a day loader.
func tsAt(ctx context.Context, day DataLoader, ticker, pkg, category string, idx int) (int64, error) {
	raw, err := day.GetRawAtIndex(ctx, ticker, pkg, category, idx)
	if err != nil {
		return 0, err
	}
	return extractTimestampFromRaw(raw), nil
}
