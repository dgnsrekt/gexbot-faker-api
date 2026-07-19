# Gexbot Historical Data Batch Downloader - Implementation Plan

## Overview

Build a Go-based CLI tool that downloads historical options/derivatives data from the Gexbot API, providing a reliable and fully customizable batch downloader.

## Current State Analysis

**What exists:**

- `quant-historical/main.py` - Simple Python script demonstrating API authentication and endpoint structure
- Research document documenting API structure: `.strategic-claude-basic/research/RESEARCH_0001_29-11-2025_sat_quant-historical-analysis.md`

**What's missing:**

- Direct API-based batch downloader in Go
- YAML configuration system for flexible data selection
- Concurrent download capability with proper rate limiting
- Atomic staging to prevent partial data directories

**Key Constraints:**

- Files can be 26-145 MB each - must use streaming downloads
- API rate limits unknown - need conservative defaults with backoff
- Pre-signed URLs may expire quickly - fetch fresh URL per download
- 404 responses are expected for unavailable ticker/date combinations

### Key Discoveries:

- API endpoint: `GET https://api.gex.bot/v2/hist/{ticker}/{package}/{category}/{date}?noredirect` (from `quant-historical/main.py:167`)
- Authentication: `Authorization: Basic {API_KEY}` header (from `quant-historical/main.py:159`)
- Response: JSON with `url` field containing pre-signed download URL
- 41 supported tickers across indices, ETFs, and stocks (from `quant-historical/main.py:22-71`)

## Desired End State

A working Go CLI tool (`gexbot-downloader`) that:

1. Downloads historical data from Gexbot API using YAML configuration
2. Supports concurrent downloads with configurable worker count
3. Implements atomic staging to prevent partial data directories
4. Provides resume capability by skipping existing valid files
5. Handles rate limiting and errors gracefully

**Verification:**

```bash
# Build and run
make build
./gexbot-downloader download --dry-run 2025-11-14

# Download with defaults from config
./gexbot-downloader download 2025-11-14

# Verify output structure
ls -la data/2025-11-14/SPX/state/
```

## What We're NOT Doing

- Parquet output format (JSON only for now)
- File validation beyond existence check (no JSON parsing on download)
- Web UI or dashboard
- Real-time streaming data
- Multi-region/failover support
- Database storage (files only)

## Implementation Approach

Follow Go best practices with clean separation of concerns:

1. **Data model first** - Define packages, categories, tickers as types
2. **Config system** - YAML with viper, env variable substitution
3. **HTTP client interface** - Mockable for testing, retry logic
4. **Worker pool** - Concurrent downloads with rate limiting
5. **Staging system** - Atomic directory operations
6. **CLI layer** - Thin cobra commands

**Codex Mentorship Insights:**

- Use `io.Copy` streaming to avoid buffering large files
- Thread `context.Context` from cobra for graceful shutdown
- Add token bucket rate limiter (configurable)
- Treat 404 as non-fatal "not available" status
- Use interface for HTTP client to enable testing

---

## Phase 1: Project Setup & Data Model

### Overview

Initialize Go module, create directory structure, define core data types for tickers, packages, and categories.

### Changes Required:

#### 1. Initialize Go Module

**File**: `go.mod`
**Changes**: Create Go module

```go
module github.com/dgnsrekt/gexbot-downloader

go 1.22

require (
    github.com/spf13/cobra v1.8.0
    github.com/spf13/viper v1.18.0
    go.uber.org/zap v1.27.0
    golang.org/x/time v0.5.0
)
```

#### 2. Create Directory Structure

```bash
mkdir -p cmd/downloader
mkdir -p internal/{api,config,download,staging}
mkdir -p configs
mkdir -p data
```

#### 3. Define Data Model

**File**: `internal/config/types.go`
**Changes**: Define core types for packages, categories, and tickers

```go
package config

// Package represents a data package type
type Package string

const (
    PackageState     Package = "state"
    PackageClassic   Package = "classic"
    PackageOrderflow Package = "orderflow"
)

// ValidCategories returns valid categories for each package
var ValidCategories = map[Package][]string{
    PackageState: {
        "gex_full", "gex_zero", "gex_one",
        "delta_zero", "delta_one",
        "gamma_zero", "gamma_one",
        "vanna_zero", "vanna_one",
        "charm_zero", "charm_one",
    },
    PackageClassic:   {"gex_full"},
    PackageOrderflow: {"orderflow"},
}

// DefaultTickers lists all supported tickers
var DefaultTickers = []string{
    "SPX", "NDX", "RUT", "SPY", "QQQ", "IWM",
    "VIX", "UVXY", "AAPL", "TSLA", "NVDA", "META",
    "AMZN", "GOOG", "GOOGL", "NFLX", "AMD", "ORCL", "BABA",
}
```

#### 4. Create Makefile

**File**: `Makefile`
**Changes**: Build, run, and development targets

```makefile
.PHONY: build run clean test lint

build:
	go build -o bin/gexbot-downloader ./cmd/downloader

run: build
	./bin/gexbot-downloader

clean:
	rm -rf bin/
	rm -rf data/.staging/

test:
	go test -v ./...

lint:
	golangci-lint run
```

### Success Criteria:

#### Automated Verification:

- [x] `go mod tidy` runs without errors
- [x] `go build ./...` compiles successfully
- [x] Directory structure exists: `ls internal/api internal/config internal/download internal/staging`

#### Manual Verification:

- [x] Project structure matches the design
- [x] Types are properly defined with documentation

---

## Phase 2: Configuration System

### Overview

Implement YAML configuration loading with viper, environment variable substitution, and validation.

### Changes Required:

#### 1. Configuration Schema

**File**: `internal/config/config.go`
**Changes**: Define config struct and loader

```go
package config

import (
    "fmt"
    "strings"

    "github.com/spf13/viper"
)

type Config struct {
    API      APIConfig      `mapstructure:"api"`
    Download DownloadConfig `mapstructure:"download"`
    Tickers  []string       `mapstructure:"tickers"`
    Packages PackagesConfig `mapstructure:"packages"`
    Output   OutputConfig   `mapstructure:"output"`
}

type APIConfig struct {
    BaseURL    string `mapstructure:"base_url"`
    APIKey     string `mapstructure:"api_key"`
    TimeoutSec int    `mapstructure:"timeout_sec"`
    RetryCount int    `mapstructure:"retry_count"`
    RetryDelay int    `mapstructure:"retry_delay_sec"`
}

type DownloadConfig struct {
    Workers       int  `mapstructure:"workers"`
    RatePerSecond int  `mapstructure:"rate_per_second"`
    ResumeEnabled bool `mapstructure:"resume_enabled"`
}

type PackagesConfig struct {
    State     PackageConfig `mapstructure:"state"`
    Classic   PackageConfig `mapstructure:"classic"`
    Orderflow PackageConfig `mapstructure:"orderflow"`
}

type PackageConfig struct {
    Enabled    bool     `mapstructure:"enabled"`
    Categories []string `mapstructure:"categories"`
}

type OutputConfig struct {
    Directory string `mapstructure:"directory"`
}

func Load(configPath string) (*Config, error) {
    v := viper.New()

    // Set defaults
    v.SetDefault("api.base_url", "https://api.gex.bot")
    v.SetDefault("api.timeout_sec", 300)
    v.SetDefault("api.retry_count", 3)
    v.SetDefault("api.retry_delay_sec", 5)
    v.SetDefault("download.workers", 3)
    v.SetDefault("download.rate_per_second", 2)
    v.SetDefault("download.resume_enabled", true)
    v.SetDefault("output.directory", "data")

    // Environment variable support
    v.SetEnvPrefix("GEXBOT")
    v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
    v.AutomaticEnv()

    // Load config file
    if configPath != "" {
        v.SetConfigFile(configPath)
    } else {
        v.SetConfigName("default")
        v.SetConfigType("yaml")
        v.AddConfigPath("./configs")
        v.AddConfigPath(".")
    }

    if err := v.ReadInConfig(); err != nil {
        if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
            return nil, fmt.Errorf("reading config: %w", err)
        }
    }

    var cfg Config
    if err := v.Unmarshal(&cfg); err != nil {
        return nil, fmt.Errorf("unmarshaling config: %w", err)
    }

    if err := cfg.Validate(); err != nil {
        return nil, fmt.Errorf("validating config: %w", err)
    }

    return &cfg, nil
}

func (c *Config) Validate() error {
    if c.API.APIKey == "" {
        return fmt.Errorf("api_key is required (set GEXBOT_API_KEY env var)")
    }
    if c.Download.Workers < 1 {
        return fmt.Errorf("workers must be >= 1")
    }
    return nil
}
```

#### 2. Default Configuration

**File**: `configs/default.yaml`
**Changes**: Create default YAML config

```yaml
api:
  base_url: "https://api.gex.bot"
  api_key: "${GEXBOT_API_KEY}"
  timeout_sec: 300
  retry_count: 3
  retry_delay_sec: 5

download:
  workers: 3
  rate_per_second: 2
  resume_enabled: true

tickers:
  - SPX
  - NDX
  - RUT
  - SPY
  - QQQ
  - IWM

packages:
  state:
    enabled: true
    categories:
      - gex_full
      - gex_zero
  classic:
    enabled: true
    categories:
      - gex_full
  orderflow:
    enabled: false
    categories:
      - orderflow

output:
  directory: "data"
```

### Success Criteria:

#### Automated Verification:

- [x] `go build ./...` compiles with config package
- [x] Config loads with `GEXBOT_API_KEY` env var set
- [x] Config validation fails without API key

#### Manual Verification:

- [x] YAML structure is readable and well-documented
- [x] Environment variable substitution works correctly

---

## Phase 3: API Client

### Overview

Implement HTTP client with Basic Auth, retry logic with exponential backoff, and rate limiting.

### Changes Required:

#### 1. API Client Interface

**File**: `internal/api/client.go`
**Changes**: Define client interface and implementation

```go
package api

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"

    "go.uber.org/zap"
    "golang.org/x/time/rate"
)

// Client interface for testability
type Client interface {
    GetDownloadURL(ctx context.Context, ticker, pkg, category, date string) (string, error)
    DownloadFile(ctx context.Context, url string, dest io.Writer) (int64, error)
}

type HTTPClient struct {
    httpClient  *http.Client
    baseURL     string
    apiKey      string
    limiter     *rate.Limiter
    retryCount  int
    retryDelay  time.Duration
    logger      *zap.Logger
}

type HistoryResponse struct {
    URL string `json:"url"`
}

func NewClient(baseURL, apiKey string, ratePerSec int, timeout, retryDelay time.Duration, retryCount int, logger *zap.Logger) *HTTPClient {
    transport := &http.Transport{
        MaxIdleConns:        100,
        MaxConnsPerHost:     10,
        IdleConnTimeout:     90 * time.Second,
        DisableCompression:  false,
    }

    return &HTTPClient{
        httpClient: &http.Client{
            Transport: transport,
            Timeout:   timeout,
        },
        baseURL:    baseURL,
        apiKey:     apiKey,
        limiter:    rate.NewLimiter(rate.Limit(ratePerSec), ratePerSec*2),
        retryCount: retryCount,
        retryDelay: retryDelay,
        logger:     logger,
    }
}

func (c *HTTPClient) GetDownloadURL(ctx context.Context, ticker, pkg, category, date string) (string, error) {
    // Wait for rate limiter
    if err := c.limiter.Wait(ctx); err != nil {
        return "", fmt.Errorf("rate limiter: %w", err)
    }

    url := fmt.Sprintf("%s/v2/hist/%s/%s/%s/%s?noredirect", c.baseURL, ticker, pkg, category, date)

    var lastErr error
    for attempt := 0; attempt <= c.retryCount; attempt++ {
        if attempt > 0 {
            delay := c.retryDelay * time.Duration(1<<(attempt-1)) // Exponential backoff
            c.logger.Debug("retrying request", zap.Int("attempt", attempt), zap.Duration("delay", delay))

            select {
            case <-ctx.Done():
                return "", ctx.Err()
            case <-time.After(delay):
            }
        }

        req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
        if err != nil {
            return "", fmt.Errorf("creating request: %w", err)
        }

        req.Header.Set("Authorization", "Basic "+c.apiKey)
        req.Header.Set("Accept", "application/json")

        resp, err := c.httpClient.Do(req)
        if err != nil {
            lastErr = err
            continue
        }
        defer resp.Body.Close()

        if resp.StatusCode == http.StatusNotFound {
            return "", ErrNotFound
        }

        if resp.StatusCode == http.StatusTooManyRequests {
            lastErr = ErrRateLimited
            continue
        }

        if resp.StatusCode >= 500 {
            lastErr = fmt.Errorf("server error: %d", resp.StatusCode)
            continue
        }

        if resp.StatusCode != http.StatusOK {
            return "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
        }

        var histResp HistoryResponse
        if err := json.NewDecoder(resp.Body).Decode(&histResp); err != nil {
            return "", fmt.Errorf("decoding response: %w", err)
        }

        return histResp.URL, nil
    }

    return "", fmt.Errorf("max retries exceeded: %w", lastErr)
}

func (c *HTTPClient) DownloadFile(ctx context.Context, url string, dest io.Writer) (int64, error) {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if err != nil {
        return 0, fmt.Errorf("creating request: %w", err)
    }

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return 0, fmt.Errorf("executing request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return 0, fmt.Errorf("unexpected status: %d", resp.StatusCode)
    }

    // Stream to destination
    return io.Copy(dest, resp.Body)
}
```

#### 2. Error Types

**File**: `internal/api/errors.go`
**Changes**: Define custom errors

```go
package api

import "errors"

var (
    ErrNotFound    = errors.New("data not found for this ticker/date")
    ErrRateLimited = errors.New("rate limited by API")
    ErrAuthFailed  = errors.New("authentication failed")
)
```

### Success Criteria:

#### Automated Verification:

- [x] `go build ./...` compiles with api package
- [x] `go test ./internal/api/...` passes (mock tests)

#### Manual Verification:

- [x] Client properly adds Basic Auth header
- [x] Retry logic works with exponential backoff
- [x] Rate limiter prevents request floods

---

## Phase 4: Download Engine

### Overview

Implement worker pool for concurrent downloads with progress reporting and graceful shutdown.

### Changes Required:

#### 1. Task Definition

**File**: `internal/download/task.go`
**Changes**: Define download task and result types

```go
package download

import (
    "fmt"
    "path/filepath"
)

type Task struct {
    Ticker   string
    Package  string
    Category string
    Date     string
}

func (t Task) APIPath() string {
    return fmt.Sprintf("%s/%s/%s/%s", t.Ticker, t.Package, t.Category, t.Date)
}

func (t Task) OutputPath(baseDir string) string {
    return filepath.Join(baseDir, t.Date, t.Ticker, t.Package, t.Category+".json")
}

func (t Task) String() string {
    return fmt.Sprintf("%s/%s/%s/%s", t.Date, t.Ticker, t.Package, t.Category)
}

type TaskResult struct {
    Task       Task
    Success    bool
    Skipped    bool
    NotFound   bool
    BytesSize  int64
    Error      error
}
```

#### 2. Download Manager

**File**: `internal/download/manager.go`
**Changes**: Implement worker pool orchestration

```go
package download

import (
    "context"
    "fmt"
    "os"
    "sync"

    "go.uber.org/zap"

    "github.com/dgnsrekt/gexbot-downloader/internal/api"
    "github.com/dgnsrekt/gexbot-downloader/internal/staging"
)

type Manager struct {
    client   api.Client
    staging  *staging.Manager
    workers  int
    logger   *zap.Logger
}

type BatchResult struct {
    Total     int
    Success   int
    Skipped   int
    NotFound  int
    Failed    int
    Errors    []string
}

func NewManager(client api.Client, staging *staging.Manager, workers int, logger *zap.Logger) *Manager {
    return &Manager{
        client:  client,
        staging: staging,
        workers: workers,
        logger:  logger,
    }
}

func (m *Manager) Execute(ctx context.Context, tasks []Task) (*BatchResult, error) {
    result := &BatchResult{Total: len(tasks)}

    jobs := make(chan Task, len(tasks))
    results := make(chan TaskResult, len(tasks))

    // Start workers
    var wg sync.WaitGroup
    for i := 0; i < m.workers; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            m.worker(ctx, workerID, jobs, results)
        }(i)
    }

    // Send jobs
    go func() {
        for _, task := range tasks {
            select {
            case <-ctx.Done():
                return
            case jobs <- task:
            }
        }
        close(jobs)
    }()

    // Wait for workers and close results
    go func() {
        wg.Wait()
        close(results)
    }()

    // Collect results
    for r := range results {
        if r.Skipped {
            result.Skipped++
        } else if r.NotFound {
            result.NotFound++
        } else if r.Success {
            result.Success++
        } else {
            result.Failed++
            if r.Error != nil {
                result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", r.Task, r.Error))
            }
        }
    }

    return result, nil
}

func (m *Manager) worker(ctx context.Context, id int, jobs <-chan Task, results chan<- TaskResult) {
    for task := range jobs {
        select {
        case <-ctx.Done():
            return
        default:
        }

        result := m.processTask(ctx, task)

        select {
        case <-ctx.Done():
            return
        case results <- result:
        }
    }
}

func (m *Manager) processTask(ctx context.Context, task Task) TaskResult {
    result := TaskResult{Task: task}

    outputPath := task.OutputPath(m.staging.FinalDir())

    // Check if file exists (resume)
    if _, err := os.Stat(outputPath); err == nil {
        m.logger.Debug("skipping existing file", zap.String("task", task.String()))
        result.Skipped = true
        result.Success = true
        return result
    }

    m.logger.Info("downloading", zap.String("task", task.String()))

    // Get signed URL
    signedURL, err := m.client.GetDownloadURL(ctx, task.Ticker, task.Package, task.Category, task.Date)
    if err != nil {
        if err == api.ErrNotFound {
            m.logger.Debug("not found", zap.String("task", task.String()))
            result.NotFound = true
            return result
        }
        result.Error = err
        return result
    }

    // Download to staging
    stagingPath := task.OutputPath(m.staging.StagingDir(task.Date))
    size, err := m.staging.DownloadToStaging(ctx, m.client, signedURL, stagingPath)
    if err != nil {
        result.Error = err
        return result
    }

    result.Success = true
    result.BytesSize = size
    m.logger.Info("downloaded", zap.String("task", task.String()), zap.Int64("bytes", size))

    return result
}
```

### Success Criteria:

#### Automated Verification:

- [x] `go build ./...` compiles with download package
- [x] `go test ./internal/download/...` passes

#### Manual Verification:

- [x] Worker pool processes tasks concurrently
- [x] Progress logging shows download status
- [x] Graceful shutdown works on SIGINT

---

## Phase 5: Staging System

### Overview

Implement atomic directory operations with temp files, staging directory, and final commit.

### Changes Required:

#### 1. Staging Manager

**File**: `internal/staging/staging.go`
**Changes**: Implement atomic file operations

```go
package staging

import (
    "context"
    "fmt"
    "io"
    "os"
    "path/filepath"

    "github.com/dgnsrekt/gexbot-downloader/internal/api"
)

type Manager struct {
    baseDir     string
    stagingRoot string
}

func NewManager(baseDir string) *Manager {
    return &Manager{
        baseDir:     baseDir,
        stagingRoot: filepath.Join(baseDir, ".staging"),
    }
}

func (m *Manager) FinalDir() string {
    return m.baseDir
}

func (m *Manager) StagingDir(date string) string {
    return filepath.Join(m.stagingRoot, date)
}

func (m *Manager) PrepareStaging(date string) error {
    dir := m.StagingDir(date)
    return os.MkdirAll(dir, 0755)
}

func (m *Manager) DownloadToStaging(ctx context.Context, client api.Client, url, destPath string) (int64, error) {
    // Create parent directories
    if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
        return 0, fmt.Errorf("creating directories: %w", err)
    }

    // Download to temp file
    tmpPath := destPath + ".tmp"
    f, err := os.Create(tmpPath)
    if err != nil {
        return 0, fmt.Errorf("creating temp file: %w", err)
    }

    size, err := client.DownloadFile(ctx, url, f)
    if closeErr := f.Close(); closeErr != nil && err == nil {
        err = closeErr
    }

    if err != nil {
        os.Remove(tmpPath)
        return 0, fmt.Errorf("downloading file: %w", err)
    }

    // Atomic rename
    if err := os.Rename(tmpPath, destPath); err != nil {
        os.Remove(tmpPath)
        return 0, fmt.Errorf("renaming temp file: %w", err)
    }

    return size, nil
}

func (m *Manager) CommitStaging(date string) error {
    stagingDir := m.StagingDir(date)
    finalDir := filepath.Join(m.baseDir, date)

    // Walk staging and move files
    return filepath.Walk(stagingDir, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        if info.IsDir() {
            return nil
        }

        relPath, err := filepath.Rel(stagingDir, path)
        if err != nil {
            return err
        }

        destPath := filepath.Join(finalDir, relPath)
        if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
            return err
        }

        return os.Rename(path, destPath)
    })
}

func (m *Manager) CleanupStaging(date string) error {
    return os.RemoveAll(m.StagingDir(date))
}
```

### Success Criteria:

#### Automated Verification:

- [x] `go build ./...` compiles with staging package
- [x] `go test ./internal/staging/...` passes

#### Manual Verification:

- [x] Files download to `.staging/` first
- [x] Temp files use `.tmp` suffix
- [x] Commit moves files to final directory

---

## Phase 6: CLI Commands

### Overview

Implement cobra CLI with download command, flags, and graceful shutdown.

### Changes Required:

#### 1. Root Command

**File**: `cmd/downloader/main.go`
**Changes**: Create CLI entry point

```go
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"

    "github.com/spf13/cobra"
    "go.uber.org/zap"

    "github.com/dgnsrekt/gexbot-downloader/internal/config"
)

var (
    cfgFile  string
    verbose  bool
    logger   *zap.Logger
    cfg      *config.Config
)

func main() {
    rootCmd := &cobra.Command{
        Use:   "gexbot-downloader",
        Short: "Download historical data from Gexbot API",
        PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
            // Setup logger
            var err error
            if verbose {
                logger, err = zap.NewDevelopment()
            } else {
                logger, err = zap.NewProduction()
            }
            if err != nil {
                return err
            }

            // Load config
            cfg, err = config.Load(cfgFile)
            if err != nil {
                return err
            }

            return nil
        },
    }

    rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file path")
    rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

    rootCmd.AddCommand(downloadCmd())

    // Setup signal handling
    ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer cancel()

    if err := rootCmd.ExecuteContext(ctx); err != nil {
        os.Exit(1)
    }
}
```

#### 2. Download Command

**File**: `cmd/downloader/download.go`
**Changes**: Implement download subcommand

```go
package main

import (
    "fmt"
    "time"

    "github.com/spf13/cobra"
    "go.uber.org/zap"

    "github.com/dgnsrekt/gexbot-downloader/internal/api"
    "github.com/dgnsrekt/gexbot-downloader/internal/download"
    "github.com/dgnsrekt/gexbot-downloader/internal/staging"
)

func downloadCmd() *cobra.Command {
    var (
        dryRun   bool
        tickers  []string
        packages []string
    )

    cmd := &cobra.Command{
        Use:   "download [DATE] [END_DATE]",
        Short: "Download historical data for specified date(s)",
        Args:  cobra.RangeArgs(1, 2),
        RunE: func(cmd *cobra.Command, args []string) error {
            ctx := cmd.Context()

            // Parse dates
            dates, err := parseDates(args)
            if err != nil {
                return err
            }

            // Generate tasks
            tasks := generateTasks(dates, tickers, packages)

            logger.Info("generated tasks", zap.Int("count", len(tasks)))

            if dryRun {
                for _, t := range tasks {
                    fmt.Printf("Would download: %s\n", t)
                }
                return nil
            }

            // Create client
            client := api.NewClient(
                cfg.API.BaseURL,
                cfg.API.APIKey,
                cfg.Download.RatePerSecond,
                time.Duration(cfg.API.TimeoutSec)*time.Second,
                time.Duration(cfg.API.RetryDelay)*time.Second,
                cfg.API.RetryCount,
                logger,
            )

            // Create staging manager
            stgMgr := staging.NewManager(cfg.Output.Directory)

            // Create download manager
            dlMgr := download.NewManager(client, stgMgr, cfg.Download.Workers, logger)

            // Execute downloads
            result, err := dlMgr.Execute(ctx, tasks)
            if err != nil {
                return err
            }

            // Print summary
            logger.Info("download complete",
                zap.Int("total", result.Total),
                zap.Int("success", result.Success),
                zap.Int("skipped", result.Skipped),
                zap.Int("not_found", result.NotFound),
                zap.Int("failed", result.Failed),
            )

            if result.Failed > 0 {
                return fmt.Errorf("%d downloads failed", result.Failed)
            }

            return nil
        },
    }

    cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be downloaded")
    cmd.Flags().StringSliceVar(&tickers, "tickers", nil, "override tickers from config")
    cmd.Flags().StringSliceVar(&packages, "packages", nil, "override packages from config")

    return cmd
}
```

### Success Criteria:

#### Automated Verification:

- [x] `go build ./cmd/downloader` produces executable
- [x] `./bin/gexbot-downloader --help` shows usage
- [x] `./bin/gexbot-downloader download --dry-run 2025-11-14` lists tasks

#### Manual Verification:

- [x] SIGINT gracefully stops downloads
- [x] Progress logging is clear and informative
- [x] Config overrides work via flags

---

## Phase 7: Polish & Documentation

### Overview

Add README, update .gitignore, and test with real API.

### Changes Required:

#### 1. README

**File**: `README.md`
**Changes**: Usage documentation

#### 2. Gitignore

**File**: `.gitignore`
**Changes**: Add data directory and binaries

```gitignore
# Binaries
bin/
*.exe

# Data
data/

# Environment
.env

# IDE
.idea/
.vscode/
```

### Success Criteria:

#### Automated Verification:

- [x] `make build` succeeds
- [x] `make test` passes
- [ ] `make lint` passes (if golangci-lint installed)

#### Manual Verification:

- [x] README contains clear usage examples
- [ ] Real API download works for single date
- [ ] Resume capability works when re-running

---

## Test Plan Reference

Testing for this implementation will primarily be:

1. **Unit tests** with mock HTTP client for API and download packages
2. **Integration tests** (skipped by default, require API key)
3. **Manual testing** with real API

Detailed test implementation is deferred until Phase 7.

## Performance Considerations

- **Streaming downloads**: Use `io.Copy` to avoid buffering 145 MB files in memory
- **Connection pooling**: `http.Transport` with `MaxConnsPerHost: 10`
- **Rate limiting**: Default 2 RPS with token bucket, configurable via YAML
- **Concurrent downloads**: Default 3 workers, adjustable via config

## Migration Notes

No migration needed - this is a greenfield implementation.

## References

- Research: `.strategic-claude-basic/research/RESEARCH_0001_29-11-2025_sat_quant-historical-analysis.md`
- API pattern: `/home/dgnsrekt/Development/GEXBOT_RESEARCH/quant-historical/main.py:159-167`

## Codex Mentorship Summary

Key insights from Codex validation:

1. **Streaming required** - Use `io.Copy` for large files (26-145 MB)
2. **Rate limiting** - Add token bucket with configurable RPS, handle 429 with backoff
3. **404 handling** - Treat as non-fatal "not available" status
4. **Context threading** - Use `signal.NotifyContext` for graceful shutdown
5. **Interface-based design** - Enable testing with mock HTTP client
6. **Connection management** - Configure `http.Transport` with proper limits
7. **Secret handling** - Never log API key, support env variable override
