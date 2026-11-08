# Build and develop Tala

[Back to the product overview](../README.md) · [User guide](GUIDE.md)

Prebuilt apps and CLI packages are available from the [latest release](https://github.com/Ajtn05/Tala/releases/latest). These instructions are for building, packaging, and checking the source.

## macOS

Install Go 1.25+, CMake, pkg-config, Git, and Xcode Command Line Tools. If needed, run `xcode-select --install` and finish its installer first. With Homebrew installed:

```sh
brew install go cmake pkg-config
git clone https://github.com/Ajtn05/Tala.git
cd Tala
make setup
make test
make build
bash scripts/bundle.sh
open 'dist/Tala.app'
```

`make setup` builds pinned whisper.cpp v1.8.3 and PortAudio v19.7.0 statically inside `third_party/`. It does **not** download a model. The dashboard assets are embedded in the binary. The app links only macOS system libraries.

Apple Silicon builds use a portable ARM baseline with Metal acceleration, so a build made on a newer Mac does not require that Mac's newer CPU instructions.

`make clean` removes development binaries, archived builds in `dist/previous-builds/`, disposable `.verification/` test output, the Go build cache, and the formatter cache. It keeps the latest app and DMG, the local Go toolchain, native build dependencies, settings, transcripts outside the test directory, and installed models. A generated directory containing a `.bin` or `.gguf` model path is preserved.

`bash scripts/package.sh` creates `dist/Tala.dmg`. The bundle is ad-hoc signed; it is not Developer ID signed or notarized. Normal macOS Gatekeeper approval may be required. Intel builds must be built on Intel; the generated binary matches the build machine.

## Linux

Install Go **1.25 or later** from [Go's downloads](https://go.dev/dl/) if your distribution does not provide it. Debian/Ubuntu build prerequisites:

```sh
sudo apt update
sudo apt install build-essential git cmake pkg-config libasound2-dev ffmpeg
# Optional, for media URLs (use a current yt-dlp package):
sudo apt install yt-dlp

git clone https://github.com/Ajtn05/Tala.git
cd Tala
make setup
make test
make build
make package # Creates dist/tala-linux-x86_64.tar.gz on x86_64
```

`make setup` builds the same pinned Whisper and PortAudio versions, with Metal disabled on Linux. It does not download a model. Linux CI builds the native libraries, runs Go tests, race checks, UI regression tests and CLI smoke checks, and packages the CLI. Real microphone hardware still needs manual validation; see the [verification notes](VERIFICATION.md).

After building, complete the [model setup step](GUIDE.md#2-download-and-verify-the-model) before your first transcription.

## Architecture

The WKWebView loads only bundled assets and uses a narrow native script bridge, without a listening HTTP server. Navigation is blocked; the content security policy prevents external assets/network calls. Transcript text is inserted as text, not HTML. Event delivery is bounded and recovers by requesting a snapshot after overflow. Slow UI delivery does not block audio capture/inference. The controller serializes GPU access, and session persistence is independent of the window lifecycle.

- `cmd/tala/`: desktop entry point and CLI commands.
- `internal/app/`: shared controller, recording state, queue, and serialized inference.
- `internal/audio/`: CoreAudio and PortAudio capture.
- `internal/media/`: FFmpeg streaming, time ranges, yt-dlp downloads and authentication.
- `internal/session/`: saved sessions, segment journals, corrections, and exports.
- `internal/models/`: model discovery, verification, downloads, and settings.
- `internal/dashboard/`: AppKit host, native dialogs/playback, and bundled UI assets.

Use the Makefile for native include paths and platform libraries. Keep models and user data outside builds; see [storage details](GUIDE.md#local-data-and-storage).

## Verification

The [Build and test workflow](https://github.com/Ajtn05/Tala/actions/workflows/build.yml) builds and tests macOS and Linux packages on pushes to `main`, pull requests, and manual runs. Each platform uploads its package as a workflow artifact. Version tags such as `v2.2.0` publish a release with both packages and SHA-256 checksums after both platform jobs pass. The macOS bundle version matches the release tag; routine CI builds do not create releases.

```sh
make test
make race
make test-setup # Offline native setup/cache recovery checks for macOS and Linux
make test-ui # Optional Node.js checks for dashboard recording, imports, and updates
# Optional real-inference parity and CPU-thread timing comparison (macOS needs Metal access):
TALA_TEST_MODEL="/path/to/ggml-large-v3-turbo.bin" make test-import-speed
# Optional real installed-model checksum integration:
TALA_TEST_MODEL="$HOME/Applications/Kian.app/Contents/Resources/models/ggml-large-v3-turbo.bin" make test
# Direct local batch smoke test, without the dashboard:
./tala --transcribe third_party/whisper.cpp/samples/jfk.mp3 --output /tmp/tala-smoke
```

See [verification notes](VERIFICATION.md) for exercised paths and remaining manual checks, and [import performance experiments](IMPORT_PERFORMANCE.md) for timing and transcript-parity results. Floating captions, playlist expansion, and current-playback segment highlighting are not implemented.

Application code is MIT; upstream license and attribution are preserved. Manrope is bundled under the [SIL Open Font License](../internal/dashboard/assets/Manrope-OFL.txt); its license is also included in the app’s Resources folder. Tala is an independent extension of [Kian](https://github.com/viktorminko/kian).
