# Kian Studio — development context

Kian Studio extends Kian with an AppKit/WKWebView dashboard, live recording,
local media and URL imports, a sequential queue, and editable local sessions.
The original menu bar, CoreAudio taps, microphone capture, and whisper.cpp
Large v3 Turbo engine remain in use. Minimum macOS version: 14.2.

## Architecture

- `cmd/kian/`: entry point, single-instance lock, diagnostic/batch CLI flags.
- `internal/app/`: shared recording controller, one lazy-loaded model, serialized
  inference, live-priority batch scheduling, queue persistence and events.
- `internal/audio/`: PortAudio microphone capture and CoreAudio system taps.
- `internal/pipeline/`: capture, transcription, finalized events, legacy TXT output.
- `internal/transcriber/`: the pinned whisper.cpp Go binding; Metal stays enabled.
- `internal/models/`: model discovery, GGML/SHA-256 verification, safe downloads,
  and persistent recognition/model settings.
- `internal/session/`: atomic session metadata, append-only segment journals,
  separate corrections, legacy import, search data, and exports.
- `internal/media/`: FFmpeg streaming and yt-dlp integration using argument arrays.
- `internal/dashboard/`: AppKit host, bundled web assets, native dialogs/playback,
  and a script bridge with no listening HTTP server or external web content.
- `internal/ui/`: menu bar controls using the same controller as the dashboard.
- `internal/paths/`, `internal/config/`: legacy path helpers and capture defaults.

## Build and checks

```sh
make setup                  # Build pinned native dependencies; no model download
make build                  # Build the development binary
make test                   # Unit and synthetic-media tests
make race                   # Go race checks
make bundle                 # dist/Kian Studio.app
make package                # dist/Kian-Studio.dmg
bash scripts/test-install.sh # Isolated installation-preservation test
make clean                  # Remove stale builds/caches; keep latest distribution
```

Use the Makefile: it supplies include paths, static libraries, frameworks, and
pkg-config settings. The optional local Go toolchain lives in `.tools/go/`;
whisper.cpp v1.8.3 and PortAudio v19.7.0 live in ignored `third_party/` trees.
There is no Node runtime dependency. FFmpeg and yt-dlp are optional external
media tools, discovered in PATH and standard Homebrew locations.

## Preserve models and user data

Check `internal/models` before changing model handling. Discovery verifies
existing files before any download, including:

`~/Applications/Kian.app/Contents/Resources/models/ggml-large-v3-turbo.bin`

Use a valid model directly from its current location. Do not duplicate it into
new bundles. Downloads belong in `~/Library/Application Support/Kian/models/`.
Never delete the original Kian installation or its model without explicit user
confirmation. Normal builds and installs preserve existing applications.
`make clean` only targets disposable workspace artifacts, and skips directories
containing `.bin` or `.gguf` model paths.

Transcripts and queue metadata default to `~/Documents/Kian/`. Settings default
to `~/Library/Application Support/Kian/settings.json`. For isolated tests use
`KIAN_DATA_DIR` and `KIAN_OUTPUT_DIR`, or the CLI `--output` flag. Preserve
original text, corrections, local media, and legacy files when changing storage.

## Runtime constraints

- Keep one loaded Whisper model and serialize inference. Batch jobs wait at
  audio checkpoints while live recording is active.
- Keep capture/inference off the main UI thread. Events are bounded and recover
  from overflow through a snapshot; do not poll transcript files.
- Stop capture and drain queued audio before completing a live session.
- Preserve automatic multilingual detection and keep translation disabled.
- UI preferences use NSUserDefaults: opaque-origin WKWebView pages cannot rely
  on localStorage. Initialize Documents storage after AppKit starts so macOS can
  present permission prompts.
- Ad-hoc signed rebuilds may require macOS microphone/audio permission again.
  Sandboxed terminal inference may not have Metal access.

See README.md for user workflows and docs/VERIFICATION.md for verified behavior
and remaining manual checks. Do not present untested hardware/network flows as
verified.
