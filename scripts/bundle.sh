#!/usr/bin/env bash
# Build side by side with the original Kian. Never remove an application or model.
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
TALA_VERSION="${TALA_VERSION:-2.1.0}"
if [[ ! "$TALA_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "TALA_VERSION must use major.minor.patch, for example 2.1.0."
    exit 1
fi
cd "$ROOT_DIR"
make build
DIST_DIR="$ROOT_DIR/dist"
APP_DIR="$DIST_DIR/Tala.app"
mkdir -p "$DIST_DIR"
STAGE="$(mktemp -d "$DIST_DIR/.bundle.XXXXXX")"
trap 'rm -rf "$STAGE"' EXIT
BUNDLE="$STAGE/Tala.app"
mkdir -p "$BUNDLE/Contents/MacOS" "$BUNDLE/Contents/Resources"
cp tala "$BUNDLE/Contents/MacOS/tala"
cp icons/AppIcon.icns "$BUNDLE/Contents/Resources/"
cp internal/dashboard/assets/Manrope-OFL.txt "$BUNDLE/Contents/Resources/"
cat > "$BUNDLE/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleName</key><string>Tala</string>
<key>CFBundleDisplayName</key><string>Tala</string>
<key>CFBundleIdentifier</key><string>io.github.ajtn05.tala</string>
<key>CFBundleVersion</key><string>$TALA_VERSION</string>
<key>CFBundleShortVersionString</key><string>$TALA_VERSION</string>
<key>CFBundleExecutable</key><string>tala</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>LSMinimumSystemVersion</key><string>14.2</string>
<key>LSUIElement</key><false/>
<key>NSMicrophoneUsageDescription</key><string>Tala transcribes microphone audio locally.</string>
<key>NSAudioCaptureUsageDescription</key><string>Tala transcribes system audio locally.</string>
<key>CFBundleIconFile</key><string>AppIcon</string>
<key>NSHighResolutionCapable</key><true/>
</dict></plist>
PLIST
codesign --force --deep --sign - "$BUNDLE"
if [[ -e "$APP_DIR" ]]; then
    BACKUP="$DIST_DIR/previous-builds/$(date +%Y%m%d-%H%M%S)-$$"
    mkdir -p "$BACKUP"
    mv "$APP_DIR" "$BACKUP/Tala.app.backup"
    echo "Previous build preserved at $BACKUP"
fi
mv "$BUNDLE" "$APP_DIR"
echo "Built: $APP_DIR"
echo "No model was bundled, duplicated, downloaded, or deleted."
