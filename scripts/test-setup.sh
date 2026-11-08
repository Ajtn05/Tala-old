#!/usr/bin/env bash
# Exercise native setup without network access, compilers, or installed models.
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
TALA_SETUP_TEST_WHISPER_VERSION="$(tr -d '[:space:]' < "$ROOT_DIR/WHISPER_VERSION")"
export TALA_SETUP_TEST_WHISPER_VERSION
TEST_DIR="$(mktemp -d)"
trap 'rm -rf "$TEST_DIR"' EXIT
MOCK_BIN="$TEST_DIR/bin"
mkdir -p "$MOCK_BIN"

cat > "$MOCK_BIN/uname" <<'MOCK'
#!/usr/bin/env bash
if [[ "$1" == -s ]]; then echo "$TALA_SETUP_TEST_OS"; else echo arm64; fi
MOCK
for tool in sysctl getconf; do
    printf '#!/usr/bin/env bash\necho 2\n' > "$MOCK_BIN/$tool"
done
cat > "$MOCK_BIN/pkg-config" <<'MOCK'
#!/usr/bin/env bash
if [[ "$1" == --libs ]]; then echo '-lasound'; fi
MOCK
printf '#!/usr/bin/env bash\nexit 0\n' > "$MOCK_BIN/go"
cat > "$MOCK_BIN/git" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
[[ "$1" == clone && "$2" == --depth && "$3" == 1 && "$4" == --branch ]]
version="$5"
repository="$6"
destination="$7"
printf '%s %s\n' "$repository" "$version" >> "$TALA_SETUP_TEST_CLONES"
mkdir -p "$destination/.git" "$destination/build"
printf 'fixture source\n' > "$destination/CMakeLists.txt"
printf 'upstream build source\n' > "$destination/build/upstream-source.txt"
printf 'fixture git metadata\n' > "$destination/.git/fixture"
if [[ "${TALA_SETUP_TEST_FAIL_CLONE:-}" == 1 ]]; then exit 1; fi
case "$repository" in
    https://github.com/PortAudio/portaudio.git) [[ "$version" == v19.7.0 ]] ;;
    https://github.com/ggml-org/whisper.cpp.git) [[ "$version" == "$TALA_SETUP_TEST_WHISPER_VERSION" ]] ;;
    *) exit 1 ;;
esac
MOCK
cat > "$MOCK_BIN/cmake" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
if [[ "$1" == -S ]]; then
    if [[ ! -f "$2/CMakeLists.txt" ]]; then
        echo "CMake source is missing: $2/CMakeLists.txt" >&2
        exit 1
    fi
    [[ "$3" == -B ]]
    mkdir -p "$4"
elif [[ "$1" == --build && "$2" == */portaudio/build ]]; then
    printf 'fixture static library\n' > "$2/libportaudio.a"
fi
MOCK
chmod +x "$MOCK_BIN/"*
export PATH="$MOCK_BIN:$PATH"

prepare_workspace() {
    local workspace="$1"
    mkdir -p "$workspace/scripts"
    cp "$ROOT_DIR/scripts/setup.sh" "$ROOT_DIR/scripts/setup-deps.sh" "$workspace/scripts/"
    cp "$ROOT_DIR/Makefile" "$ROOT_DIR/WHISPER_VERSION" "$workspace/"
    export TALA_SETUP_TEST_CLONES="$workspace/clones.log"
    : > "$TALA_SETUP_TEST_CLONES"
}

for platform in Darwin Linux; do
    export TALA_SETUP_TEST_OS="$platform"
    for scenario in fresh restored-build-only; do
        workspace="$TEST_DIR/$platform-$scenario"
        prepare_workspace "$workspace"
        if [[ "$scenario" == restored-build-only ]]; then
            for dependency in portaudio whisper.cpp; do
                mkdir -p "$workspace/third_party/$dependency/build"
                printf 'cached build\n' > "$workspace/third_party/$dependency/build/CMakeCache.txt"
                printf 'existing source\n' > "$workspace/third_party/$dependency/build/upstream-source.txt"
                printf 'preserve model\n' > "$workspace/third_party/$dependency/build/model.bin"
            done
        fi
        make -s -C "$workspace" setup
        [[ "$(wc -l < "$TALA_SETUP_TEST_CLONES" | tr -d ' ')" == 2 ]]
        [[ -f "$workspace/third_party/portaudio_static/libportaudio.a" ]]
        for dependency in portaudio whisper.cpp; do
            [[ -f "$workspace/third_party/$dependency/CMakeLists.txt" ]]
            [[ -f "$workspace/third_party/$dependency/.git/fixture" ]]
            if [[ "$scenario" == restored-build-only ]]; then
                [[ "$(cat "$workspace/third_party/$dependency/build/CMakeCache.txt")" == 'cached build' ]]
                [[ "$(cat "$workspace/third_party/$dependency/build/upstream-source.txt")" == 'existing source' ]]
                [[ "$(cat "$workspace/third_party/$dependency/build/model.bin")" == 'preserve model' ]]
            fi
        done
        # A complete cached checkout must not clone again.
        make -s -C "$workspace" setup
        [[ "$(wc -l < "$TALA_SETUP_TEST_CLONES" | tr -d ' ')" == 2 ]]
        echo "PASS: $platform $scenario and complete-source reuse"
    done

    workspace="$TEST_DIR/$platform-failed-clone"
    prepare_workspace "$workspace"
    mkdir -p "$workspace/third_party/portaudio/build"
    printf 'preserve model\n' > "$workspace/third_party/portaudio/build/model.bin"
    if TALA_SETUP_TEST_FAIL_CLONE=1 bash "$workspace/scripts/setup-deps.sh" --no-build; then
        echo "FAIL: setup accepted an incomplete clone" >&2
        exit 1
    fi
    [[ ! -e "$workspace/third_party/portaudio/CMakeLists.txt" ]]
    [[ ! -e "$workspace/third_party/whisper.cpp/CMakeLists.txt" ]]
    [[ "$(cat "$workspace/third_party/portaudio/build/model.bin")" == 'preserve model' ]]
    echo "PASS: $platform failed clone did not publish partial sources"
done
