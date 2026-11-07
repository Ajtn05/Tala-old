#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
bash "$ROOT_DIR/scripts/bundle.sh"
STAGE="$(mktemp -d "$ROOT_DIR/dist/.dmg.XXXXXX")"
trap 'rm -rf "$STAGE"' EXIT
# App is a small signed bundle, with no model or extra inference engine.
ditto "$ROOT_DIR/dist/Tala.app" "$STAGE/Tala.app"
cp "$ROOT_DIR/scripts/install.sh" "$STAGE/install.sh"
cat > "$STAGE/Read me.txt" <<'TXT'
Tala — local multilingual transcription for macOS 14.2+
Drag Tala.app to ~/Applications, or run install.sh with its path. No previous Kian installation is required.
English, Filipino/Tagalog, Taglish, and other Whisper languages are supported. Leave Language on Automatic / multilingual to preserve mixed-language speech.
Keep your original Kian.app: its verified model is reused directly, without a copy.
Open Settings to choose a model path or explicitly download one if no valid model exists.
Models downloaded by Tala live outside the app, in ~/Library/Application Support/Kian/models.
New transcription > Import audio or video and Settings check FFmpeg, ffprobe, and yt-dlp. Use Check again after installing optional media tools: brew install ffmpeg yt-dlp
Live recording works without these media tools.
TXT
OUTPUT="$ROOT_DIR/dist/Tala.dmg"
if [[ -e "$OUTPUT" ]]; then
    ARCHIVE_DIR="$ROOT_DIR/dist/previous-builds/$(date +%Y%m%d-%H%M%S)-$$"
    mkdir -p "$ARCHIVE_DIR"
    mv "$OUTPUT" "$ARCHIVE_DIR/Tala.dmg"
fi
hdiutil create -volname "Tala" -srcfolder "$STAGE" -format UDZO "$OUTPUT"
echo "Created: $OUTPUT"
