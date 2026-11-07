#!/usr/bin/env bash
# Build pinned native dependencies locally. No model is downloaded by setup.
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
WHISPER_DIR="$ROOT_DIR/third_party/whisper.cpp"
PA_DIR="$ROOT_DIR/third_party/portaudio"
STATIC_DIR="$ROOT_DIR/third_party/portaudio_static"
mkdir -p "$STATIC_DIR"
if [[ ! -d "$PA_DIR" ]]; then
    git clone --depth 1 --branch v19.7.0 https://github.com/PortAudio/portaudio.git "$PA_DIR"
fi
if [[ ! -d "$WHISPER_DIR" ]]; then
    git clone --depth 1 --branch "$(tr -d '[:space:]' < "$ROOT_DIR/WHISPER_VERSION")" https://github.com/ggml-org/whisper.cpp.git "$WHISPER_DIR"
fi
# The Go binding asks pkg-config for PortAudio even when linking statically.
cat > "$STATIC_DIR/portaudio-2.0.pc" <<PC
Name: PortAudio
Description: Kian local static PortAudio
Version: 19.7.0
Libs: -L$STATIC_DIR -lportaudio
Cflags: -I$PA_DIR/include
PC
if [[ "${1:-}" == "--no-build" ]]; then exit 0; fi
cmake -S "$PA_DIR" -B "$PA_DIR/build" -DCMAKE_POLICY_VERSION_MINIMUM=3.5 \
    -DPA_BUILD_SHARED=OFF -DPA_BUILD_STATIC=ON -DPA_BUILD_EXAMPLES=OFF -DPA_BUILD_TESTS=OFF \
    -DCMAKE_BUILD_TYPE=Release -DCMAKE_OSX_DEPLOYMENT_TARGET=14.2
cmake --build "$PA_DIR/build" --config Release -j "$(sysctl -n hw.ncpu)"
cp "$PA_DIR/build/libportaudio.a" "$STATIC_DIR/"
cmake -S "$WHISPER_DIR" -B "$WHISPER_DIR/build" -DGGML_METAL=ON -DBUILD_SHARED_LIBS=OFF \
    -DWHISPER_BUILD_TESTS=OFF -DWHISPER_BUILD_EXAMPLES=OFF -DCMAKE_OSX_DEPLOYMENT_TARGET=14.2 -DCMAKE_BUILD_TYPE=Release
cmake --build "$WHISPER_DIR/build" --config Release -j "$(sysctl -n hw.ncpu)"
