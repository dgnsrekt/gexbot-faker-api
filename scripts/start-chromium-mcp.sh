#!/bin/sh
#
# start-playwright-mcp.sh - Launch Playwright MCP server with CDP connection
#
# This script launches the Playwright MCP server configured to connect to
# a running Chromium instance via Chrome DevTools Protocol (CDP).
#
# Configuration is loaded from .env file in the project root.
# Required:
#   CHROMIUM_CDP_ADDRESS (e.g., 127.0.0.1)
#   CHROMIUM_CDP_PORT (e.g., 9226)
# Optional:
#   CHROMIUM_MCP_SERVER (default: playwright)
#     Valid values: "playwright" or "chrome-devtools"
#
# The MCP server will connect to the WebSocket URL obtained from:
#   http://${CHROMIUM_CDP_ADDRESS}:${CHROMIUM_CDP_PORT}/json/version
#

set -e

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# Go up one level to project root
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ENV_FILE="${PROJECT_ROOT}/.env"

# Load configuration from .env file
if [ ! -f "${ENV_FILE}" ]; then
    echo "ERROR: .env file not found at ${ENV_FILE}" >&2
    echo "" >&2
    echo "Please create a .env file in the project root with:" >&2
    echo "  CHROMIUM_CDP_ADDRESS=127.0.0.1" >&2
    echo "  CHROMIUM_CDP_PORT=9226" >&2
    exit 1
fi

# Source .env file
set -a
. "${ENV_FILE}"
set +a

# Verify required configuration
if [ -z "${CHROMIUM_CDP_PORT}" ]; then
    echo "ERROR: CHROMIUM_CDP_PORT not set in .env file" >&2
    echo "" >&2
    echo "Please add to your .env file:" >&2
    echo "  CHROMIUM_CDP_PORT=9226" >&2
    exit 1
fi

if [ -z "${CHROMIUM_CDP_ADDRESS}" ]; then
    echo "ERROR: CHROMIUM_CDP_ADDRESS not set in .env file" >&2
    echo "" >&2
    echo "Please add to your .env file:" >&2
    echo "  CHROMIUM_CDP_ADDRESS=127.0.0.1" >&2
    exit 1
fi

# Set default for optional environment variable
CHROMIUM_MCP_SERVER="${CHROMIUM_MCP_SERVER:-playwright}"

# Validate MCP server type
if [ "${CHROMIUM_MCP_SERVER}" != "playwright" ] && [ "${CHROMIUM_MCP_SERVER}" != "chrome-devtools" ]; then
    echo "ERROR: Invalid CHROMIUM_MCP_SERVER value: ${CHROMIUM_MCP_SERVER}" >&2
    echo "" >&2
    echo "Valid values are:" >&2
    echo "  - playwright (default)" >&2
    echo "  - chrome-devtools" >&2
    exit 1
fi

echo "MCP Server: ${CHROMIUM_MCP_SERVER}"

# Fetch the WebSocket debugger URL from the running Chromium instance
# The CDP endpoint returns JSON with the webSocketDebuggerUrl
CDP_JSON_URL="http://${CHROMIUM_CDP_ADDRESS}:${CHROMIUM_CDP_PORT}/json/version"

# Check if required tools are available
if ! command -v curl >/dev/null 2>&1; then
    echo "ERROR: curl is required but not installed" >&2
    exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
    echo "ERROR: python3 is required but not installed" >&2
    exit 1
fi

# Fetch the WebSocket URL with a timeout
CDP_ENDPOINT=$(curl -s --connect-timeout 5 "${CDP_JSON_URL}" | python3 -c "import sys, json; print(json.load(sys.stdin).get('webSocketDebuggerUrl', ''))" 2>/dev/null)

if [ -z "${CDP_ENDPOINT}" ]; then
    echo "ERROR: Could not connect to Chromium at port ${CHROMIUM_CDP_PORT}" >&2
    echo "" >&2
    echo "Please ensure Chromium is running with remote debugging enabled:" >&2
    echo "  just start-browser" >&2
    echo "  or: ./scripts/start-chromium.sh" >&2
    exit 1
fi

# Launch MCP server with CDP connection
if [ "${CHROMIUM_MCP_SERVER}" = "playwright" ]; then
    echo "Launching Playwright MCP server..."
    exec npx @playwright/mcp@latest --cdp-endpoint "${CDP_ENDPOINT}"
else
    echo "Launching Chrome DevTools MCP server..."
    exec npx -y chrome-devtools-mcp@latest --wsEndpoint="${CDP_ENDPOINT}"
fi