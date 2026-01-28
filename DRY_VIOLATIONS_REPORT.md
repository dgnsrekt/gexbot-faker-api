# DRY Violations Audit Report

**Project:** GEX Faker API
**Date:** 2026-01-28
**Total Estimated Duplicated Lines:** ~1,500+ lines across the codebase

---

## Executive Summary

This audit identified significant DRY (Don't Repeat Yourself) violations across the codebase, primarily in:
1. **WebSocket streamers** (~850 duplicated lines, 83% of streamer code)
2. **REST handlers** (~350-400 duplicated lines)
3. **cmd/ entry points** (~300+ duplicated lines between downloader and daemon)
4. **Data layer** (~150 duplicated lines between memory and stream loaders)
5. **Configuration/utilities** (~50+ duplicated lines)

---

## 1. WebSocket Streamers (Highest Priority)

**Files:** `internal/ws/gex_streamer.go`, `classic_streamer.go`, `greek_streamer.go`, `greek_one_streamer.go`, `streamer.go`

The five streamer implementations are **83% duplicated code** (~850 lines of ~1,018 total).

### 1.1 Identical Struct Definitions

| File | Struct | Lines |
|------|--------|-------|
| `gex_streamer.go` | `GexStreamer` | 16-24 |
| `classic_streamer.go` | `ClassicStreamer` | 16-24 |
| `greek_streamer.go` | `GreekStreamer` | 16-24 |
| `greek_one_streamer.go` | `GreekOneStreamer` | 16-24 |
| `streamer.go` | `Streamer` | 21-29 |

**Duplicated snippet:**
```go
type GexStreamer struct {
    hub           *Hub
    loader        data.DataLoader
    cache         *data.IndexCache
    encoder       *Encoder
    interval      time.Duration
    logger        *zap.Logger
    reloadChecker ReloadChecker
}
```

**Consolidation notes:** Create a generic `BaseStreamer` struct with these common fields. Each specialized streamer can embed it.

### 1.2 Identical Constructor Functions (5 instances, ~80 lines)

All five `New*Streamer()` functions are identical except for return type.

| File | Function | Lines |
|------|----------|-------|
| `gex_streamer.go` | `NewGexStreamer` | 27-42 |
| `classic_streamer.go` | `NewClassicStreamer` | 27-42 |
| `greek_streamer.go` | `NewGreekStreamer` | 27-42 |
| `greek_one_streamer.go` | `NewGreekOneStreamer` | 27-42 |
| `streamer.go` | `NewStreamer` | 32-47 |

**Consolidation notes:** Create a generic constructor or factory function that returns a configured base streamer.

### 1.3 Identical `Run()` Methods (5 instances, ~185 lines)

All `Run()` methods are identical except for log message strings ("gex streamer", "classic streamer", etc.).

| File | Lines |
|------|-------|
| `gex_streamer.go` | 46-82 |
| `classic_streamer.go` | 46-82 |
| `greek_streamer.go` | 46-82 |
| `greek_one_streamer.go` | 46-82 |
| `streamer.go` | 51-87 |

**Consolidation notes:** Move `Run()` to the base streamer. Pass the name as a configuration field for logging.

### 1.4 Nearly Identical `broadcastNext()` Methods (5 instances, ~430 lines)

The `broadcastNext()` implementations differ only in 5-7 configuration values:

| Parameter | gex | classic | greek | greek_one | orderflow |
|-----------|-----|---------|-------|-----------|-----------|
| Extract function | `extractGexTickerAndCategory` | `extractClassicTickerAndCategory` | `extractGreekTickerAndCategory` | `extractGreekOneTickerAndCategory` | `extractTicker` |
| Package | `"state"` | `"classic"` | `"state"` | `"state"` | `"orderflow"` |
| Cache prefix | `"state_gex"` | `"classic"` | `"state_greeks_zero"` | `"state_greeks_one"` | `"orderflow"` |
| Encode method | `EncodeGex` | `EncodeGex` | `EncodeGreek` | `EncodeGreek` | `EncodeOrderflow` |
| Proto type | `"proto.gex"` | `"proto.gex"` | `"proto.greek"` | `"proto.greek"` | `"proto.orderflow"` |

**Consolidation notes:** Create a `StreamerConfig` struct with these parameters and a single parameterized `broadcastNext()` implementation.

### 1.5 Nearly Identical Extract Functions (4 instances, ~112 lines)

| File | Function | Lines | Separator | Valid Categories |
|------|----------|-------|-----------|------------------|
| `gex_streamer.go` | `extractGexTickerAndCategory` | 178-205 | `"_state_"` | `"gex_full", "gex_zero", "gex_one"` |
| `classic_streamer.go` | `extractClassicTickerAndCategory` | 178-205 | `"_classic_"` | `"gex_full", "gex_zero", "gex_one"` |
| `greek_streamer.go` | `extractGreekTickerAndCategory` | 178-205 | `"_state_"` | `"delta_zero", "gamma_zero", "vanna_zero", "charm_zero"` |
| `greek_one_streamer.go` | `extractGreekOneTickerAndCategory` | 178-205 | `"_state_"` | `"delta_one", "gamma_one", "vanna_one", "charm_one"` |

**Consolidation notes:** Create a generic `extractTickerAndCategory(group, separator string, validCategories []string)` function.

---

## 2. REST API Handlers (High Priority)

**File:** `internal/server/handlers.go` (~1,000 lines)

### 2.1 Repeated "Data Not Found" Check (7 instances)

**Lines:** 83-87, 169-173, 267-271, 600-604, 748-752, 836-840, 931-935

```go
if !s.loader.Exists(ticker, pkg, category) {
    return generated.GetClassicGexMajors404JSONResponse{
        Error: ptr("Data not found for " + ticker + "/classic/" + aggregation),
    }, nil
}
```

### 2.2 Repeated GetLength Error Handling (7 instances)

**Lines:** 90-95, 176-181, 274-279, 607-612, 755-760, 843-848, 938-943

```go
length, err := s.loader.GetLength(ticker, pkg, category)
if err != nil {
    return generated.GetClassicGexMajors404JSONResponse{
        Error: ptr(err.Error()),
    }, nil
}
```

### 2.3 Repeated Cache Key Building (7 instances)

**Lines:** 98-105, 184-191, 282-289, 615-622, 763-770, 851-858, 946-951

```go
var cacheKey string
if s.config.EndpointCacheMode == "shared" {
    cacheKey = data.SharedCacheKey(ticker, pkg, apiKey)
} else {
    cacheKey = data.CacheKey(ticker, pkg, category+"_majors", apiKey)
}
idx, exhausted := s.cache.GetAndAdvance(cacheKey, length)
```

### 2.4 Repeated Data Exhausted Handling (7 instances)

**Lines:** 107-116, 193-202, 291-300, 626-635, 774-783, 862-871, 955-964

```go
if exhausted {
    s.logger.Debug("data exhausted",
        zap.String("cacheKey", maskCacheKey(cacheKey)),
        zap.Int("index", idx),
        zap.Int("length", length),
    )
    return generated.GetClassicGexMajors404JSONResponse{
        Error: ptr("No more data available"),
    }, nil
}
```

### 2.5 Repeated Index Out of Bounds Check (7 instances)

**Lines:** 119-129, 205-215, 303-313, 638-648, 786-796, 874-884, 967-977

```go
gexData, err := s.loader.GetAtIndex(ctx, ticker, pkg, category, idx)
if err != nil {
    if errors.Is(err, data.ErrIndexOutOfBounds) {
        return generated.GetClassicGexMajors404JSONResponse{
            Error: ptr("Index out of bounds"),
        }, nil
    }
    return generated.GetClassicGexMajors404JSONResponse{
        Error: ptr(err.Error()),
    }, nil
}
```

### 2.6 Category Mapping (5 instances)

**Lines:** 72, 158, 256, 737, 825

```go
category := "gex_" + aggregation // full->gex_full, zero->gex_zero, one->gex_one
```

### 2.7 Date Format Validation (2 instances)

| File | Lines |
|------|-------|
| `handlers.go` | 1068 |
| `reload.go` | 188-191 |

```go
datePattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
```

### 2.8 API Key Masking (2 instances)

| Location | Lines |
|----------|-------|
| `handlers.go` | 1043-1048 |
| `server.go` | 131-134 |

**Consolidation notes:** Create a generic handler helper with:
- `checkDataExists(ticker, pkg, category) error`
- `getLengthOrError(ticker, pkg, category) (int, error)`
- `buildCacheKey(config, ticker, pkg, category, apiKey, suffix) string`
- `handleExhausted(cacheKey, idx, length) error`
- `getDataAtIndexOrError(ctx, ticker, pkg, category, idx) (*GexData, error)`

---

## 3. Data Layer (Medium Priority)

**Files:** `internal/data/memory.go`, `stream.go`

### 3.1 Identical `extractTimestamp` Function (2 instances, 100% identical)

| File | Function | Lines |
|------|----------|-------|
| `memory.go` | `extractTimestamp` | 189-196 |
| `stream.go` | `extractTimestampFromRaw` | 269-276 |

```go
func extractTimestamp(raw []byte) int64 {
    var partial struct {
        Timestamp int64 `json:"timestamp"`
    }
    _ = json.Unmarshal(raw, &partial)
    return partial.Timestamp
}
```

**Consolidation notes:** Move to `loader.go` as a shared utility.

### 3.2 Identical `GetAtIndex` Method (2 instances, 95% identical)

| File | Lines |
|------|-------|
| `memory.go` | 107-118 |
| `stream.go` | 129-141 |

```go
func (m *MemoryLoader) GetAtIndex(ctx context.Context, ticker, pkg, category string, index int) (*GexData, error) {
    rawData, err := m.GetRawAtIndex(ctx, ticker, pkg, category, index)
    if err != nil {
        return nil, err
    }

    var gex GexData
    if err := json.Unmarshal(rawData, &gex); err != nil {
        return nil, fmt.Errorf("unmarshal error: %w", err)
    }
    return &gex, nil
}
```

**Consolidation notes:** Consider a default implementation via embedding or a shared helper function.

### 3.3 Duplicated Directory Walking Logic (2 instances, 85% identical)

| File | Lines |
|------|-------|
| `memory.go` | 26-71 |
| `stream.go` | 37-86 |

Both constructors have identical:
- `dateDir` computation
- `filepath.Walk` with identical filtering
- Path parsing to extract ticker/pkg/category
- Key generation using `DataKey()`

**Consolidation notes:** Create a shared `walkDataDir(dataDir, date string, callback func(key, path string) error) error` function.

### 3.4 Similar `GetLoadedKeys` Method (2 instances, 90% identical)

| File | Lines |
|------|-------|
| `memory.go` | 152-159 |
| `stream.go` | 201-210 |

### 3.5 Similar `FindIndexByTimestamp` Binary Search (2 instances, 75% identical)

| File | Lines |
|------|-------|
| `memory.go` | 161-187 |
| `stream.go` | 227-267 |

### 3.6 Suffix-Based Key Matching in Cache (2 instances, 95% identical)

**File:** `internal/data/cache.go`

| Function | Lines |
|----------|-------|
| `Reset` | 86-94 |
| `GetPositionsByAPIKey` | 117-125 |

```go
suffix := "/" + apiKey
for k := range c.indexes {
    if len(k) > len(suffix) && k[len(k)-len(suffix):] == suffix {
        // action differs: delete vs collect
    }
}
```

**Consolidation notes:** Create `filterKeysBySuffix(suffix string, action func(key string))` helper.

---

## 4. Command Entry Points (Medium Priority)

**Files:** `cmd/downloader/`, `cmd/daemon/`

### 4.1 Identical API Client Creation (100% identical, 9 lines each)

| File | Lines |
|------|-------|
| `downloader/download.go` | 88-96 |
| `daemon/download.go` | 59-67 |

```go
client := api.NewClient(
    cfg.API.BaseURL,
    cfg.API.APIKey,
    cfg.Download.RatePerSecond,
    time.Duration(cfg.API.TimeoutSec)*time.Second,
    time.Duration(cfg.API.RetryDelay)*time.Second,
    cfg.API.RetryCount,
    logger,
)
```

### 4.2 Identical Download Manager Setup (100% identical, 5 lines each)

| File | Lines |
|------|-------|
| `downloader/download.go` | 98-102 |
| `daemon/download.go` | 69-73 |

```go
stgMgr := staging.NewManager(cfg.Output.Directory)
dlMgr := download.NewManager(client, stgMgr, cfg.Download.Workers, logger)
```

### 4.3 Nearly Identical `convertJSONToJSONL` Function (~95% identical)

| File | Lines |
|------|-------|
| `downloader/convert.go` | 39-98 |
| `daemon/download.go` | 178-234 |

Differences: daemon takes logger as parameter, minor log level differences.

### 4.4 Nearly Identical `convertFile` Function (~90% identical)

| File | Lines |
|------|-------|
| `downloader/convert.go` | 100-137 |
| `daemon/download.go` | 237-274 |

Differences: downloader wraps errors with context, daemon returns raw errors.

### 4.5 Similar Task Generation Logic (High similarity)

| File | Function | Lines |
|------|----------|-------|
| `downloader/helpers.go` | `generateTasks` | 44-131 |
| `daemon/download.go` | `generateTasksForDate` | 127-176 |

### 4.6 Similar Market Day Filtering

| File | Function | Lines |
|------|----------|-------|
| `downloader/helpers.go` | `filterMarketDays` | 133-157 |
| `daemon/scheduler.go` | `IsMarketDay` | 42-50 |

**Consolidation notes:** Create a shared `internal/shared/` or `internal/common/` package with:
- `createAPIClient(cfg, logger)`
- `createDownloadManager(client, cfg, logger)`
- `convertJSONToJSONL(dir, logger)`
- `convertFile(jsonPath, jsonlPath)`
- `generateTasks(cfg, dates)`
- `isMarketDay(date, location, calendar)`

---

## 5. Utility Functions (Low Priority)

### 5.1 `getEnvOrDefault` - 3 Exact Duplicates

| File | Lines |
|------|-------|
| `internal/notify/config.go` | 52-57 |
| `internal/config/server.go` | 132-137 |
| `cmd/daemon/config.go` | 30-35 |

```go
func getEnvOrDefault(key, defaultVal string) string {
    if val := os.Getenv(key); val != "" {
        return val
    }
    return defaultVal
}
```

### 5.2 `getEnvBoolOrDefault` - 2 Exact Duplicates

| File | Lines |
|------|-------|
| `internal/notify/config.go` | 59-66 |
| `cmd/daemon/config.go` | 46-53 |

```go
func getEnvBoolOrDefault(key string, defaultVal bool) bool {
    if val := os.Getenv(key); val != "" {
        if b, err := strconv.ParseBool(val); err == nil {
            return b
        }
    }
    return defaultVal
}
```

**Consolidation notes:** Create `internal/envutil/env.go` with these helpers.

### 5.3 Enum Validation Pattern in server.go (3 instances)

**File:** `internal/config/server.go:84-92`

```go
if cfg.DataMode != "memory" && cfg.DataMode != "stream" {
    return nil, fmt.Errorf("invalid DATA_MODE: %s (must be 'memory' or 'stream')", cfg.DataMode)
}
```

**Consolidation notes:** Create `validateEnum(name, value string, allowed []string) error`.

### 5.4 Duration Parsing Pattern in server.go (2 instances)

**Lines:** 42-47, 49-54

```go
wsIntervalStr := getEnvOrDefault("WS_STREAM_INTERVAL", "1s")
wsInterval, err := time.ParseDuration(wsIntervalStr)
if err != nil {
    wsInterval = time.Second
}
```

**Consolidation notes:** Create `getDurationEnv(key, defaultVal string) time.Duration`.

---

## 6. Sync Package (Low Priority)

**File:** `internal/sync/types.go`, `broadcaster.go`

### 6.1 Identical Struct Definitions

`SyncBatch` (lines 4-11) and `SyncSnapshot` (lines 24-31) have identical fields.

### 6.2 Nearly Identical Build Functions

`buildSnapshot()` (lines 166-182) and `buildBatch()` (lines 184-200) differ only in return type.

**Consolidation notes:** Consider type aliasing or a single generic builder if the semantic distinction is not critical.

---

## Prioritized Recommendations

### High Priority (Immediate Impact)

1. **Consolidate WebSocket streamers** - Create a generic `BaseStreamer` with configurable parameters. This would eliminate ~850 lines of duplicate code.

2. **Create handler helpers** - Abstract repeated error handling patterns in `handlers.go` into helper functions. This would reduce ~350 lines and improve maintainability.

3. **Unify cmd/ shared code** - Move `convertFile`, `convertJSONToJSONL`, and API client creation to a shared package.

### Medium Priority

4. **Consolidate data layer** - Move `extractTimestamp` and directory walking logic to shared utilities.

5. **Create envutil package** - Centralize `getEnvOrDefault` and related functions.

### Low Priority

6. **Sync package cleanup** - Consider whether `SyncBatch` and `SyncSnapshot` need to remain separate types.

7. **Validation helpers** - Create generic enum and duration parsing helpers.

---

## Summary Statistics

| Area | Duplicated Lines | Files Affected | Priority |
|------|-----------------|----------------|----------|
| WebSocket streamers | ~850 | 5 | High |
| REST handlers | ~350-400 | 3 | High |
| cmd/ entry points | ~300+ | 7 | Medium |
| Data layer | ~150 | 4 | Medium |
| Utilities | ~50+ | 5 | Low |
| **Total** | **~1,500+** | - | - |
