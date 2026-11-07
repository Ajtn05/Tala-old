#!/usr/bin/env bash
# Non-destructive installation. Existing Kian and Kian Studio bundles stay in place.
set -euo pipefail
SOURCE="${1:?Usage: install.sh /path/to/Kian Studio.app [install-directory]}"
INSTALL_DIR="${2:-$HOME/Applications}"
[[ -x "$SOURCE/Contents/MacOS/kian" ]] || { echo "Invalid Kian Studio bundle"; exit 1; }
# Verify before copying or downloading. This uses the exact same discovery rules as runtime.
if ! "$SOURCE/Contents/MacOS/kian" --check-model; then
    echo "No compatible model was found. Install without downloading and configure it in Settings."
fi
mkdir -p "$INSTALL_DIR"
DEST="$INSTALL_DIR/Kian Studio.app"
if [[ -e "$DEST" ]]; then
    DEST="$INSTALL_DIR/Kian Studio $(date +%Y%m%d-%H%M%S)-$$.app"
fi
# ditto copies only the small app; models are never part of the build artifact.
ditto "$SOURCE" "$DEST"
echo "Installed: $DEST"
echo "All previous installations and models have been preserved."
# Downloads are explicit, and Ensure checks all existing models again before network access.
if [[ "${KIAN_DOWNLOAD_IF_MISSING:-0}" == "1" ]]; then
    "$DEST/Contents/MacOS/kian" --ensure-model
fi
if [[ "${KIAN_NO_LAUNCH:-0}" != "1" ]]; then open "$DEST"; fi
