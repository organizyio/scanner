#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

# Same targets as `make build-all` (POSIX env syntax). On Windows, prefer: make build-all
OUTPUT="dist"
echo "Building scan-cli into $OUTPUT/ (subcommands: catalog, fs)"
echo "Note: Fyne GUI (cmd/scan-gui) is not cross-built here — build natively per OS (make build-gui)."

# Linux
GOOS=linux GOARCH=amd64 go build -o "$OUTPUT/scan-cli-linux-amd64" ./cmd/scan-cli
GOOS=linux GOARCH=arm64 go build -o "$OUTPUT/scan-cli-linux-arm64" ./cmd/scan-cli

# macOS
GOOS=darwin GOARCH=amd64 go build -o "$OUTPUT/scan-cli-macos-amd64" ./cmd/scan-cli
GOOS=darwin GOARCH=arm64 go build -o "$OUTPUT/scan-cli-macos-arm64" ./cmd/scan-cli

# Windows
GOOS=windows GOARCH=amd64 go build -o "$OUTPUT/scan-cli-windows-amd64.exe" ./cmd/scan-cli
GOOS=windows GOARCH=arm64 go build -o "$OUTPUT/scan-cli-windows-arm64.exe" ./cmd/scan-cli

echo "✔ Done."
