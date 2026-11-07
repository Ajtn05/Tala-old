#!/usr/bin/env bash
# Uses synthetic installations in an isolated directory. Never touches real apps/models.
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
TEST_DIR="$(mktemp -d)"
trap 'rm -rf "$TEST_DIR"' EXIT
SOURCE="$TEST_DIR/source/Kian Studio.app"
TARGET="$TEST_DIR/installed"
mkdir -p "$SOURCE/Contents/MacOS" "$TARGET/Kian.app/Contents/Resources/models" "$TARGET/Kian Studio.app/Contents/Resources/models"
printf 'original-model' > "$TARGET/Kian.app/Contents/Resources/models/ggml-large-v3-turbo.bin"
printf 'previous-studio-model' > "$TARGET/Kian Studio.app/Contents/Resources/models/ggml-large-v3-turbo.bin"
cat > "$SOURCE/Contents/MacOS/kian" <<'FAKE'
#!/bin/bash
printf '%s\n' "$*" >> "$KIAN_INSTALL_TEST_LOG"
printf '{"state":"reused","path":"existing-model"}\n'
FAKE
chmod +x "$SOURCE/Contents/MacOS/kian"
export KIAN_INSTALL_TEST_LOG="$TEST_DIR/calls.log"
export KIAN_NO_LAUNCH=1
bash "$ROOT_DIR/scripts/install.sh" "$SOURCE" "$TARGET"
bash "$ROOT_DIR/scripts/install.sh" "$SOURCE" "$TARGET"
[[ "$(cat "$TARGET/Kian.app/Contents/Resources/models/ggml-large-v3-turbo.bin")" == original-model ]]
[[ "$(cat "$TARGET/Kian Studio.app/Contents/Resources/models/ggml-large-v3-turbo.bin")" == previous-studio-model ]]
[[ "$(find "$TARGET" -maxdepth 1 -name 'Kian Studio*.app' | wc -l | tr -d ' ')" == 3 ]]
[[ "$(cat "$KIAN_INSTALL_TEST_LOG")" == $'--check-model\n--check-model' ]]
echo 'PASS: repeat installation preserved both original bundles/models and never requested a download.'
