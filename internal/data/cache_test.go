package data

import "testing"

func TestAllPositionsGroupsByAPIKey(t *testing.T) {
	c := NewIndexCache(CacheModeExhaust)
	c.SetIndex(CacheKey("SPX", "classic", "gex_zero", "keyA"), 5)
	c.SetIndex(CacheKey("SPX", "state", "gex_full", "keyA"), 9)
	c.SetIndex(WSCacheKey("classic", "NDX", "gex_zero", "keyB"), 3)
	c.SetIndex("malformed-no-slash", 1) // must be skipped

	all := c.AllPositions()

	if len(all) != 2 {
		t.Fatalf("expected 2 api keys, got %d: %v", len(all), all)
	}
	a, ok := all["keyA"]
	if !ok || len(a) != 2 {
		t.Fatalf("keyA: expected 2 streams, got %v", a)
	}
	if a["SPX/classic/gex_zero"] != 5 || a["SPX/state/gex_full"] != 9 {
		t.Errorf("keyA positions wrong: %v", a)
	}
	b, ok := all["keyB"]
	if !ok || b["ws/classic/NDX/gex_zero"] != 3 {
		t.Errorf("keyB: expected ws position 3, got %v", b)
	}
	if _, bad := all["malformed-no-slash"]; bad {
		t.Error("malformed key should have been skipped")
	}
}
