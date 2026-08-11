# gexbot-faker-api justfile
# Browser automation and Go build recipes

# Load .env file automatically for all recipes
set dotenv-load

# Show available commands
help:
    @echo "Downloader Commands"
    @echo ""
    @echo "  just build               Build the downloader binary"
    @echo "  just build-gexfakercli   Build the gexfakercli client (LLM CLI + skill + setup)"
    @echo "  just download            Download data for GEXBOT_DOWNLOADER_DATE"
    @echo "  just download-lookback N Download last N days of data (max 90)"
    @echo "  just convert-to-jsonl    Convert JSON files to JSONL format"
    @echo ""
    @echo "Server Commands"
    @echo ""
    @echo "  just build-gex-faker              Build the GEX Faker server binary"
    @echo "  just studio-build                 Build the GEX Faker Studio web UI (/studio)"
    @echo "  just docs-build                   Build the guides docs site (/guides, from knowledge/)"
    @echo "  just docs-dev                     Run the guides docs site in dev mode"
    @echo "  just demos-render                 Render CLI demo GIFs (VHS) into website/public/demos"
    @echo "  just serve-gex-faker              Run the GEX Faker server (development)"
    @echo "  just generate-gex-faker-api-spec  Generate API code from OpenAPI spec"
    @echo "  just generate-protos              Generate protobuf code for WebSocket"
    @echo ""
    @echo "Docker Commands"
    @echo ""
    @echo "  just up                  Rebuild and start all containers"
    @echo "  just down                Stop and remove all containers"
    @echo "  just restart-api         Rebuild and restart API container"
    @echo "  just restart-daemon      Rebuild and restart daemon container"
    @echo "  just logs                Follow all container logs"
    @echo "  just api-logs            Follow API logs only"
    @echo "  just daemon-logs         Follow daemon logs only"
    @echo ""
    @echo "Common Commands"
    @echo ""
    @echo "  just test                Run tests"
    @echo "  just lint                Run linter"
    @echo "  just install-hooks       Install pre-commit hooks"
    @echo "  just pre-commit          Run pre-commit on all files"
    @echo "  just clean               Clean build artifacts"
    @echo ""
    @echo "Browser Automation Commands"
    @echo ""
    @echo "  just start-browser       Start Chrome with remote debugging"
    @echo "  just start-browser-logs  Start Chrome with debugging and console logs"
    @echo ""

# --- Downloader Commands ---

# Build the downloader binary
build:
    go build -o bin/gexbot-downloader ./cmd/downloader

# Build the gexfakercli binary (LLM-first REST client + skill + auto-setup)
build-gexfakercli:
    go build -o bin/gexfakercli ./cmd/gexfakercli

# --- Server Commands ---

# Generate API code from OpenAPI spec
generate-gex-faker-api-spec:
    go generate ./api

# Generate protobuf code for WebSocket protocol
generate-protos:
    ~/bin/protoc --proto_path=proto --proto_path=$HOME/bin/include --go_out=internal/ws/generated/orderflow --go_opt=paths=source_relative proto/orderflow.proto
    ~/bin/protoc --proto_path=proto --proto_path=$HOME/bin/include --go_out=internal/ws/generated/webpubsub --go_opt=paths=source_relative proto/webpubsub_messages.proto
    ~/bin/protoc --proto_path=proto --proto_path=$HOME/bin/include --go_out=internal/ws/generated/gex --go_opt=paths=source_relative proto/gex.proto

# Build the GEX Faker Studio web UI (embedded into the server via //go:embed).
# Re-create dist/.gitkeep afterwards: `vite build` empties the dir, and the
# checked-in .gitkeep must survive so the Go package still compiles on a clean
# checkout that hasn't run a UI build.
studio-build:
    cd web && npm ci && npm run build && touch dist/.gitkeep

# Regenerate llms.txt / llms-full.txt from the knowledge/ OKF bundle.
docs-llms:
    cd website && npm ci && npm run gen

# Build the Starlight docs site (embedded into the server via //go:embed at
# /guides). Re-create dist/.gitkeep afterwards for the same reason as studio-build.
docs-build:
    cd website && npm ci && npm run build:embed && touch dist/.gitkeep

# Run the docs site in dev mode (hot-reloads knowledge/ edits via the sync step).
docs-dev:
    cd website && npm ci && npm run dev

# Render the CLI demo GIFs from demos/cli/*.tape (charmbracelet VHS). Needs vhs +
# ttyd + ffmpeg (see demos/README.md) and a running faker on :8080. GIFs land in
# website/public/demos/ (committed, served by the guides at /guides/demos/).
demos-render:
    @curl -sf -m2 http://127.0.0.1:8080/health >/dev/null || (echo "start a faker on :8080 first (just serve-gex-faker)" && exit 1)
    @command -v vhs >/dev/null || (echo "vhs not installed — see demos/README.md" && exit 1)
    mkdir -p website/public/demos
    for tape in demos/cli/*.tape; do echo "rendering $tape"; vhs "$tape"; done

# Build the GEX Faker server binary (embeds the Studio UI and the docs site)
build-gex-faker: generate-gex-faker-api-spec studio-build docs-build
    go build -o bin/gexbot-server ./cmd/server

# Run the GEX Faker server (development)
serve-gex-faker: build-gex-faker
    ./bin/gexbot-server

# Download historical data for GEXBOT_DOWNLOADER_DATE
download: build
    ./bin/gexbot-downloader download $GEXBOT_DOWNLOADER_DATE

# Download historical data for a lookback window (max 90 days)
# Usage: just download-lookback 30
download-lookback days: build
    #!/usr/bin/env bash
    set -euo pipefail

    # Validate lookback is within 90-day limit
    if [ "{{days}}" -gt 90 ]; then
        echo "Error: Lookback window cannot exceed 90 days (got {{days}})"
        exit 1
    fi

    if [ "{{days}}" -lt 1 ]; then
        echo "Error: Lookback window must be at least 1 day"
        exit 1
    fi

    # Calculate dates (cross-platform: macOS uses -v, Linux uses -d)
    if [[ "$OSTYPE" == "darwin"* ]]; then
        START_DATE=$(date -v-{{days}}d +%Y-%m-%d)
        END_DATE=$(date -v-1d +%Y-%m-%d)
    else
        START_DATE=$(date -d "{{days}} days ago" +%Y-%m-%d)
        END_DATE=$(date -d "yesterday" +%Y-%m-%d)
    fi

    echo "Downloading data from $START_DATE to $END_DATE ({{days}} day lookback)"
    ./bin/gexbot-downloader download "$START_DATE" "$END_DATE"

# Convert JSON files to JSONL format for GEXBOT_DOWNLOADER_DATE
convert-to-jsonl: build
    ./bin/gexbot-downloader convert-to-jsonl $GEXBOT_DOWNLOADER_DATE

# Run tests
test:
    go test -v ./...

# Run linter
lint:
    golangci-lint run

# Install pre-commit hooks
install-hooks:
    pre-commit install

# Run pre-commit on all files
pre-commit:
    pre-commit run --all-files

# Clean build artifacts, staging, and logs
clean:
    rm -rf bin/
    rm -rf data/.staging/
    rm -rf logs/

# --- Docker Commands ---

# Rebuild and start all containers
up:
    docker compose up -d --build

# Stop and remove all containers
down:
    docker compose down

# Rebuild and restart API container
restart-api:
    docker compose up -d --build gex-faker-api

# Rebuild and restart daemon container
restart-daemon:
    docker compose up -d --build gex-daemon

# Follow all container logs
logs:
    docker compose logs -f --tail 100

# Follow API logs only
api-logs:
    docker compose logs -f --tail 100 gex-faker-api

# Follow daemon logs only
daemon-logs:
    docker compose logs -f --tail 100 gex-daemon
