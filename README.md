# Kian Studio

A native macOS transcription workspace extending [Kian](https://github.com/viktorminko/kian). Go, AppKit, WKWebView, and the original whisper.cpp Large v3 Turbo engine with Metal. No accounts, subscriptions, cloud inference, Electron, or runtime Node server. macOS 14.2 or later.

## Build and open

Install Go 1.25+, CMake, pkg-config, Git, and Xcode Command Line Tools. Homebrew can install the build tools:

```sh
brew install go cmake pkg-config
make setup
make test
make build
bash scripts/bundle.sh
open 'dist/Kian Studio.app'
```

`make setup` builds pinned whisper.cpp v1.8.3 and PortAudio v19.7.0 statically inside `third_party/`. It does **not** download a model. The dashboard assets are embedded in the binary. The app links only macOS system libraries.

`make clean` removes development binaries, archived builds in `dist/previous-builds/`, disposable `.verification/` test output, the Go build cache, and the formatter cache. It keeps the latest app and DMG, the local Go toolchain, native build dependencies, settings, transcripts outside the test directory, and installed models. A generated directory containing a `.bin` or `.gguf` model path is preserved.

`bash scripts/package.sh` creates `dist/Kian-Studio.dmg`. The bundle is ad-hoc signed; it is not Developer ID signed or notarized. Normal macOS Gatekeeper approval may be required. Intel builds must be built on Intel; the generated binary matches the build machine.

## Reuse your existing model

Kian Studio verifies the GGML header and the full SHA-256 checksum before using a model. The pinned unquantized multilingual Large v3 Turbo model has SHA-256:

```text
1fc70f774d38eb169993ac391eea357ef47c88757ef72ee5943879b7e8e2bc69
```

Discovery checks, in preference order:

1. `KIAN_MODEL_PATH` and the path saved in Settings.
2. `KIAN_MODEL_DIR` and the additional model directory saved in Settings.
3. `~/Applications/Kian.app/Contents/Resources/models/ggml-large-v3-turbo.bin`.
4. `/Applications/Kian.app/Contents/Resources/models/ggml-large-v3-turbo.bin`.
5. `~/Library/Application Support/Kian/models/ggml-large-v3-turbo.bin`.
6. The running bundle's Resources/models directory and the development models directory.
7. Other `Kian*.app` installations directly under `~/Applications` and `/Applications`.

A valid existing model is opened **at its current location**, including through a symlink. No copy is made. In Settings, the status explicitly says **Existing model reused** or **Model downloaded**, and displays the resolved path. Other models, formats, or quantized variants are deliberately rejected by the pinned checksum.

If none is valid, use Settings → Choose model, enter an existing model path/directory, or explicitly choose Download model. The download action searches and verifies all candidates again before opening a network connection. Downloads go to Application Support, outside the app bundle. They use a temporary file and are published only after verification. Existing files, including invalid ones, are never overwritten or deleted. Cancellation and download errors remove only the download's own temporary file.

Read-only verification is also available from Terminal:

```sh
./kian --check-model
# Explicitly permit a download only when discovery finds no valid model:
./kian --ensure-model
```

Keep the original Kian installation while reusing a model inside it. To move away from that installation later, first copy its model to a durable directory yourself and choose that path in Settings; verify it successfully before changing anything else. Kian Studio never removes the original app or model.

## Install and update safely

Kian Studio uses a separate application name and bundle ID (`app.getkian.studio`). It can coexist with the original Kian. Stop recording in the original app before recording in Studio to avoid competing audio capture and inference.

```sh
bash scripts/install.sh 'dist/Kian Studio.app'
```

The installer checks model reuse first, installs the small app, and opens it. It never replaces an existing app: repeated installs receive a dated destination name. It never downloads a model by default. For unattended setup, explicitly set `KIAN_DOWNLOAD_IF_MISSING=1`; the same verified discovery still runs before any download. `KIAN_NO_LAUNCH=1` suppresses launch. An optional second argument selects the installation directory.

Rebuilds preserve previous bundles in `dist/previous-builds/`, including any files someone may have placed inside them. Installation and packaging scripts have no app/model deletion path. Application Support model files and settings, and Documents transcripts, survive rebuilds, updates, and reinstallations. No model is included in the 13 MB application bundle.

## Live recording

Open Live transcription, choose Microphone, System audio, or both, and start recording. The menu bar and dashboard use the same controller, source selection, and start/stop state. Changing sources requires stopping first. Closing the dashboard hides its window; capture continues. Reopen it from the menu bar.

Grant microphone and audio capture permissions when macOS asks. Settings → Privacy & Security → Microphone / Audio Capture can restore denied access. A rebuilt ad-hoc signed app may need fresh permission. Documents access may also be requested. All capture and inference stay on the Mac.

Finalized engine chunks appear immediately. There is no simulated word streaming. Stop ends capture and drains buffered chunks before marking the session complete. The existing CoreAudio taps, PortAudio microphone capture, echo suppression, and legacy text output remain in use.

## Local files and URLs

Install optional media tools once:

```sh
brew install ffmpeg yt-dlp
```

They are external tools, not bundled. Studio finds them in PATH, `/opt/homebrew/bin`, `/usr/local/bin`, or a bundle Resources/tools directory. No manual Python environment configuration is needed. Live recording works without either tool. Settings reports each tool's availability and location.

Choose Import media, select multiple files or drag them from Finder, review their actual audio format and duration, then add them to the queue. FFmpeg decodes the first audio stream from common audio/video containers. Audio is streamed as 16 kHz mono float PCM in non-overlapping 30-second windows, without playback or deliberate delays. Memory use for decoding is bounded; a whole long video is never loaded into RAM. Recognition near a window boundary may be less accurate than within a window.

For a URL, fetch its metadata, review the title and duration, and add it to the queue. Accessible YouTube videos, Shorts, direct media URLs, and sites supported by the installed yt-dlp are supported. Playlists are limited to one selected video; multi-video playlist expansion is not implemented. Restricted, private, DRM-protected, unavailable, and network-failed media report tool errors. Studio does not supply credentials or bypass access controls. Only import content you are authorized to access.

URL audio is downloaded into a private temporary directory. Temporary downloads are removed on success, error, and cancellation. An explicit Keep downloaded media option retains one durable copy under Documents/Kian/media for review. Original local files are referenced in place and never deleted. A crash or forced kill can leave an OS temporary directory; it does not affect original media.

The queue processes one job at a time and retains partial transcripts on cancellation/error. Cancel and Retry are available. A restart restores waiting and interrupted imports from the beginning in a new session; the earlier partial session remains marked Interrupted. Completed sessions remain available. Progress reflects decoded audio that has actually finished inference. Elapsed time includes preparation and any live-recording pause. Processing speed depends on the Mac and media; no fixed faster-than-real-time speed is promised.

When live recording starts, the current import finishes its active inference window, then waits at the next checkpoint until live recording stops. There is only one loaded model and inference is serialized. Stopping a batch job cancels its decoder/downloader process group and prevents subsequent inference windows. The pinned Whisper binding checks cancellation at encoder checkpoints, so stopping an active native inference call may take time.

## Review, edit, and export

Live, file, and URL sessions share one viewer. Choose Clean text, Timestamped, or Source-aware without rerunning inference. Adjust font size, auto-scroll, or search with Cmd+F. Cmd+N starts a new transcription. Standard Cmd+C / Cmd+A / Cmd+V editing shortcuts are available. The pin button keeps the dashboard above other windows.

Hover or focus a segment and choose Edit. Save correction stores it separately from the original text. Restore original removes the correction. Incoming segments do not replace saved corrections. History searches titles and corrected transcript contents; sessions can be renamed or deleted with a confirmation. Deletion removes only structured session files, preserving original media, legacy TXT files, and models. A small deletion record prevents a preserved legacy file from reappearing in history on the next launch.

Copy transcript uses the selected display mode. Text selection and Cmd+C copy a selection. Export through the native save dialog to TXT, Markdown, SRT, or VTT. Subtitle formats use actual segment times, without invented speaker identities or word timings. Untimed legacy transcripts cannot be exported as subtitles.

For retained/imported local media, Review media opens native AVPlayer controls. In timestamped/source-aware view, clicking a timestamp seeks without automatically starting playback. URL sessions with discarded media retain their original source URL. Finder playback requires a codec supported by AVFoundation; FFmpeg may transcribe additional formats that AVFoundation cannot play.

Automatic multilingual detection is the default for English, Filipino, and Taglish. Optional English/Filipino settings and a vocabulary/context hint are available. Translation is always disabled; there is no cleanup or rewriting model.

## Local data and architecture

- `~/Documents/Kian/sessions/<id>.json`: atomically written session metadata and corrections.
- `~/Documents/Kian/sessions/<id>.jsonl`: append-only finalized segment journal, synced on append. Complete records survive a truncated final record.
- `~/Documents/Kian/transcript_*.txt`: compatible live text output; existing files are imported without modification or fabricated timestamps.
- `~/Documents/Kian/queue.json`: atomically written queue metadata.
- `~/Library/Application Support/Kian/settings.json`: model path, model directory, recognition options.
- `~/Library/Application Support/Kian/models/`: verified downloads only.
- macOS preferences: window geometry, selected session, display mode, font size.

`KIAN_DATA_DIR` overrides Application Support storage for isolated testing. `--output /path` or `KIAN_OUTPUT_DIR` overrides transcript storage. The normal defaults above are unchanged.

The WKWebView loads only bundled assets and uses a narrow native script bridge, without a listening HTTP server. Navigation is blocked; the content security policy prevents external assets/network calls. Transcript text is inserted as text, not HTML. Event delivery is bounded and recovers by requesting a snapshot after overflow. Slow UI delivery does not block audio capture/inference. The controller serializes GPU access, and session persistence is independent of the window lifecycle.

## Verification

```sh
make test
make race
# Optional real installed-model checksum integration:
KIAN_TEST_MODEL="$HOME/Applications/Kian.app/Contents/Resources/models/ggml-large-v3-turbo.bin" make test
# Direct local batch smoke test, without the dashboard:
./kian --transcribe third_party/whisper.cpp/samples/jfk.mp3 --output /tmp/kian-smoke
```

See [verification notes](docs/VERIFICATION.md) for exercised paths and remaining manual checks. Floating captions, playlist expansion, and current-playback segment highlighting are not implemented.

MIT; upstream license and attribution are preserved.
