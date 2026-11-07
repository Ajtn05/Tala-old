#!/usr/bin/env bash
# Uses synthetic installations in an isolated directory. Never touches real apps/models.
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
TEST_DIR="$(mktemp -d)"
trap 'rm -rf "$TEST_DIR"' EXIT
SOURCE="$TEST_DIR/source/Tala.app"
TARGET="$TEST_DIR/installed"
mkdir -p "$SOURCE/Contents/MacOS" "$TARGET/Kian.app/Contents/Resources/models" "$TARGET/Tala.app/Contents/Resources/models"
printf 'original-model' > "$TARGET/Kian.app/Contents/Resources/models/ggml-large-v3-turbo.bin"
printf 'previous-tala-model' > "$TARGET/Tala.app/Contents/Resources/models/ggml-large-v3-turbo.bin"
cat > "$SOURCE/Contents/MacOS/tala" <<'FAKE'
#!/bin/bash
printf '%s\n' "$*" >> "$TALA_INSTALL_TEST_LOG"
printf '{"state":"reused","path":"existing-model"}\n'
FAKE
chmod +x "$SOURCE/Contents/MacOS/tala"
export TALA_INSTALL_TEST_LOG="$TEST_DIR/calls.log"
export TALA_NO_LAUNCH=1
bash "$ROOT_DIR/scripts/install.sh" "$SOURCE" "$TARGET"
bash "$ROOT_DIR/scripts/install.sh" "$SOURCE" "$TARGET"
[[ "$(cat "$TARGET/Kian.app/Contents/Resources/models/ggml-large-v3-turbo.bin")" == original-model ]]
[[ "$(cat "$TARGET/Tala.app/Contents/Resources/models/ggml-large-v3-turbo.bin")" == previous-tala-model ]]
[[ "$(find "$TARGET" -maxdepth 1 -name 'Tala*.app' | wc -l | tr -d ' ')" == 3 ]]
[[ "$(cat "$TALA_INSTALL_TEST_LOG")" == $'--check-model\n--check-dependencies\n--check-model\n--check-dependencies' ]]
echo 'PASS: repeat installation preserved both original bundles/models and never requested a download.'
