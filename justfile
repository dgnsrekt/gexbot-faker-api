# gexbot-faker-api justfile
# Browser automation and Go build recipes

# Load .env file automatically for all recipes
set dotenv-load

# Show available commands
help:
    @echo "Downloader Commands"
    @echo ""
    @echo "  just build               Build the downloader binary"
    @echo "  just download            Download data for GEXBOT_DOWNLOADER_DATE"
    @echo "  just download-lookback N Download last N days of data (max 90)"
    @echo "  just convert-to-jsonl    Convert JSON files to JSONL format"
    @echo ""
    @echo "Server Commands"
    @echo ""
    @echo "  just build-gex-faker              Build the GEX Faker server binary"
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
    @echo "  just observability-up    Start authenticated monitoring"
    @echo "  just observability-up-no-auth  Start monitoring without auth"
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

# --- Server Commands ---

# Generate API code from OpenAPI spec
generate-gex-faker-api-spec:
    go generate ./api

# Generate protobuf code for WebSocket protocol
generate-protos:
    ~/bin/protoc --proto_path=proto --proto_path=$HOME/bin/include --go_out=internal/ws/generated/orderflow --go_opt=paths=source_relative proto/orderflow.proto
    ~/bin/protoc --proto_path=proto --proto_path=$HOME/bin/include --go_out=internal/ws/generated/webpubsub --go_opt=paths=source_relative proto/webpubsub_messages.proto
    ~/bin/protoc --proto_path=proto --proto_path=$HOME/bin/include --go_out=internal/ws/generated/gex --go_opt=paths=source_relative proto/gex.proto

# Build the GEX Faker server binary
build-gex-faker: generate-gex-faker-api-spec
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

# Start the opt-in observability stack with gateway authentication.
observability-up:
    #!/usr/bin/env sh
    set -eu
    test -n "${OBSERVABILITY_PASSWORD:-}" || { echo "OBSERVABILITY_PASSWORD is required" >&2; exit 1; }
    test -n "${NTFY_TOPIC:-}" || { echo "NTFY_TOPIC is required for alert delivery" >&2; exit 1; }
    docker compose --profile observability up -d --build

# Trusted-LAN convenience mode; does not change the faker API.
observability-up-no-auth:
    #!/usr/bin/env sh
    set -eu
    test -n "${NTFY_TOPIC:-}" || { echo "NTFY_TOPIC is required for alert delivery" >&2; exit 1; }
    OBSERVABILITY_AUTH_ENABLED=false docker compose --profile observability up -d --build

observability-down:
    docker compose --profile observability down

observability-logs:
    docker compose --profile observability logs -f --tail 100
