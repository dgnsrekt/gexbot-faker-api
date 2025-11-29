#!/bin/sh
#
# start-chromium.sh - Launch Chromium with remote debugging for CDP connection
#
# Usage:
#   ./scripts/start-chromium.sh              # Without logging
#   ./scripts/start-chromium.sh --with-logs  # With Chromium internal logging
#
# This script launches Chromium with remote debugging enabled, allowing
# connections via Chrome DevTools Protocol (CDP).
#
# Configuration is loaded from .env file in the project root.
# Required:
#   CHROMIUM_CDP_ADDRESS (e.g., 127.0.0.1)
#   CHROMIUM_CDP_PORT (e.g., 9226)
#   CHROMIUM_START_URL (e.g., about:blank)
# Optional:
#   CHROMIUM_PROFILE_DIR (default: ./chromium-profile)
#   CHROMIUM_LOG_FILE_DIR (default: logs)
#
# The browser uses a local profile directory to keep
# test browsing isolated from your personal browser data.
#

set -e

# Parse command-line arguments
ENABLE_LOGGING=false
for arg in "$@"; do
    case $arg in
        --with-logs)
            ENABLE_LOGGING=true
            shift
            ;;
        *)
            # Unknown option
            ;;
    esac
done

# Colors for output (needed for error messages during .env loading)
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print colored message
print_info() {
    printf "${BLUE}ℹ${NC} %s\n" "$1"
}

print_success() {
    printf "${GREEN}✓${NC} %s\n" "$1"
}

print_warning() {
    printf "${YELLOW}⚠${NC} %s\n" "$1"
}

print_error() {
    printf "${RED}✗${NC} %s\n" "$1" >&2
}

# Load configuration from .env file
# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# Go up one level to project root
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ENV_FILE="${PROJECT_ROOT}/.env"

if [ ! -f "${ENV_FILE}" ]; then
    print_error ".env file not found at ${ENV_FILE}"
    echo ""
    echo "Please create a .env file in the project root with:"
    echo "  CHROMIUM_CDP_ADDRESS=127.0.0.1"
    echo "  CHROMIUM_CDP_PORT=9226"
    echo "  CHROMIUM_START_URL=about:blank"
    exit 1
fi

# Source .env file
set -a
. "${ENV_FILE}"
set +a

# Verify required configuration
if [ -z "${CHROMIUM_CDP_PORT}" ]; then
    print_error "CHROMIUM_CDP_PORT not set in .env file"
    echo ""
    echo "Please add to your .env file:"
    echo "  CHROMIUM_CDP_PORT=9226"
    exit 1
fi

if [ -z "${CHROMIUM_CDP_ADDRESS}" ]; then
    print_error "CHROMIUM_CDP_ADDRESS not set in .env file"
    echo ""
    echo "Please add to your .env file:"
    echo "  CHROMIUM_CDP_ADDRESS=127.0.0.1"
    exit 1
fi

if [ -z "${CHROMIUM_START_URL}" ]; then
    print_error "CHROMIUM_START_URL not set in .env file"
    echo ""
    echo "Please add to your .env file:"
    echo "  CHROMIUM_START_URL=about:blank"
    exit 1
fi

# Set defaults for optional environment variables
if [ -z "${CHROMIUM_PROFILE_DIR}" ]; then
    CHROMIUM_PROFILE_DIR="./chromium-profile"
fi

if [ -z "${CHROMIUM_LOG_FILE_DIR}" ]; then
    CHROMIUM_LOG_FILE_DIR="logs"
fi

print_success "Loaded configuration from .env"
print_info "CDP Address: ${CHROMIUM_CDP_ADDRESS}"
print_info "CDP Port: ${CHROMIUM_CDP_PORT}"
print_info "Start URL: ${CHROMIUM_START_URL}"
print_info "Profile Directory: ${CHROMIUM_PROFILE_DIR}"
print_info "Log File Directory: ${CHROMIUM_LOG_FILE_DIR}"

# Configuration
DEBUG_ADDRESS="${CHROMIUM_CDP_ADDRESS}"
PROFILE_DIR="${CHROMIUM_PROFILE_DIR}"
WINDOW_SIZE="1920,1080"

# Detect available browser
detect_browser() {
    if command -v chromium-browser >/dev/null 2>&1; then
        echo "chromium-browser"
    elif command -v chromium >/dev/null 2>&1; then
        echo "chromium"
    elif command -v google-chrome >/dev/null 2>&1; then
        echo "google-chrome"
    elif [ -f "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" ]; then
        echo "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
    else
        return 1
    fi
}

# Check if port is already in use
check_port() {
    if command -v lsof >/dev/null 2>&1; then
        lsof -ti:${CHROMIUM_CDP_PORT} >/dev/null 2>&1
    elif command -v netstat >/dev/null 2>&1; then
        netstat -tuln | grep -q ":${CHROMIUM_CDP_PORT} "
    elif command -v ss >/dev/null 2>&1; then
        ss -tuln | grep -q ":${CHROMIUM_CDP_PORT} "
    else
        # Can't check, assume available
        return 1
    fi
}

# Main execution
main() {
    print_info "Starting Chromium with remote debugging..."

    # Detect browser
    if ! BROWSER=$(detect_browser); then
        print_error "No supported browser found"
        echo ""
        echo "Please install one of:"
        echo "  - chromium-browser (Linux)"
        echo "  - chromium (Linux)"
        echo "  - google-chrome (Linux)"
        echo "  - Google Chrome (macOS)"
        exit 1
    fi

    print_success "Found browser: ${BROWSER}"

    # Check port availability
    if check_port; then
        print_warning "Port ${CHROMIUM_CDP_PORT} is already in use"
        print_info "Another Chromium instance may already be running with debugging enabled"
        print_info "Or run: lsof -ti:${CHROMIUM_CDP_PORT} | xargs kill -9"
        exit 1
    fi

    # Create profile directory
    if [ ! -d "${PROFILE_DIR}" ]; then
        mkdir -p "${PROFILE_DIR}"
        print_success "Created profile directory: ${PROFILE_DIR}"
    fi

    # Display connection info
    echo ""
    print_success "Launching Chromium..."
    print_info "Remote debugging: ${DEBUG_ADDRESS}:${CHROMIUM_CDP_PORT}"
    print_info "WebSocket URL: ws://${DEBUG_ADDRESS}:${CHROMIUM_CDP_PORT}"
    print_info "Profile directory: ${PROFILE_DIR}"
    echo ""
    print_info "Browser will open: ${CHROMIUM_START_URL}"
    print_info "Press Ctrl+C to stop browser"
    echo ""

    # Create logs directory if it doesn't exist
    mkdir -p "${CHROMIUM_LOG_FILE_DIR}"

    # Build browser command with base flags
    # Flags explained:
    #   --remote-debugging-port: Enable CDP on this port
    #   --remote-debugging-address: Bind to localhost only (security)
    #   --user-data-dir: Isolated profile for testing
    #   --no-first-run: Skip welcome screens and setup
    #   --disable-dev-shm-usage: Prevent /dev/shm issues in containers
    #   --window-size: Consistent viewport for testing

    if [ "$ENABLE_LOGGING" = true ]; then
        # Launch with logging enabled
        print_info "Chromium logging enabled: ${CHROMIUM_LOG_FILE_DIR}/chromium-debug.log"
        # Clear previous log file for fresh start
        if [ -f "${CHROMIUM_LOG_FILE_DIR}/chromium-debug.log" ]; then
            truncate -s 0 "${CHROMIUM_LOG_FILE_DIR}/chromium-debug.log"
            print_info "Cleared previous log file"
        fi
        exec "${BROWSER}" \
            --remote-debugging-port=${CHROMIUM_CDP_PORT} \
            --remote-debugging-address=${DEBUG_ADDRESS} \
            --user-data-dir="${PROFILE_DIR}" \
            --no-first-run \
            --disable-dev-shm-usage \
            --window-size=${WINDOW_SIZE} \
            --enable-logging \
            --v=1 \
            --log-file="${CHROMIUM_LOG_FILE_DIR}/chromium-debug.log" \
            "${CHROMIUM_START_URL}"
    else
        # Launch without logging
        print_info "Chromium logging disabled"
        exec "${BROWSER}" \
            --remote-debugging-port=${CHROMIUM_CDP_PORT} \
            --remote-debugging-address=${DEBUG_ADDRESS} \
            --user-data-dir="${PROFILE_DIR}" \
            --no-first-run \
            --disable-dev-shm-usage \
            --window-size=${WINDOW_SIZE} \
            "${CHROMIUM_START_URL}"
    fi
}

main "$@"