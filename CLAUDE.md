# Kian — Project Context for Claude

## What this is

macOS menu bar app for real-time audio transcription using whisper.cpp. Captures microphone and system audio (via BlackHole virtual audio driver), transcribes locally, writes timestamped text files to `~/Documents/Kian/`.

Tray-only app — no CLI mode, no terminal UI.

## Architecture

```
cmd/kian/main.go         Entry point. Initializes PortAudio, resolves paths, launches tray UI.
internal/
  config/config.go       Single source of truth for all defaults, labels, and tuning constants.
  audio/
    capture.go           PortAudio stream reading, silence detection, RMS-based speech detection, chunk splitting.
    device.go            Device lookup: FindDeviceForMode() dispatches to aggregate/blackhole/mic helpers.
    mode.go              AudioMode enum: ModeBoth (aggregate), ModeMicrophone, ModeSystemAudio.
  pipeline/pipeline.go   Orchestrates capture → transcribe → write using errgroup goroutines.
                         Extracted methods: captureAudio(), transcribeChunks() writeSegments() flushPending().
  transcriber/
    transcriber.go       whisper.cpp Go bindings wrapper. 16kHz mono input required.
  ui/tray.go             systray menu bar app. Start/Stop recording, audio mode checkboxes, open output folder.
```

## Build

whisper.cpp is cloned into `third_party/whisper.cpp` (not a git submodule — just a clone, gitignored).

```bash
make setup              # Clones whisper.cpp, installs portaudio, builds whisper.cpp with Metal
make build              # Builds the Go binary (requires whisper.cpp build artifacts)
bash scripts/package.sh # Creates Kian.app bundle + Install Kian.app + DMG
```

The Makefile sets `C_INCLUDE_PATH`, `LIBRARY_PATH`, `CGO_CFLAGS`, and `CGO_LDFLAGS` for static linking against whisper.cpp and PortAudio. whisper.cpp is built with `-DBUILD_SHARED_LIBS=OFF` and PortAudio's static library (`libportaudio.a`) is copied to `third_party/portaudio_static/` so the linker doesn't find the dylib. The resulting binary has zero non-system dynamic dependencies. Without the env vars, `go build` will fail with linker errors — this is expected.

## Key dependencies

- `github.com/gordonklaus/portaudio` — Go bindings for PortAudio
- `github.com/ggerganov/whisper.cpp/bindings/go` — Go bindings for whisper.cpp (local replace in go.mod)
- `github.com/energye/systray` — macOS system tray with `SetOnClick` for left-click menu
- `golang.org/x/sync/errgroup` — concurrent pipeline stages

## Audio routing (macOS)

System audio capture requires two virtual devices. The built-in setup wizard (`internal/ui/wizard.go`) creates these automatically via CoreAudio API:
- **Kian Multi-Output**: routes audio to speakers/headphones AND BlackHole 2ch simultaneously
- **Kian Aggregate**: combines BlackHole 2ch (system audio) + Built-in Microphone into one input

The app's `FindDeviceForMode()` selects the right device:
- `ModeBoth` → Aggregate Device (3+ channels: BlackHole stereo + mic mono)
- `ModeMicrophone` → Built-in Microphone directly
- `ModeSystemAudio` → BlackHole 2ch directly

## .app bundle structure

```
Kian.app/
  Contents/
    MacOS/kian                              Self-contained binary (whisper.cpp statically linked)
    Resources/
      models/ggml-large-v3-turbo.bin        Whisper model (~1.5GB, downloaded by installer)
      AppIcon.icns                          App icon
    Info.plist                              LSUIElement=true (menu bar only), mic usage description
```

**Important**: Replacing the binary inside the .app revokes macOS microphone permissions silently. PortAudio reads succeed but return all zeros. The user must re-grant permission in System Settings → Privacy & Security → Microphone.

## CI

`.github/workflows/build.yml` builds the DMG on every push to main (when app code changes). Uses `macos-latest` runner, caches the whisper.cpp build, creates a GitHub Release with auto-incrementing version tag.

## Common pitfalls

- **All-zero audio from PortAudio**: macOS microphone permission not granted for the process. When launched from terminal, the terminal app itself needs mic permission. When launched as .app via `open`, the app bundle needs it.
- **Linker errors during `go build`**: Need `C_INCLUDE_PATH`, `LIBRARY_PATH`, `CGO_CFLAGS`, and `CGO_LDFLAGS` set. Use `make build` or source the env vars from the Makefile.
- **Audio capture returns silence despite permission**: Check that Multi-Output Device is selected as system output (not MacBook Speakers directly). BlackHole only receives audio when the Multi-Output Device routes to it.

## Code style

See `~/.claude/CLAUDE.md` for global Go style rules (error handling, early returns, testing conventions).
