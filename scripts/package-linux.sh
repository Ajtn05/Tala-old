#!/usr/bin/env bash
# Package the headless CLI without models or user data.
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
if [[ "$(uname -s)" != Linux ]]; then
    echo "Build Linux packages on a Linux host. Use make package on macOS for the DMG."
    exit 1
fi
cd "$ROOT_DIR"
make build
mkdir -p dist
OUTPUT="$ROOT_DIR/dist/tala-linux-$(uname -m).tar.gz"
if [[ -e "$OUTPUT" ]]; then
    ARCHIVE_DIR="$ROOT_DIR/dist/previous-builds/$(date +%Y%m%d-%H%M%S)-$$"
    mkdir -p "$ARCHIVE_DIR"
    mv "$OUTPUT" "$ARCHIVE_DIR/"
fi
tar -czf "$OUTPUT" tala README.md LICENSE WHISPER_VERSION
echo "Created: $OUTPUT"
