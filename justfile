# gexbot-faker-api justfile
# Browser automation and Go build recipes

# Load .env file automatically for all recipes
set dotenv-load

# Show available commands
help:
    @echo "Go Build Commands"
    @echo ""
    @echo "  just build               Build the downloader binary"
    @echo "  just download            Download data for GEXBOT_DOWNLOADER_DATE"
    @echo "  just test                Run tests"
    @echo "  just lint                Run linter"
    @echo "  just clean               Clean build artifacts"
    @echo ""
    @echo "Browser Automation Commands"
    @echo ""
    @echo "  just start-browser       Start Chrome with remote debugging"
    @echo "  just start-browser-logs  Start Chrome with debugging and console logs"
    @echo ""

# --- Go Build Commands ---

# Build the downloader binary
build:
    go build -o bin/gexbot-downloader ./cmd/downloader

# Download historical data for GEXBOT_DOWNLOADER_DATE
download: build
    ./bin/gexbot-downloader download $GEXBOT_DOWNLOADER_DATE

# Run tests
test:
    go test -v ./...

# Run linter
lint:
    golangci-lint run

# Clean build artifacts and staging
clean:
    rm -rf bin/
    rm -rf data/.staging/

# --- Browser Automation Commands ---

# Start Chrome with remote debugging enabled
start-browser:
    ./scripts/start-chromium.sh

# Start Chrome with debugging and console logs
start-browser-logs:
    ./scripts/start-chromium.sh --with-logs