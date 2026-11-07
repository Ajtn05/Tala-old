#!/usr/bin/env bash
# Non-destructive installation. Existing Kian and Tala bundles stay in place.
set -euo pipefail
SOURCE="${1:?Usage: install.sh /path/to/Tala.app [install-directory]}"
INSTALL_DIR="${2:-$HOME/Applications}"
[[ -x "$SOURCE/Contents/MacOS/tala" ]] || { echo "Invalid Tala bundle"; exit 1; }
# Verify before copying or downloading. This uses the exact same discovery rules as runtime.
if ! "$SOURCE/Contents/MacOS/tala" --check-model; then
    echo "No compatible model was found. Install without downloading and configure it in Settings."
fi
if ! "$SOURCE/Contents/MacOS/tala" --check-dependencies; then
    echo "Optional media tools are missing. Live recording remains available; see Settings for install instructions."
fi
mkdir -p "$INSTALL_DIR"
DEST="$INSTALL_DIR/Tala.app"
if [[ -e "$DEST" ]]; then
    DEST="$INSTALL_DIR/Tala $(date +%Y%m%d-%H%M%S)-$$.app"
fi
# ditto copies only the small app; models are never part of the build artifact.
ditto "$SOURCE" "$DEST"
echo "Installed: $DEST"
echo "All previous installations and models have been preserved."
# Downloads are explicit, and Ensure checks all existing models again before network access.
if [[ "${TALA_DOWNLOAD_IF_MISSING:-${KIAN_DOWNLOAD_IF_MISSING:-0}}" == "1" ]]; then
    "$DEST/Contents/MacOS/tala" --ensure-model
fi
if [[ "${TALA_NO_LAUNCH:-${KIAN_NO_LAUNCH:-0}}" != "1" ]]; then open "$DEST"; fi
