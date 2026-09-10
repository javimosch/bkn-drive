#!/bin/bash
# Build script for boilerplate-cli-ui-go-v2
# Compiles to a single binary with embedded UI

set -e

APP_NAME="bkn-drive"

echo "Building ${APP_NAME}..."

# Build with optimizations
go build -ldflags="-s -w" -o ${APP_NAME} .

echo "Built: ${APP_NAME}"
ls -lh ${APP_NAME}

echo ""
echo "Usage:"
echo "  ./${APP_NAME} start           # Start server with UI"
echo "  ./${APP_NAME} start -daemon   # Start as daemon"
echo "  ./${APP_NAME} stop            # Stop daemon"
echo "  ./${APP_NAME} status          # Check daemon status"
