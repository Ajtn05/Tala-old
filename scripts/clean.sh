#!/usr/bin/env bash
# Remove disposable workspace output, keeping the latest distribution and models.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
for RELATIVE_PATH in tala kian kian_fullstatic 'dist/Kian Studio.app' dist/Kian-Studio.dmg dist/previous-builds .verification .gocache .tools/npm-cache; do
    TARGET="$ROOT_DIR/$RELATIVE_PATH"
    if [[ -L "$TARGET" ]]; then
        echo "Preserved symlink: $RELATIVE_PATH"
        continue
    fi
    [[ -e "$TARGET" ]] || continue

    # Never discard a model someone placed in a generated directory, including
    # a model symlink whose location may be saved in Settings.
    if [[ -d "$TARGET" ]]; then
        MODEL="$(find "$TARGET" \( -type f -o -type l \) \( -name '*.bin' -o -name '*.gguf' \) -print -quit)"
        if [[ -n "$MODEL" ]]; then
            echo "Preserved $RELATIVE_PATH: contains a model file or model path ($MODEL)"
            continue
        fi
    fi

    rm -rf -- "$TARGET"
    echo "Removed: $RELATIVE_PATH"
done

echo "Kept the latest app/DMG, source, build dependencies, and installed models."
