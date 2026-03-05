#!/usr/bin/env bash
set -euo pipefail

echo "=== Kian Dev Setup ==="

# Check prerequisites
for cmd in cmake git brew; do
    if ! command -v "$cmd" &>/dev/null; then
        echo "Error: $cmd is required but not installed."
        exit 1
    fi
done

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
bash "$SCRIPT_DIR/setup-deps.sh"

echo ""
echo "=== Setup complete ==="
echo ""
echo "Run 'make build' to build the app, or 'bash scripts/package.sh' to create the DMG."
