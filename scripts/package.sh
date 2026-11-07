#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
bash "$ROOT_DIR/scripts/bundle.sh"
STAGE="$(mktemp -d "$ROOT_DIR/dist/.dmg.XXXXXX")"
trap 'rm -rf "$STAGE"' EXIT
# App is a small signed bundle, with no model or extra inference engine.
ditto "$ROOT_DIR/dist/Kian Studio.app" "$STAGE/Kian Studio.app"
cp "$ROOT_DIR/scripts/install.sh" "$STAGE/install.sh"
cat > "$STAGE/Read me.txt" <<'TXT'
Kian Studio — macOS 14.2+
Drag Kian Studio.app to ~/Applications, or run install.sh with its path.
Keep your original Kian.app: its verified model is reused directly, without a copy.
Open Settings to choose a model path or explicitly download one if no valid model exists.
Models downloaded by Studio live outside the app, in ~/Library/Application Support/Kian/models.
FFmpeg and yt-dlp are external optional media tools: brew install ffmpeg yt-dlp
TXT
OUTPUT="$ROOT_DIR/dist/Kian-Studio.dmg"
if [[ -e "$OUTPUT" ]]; then
    ARCHIVE_DIR="$ROOT_DIR/dist/previous-builds/$(date +%Y%m%d-%H%M%S)-$$"
    mkdir -p "$ARCHIVE_DIR"
    mv "$OUTPUT" "$ARCHIVE_DIR/Kian-Studio.dmg"
fi
hdiutil create -volname "Kian Studio" -srcfolder "$STAGE" -format UDZO "$OUTPUT"
echo "Created: $OUTPUT"
