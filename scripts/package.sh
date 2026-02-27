#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
WHISPER_DIR="$ROOT_DIR/third_party/whisper.cpp"
DIST_DIR="$ROOT_DIR/dist"
APP_DIR="$DIST_DIR/Kian.app"

export MACOSX_DEPLOYMENT_TARGET="13.0"
export C_INCLUDE_PATH="$WHISPER_DIR/include:$WHISPER_DIR/ggml/include"
export LIBRARY_PATH="$WHISPER_DIR/build/src:$WHISPER_DIR/build/ggml/src:$WHISPER_DIR/build/ggml/src/ggml-metal:$WHISPER_DIR/build/ggml/src/ggml-blas"
export CGO_CFLAGS="-I$(brew --prefix portaudio)/include"
export CGO_LDFLAGS="-L$ROOT_DIR/third_party/portaudio_static -framework Foundation -framework Metal -framework MetalKit -framework Accelerate -framework CoreAudio -framework AudioToolbox -framework AudioUnit -framework CoreFoundation -framework CoreServices"

echo "=== Building kian ==="

if [ ! -d "$WHISPER_DIR/build" ]; then
    echo "whisper.cpp not built. Run 'make setup' first."
    exit 1
fi

cd "$ROOT_DIR"
go build -o kian ./cmd/kian

echo "=== Creating Kian.app bundle ==="

rm -rf "$APP_DIR"
mkdir -p "$APP_DIR/Contents/MacOS"
mkdir -p "$APP_DIR/Contents/Resources"

# --- Info.plist ---
cat > "$APP_DIR/Contents/Info.plist" << 'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleName</key>
    <string>Kian</string>
    <key>CFBundleDisplayName</key>
    <string>Kian</string>
    <key>CFBundleIdentifier</key>
    <string>app.getkian</string>
    <key>CFBundleVersion</key>
    <string>1.0.0</string>
    <key>CFBundleShortVersionString</key>
    <string>1.0.0</string>
    <key>CFBundleExecutable</key>
    <string>kian</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>LSMinimumSystemVersion</key>
    <string>13.0</string>
    <key>LSUIElement</key>
    <true/>
    <key>NSMicrophoneUsageDescription</key>
    <string>Kian needs microphone access to transcribe audio from your meetings and conversations.</string>
    <key>CFBundleIconFile</key>
    <string>AppIcon</string>
    <key>NSHighResolutionCapable</key>
    <true/>
</dict>
</plist>
PLIST

# --- App Icon ---
cp "$ROOT_DIR/icons/AppIcon.icns" "$APP_DIR/Contents/Resources/AppIcon.icns"

# --- Binary ---
cp kian "$APP_DIR/Contents/MacOS/kian"
chmod +x "$APP_DIR/Contents/MacOS/kian"

echo "=== Creating Install Kian.app ==="

INSTALLER_APP="$DIST_DIR/Install Kian.app"
rm -rf "$INSTALLER_APP"
mkdir -p "$INSTALLER_APP/Contents/MacOS"
mkdir -p "$INSTALLER_APP/Contents/Resources"

# --- Installer Info.plist ---
cat > "$INSTALLER_APP/Contents/Info.plist" << 'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleName</key>
    <string>Install Kian</string>
    <key>CFBundleDisplayName</key>
    <string>Install Kian</string>
    <key>CFBundleIdentifier</key>
    <string>app.getkian.installer</string>
    <key>CFBundleVersion</key>
    <string>1.0.0</string>
    <key>CFBundleExecutable</key>
    <string>install</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
</dict>
</plist>
PLIST

# --- Kian.app tarball inside installer resources ---
cd "$DIST_DIR"
tar czf "$INSTALLER_APP/Contents/Resources/Kian.app.tar.gz" "Kian.app"

# --- Install script (stored as a resource, run in Terminal) ---
cat > "$INSTALLER_APP/Contents/Resources/install.sh" << 'INSTALL_SH'
#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="$HOME/Applications"
APP_NAME="Kian.app"
APP_PATH="$INSTALL_DIR/$APP_NAME"
MODEL_DIR="$APP_PATH/Contents/Resources/models"
MODEL_NAME="ggml-large-v3-turbo.bin"
MODEL_URL="https://huggingface.co/ggerganov/whisper.cpp/resolve/main/$MODEL_NAME"
ARCHIVE="$1"

echo ""
echo "=== Kian Installer ==="
echo ""

# --- Homebrew ---
if ! command -v brew &>/dev/null; then
    if [ -f /opt/homebrew/bin/brew ]; then
        eval "$(/opt/homebrew/bin/brew shellenv)"
    else
        echo "Homebrew not found. Installing..."
        /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
        eval "$(/opt/homebrew/bin/brew shellenv)"
    fi
fi

# --- BlackHole ---
if ! brew list blackhole-2ch &>/dev/null; then
    echo "Installing blackhole-2ch..."
    brew install blackhole-2ch
fi

# --- Extract app (preserve model across updates) ---
SAVED_MODEL=""
if [ -f "$MODEL_DIR/$MODEL_NAME" ]; then
    SAVED_MODEL="$(mktemp -d)/model"
    mkdir -p "$SAVED_MODEL"
    mv "$MODEL_DIR/$MODEL_NAME" "$SAVED_MODEL/"
fi

if [ -d "$APP_PATH" ]; then
    echo "Updating Kian..."
    rm -rf "$APP_PATH"
fi

echo "Installing to $APP_PATH ..."
mkdir -p "$INSTALL_DIR"
tar xzf "$ARCHIVE" -C "$INSTALL_DIR"

if [ -n "$SAVED_MODEL" ] && [ -f "$SAVED_MODEL/$MODEL_NAME" ]; then
    mkdir -p "$MODEL_DIR"
    mv "$SAVED_MODEL/$MODEL_NAME" "$MODEL_DIR/"
    rm -rf "$(dirname "$SAVED_MODEL")"
    echo "Existing model preserved."
fi

# Remove quarantine attribute
xattr -cr "$APP_PATH" 2>/dev/null || true

# --- Model ---
if [ ! -f "$MODEL_DIR/$MODEL_NAME" ]; then
    echo ""
    echo "Downloading whisper model (~1.5 GB), this may take a few minutes..."
    mkdir -p "$MODEL_DIR"
    curl -L --progress-bar -o "$MODEL_DIR/$MODEL_NAME" "$MODEL_URL"
    echo "Model downloaded."
fi

echo ""
echo "Installation complete! Launching Kian..."
echo ""

open "$APP_PATH"
INSTALL_SH
chmod +x "$INSTALLER_APP/Contents/Resources/install.sh"

# --- Launcher: opens Terminal with install.sh ---
cat > "$INSTALLER_APP/Contents/MacOS/install" << 'LAUNCHER'
#!/usr/bin/env bash
DIR="$(cd "$(dirname "$0")" && pwd)"
RESOURCES="$(dirname "$DIR")/Resources"
ARCHIVE="$RESOURCES/Kian.app.tar.gz"

# Write a temp script that calls install.sh with the archive path,
# because 'open -a Terminal' doesn't forward arguments.
TMPSCRIPT=$(mktemp /tmp/kian_install.XXXXXX.sh)
cat > "$TMPSCRIPT" << EOF
#!/usr/bin/env bash
rm -f "$TMPSCRIPT"
exec "$RESOURCES/install.sh" "$ARCHIVE"
EOF
chmod +x "$TMPSCRIPT"
open -a Terminal "$TMPSCRIPT"
LAUNCHER
chmod +x "$INSTALLER_APP/Contents/MacOS/install"

echo "=== Creating DMG ==="

DMG_DIR="$DIST_DIR/dmg_staging"
DMG_PATH="$DIST_DIR/Kian-Installer.dmg"

rm -rf "$DMG_DIR" "$DMG_PATH"
mkdir -p "$DMG_DIR"
cp -R "$INSTALLER_APP" "$DMG_DIR/"

hdiutil create -volname "Kian Installer" \
    -srcfolder "$DMG_DIR" \
    -ov -format UDZO \
    "$DMG_PATH"

rm -rf "$DMG_DIR"

SIZE=$(du -sh "$DMG_PATH" | cut -f1)
echo ""
echo "=== Done ==="
echo "DMG: $DMG_PATH ($SIZE)"
echo ""
echo "Send this file. The recipient:"
echo "  1. Double-clicks the .dmg"
echo "  2. Right-clicks 'Install Kian' -> Open"
echo "  3. Clicks 'Open' in the dialog"

rm -f "$ROOT_DIR/kian"
