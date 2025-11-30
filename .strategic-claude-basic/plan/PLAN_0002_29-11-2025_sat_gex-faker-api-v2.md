# GEX Faker API v2 Implementation Plan

## Overview

Build a clean, spec-first GEX Faker API server using oapi-codegen. The new implementation replaces a 1775-line monolith with a well-organized package structure, serves JSONL data files with per-API-key sequential playback, and provides automatic request validation via OpenAPI middleware.

## Current State Analysis

### Existing gex_faker (to be replaced)
- **Location:** `/home/dgnsrekt/Development/GEXBOT_RESEARCH/gex_faker/cmd/server/main.go`
- **Issues:** 1775-line monolith, swag-based (code→spec), 15+ endpoints
- **Data Format:** JSON arrays loaded entirely into memory

### New gexbot-faker-api
- **Location:** This project (`/home/dgnsrekt/Development/GEXBOT_RESEARCH/gexbot-faker-api`)
- **Data Available:** JSONL files at `data/{DATE}/{TICKER}/{PACKAGE}/{CATEGORY}.jsonl`
- **Example:** `data/2025-11-28/SPX/state/gex_full.jsonl` (52MB, ~23K lines)

### Key Discoveries:
- cerberus_gamma uses oapi-codegen with `strict-server: true` for typed request/response
- Chi router with `oapimiddleware.OapiRequestValidator()` for automatic validation
- GexData has 15 fields, uses `json.RawMessage` for `strikes` and `max_priors`
- Per-API-key index tracking uses composite key: `{ticker}/{pkg}/{category}/{apiKey}`

## Desired End State

A running API server at `localhost:8080` that:
1. Serves GEX data from JSONL files with sequential playback per API key
2. Provides Swagger UI at `/docs` with interactive API documentation
3. Validates all requests against OpenAPI spec automatically
4. Supports both memory and streaming data loading modes via `DATA_MODE` env var
5. Handles cache modes: `exhaust` (404 at end) and `rotation` (wrap around)

### Verification:
```bash
# Health check
curl http://localhost:8080/health

# Get GEX data (advances index per API key)
curl "http://localhost:8080/SPX/state/gex_full?key=test"

# Reset cache positions
curl -X POST http://localhost:8080/reset-cache

# View Swagger docs
open http://localhost:8080/docs
```

## What We're NOT Doing

- **Hot reload:** No `/load-date` endpoint for switching dates at runtime
- **Intraday data:** No separate intraday data endpoints
- **Set-index:** No manual index positioning endpoint (simplicity)
- **File downloads:** No `/download/` endpoints for full file downloads
- **Secondary tickers:** Only primary tickers; no tier-based loading complexity

## Implementation Approach

Use oapi-codegen's spec-first approach:
1. Define OpenAPI spec → generate Go code → implement handlers
2. Chi router with validation middleware for automatic request validation
3. DataLoader interface for swappable memory/stream implementations
4. Separate cache service for per-API-key index tracking

---

## Phase 1: Project Foundation & OpenAPI Spec

### Overview
Set up project structure, dependencies, and define the OpenAPI specification.

### Changes Required:

#### 1. Go Module Dependencies

**File:** `go.mod`
**Changes:** Add oapi-codegen and chi dependencies

```go
require (
    github.com/go-chi/chi/v5 v5.2.0
    github.com/oapi-codegen/oapi-codegen/v2 v2.5.0
    github.com/oapi-codegen/runtime v1.1.2
    github.com/oapi-codegen/nethttp-middleware v1.1.2
    github.com/getkin/kin-openapi v0.132.0
    go.uber.org/zap v1.27.0  // Already present
)
```

#### 2. OpenAPI Specification

**File:** `api/openapi.yaml`
**Changes:** Create OpenAPI 3.0.3 spec defining all endpoints and schemas

```yaml
openapi: 3.0.3
info:
  title: GEX Faker API
  version: 2.0.0
  description: |
    Historical GEX data server with per-API-key sequential playback.
    Serves JSONL data files with configurable cache modes.

servers:
  - url: http://localhost:8080
    description: Local development server

paths:
  /{ticker}/{package}/{category}:
    get:
      operationId: getGexData
      summary: Get next GEX data point
      description: Returns the next GEX snapshot for this API key's playback position
      tags: [playback]
      parameters:
        - name: ticker
          in: path
          required: true
          schema:
            type: string
            pattern: '^[A-Z]{1,5}$'
          example: SPX
        - name: package
          in: path
          required: true
          schema:
            type: string
            enum: [state, classic]
          example: state
        - name: category
          in: path
          required: true
          schema:
            type: string
            enum: [gex_full, gex_zero, gex_one]
          example: gex_full
        - name: key
          in: query
          required: true
          description: API key for playback position tracking
          schema:
            type: string
            minLength: 1
          example: test
      responses:
        '200':
          description: GEX data point
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/GexData'
        '400':
          description: Invalid request parameters
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '401':
          description: Missing API key
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '404':
          description: Data not found or exhausted
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'

  /tickers:
    get:
      operationId: getTickers
      summary: List available tickers
      tags: [info]
      responses:
        '200':
          description: Available tickers
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/TickersResponse'

  /health:
    get:
      operationId: getHealth
      summary: Health check
      tags: [info]
      responses:
        '200':
          description: Server is healthy
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/HealthResponse'

  /reset-cache:
    post:
      operationId: resetCache
      summary: Reset playback positions
      description: Reset all or specific API key playback positions to index 0
      tags: [admin]
      parameters:
        - name: key
          in: query
          required: false
          description: Reset only this API key (omit for all)
          schema:
            type: string
      responses:
        '200':
          description: Cache reset successful
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ResetCacheResponse'

components:
  schemas:
    GexData:
      type: object
      required:
        - timestamp
        - ticker
      properties:
        timestamp:
          type: integer
          format: int64
          example: 1764340202
        ticker:
          type: string
          example: SPX
        min_dte:
          type: integer
          example: 0
        sec_min_dte:
          type: integer
          example: 3
        spot:
          type: number
          format: double
          example: 6822.95
        zero_gamma:
          type: number
          format: double
          example: 0
        major_pos_vol:
          type: number
          format: double
          example: 0
        major_pos_oi:
          type: number
          format: double
          example: 0
        major_neg_vol:
          type: number
          format: double
          example: 0
        major_neg_oi:
          type: number
          format: double
          example: 0
        strikes:
          type: array
          items: {}
        sum_gex_vol:
          type: number
          format: double
          example: 0
        sum_gex_oi:
          type: number
          format: double
          example: 43565.23
        delta_risk_reversal:
          type: number
          format: double
          example: 0
        max_priors:
          type: array
          items: {}

    TickersResponse:
      type: object
      properties:
        tickers:
          type: array
          items:
            $ref: '#/components/schemas/TickerInfo'
        count:
          type: integer
          example: 6

    TickerInfo:
      type: object
      properties:
        symbol:
          type: string
          example: SPX
        type:
          type: string
          enum: [index, stock]
          example: index

    HealthResponse:
      type: object
      properties:
        status:
          type: string
          example: ok
        data_date:
          type: string
          example: "2025-11-28"
        data_mode:
          type: string
          enum: [memory, stream]
          example: memory
        cache_mode:
          type: string
          enum: [exhaust, rotation]
          example: exhaust

    ResetCacheResponse:
      type: object
      properties:
        status:
          type: string
          example: success
        message:
          type: string
          example: All cache positions reset to index 0
        count:
          type: integer
          description: Number of positions reset
          example: 12

    ErrorResponse:
      type: object
      properties:
        error:
          type: string
          example: Invalid ticker symbol
```

#### 3. Code Generation Config

**File:** `api/oapi-codegen.yaml`
**Changes:** Configure oapi-codegen for strict server generation

```yaml
package: generated
output: ../internal/api/generated/server.gen.go

generate:
  models: true
  chi-server: true
  strict-server: true
  embedded-spec: true

output-options:
  skip-prune: true
```

#### 4. Generation Directive

**File:** `api/generate.go`
**Changes:** Add go:generate directive

```go
package api

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config oapi-codegen.yaml openapi.yaml
```

#### 5. Embedded Spec

**File:** `api/openapi.go`
**Changes:** Embed OpenAPI spec for serving

```go
package api

import _ "embed"

//go:embed openapi.yaml
var OpenAPISpec []byte
```

#### 6. Tools File

**File:** `tools/tools.go`
**Changes:** Pin oapi-codegen version

```go
//go:build tools

package tools

import (
    _ "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen"
)
```

### Success Criteria:

#### Automated Verification:
- [ ] `go mod tidy` completes without errors
- [ ] `go generate ./api` generates `internal/api/generated/server.gen.go`
- [ ] Generated file contains `StrictServerInterface` with 4 methods
- [ ] `go build ./...` succeeds

#### Manual Verification:
- [ ] OpenAPI spec validates (use online validator)
- [ ] Generated interface matches expected method signatures

---

## Phase 2: Configuration & Data Layer

### Overview
Implement configuration loading and the DataLoader interface with memory implementation.

### Changes Required:

#### 1. Configuration

**File:** `internal/config/server.go`
**Changes:** Server configuration with env var loading

```go
package config

import (
    "fmt"
    "os"
)

type ServerConfig struct {
    Port      string
    DataDir   string
    DataDate  string
    DataMode  string // "memory" or "stream"
    CacheMode string // "exhaust" or "rotation"
}

func LoadServerConfig() (*ServerConfig, error) {
    cfg := &ServerConfig{
        Port:      getEnvOrDefault("PORT", "8080"),
        DataDir:   getEnvOrDefault("DATA_DIR", "./data"),
        DataDate:  getEnvOrDefault("DATA_DATE", "2025-11-28"),
        DataMode:  getEnvOrDefault("DATA_MODE", "memory"),
        CacheMode: getEnvOrDefault("CACHE_MODE", "exhaust"),
    }

    // Validate
    if cfg.DataMode != "memory" && cfg.DataMode != "stream" {
        return nil, fmt.Errorf("invalid DATA_MODE: %s (must be 'memory' or 'stream')", cfg.DataMode)
    }
    if cfg.CacheMode != "exhaust" && cfg.CacheMode != "rotation" {
        return nil, fmt.Errorf("invalid CACHE_MODE: %s (must be 'exhaust' or 'rotation')", cfg.CacheMode)
    }

    return cfg, nil
}

func getEnvOrDefault(key, defaultVal string) string {
    if val := os.Getenv(key); val != "" {
        return val
    }
    return defaultVal
}
```

#### 2. GexData Model

**File:** `internal/data/models.go`
**Changes:** GexData struct matching API response

```go
package data

import "encoding/json"

type GexData struct {
    Timestamp         int64           `json:"timestamp"`
    Ticker            string          `json:"ticker"`
    MinDTE            int             `json:"min_dte"`
    SecMinDTE         int             `json:"sec_min_dte"`
    Spot              float64         `json:"spot"`
    ZeroGamma         float64         `json:"zero_gamma"`
    MajorPosVol       float64         `json:"major_pos_vol"`
    MajorPosOI        float64         `json:"major_pos_oi"`
    MajorNegVol       float64         `json:"major_neg_vol"`
    MajorNegOI        float64         `json:"major_neg_oi"`
    Strikes           json.RawMessage `json:"strikes"`
    SumGexVol         float64         `json:"sum_gex_vol"`
    SumGexOI          float64         `json:"sum_gex_oi"`
    DeltaRiskReversal float64         `json:"delta_risk_reversal"`
    MaxPriors         json.RawMessage `json:"max_priors"`
}
```

#### 3. DataLoader Interface

**File:** `internal/data/loader.go`
**Changes:** Interface for data access

```go
package data

import (
    "context"
    "errors"
)

var (
    ErrNotFound     = errors.New("data not found")
    ErrIndexOutOfBounds = errors.New("index out of bounds")
)

// DataLoader provides random access to GEX data
type DataLoader interface {
    // GetAtIndex returns the GexData at the given index
    GetAtIndex(ctx context.Context, ticker, pkg, category string, index int) (*GexData, error)

    // GetLength returns the number of data points available
    GetLength(ticker, pkg, category string) (int, error)

    // Exists checks if data exists for the given combination
    Exists(ticker, pkg, category string) bool

    // Close releases any resources
    Close() error
}

// DataKey creates a unique key for ticker/package/category
func DataKey(ticker, pkg, category string) string {
    return ticker + "/" + pkg + "/" + category
}
```

#### 4. Memory Loader Implementation

**File:** `internal/data/memory.go`
**Changes:** In-memory JSONL loading

```go
package data

import (
    "bufio"
    "context"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"

    "go.uber.org/zap"
)

type MemoryLoader struct {
    data   map[string][]GexData // key: ticker/pkg/category
    logger *zap.Logger
}

func NewMemoryLoader(dataDir, date string, logger *zap.Logger) (*MemoryLoader, error) {
    loader := &MemoryLoader{
        data:   make(map[string][]GexData),
        logger: logger,
    }

    dateDir := filepath.Join(dataDir, date)

    // Walk the date directory
    err := filepath.Walk(dateDir, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        if info.IsDir() || filepath.Ext(path) != ".jsonl" {
            return nil
        }

        // Extract ticker/pkg/category from path
        // Format: data/{date}/{ticker}/{pkg}/{category}.jsonl
        rel, _ := filepath.Rel(dateDir, path)
        // rel = "SPX/state/gex_full.jsonl"

        ticker := filepath.Dir(filepath.Dir(rel))
        pkg := filepath.Base(filepath.Dir(rel))
        category := filepath.Base(rel)
        category = category[:len(category)-6] // Remove .jsonl

        key := DataKey(ticker, pkg, category)

        data, err := loader.loadJSONL(path)
        if err != nil {
            logger.Warn("failed to load file", zap.String("path", path), zap.Error(err))
            return nil
        }

        loader.data[key] = data
        logger.Info("loaded data",
            zap.String("key", key),
            zap.Int("count", len(data)),
        )
        return nil
    })

    if err != nil {
        return nil, fmt.Errorf("walking data directory: %w", err)
    }

    if len(loader.data) == 0 {
        return nil, fmt.Errorf("no JSONL files found in %s", dateDir)
    }

    return loader, nil
}

func (m *MemoryLoader) loadJSONL(path string) ([]GexData, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    var data []GexData
    scanner := bufio.NewScanner(file)

    // Increase buffer size for large lines
    buf := make([]byte, 0, 64*1024)
    scanner.Buffer(buf, 1024*1024)

    lineNum := 0
    for scanner.Scan() {
        lineNum++
        line := scanner.Bytes()
        if len(line) == 0 {
            continue
        }

        var gex GexData
        if err := json.Unmarshal(line, &gex); err != nil {
            return nil, fmt.Errorf("line %d: %w", lineNum, err)
        }
        data = append(data, gex)
    }

    if err := scanner.Err(); err != nil {
        return nil, err
    }

    return data, nil
}

func (m *MemoryLoader) GetAtIndex(ctx context.Context, ticker, pkg, category string, index int) (*GexData, error) {
    key := DataKey(ticker, pkg, category)
    data, ok := m.data[key]
    if !ok {
        return nil, ErrNotFound
    }
    if index < 0 || index >= len(data) {
        return nil, ErrIndexOutOfBounds
    }
    return &data[index], nil
}

func (m *MemoryLoader) GetLength(ticker, pkg, category string) (int, error) {
    key := DataKey(ticker, pkg, category)
    data, ok := m.data[key]
    if !ok {
        return 0, ErrNotFound
    }
    return len(data), nil
}

func (m *MemoryLoader) Exists(ticker, pkg, category string) bool {
    key := DataKey(ticker, pkg, category)
    _, ok := m.data[key]
    return ok
}

func (m *MemoryLoader) Close() error {
    m.data = nil
    return nil
}

// GetLoadedKeys returns all loaded data keys (for /tickers endpoint)
func (m *MemoryLoader) GetLoadedKeys() []string {
    keys := make([]string, 0, len(m.data))
    for k := range m.data {
        keys = append(keys, k)
    }
    return keys
}
```

#### 5. Index Cache Service

**File:** `internal/data/cache.go`
**Changes:** Per-API-key index tracking

```go
package data

import "sync"

// CacheMode defines how playback handles end-of-data
type CacheMode string

const (
    CacheModeExhaust  CacheMode = "exhaust"  // 404 at end
    CacheModeRotation CacheMode = "rotation" // wrap to 0
)

// IndexCache tracks playback positions per API key
type IndexCache struct {
    mu      sync.RWMutex
    indexes map[string]int // key: ticker/pkg/category/apiKey
    mode    CacheMode
}

func NewIndexCache(mode CacheMode) *IndexCache {
    return &IndexCache{
        indexes: make(map[string]int),
        mode:    mode,
    }
}

// CacheKey creates the composite key for index tracking
func CacheKey(ticker, pkg, category, apiKey string) string {
    return ticker + "/" + pkg + "/" + category + "/" + apiKey
}

// GetAndAdvance returns the current index and advances it
// Returns (index, isExhausted)
func (c *IndexCache) GetAndAdvance(key string, dataLength int) (int, bool) {
    c.mu.Lock()
    defer c.mu.Unlock()

    idx := c.indexes[key]

    // Check exhaustion in exhaust mode
    if c.mode == CacheModeExhaust && idx >= dataLength {
        return idx, true
    }

    // Get current index (may need wrap in rotation mode)
    currentIdx := idx
    if c.mode == CacheModeRotation && idx >= dataLength {
        currentIdx = idx % dataLength
    }

    // Advance for next request
    if c.mode == CacheModeRotation {
        c.indexes[key] = (idx + 1) % dataLength
    } else {
        c.indexes[key] = idx + 1
    }

    return currentIdx, false
}

// Reset resets indexes, optionally for a specific API key pattern
func (c *IndexCache) Reset(apiKey string) int {
    c.mu.Lock()
    defer c.mu.Unlock()

    if apiKey == "" {
        // Reset all
        count := len(c.indexes)
        c.indexes = make(map[string]int)
        return count
    }

    // Reset matching keys (ending with /apiKey)
    suffix := "/" + apiKey
    count := 0
    for k := range c.indexes {
        if len(k) > len(suffix) && k[len(k)-len(suffix):] == suffix {
            delete(c.indexes, k)
            count++
        }
    }
    return count
}

// GetIndex returns current index without advancing (for debugging)
func (c *IndexCache) GetIndex(key string) int {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.indexes[key]
}
```

### Success Criteria:

#### Automated Verification:
- [ ] `go build ./internal/config` succeeds
- [ ] `go build ./internal/data` succeeds
- [ ] `go test ./internal/data/...` passes (after adding tests)

#### Manual Verification:
- [ ] MemoryLoader successfully loads sample JSONL files
- [ ] IndexCache correctly tracks per-key indexes

---

## Phase 3: Server Implementation

### Overview
Implement the HTTP server with handlers that satisfy the generated StrictServerInterface.

### Changes Required:

#### 1. Server Handler

**File:** `internal/server/handlers.go`
**Changes:** Implement StrictServerInterface

```go
package server

import (
    "context"
    "errors"
    "strings"

    "go.uber.org/zap"

    "github.com/dgnsrekt/gexbot-downloader/internal/api/generated"
    "github.com/dgnsrekt/gexbot-downloader/internal/config"
    "github.com/dgnsrekt/gexbot-downloader/internal/data"
)

type Server struct {
    loader *data.MemoryLoader
    cache  *data.IndexCache
    config *config.ServerConfig
    logger *zap.Logger
}

func NewServer(loader *data.MemoryLoader, cache *data.IndexCache, cfg *config.ServerConfig, logger *zap.Logger) *Server {
    return &Server{
        loader: loader,
        cache:  cache,
        config: cfg,
        logger: logger,
    }
}

// Compile-time interface verification
var _ generated.StrictServerInterface = (*Server)(nil)

// GetGexData implements generated.StrictServerInterface
func (s *Server) GetGexData(ctx context.Context, request generated.GetGexDataRequestObject) (generated.GetGexDataResponseObject, error) {
    ticker := request.Ticker
    pkg := request.Package
    category := request.Category
    apiKey := request.Params.Key

    s.logger.Debug("gex data request",
        zap.String("ticker", ticker),
        zap.String("package", pkg),
        zap.String("category", category),
        zap.String("apiKey", apiKey),
    )

    // Check if data exists
    if !s.loader.Exists(ticker, pkg, category) {
        return generated.GetGexData404JSONResponse{
            Error: ptr("Data not found for " + ticker + "/" + pkg + "/" + category),
        }, nil
    }

    // Get data length
    length, err := s.loader.GetLength(ticker, pkg, category)
    if err != nil {
        return generated.GetGexData404JSONResponse{
            Error: ptr(err.Error()),
        }, nil
    }

    // Get index and check exhaustion
    cacheKey := data.CacheKey(ticker, pkg, category, apiKey)
    idx, exhausted := s.cache.GetAndAdvance(cacheKey, length)

    if exhausted {
        s.logger.Debug("data exhausted",
            zap.String("cacheKey", cacheKey),
            zap.Int("index", idx),
            zap.Int("length", length),
        )
        return generated.GetGexData404JSONResponse{
            Error: ptr("No more data available"),
        }, nil
    }

    // Get data at index
    gexData, err := s.loader.GetAtIndex(ctx, ticker, pkg, category, idx)
    if err != nil {
        if errors.Is(err, data.ErrIndexOutOfBounds) {
            return generated.GetGexData404JSONResponse{
                Error: ptr("Index out of bounds"),
            }, nil
        }
        return generated.GetGexData404JSONResponse{
            Error: ptr(err.Error()),
        }, nil
    }

    s.logger.Debug("returning data",
        zap.String("cacheKey", cacheKey),
        zap.Int("index", idx),
        zap.Int64("timestamp", gexData.Timestamp),
    )

    return generated.GetGexData200JSONResponse{
        Timestamp:         &gexData.Timestamp,
        Ticker:            &gexData.Ticker,
        MinDte:            &gexData.MinDTE,
        SecMinDte:         &gexData.SecMinDTE,
        Spot:              &gexData.Spot,
        ZeroGamma:         &gexData.ZeroGamma,
        MajorPosVol:       &gexData.MajorPosVol,
        MajorPosOi:        &gexData.MajorPosOI,
        MajorNegVol:       &gexData.MajorNegVol,
        MajorNegOi:        &gexData.MajorNegOI,
        Strikes:           &gexData.Strikes,
        SumGexVol:         &gexData.SumGexVol,
        SumGexOi:          &gexData.SumGexOI,
        DeltaRiskReversal: &gexData.DeltaRiskReversal,
        MaxPriors:         &gexData.MaxPriors,
    }, nil
}

// GetTickers implements generated.StrictServerInterface
func (s *Server) GetTickers(ctx context.Context, request generated.GetTickersRequestObject) (generated.GetTickersResponseObject, error) {
    keys := s.loader.GetLoadedKeys()

    // Extract unique tickers
    tickerSet := make(map[string]bool)
    for _, key := range keys {
        parts := strings.Split(key, "/")
        if len(parts) >= 1 {
            tickerSet[parts[0]] = true
        }
    }

    tickers := make([]generated.TickerInfo, 0, len(tickerSet))
    for ticker := range tickerSet {
        tickerType := "stock"
        if ticker == "SPX" || ticker == "NDX" || ticker == "RUT" || ticker == "VIX" {
            tickerType = "index"
        }
        tickers = append(tickers, generated.TickerInfo{
            Symbol: &ticker,
            Type:   (*generated.TickerInfoType)(&tickerType),
        })
    }

    count := len(tickers)
    return generated.GetTickers200JSONResponse{
        Tickers: &tickers,
        Count:   &count,
    }, nil
}

// GetHealth implements generated.StrictServerInterface
func (s *Server) GetHealth(ctx context.Context, request generated.GetHealthRequestObject) (generated.GetHealthResponseObject, error) {
    status := "ok"
    return generated.GetHealth200JSONResponse{
        Status:    &status,
        DataDate:  &s.config.DataDate,
        DataMode:  (*generated.HealthResponseDataMode)(&s.config.DataMode),
        CacheMode: (*generated.HealthResponseCacheMode)(&s.config.CacheMode),
    }, nil
}

// ResetCache implements generated.StrictServerInterface
func (s *Server) ResetCache(ctx context.Context, request generated.ResetCacheRequestObject) (generated.ResetCacheResponseObject, error) {
    apiKey := ""
    if request.Params.Key != nil {
        apiKey = *request.Params.Key
    }

    count := s.cache.Reset(apiKey)

    status := "success"
    message := "All cache positions reset to index 0"
    if apiKey != "" {
        message = "Cache positions reset for key: " + apiKey
    }

    s.logger.Info("cache reset",
        zap.String("apiKey", apiKey),
        zap.Int("count", count),
    )

    return generated.ResetCache200JSONResponse{
        Status:  &status,
        Message: &message,
        Count:   &count,
    }, nil
}

func ptr[T any](v T) *T { return &v }
```

#### 2. Server Setup

**File:** `internal/server/server.go`
**Changes:** Chi router with middleware

```go
package server

import (
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    oapimiddleware "github.com/oapi-codegen/nethttp-middleware"
    "go.uber.org/zap"

    "github.com/dgnsrekt/gexbot-downloader/api"
    "github.com/dgnsrekt/gexbot-downloader/internal/api/generated"
)

func NewRouter(server *Server, logger *zap.Logger) (http.Handler, error) {
    // Load OpenAPI spec for validation
    swagger, err := generated.GetSwagger()
    if err != nil {
        return nil, err
    }
    swagger.Servers = nil // Allow any host

    r := chi.NewRouter()

    // Global middleware
    r.Use(middleware.RequestID)
    r.Use(middleware.RealIP)
    r.Use(middleware.Recoverer)
    r.Use(middleware.Compress(5))
    r.Use(corsMiddleware)
    r.Use(zapLoggerMiddleware(logger))

    // Non-validated routes
    r.Get("/openapi.yaml", openapiHandler)
    r.Get("/docs", swaggerUIHandler)

    // API routes with OpenAPI validation
    r.Group(func(apiRouter chi.Router) {
        apiRouter.Use(oapimiddleware.OapiRequestValidator(swagger))

        strictHandler := generated.NewStrictHandler(server, nil)
        generated.HandlerFromMux(strictHandler, apiRouter)
    })

    return r, nil
}

func corsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "*")

        if r.Method == "OPTIONS" {
            w.WriteHeader(http.StatusOK)
            return
        }

        next.ServeHTTP(w, r)
    })
}

func zapLoggerMiddleware(logger *zap.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            logger.Debug("request",
                zap.String("method", r.Method),
                zap.String("path", r.URL.Path),
                zap.String("query", r.URL.RawQuery),
            )
            next.ServeHTTP(w, r)
        })
    }
}

func openapiHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/yaml")
    w.Write(api.OpenAPISpec)
}

func swaggerUIHandler(w http.ResponseWriter, r *http.Request) {
    html := `<!DOCTYPE html>
<html>
<head>
    <title>GEX Faker API</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.10.3/swagger-ui.css">
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5.10.3/swagger-ui-bundle.js"></script>
    <script>
        window.onload = function() {
            SwaggerUIBundle({
                url: "/openapi.yaml",
                dom_id: '#swagger-ui',
            });
        };
    </script>
</body>
</html>`
    w.Header().Set("Content-Type", "text/html")
    w.Write([]byte(html))
}
```

#### 3. Main Entry Point

**File:** `cmd/server/main.go`
**Changes:** Wire everything together

```go
package main

import (
    "fmt"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "go.uber.org/zap"

    "github.com/dgnsrekt/gexbot-downloader/internal/config"
    "github.com/dgnsrekt/gexbot-downloader/internal/data"
    "github.com/dgnsrekt/gexbot-downloader/internal/server"
)

func main() {
    os.Exit(run())
}

func run() int {
    // Setup logger
    logger, err := zap.NewDevelopment()
    if err != nil {
        fmt.Fprintf(os.Stderr, "failed to create logger: %v\n", err)
        return 1
    }
    defer logger.Sync()

    // Load config
    cfg, err := config.LoadServerConfig()
    if err != nil {
        logger.Error("failed to load config", zap.Error(err))
        return 1
    }

    logger.Info("configuration loaded",
        zap.String("port", cfg.Port),
        zap.String("dataDir", cfg.DataDir),
        zap.String("dataDate", cfg.DataDate),
        zap.String("dataMode", cfg.DataMode),
        zap.String("cacheMode", cfg.CacheMode),
    )

    // Load data
    logger.Info("loading data...")
    start := time.Now()

    loader, err := data.NewMemoryLoader(cfg.DataDir, cfg.DataDate, logger)
    if err != nil {
        logger.Error("failed to load data", zap.Error(err))
        return 1
    }
    defer loader.Close()

    logger.Info("data loaded", zap.Duration("duration", time.Since(start)))

    // Create index cache
    cacheMode := data.CacheModeExhaust
    if cfg.CacheMode == "rotation" {
        cacheMode = data.CacheModeRotation
    }
    cache := data.NewIndexCache(cacheMode)

    // Create server
    srv := server.NewServer(loader, cache, cfg, logger)

    // Create router
    router, err := server.NewRouter(srv, logger)
    if err != nil {
        logger.Error("failed to create router", zap.Error(err))
        return 1
    }

    // Setup HTTP server
    httpServer := &http.Server{
        Addr:         ":" + cfg.Port,
        Handler:      router,
        ReadTimeout:  30 * time.Second,
        WriteTimeout: 30 * time.Second,
    }

    // Start server in goroutine
    go func() {
        logger.Info("starting server", zap.String("addr", httpServer.Addr))
        if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            logger.Error("server error", zap.Error(err))
        }
    }()

    // Wait for interrupt
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    logger.Info("shutting down server...")
    return 0
}
```

### Success Criteria:

#### Automated Verification:
- [ ] `go build ./cmd/server` succeeds
- [ ] `go test ./internal/server/...` passes
- [ ] Server starts without errors: `go run ./cmd/server`
- [ ] Health endpoint returns 200: `curl http://localhost:8080/health`

#### Manual Verification:
- [ ] Swagger UI accessible at `http://localhost:8080/docs`
- [ ] GEX data endpoint returns data with sequential playback
- [ ] Reset cache endpoint works
- [ ] Invalid requests return proper validation errors

---

## Phase 4: Justfile & Integration

### Overview
Add justfile recipes and finalize integration.

### Changes Required:

#### 1. Justfile Updates

**File:** `justfile`
**Changes:** Add server recipes

```just
# --- Server Commands ---

# Build the server binary
build-server:
    go build -o bin/gexbot-server ./cmd/server

# Run the server (development)
serve: build-server
    ./bin/gexbot-server

# Generate API code from OpenAPI spec
generate:
    go generate ./api

# Run all tests
test:
    go test -v ./...
```

#### 2. Example Environment File

**File:** `example.server.env`
**Changes:** Document server environment variables

```env
# Server Configuration
PORT=8080

# Data Configuration
DATA_DIR=./data
DATA_DATE=2025-11-28
DATA_MODE=memory    # memory | stream

# Cache Configuration
CACHE_MODE=exhaust  # exhaust | rotation
```

### Success Criteria:

#### Automated Verification:
- [ ] `just generate` regenerates API code
- [ ] `just build-server` creates binary
- [ ] `just serve` starts server
- [ ] `just test` passes all tests

#### Manual Verification:
- [ ] Complete workflow: generate → build → serve → test endpoints
- [ ] Sequential playback works correctly per API key
- [ ] Cache modes work as expected

---

## Performance Considerations

- **Memory Mode:** ~50-100MB per JSONL file loaded into memory
- **Large datasets:** Consider streaming mode for production with many dates
- **Mutex contention:** Current simple mutex is fine for moderate load; sharded locks for high concurrency

## Migration Notes

Not applicable - this is a new implementation alongside existing gex_faker.

## References

- Existing gex_faker: `/home/dgnsrekt/Development/GEXBOT_RESEARCH/gex_faker/cmd/server/main.go`
- cerberus_gamma oapi-codegen pattern: `/home/dgnsrekt/Development/CERBERUS_GAMMA/cerberus_gamma/api/`
- Existing downloader config: `internal/config/config.go:1-50`

## Codex Mentorship Summary

Codex provided guidance on:
1. **Use `strict-server: true`** for typed request/response objects and compile-time verification
2. **Keep generated code in `internal/api/generated/`** to make it internal
3. **Keep index tracking separate from DataLoader** for separation of concerns
4. **Add `Exists()` method** to DataLoader for explicit not-found checks
5. **Define clear error taxonomy:** 404 for missing data, 400 for invalid params, 500 for internal errors
6. **Validate config upfront** with fail-fast behavior
7. **Consider sharded locks** for high-concurrency scenarios (future optimization)
