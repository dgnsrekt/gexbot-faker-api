# Gexbot Historical Data Downloader

A Go-based CLI tool for downloading historical options/derivatives data from the Gexbot API.

## Features

- YAML configuration with environment variable substitution
- Concurrent downloads with configurable worker count
- Rate limiting with token bucket algorithm
- Retry logic with exponential backoff
- Atomic staging to prevent partial data directories
- Resume capability (skips existing files)
- Graceful shutdown on SIGINT/SIGTERM

## Installation

```bash
# Build from source
just build

# Binary is created at bin/gexbot-downloader
```

## Configuration

Set your API key as an environment variable:

```bash
export GEXBOT_API_KEY=your_api_key_here
```

Or create a `configs/default.yaml` file (see `configs/default.yaml` for template).

### Configuration Options

| Option                     | Default               | Description                  |
| -------------------------- | --------------------- | ---------------------------- |
| `api.base_url`             | `https://api.gex.bot` | API base URL                 |
| `api.timeout_sec`          | 300                   | Request timeout in seconds   |
| `api.retry_count`          | 3                     | Number of retries on failure |
| `api.retry_delay_sec`      | 5                     | Base delay between retries   |
| `download.workers`         | 3                     | Concurrent download workers  |
| `download.rate_per_second` | 2                     | Rate limit (requests/second) |
| `download.resume_enabled`  | true                  | Skip existing files          |
| `output.directory`         | `data`                | Output directory             |

## Usage

```bash
# Download single date with config defaults
./bin/gexbot-downloader download 2025-11-14

# Download date range
./bin/gexbot-downloader download 2025-11-01 2025-11-14

# Override tickers from config
./bin/gexbot-downloader download --tickers SPX,NDX 2025-11-14

# Override packages
./bin/gexbot-downloader download --packages state 2025-11-14

# Dry run (show what would be downloaded)
./bin/gexbot-downloader download --dry-run 2025-11-14

# Use custom config file
./bin/gexbot-downloader download --config ./my-config.yaml 2025-11-14

# Verbose output
./bin/gexbot-downloader -v download 2025-11-14
```

## Output Structure

Downloaded files are organized by date, ticker, package, and category:

```
data/
├── 2025-11-14/
│   ├── SPX/
│   │   ├── state/
│   │   │   ├── gex_full.json
│   │   │   └── gex_zero.json
│   │   └── classic/
│   │       └── gex_full.json
│   └── NDX/
│       └── ...
└── 2025-11-15/
    └── ...
```

## Available Data

### Packages and Categories

| Package   | Categories                                                                                                              |
| --------- | ----------------------------------------------------------------------------------------------------------------------- |
| state     | gex_full, gex_zero, gex_one, delta_zero, delta_one, gamma_zero, gamma_one, vanna_zero, vanna_one, charm_zero, charm_one |
| classic   | gex_full, gex_zero, gex_one                                                                                             |
| orderflow | orderflow                                                                                                               |

### Default Tickers

SPX, NDX, RUT, SPY, QQQ, IWM (configurable via YAML)

## Development

```bash
# Run tests
just test

# Build
just build

# Run linter
just lint

# Clean build artifacts
just clean

# Show all available commands
just help
```

## License

See LICENSE file.
