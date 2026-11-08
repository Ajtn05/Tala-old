#!/usr/bin/env bash
# Build pinned native dependencies locally. No model is downloaded by setup.
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
WHISPER_DIR="$ROOT_DIR/third_party/whisper.cpp"
PA_DIR="$ROOT_DIR/third_party/portaudio"
STATIC_DIR="$ROOT_DIR/third_party/portaudio_static"
PA_OPTIONS=()
WHISPER_OPTIONS=()
PORTAUDIO_LIBS=""
case "$(uname -s)" in
    Darwin)
        BUILD_JOBS="$(sysctl -n hw.ncpu)"
        PA_OPTIONS+=(-DCMAKE_OSX_DEPLOYMENT_TARGET=14.2)
        WHISPER_OPTIONS+=(-DCMAKE_OSX_DEPLOYMENT_TARGET=14.2 -DGGML_METAL=ON -DGGML_NATIVE=OFF)
        if [[ "$(uname -m)" == arm64 ]]; then
            # Keep binaries built on newer Macs compatible with the M1 baseline.
            WHISPER_OPTIONS+=(-DGGML_CPU_ARM_ARCH=armv8.2-a+dotprod+fp16)
        fi
        ;;
    Linux)
        BUILD_JOBS="$(getconf _NPROCESSORS_ONLN)"
        if ! pkg-config --exists alsa; then
            echo "ALSA development files are required. On Debian/Ubuntu: sudo apt install libasound2-dev"
            exit 1
        fi
        PA_OPTIONS+=(-DPA_USE_ALSA=ON -DPA_USE_JACK=OFF -DPA_USE_OSS=OFF)
        PORTAUDIO_LIBS="$(pkg-config --libs alsa) -lpthread -ldl -lm"
        WHISPER_OPTIONS+=(-DGGML_METAL=OFF -DGGML_BLAS=OFF -DGGML_NATIVE=OFF)
        ;;
    *) echo "Supported build platforms: macOS and Linux."; exit 1 ;;
esac
mkdir -p "$STATIC_DIR"
# A restored build-only cache creates these directories without their sources.
# Check the source entry point, and merge a complete clone without overwriting
# existing build artifacts, local files, or models. Publish only successful clones.
prepare_source() (
    local source_dir="$1" repository="$2" version="$3" source_stage
    if [[ -f "$source_dir/CMakeLists.txt" ]]; then return; fi
    source_stage="$(mktemp -d)"
    trap 'rm -rf -- "$source_stage"' EXIT
    git clone --depth 1 --branch "$version" "$repository" "$source_stage/source"
    mkdir -p "$source_dir"
    cp -R -n "$source_stage/source/." "$source_dir/"
    if [[ ! -f "$source_dir/CMakeLists.txt" ]]; then
        echo "Native source preparation failed: $source_dir/CMakeLists.txt is missing." >&2
        return 1
    fi
)
prepare_source "$PA_DIR" https://github.com/PortAudio/portaudio.git v19.7.0
prepare_source "$WHISPER_DIR" https://github.com/ggml-org/whisper.cpp.git "$(tr -d '[:space:]' < "$ROOT_DIR/WHISPER_VERSION")"
# The Go binding asks pkg-config for PortAudio even when linking statically.
cat > "$STATIC_DIR/portaudio-2.0.pc" <<PC
Name: PortAudio
Description: Tala local static PortAudio
Version: 19.7.0
Libs: -L$STATIC_DIR -lportaudio $PORTAUDIO_LIBS
Cflags: -I$PA_DIR/include
PC
if [[ "${1:-}" == "--no-build" ]]; then exit 0; fi
cmake -S "$PA_DIR" -B "$PA_DIR/build" -DCMAKE_POLICY_VERSION_MINIMUM=3.5 \
    -DPA_BUILD_SHARED=OFF -DPA_BUILD_STATIC=ON -DPA_BUILD_EXAMPLES=OFF -DPA_BUILD_TESTS=OFF \
    -DCMAKE_BUILD_TYPE=Release "${PA_OPTIONS[@]}"
cmake --build "$PA_DIR/build" --config Release -j "$BUILD_JOBS"
cp "$PA_DIR/build/libportaudio.a" "$STATIC_DIR/"
# Pinned ggml's Metal Objective-C sources do not inherit all public include
# directories. CPATH covers Objective-C as well as C/C++ on clean CI runners.
export CPATH="$WHISPER_DIR/ggml/include:$WHISPER_DIR/ggml/src${CPATH:+:$CPATH}"
cmake -S "$WHISPER_DIR" -B "$WHISPER_DIR/build" -DBUILD_SHARED_LIBS=OFF \
    -DWHISPER_BUILD_TESTS=OFF -DWHISPER_BUILD_EXAMPLES=OFF -DCMAKE_BUILD_TYPE=Release "${WHISPER_OPTIONS[@]}"
cmake --build "$WHISPER_DIR/build" --config Release -j "$BUILD_JOBS"
