#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
WHISPER_DIR="$ROOT_DIR/third_party/whisper.cpp"

echo "=== Kian Dev Setup ==="

# Check prerequisites
for cmd in cmake git brew; do
    if ! command -v "$cmd" &>/dev/null; then
        echo "Error: $cmd is required but not installed."
        exit 1
    fi
done

if ! brew list portaudio &>/dev/null; then
    echo "Installing portaudio..."
    brew install portaudio
else
    echo "portaudio already installed."
fi

# Copy portaudio static library (so the linker won't find the dylib)
PORTAUDIO_STATIC_DIR="$ROOT_DIR/third_party/portaudio_static"
mkdir -p "$PORTAUDIO_STATIC_DIR"
cp "$(brew --prefix portaudio)/lib/libportaudio.a" "$PORTAUDIO_STATIC_DIR/"
echo "Copied portaudio static library."

# Clone whisper.cpp at pinned commit
WHISPER_COMMIT="$(cat "$ROOT_DIR/WHISPER_VERSION" | tr -d '[:space:]')"
if [ ! -d "$WHISPER_DIR" ]; then
    echo "Cloning whisper.cpp..."
    git clone https://github.com/ggerganov/whisper.cpp.git "$WHISPER_DIR"
    cd "$WHISPER_DIR"
    git checkout "$WHISPER_COMMIT"
    cd "$ROOT_DIR"
else
    echo "whisper.cpp already cloned."
fi

# Build whisper.cpp
echo "Building whisper.cpp..."
cd "$WHISPER_DIR"
cmake -B build -DGGML_METAL=ON -DBUILD_SHARED_LIBS=OFF -DCMAKE_OSX_DEPLOYMENT_TARGET=13.0 -DCMAKE_INCLUDE_PATH="$(pwd)/ggml/include:$(pwd)/include"
cmake --build build --config Release -j "$(sysctl -n hw.ncpu)"

echo ""
echo "=== Setup complete ==="
echo ""
echo "Run 'make build' to build the app, or 'bash scripts/package.sh' to create the DMG."
