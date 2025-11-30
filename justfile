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
    @echo "  just convert-to-jsonl    Convert JSON files to JSONL format"
    @echo ""
    @echo "Server Commands"
    @echo ""
    @echo "  just build-gex-faker              Build the GEX Faker server binary"
    @echo "  just serve-gex-faker              Run the GEX Faker server (development)"
    @echo "  just generate-gex-faker-api-spec  Generate API code from OpenAPI spec"
    @echo ""
    @echo "Common Commands"
    @echo ""
    @echo "  just test                Run tests"
    @echo "  just lint                Run linter"
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

# Build the GEX Faker server binary
build-gex-faker: generate-gex-faker-api-spec
    go build -o bin/gexbot-server ./cmd/server

# Run the GEX Faker server (development)
serve-gex-faker: build-gex-faker
    ./bin/gexbot-server

# Download historical data for GEXBOT_DOWNLOADER_DATE
download: build
    ./bin/gexbot-downloader download $GEXBOT_DOWNLOADER_DATE

# Convert JSON files to JSONL format for GEXBOT_DOWNLOADER_DATE
convert-to-jsonl: build
    ./bin/gexbot-downloader convert-to-jsonl $GEXBOT_DOWNLOADER_DATE

# Run tests
test:
    go test -v ./...

# Run linter
lint:
    golangci-lint run

# Clean build artifacts, staging, and logs
clean:
    rm -rf bin/
    rm -rf data/.staging/
    rm -rf logs/

# --- Browser Automation Commands ---

# Start Chrome with remote debugging enabled
start-browser:
    ./scripts/start-chromium.sh

# Start Chrome with debugging and console logs
start-browser-logs:
    ./scripts/start-chromium.sh --with-logs