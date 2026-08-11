package data

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"
)

// writeDay writes a day's JSONL fixture for one key at dataDir/{date}/{ticker}/{pkg}/{category}.jsonl,
// one row per timestamp (only "timestamp" matters for the loader/seek logic).
func writeDay(t *testing.T, root, date, ticker, pkg, category string, tss []int64) {
	t.Helper()
	dir := filepath.Join(root, date, ticker, pkg)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	var b strings.Builder
	for _, ts := range tss {
		fmt.Fprintf(&b, "{\"timestamp\":%d,\"spot\":%d}\n", ts, ts)
	}
	if err := os.WriteFile(filepath.Join(dir, category+".jsonl"), []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// twoDayRange builds a stream-mode RangeLoader over two days with:
//
//	SPX/classic/gex_zero : day1 [100,200,300]  day2 [400,500]   (present both days)
//	QQQ/classic/gex_zero : day2 [1000,1100]                     (present only day2)
func twoDayRange(t *testing.T) (*RangeLoader, string) {
	t.Helper()
	root := t.TempDir()
	writeDay(t, root, "2026-08-06", "SPX", "classic", "gex_zero", []int64{100, 200, 300})
	writeDay(t, root, "2026-08-07", "SPX", "classic", "gex_zero", []int64{400, 500})
	writeDay(t, root, "2026-08-07", "QQQ", "classic", "gex_zero", []int64{1000, 1100})
	rl, err := NewRangeLoader(root, []string{"2026-08-07", "2026-08-06"}, "stream", zap.NewNop()) // unsorted on purpose
	if err != nil {
		t.Fatalf("NewRangeLoader: %v", err)
	}
	t.Cleanup(func() { _ = rl.Close() })
	return rl, root
}

func rawTs(t *testing.T, rl *RangeLoader, ticker, pkg, cat string, idx int) int64 {
	t.Helper()
	raw, err := rl.GetRawAtIndex(context.Background(), ticker, pkg, cat, idx)
	if err != nil {
		t.Fatalf("GetRawAtIndex(%d): %v", idx, err)
	}
	return extractTimestampFromRaw(raw)
}

func TestRangeLoader_TranslationAndLength(t *testing.T) {
	rl, _ := twoDayRange(t)

	// Dates come back sorted chronologically regardless of input order.
	if got := rl.Dates(); len(got) != 2 || got[0] != "2026-08-06" || got[1] != "2026-08-07" {
		t.Fatalf("Dates() = %v, want [2026-08-06 2026-08-07]", got)
	}

	if n, err := rl.GetLength("SPX", "classic", "gex_zero"); err != nil || n != 5 {
		t.Fatalf("GetLength SPX = %d,%v; want 5,nil", n, err)
	}
	// Global index maps day1 rows then day2 rows.
	want := []int64{100, 200, 300, 400, 500}
	for i, w := range want {
		if got := rawTs(t, rl, "SPX", "classic", "gex_zero", i); got != w {
			t.Errorf("global idx %d ts = %d, want %d", i, got, w)
		}
	}
	if _, err := rl.GetRawAtIndex(context.Background(), "SPX", "classic", "gex_zero", 5); !errors.Is(err, ErrIndexOutOfBounds) {
		t.Errorf("idx 5 err = %v, want ErrIndexOutOfBounds", err)
	}
	if _, err := rl.GetLength("SPX", "classic", "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing key err = %v, want ErrNotFound", err)
	}
}

func TestRangeLoader_SeekWithinAndAcrossDays(t *testing.T) {
	rl, _ := twoDayRange(t)
	ctx := context.Background()
	cases := []struct {
		target    int64
		wantIndex int
		wantTs    int64
		wantDate  string
		wantInGap bool
		wantClamp string
	}{
		{target: 250, wantIndex: 2, wantTs: 300, wantDate: "2026-08-06"},                    // within day1
		{target: 300, wantIndex: 2, wantTs: 300, wantDate: "2026-08-06"},                    // exact within day1
		{target: 400, wantIndex: 3, wantTs: 400, wantDate: "2026-08-07"},                    // first row of day2
		{target: 450, wantIndex: 4, wantTs: 500, wantDate: "2026-08-07"},                    // within day2
		{target: 350, wantIndex: 3, wantTs: 400, wantDate: "2026-08-07", wantInGap: true},   // overnight gap → next open
		{target: 50, wantIndex: 0, wantTs: 100, wantDate: "2026-08-06", wantClamp: "start"}, // before span
	}
	for _, c := range cases {
		res, err := rl.SeekResolve(ctx, "SPX", "classic", "gex_zero", c.target, "clamp")
		if err != nil {
			t.Fatalf("SeekResolve(%d): %v", c.target, err)
		}
		clamp := c.wantClamp
		if clamp == "" {
			clamp = "none"
		}
		if res.Index != c.wantIndex || res.ResolvedTs != c.wantTs || res.Date != c.wantDate || res.InGap != c.wantInGap || res.Clamped != clamp {
			t.Errorf("SeekResolve(%d) = %+v; want idx=%d ts=%d date=%s inGap=%v clamp=%s",
				c.target, res, c.wantIndex, c.wantTs, c.wantDate, c.wantInGap, clamp)
		}
	}
}

func TestRangeLoader_AfterSpanEndPolicy(t *testing.T) {
	rl, _ := twoDayRange(t)
	ctx := context.Background()

	// clamp → last row of the span.
	res, err := rl.SeekResolve(ctx, "SPX", "classic", "gex_zero", 9999, "clamp")
	if err != nil {
		t.Fatalf("clamp SeekResolve: %v", err)
	}
	if res.Index != 4 || res.ResolvedTs != 500 || res.Clamped != "end" || res.Date != "2026-08-07" {
		t.Errorf("clamp after-span = %+v; want idx=4 ts=500 clamp=end date=2026-08-07", res)
	}

	// error → ErrTimestampOutOfRange (and FindIndexByTimestamp uses the strict path).
	if _, err := rl.SeekResolve(ctx, "SPX", "classic", "gex_zero", 9999, "error"); !errors.Is(err, ErrTimestampOutOfRange) {
		t.Errorf("error policy err = %v, want ErrTimestampOutOfRange", err)
	}
	if _, _, err := rl.FindIndexByTimestamp(ctx, "SPX", "classic", "gex_zero", 9999); !errors.Is(err, ErrTimestampOutOfRange) {
		t.Errorf("FindIndexByTimestamp after-span err = %v, want ErrTimestampOutOfRange", err)
	}
}

func TestRangeLoader_TickerAbsentOnSomeDays(t *testing.T) {
	rl, _ := twoDayRange(t)
	ctx := context.Background()

	// QQQ exists only on day2: length is day2-only, global 0 addresses day2's first row.
	if n, err := rl.GetLength("QQQ", "classic", "gex_zero"); err != nil || n != 2 {
		t.Fatalf("GetLength QQQ = %d,%v; want 2,nil", n, err)
	}
	if got := rawTs(t, rl, "QQQ", "classic", "gex_zero", 0); got != 1000 {
		t.Errorf("QQQ idx0 ts = %d, want 1000", got)
	}
	// Seek before QQQ's first row → clamp to start (its day2 open), not a gap/error.
	res, err := rl.SeekResolve(ctx, "QQQ", "classic", "gex_zero", 500, "clamp")
	if err != nil {
		t.Fatalf("SeekResolve QQQ: %v", err)
	}
	if res.Index != 0 || res.ResolvedTs != 1000 || res.Date != "2026-08-07" || res.Clamped != "start" {
		t.Errorf("QQQ pre-seek = %+v; want idx=0 ts=1000 date=2026-08-07 clamp=start", res)
	}
	if !rl.Exists("QQQ", "classic", "gex_zero") {
		t.Error("Exists(QQQ) = false, want true")
	}
	keys := rl.GetLoadedKeys()
	if len(keys) != 2 {
		t.Errorf("GetLoadedKeys = %v, want 2 (SPX+QQQ union)", keys)
	}
}

// TestRangeLoader_CursorRollover proves A3: with dataLength = the range total, IndexCache advances
// straight from day1's rows into day2's and only exhausts at the span's end — no cache change.
func TestRangeLoader_CursorRollover(t *testing.T) {
	rl, _ := twoDayRange(t)
	cache := NewIndexCache(CacheModeExhaust)
	length, _ := rl.GetLength("SPX", "classic", "gex_zero") // 5 across both days
	key := CacheKey("SPX", "classic", "gex_zero", "k1")
	want := []int64{100, 200, 300, 400, 500}
	for i := 0; i < len(want); i++ {
		idx, exhausted := cache.GetAndAdvance(key, length)
		if exhausted {
			t.Fatalf("pull %d exhausted early", i)
		}
		if got := rawTs(t, rl, "SPX", "classic", "gex_zero", idx); got != want[i] {
			t.Errorf("pull %d idx %d ts = %d, want %d", i, idx, got, want[i])
		}
	}
	if _, exhausted := cache.GetAndAdvance(key, length); !exhausted {
		t.Error("pull past span end did not exhaust")
	}
}

func TestRangeLoader_SingleDayEquivalence(t *testing.T) {
	root := t.TempDir()
	writeDay(t, root, "2026-08-06", "SPX", "classic", "gex_zero", []int64{100, 200, 300})
	ctx := context.Background()

	single, err := NewStreamLoader(root, "2026-08-06", zap.NewNop())
	if err != nil {
		t.Fatalf("NewStreamLoader: %v", err)
	}
	defer single.Close()
	rng, err := NewRangeLoader(root, []string{"2026-08-06"}, "stream", zap.NewNop())
	if err != nil {
		t.Fatalf("NewRangeLoader: %v", err)
	}
	defer rng.Close()

	sn, _ := single.GetLength("SPX", "classic", "gex_zero")
	rn, _ := rng.GetLength("SPX", "classic", "gex_zero")
	if sn != rn || rn != 3 {
		t.Fatalf("lengths differ: single=%d range=%d", sn, rn)
	}
	for i := 0; i < 3; i++ {
		sIdx, sTs, sErr := single.FindIndexByTimestamp(ctx, "SPX", "classic", "gex_zero", int64(50+100*i))
		rIdx, rTs, rErr := rng.FindIndexByTimestamp(ctx, "SPX", "classic", "gex_zero", int64(50+100*i))
		if sIdx != rIdx || sTs != rTs || (sErr == nil) != (rErr == nil) {
			t.Errorf("i=%d single=(%d,%d,%v) range=(%d,%d,%v)", i, sIdx, sTs, sErr, rIdx, rTs, rErr)
		}
	}
}
