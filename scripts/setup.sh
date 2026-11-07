#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
export PATH="$ROOT_DIR/.tools/go/bin:$PATH"
for cmd in go cmake git pkg-config; do
    if ! command -v "$cmd" >/dev/null; then
        echo "Missing $cmd. See README.md for macOS and Linux build prerequisites."
        exit 1
    fi
done
bash "$ROOT_DIR/scripts/setup-deps.sh"
echo "Setup complete. No models were downloaded. Run make build."
