# Kian — Project Context for Claude

## What this is

macOS menu bar app for real-time audio transcription using whisper.cpp. Captures microphone and system audio (via CoreAudio process taps), transcribes locally, writes timestamped text files to `~/Documents/Kian/`.

Tray-only app — no CLI mode, no terminal UI.

## Architecture

```
cmd/kian/main.go         Entry point. Initializes PortAudio, resolves paths, launches tray UI.
internal/
  config/config.go       Single source of truth for all defaults, labels, and tuning constants.
  audio/
    capture.go           PortAudio stream reading, silence detection, RMS-based speech detection, chunk splitting.
    device.go            Device lookup: FindDeviceForMode() dispatches to tap/mic helpers.
    mode.go              AudioMode enum: ModeBoth, ModeMicrophone, ModeSystem.
    tap.go               CoreAudio process tap creation/destruction via cgo.
    tap_darwin.m          Objective-C implementation of CoreAudio tap functions.
    tap_darwin.h          C header for tap functions (used by cgo).
    setup.go             CoreAudio device enumeration, device UID lookup.
  pipeline/pipeline.go   Orchestrates capture → transcribe → write using errgroup goroutines.
                         Creates/destroys tap device for system audio modes.
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

System audio capture uses CoreAudio process taps (macOS 14.2+). No virtual audio drivers or manual setup needed.

When recording starts with system audio enabled, the pipeline:
1. Creates a process tap via `AudioHardwareCreateProcessTap()` — captures all system audio
2. Wraps the tap in an aggregate device that PortAudio can read from
3. For `ModeBoth`, includes the built-in microphone in the aggregate (3-channel: tap L, tap R, mic)
4. Cycles PortAudio (`Terminate` + `Initialize`) so it discovers the new device
5. On recording stop, destroys the aggregate device and process tap

The app's `FindDeviceForMode()` selects the right device:
- `ModeBoth` → "Kian Tap+Mic" aggregate (3 channels: tap stereo + mic mono)
- `ModeMicrophone` → Built-in Microphone directly
- `ModeSystem` → "Kian Tap" aggregate (2 channels: tap stereo)

## .app bundle structure

```
Kian.app/
  Contents/
    MacOS/kian                              Self-contained binary (whisper.cpp statically linked)
    Resources/
      models/ggml-large-v3-turbo.bin        Whisper model (~1.5GB, downloaded by installer)
      AppIcon.icns                          App icon
    Info.plist                              LSUIElement=true (menu bar only), mic + audio capture usage descriptions
```

**Important**: Replacing the binary inside the .app revokes macOS microphone permissions silently. PortAudio reads succeed but return all zeros. The user must re-grant permission in System Settings → Privacy & Security → Microphone.

## CI

`.github/workflows/build.yml` builds the DMG on every push to main (when app code changes). Uses `macos-latest` runner, caches the whisper.cpp build, creates a GitHub Release with auto-incrementing version tag.

## Common pitfalls

- **All-zero audio from PortAudio**: macOS microphone permission not granted for the process. When launched from terminal, the terminal app itself needs mic permission. When launched as .app via `open`, the app bundle needs it.
- **Linker errors during `go build`**: Need `C_INCLUDE_PATH`, `LIBRARY_PATH`, `CGO_CFLAGS`, and `CGO_LDFLAGS` set. Use `make build` or source the env vars from the Makefile.
- **CoreAudio tap errors**: Process taps require macOS 14.2+. The `CATapDescription` and `AudioHardwareCreateProcessTap` APIs are only available in Objective-C headers (`#ifdef __OBJC__`), which is why tap code uses separate `.m`/`.h` files instead of inline C in cgo.
- **PortAudio doesn't see tap device**: PortAudio caches device list on `Initialize()`. After creating a tap, must call `Terminate()` + `Initialize()` before `FindDeviceForMode()`.

## Code style

See `~/.claude/CLAUDE.md` for global Go style rules (error handling, early returns, testing conventions).
