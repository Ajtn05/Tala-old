#!/usr/bin/env bash
# Build side by side with the original Kian. Never remove an application or model.
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"
make build
DIST_DIR="$ROOT_DIR/dist"
APP_DIR="$DIST_DIR/Kian Studio.app"
mkdir -p "$DIST_DIR"
STAGE="$(mktemp -d "$DIST_DIR/.bundle.XXXXXX")"
trap 'rm -rf "$STAGE"' EXIT
BUNDLE="$STAGE/Kian Studio.app"
mkdir -p "$BUNDLE/Contents/MacOS" "$BUNDLE/Contents/Resources"
cp kian "$BUNDLE/Contents/MacOS/kian"
cp icons/AppIcon.icns "$BUNDLE/Contents/Resources/"
cat > "$BUNDLE/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleName</key><string>Kian Studio</string>
<key>CFBundleDisplayName</key><string>Kian Studio</string>
<key>CFBundleIdentifier</key><string>app.getkian.studio</string>
<key>CFBundleVersion</key><string>2.0.0</string>
<key>CFBundleShortVersionString</key><string>2.0.0</string>
<key>CFBundleExecutable</key><string>kian</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>LSMinimumSystemVersion</key><string>14.2</string>
<key>LSUIElement</key><true/>
<key>NSMicrophoneUsageDescription</key><string>Kian Studio transcribes microphone audio locally.</string>
<key>NSAudioCaptureUsageDescription</key><string>Kian Studio transcribes system audio locally.</string>
<key>CFBundleIconFile</key><string>AppIcon</string>
<key>NSHighResolutionCapable</key><true/>
</dict></plist>
PLIST
codesign --force --deep --sign - "$BUNDLE"
if [[ -e "$APP_DIR" ]]; then
    BACKUP="$DIST_DIR/previous-builds/$(date +%Y%m%d-%H%M%S)-$$"
    mkdir -p "$BACKUP"
    mv "$APP_DIR" "$BACKUP/Kian Studio.app.backup"
    echo "Previous build preserved at $BACKUP"
fi
mv "$BUNDLE" "$APP_DIR"
echo "Built: $APP_DIR"
echo "No model was bundled, duplicated, downloaded, or deleted."
