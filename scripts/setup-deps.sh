#!/usr/bin/env bash
# Shared dependency setup for both local dev and CI.
# Installs portaudio, clones whisper.cpp at the pinned version, and builds it.
#
# Usage:
#   bash scripts/setup-deps.sh           # full setup (install + clone + build)
#   bash scripts/setup-deps.sh --no-build # install + clone only (CI uses cache)
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
WHISPER_DIR="$ROOT_DIR/third_party/whisper.cpp"
PORTAUDIO_STATIC_DIR="$ROOT_DIR/third_party/portaudio_static"

BUILD=true
if [[ "${1:-}" == "--no-build" ]]; then
    BUILD=false
fi

# --- portaudio ---
if command -v brew &>/dev/null; then
    if ! brew list portaudio &>/dev/null; then
        echo "Installing portaudio..."
        brew install portaudio
    fi
fi

mkdir -p "$PORTAUDIO_STATIC_DIR"
cp "$(brew --prefix portaudio)/lib/libportaudio.a" "$PORTAUDIO_STATIC_DIR/"
echo "Copied portaudio static library."

# --- whisper.cpp ---
WHISPER_COMMIT="$(tr -d '[:space:]' < "$ROOT_DIR/WHISPER_VERSION")"
if [ ! -d "$WHISPER_DIR" ]; then
    echo "Cloning whisper.cpp at $WHISPER_COMMIT..."
    git clone https://github.com/ggerganov/whisper.cpp.git "$WHISPER_DIR"
    cd "$WHISPER_DIR"
    git checkout "$WHISPER_COMMIT"
    cd "$ROOT_DIR"
else
    echo "whisper.cpp already cloned."
fi

if [ "$BUILD" = true ]; then
    echo "Building whisper.cpp..."
    cd "$WHISPER_DIR"
    cmake -B build -DGGML_METAL=ON -DBUILD_SHARED_LIBS=OFF \
        -DCMAKE_OSX_DEPLOYMENT_TARGET=13.0 \
        -DCMAKE_INCLUDE_PATH="$(pwd)/ggml/include:$(pwd)/include"
    cmake --build build --config Release -j "$(sysctl -n hw.ncpu)"
fi
