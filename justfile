# lightweight-charts justfile
# Browser automation recipes

# Load .env file automatically for all recipes
set dotenv-load

# Show available commands
help:
	@echo "Browser Automation Commands"
	@echo ""
	@echo "  just start-browser       Start Chrome with remote debugging"
	@echo "  just start-browser-logs  Start Chrome with debugging and console logs"
	@echo ""

# Start Chrome with remote debugging enabled
start-browser:
    ./scripts/start-chromium.sh

# Start Chrome with debugging and console logs
start-browser-logs:
    ./scripts/start-chromium.sh --with-logs