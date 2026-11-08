# Tala

[![Build and test](https://github.com/Ajtn05/Tala/actions/workflows/build.yml/badge.svg)](https://github.com/Ajtn05/Tala/actions/workflows/build.yml)

Local transcription with a native dashboard on macOS 14.2 or later and a headless command-line interface on Linux. Record your microphone or system audio, or transcribe audio/video files and supported media URLs. Tala is free, with no accounts, subscriptions, or cloud inference. It builds on [Kian](https://github.com/viktorminko/kian) and Whisper Large v3 Turbo with Metal acceleration on macOS and CPU inference on Linux. See [Linux and command-line usage](#linux-and-command-line-usage).

**Multilingual by design:** transcribe English, Filipino/Tagalog, Taglish, and other languages supported by Whisper. Automatic language detection preserves the original speech; Tala does not translate, formalize, or rewrite the transcript.

## Downloads

Prebuilt packages are available from the [latest release](https://github.com/Ajtn05/Tala/releases/latest).

| Platform | Download | Requirements |
| --- | --- | --- |
| macOS app | [Tala.dmg](https://github.com/Ajtn05/Tala/releases/latest/download/Tala.dmg) | Apple Silicon (M1 or later), macOS 14.2 or later |
| Linux CLI | [tala-linux-x86_64.tar.gz](https://github.com/Ajtn05/Tala/releases/latest/download/tala-linux-x86_64.tar.gz) | x86_64; built on Ubuntu 24.04, with compatible glibc, ALSA, C++ and OpenMP runtimes |
| Checksums | [SHA256SUMS.txt](https://github.com/Ajtn05/Tala/releases/latest/download/SHA256SUMS.txt) | SHA-256 checksums for both packages |

The downloads contain no transcription model. Before your first transcription, complete the [macOS model-download step](#3-download-the-transcription-model-once) or the [Linux model-download step](#2-download-and-verify-the-model). Intel Mac users can [build from source](#build-from-source). Linux installation and commands are [below](#linux-and-command-line-usage).

## Quickstart — no existing Kian installation needed

Tala works on its own. You need **macOS 14.2 or later**, an internet connection for the initial model download, and at least **2 GB of free space** for the app and model. Media imports may need additional temporary space.

### 1. Install and open Tala

Download [Tala.dmg](https://github.com/Ajtn05/Tala/releases/latest/download/Tala.dmg) for Apple Silicon from the [latest release](https://github.com/Ajtn05/Tala/releases/latest). For an Intel Mac, follow [Build from source](#build-from-source) below.

Open the DMG, copy **Tala.app** into `~/Applications/` (create that folder in your home folder if needed), and open the installed app. You do not need Go or the development tools to run a prebuilt app.

The app is ad-hoc signed. If macOS blocks opening it, use the app's **Open** action in Finder or **System Settings → Privacy & Security → Open Anyway**. Tala appears in the Dock while running. Click its Dock icon to reopen the window. Optional recording controls are available through **Settings → Application → Show Tala in the menu bar**, which is off by default. The title bar is integrated into the interface; standard close, minimize, and full-screen controls remain available.

### 2. Check optional media dependencies

Open **Settings → Media tools**. Tala automatically checks for **ffmpeg**, **ffprobe**, and **yt-dlp**, shows which are available, and explains what is missing for each workflow.

| What you want to do                    | Extra tools needed          |
| -------------------------------------- | --------------------------- |
| Record microphone or system audio live | None                        |
| Import a local audio/video file        | FFmpeg and ffprobe          |
| Import a media URL                     | FFmpeg, ffprobe, and yt-dlp |

For local files, install FFmpeg; its Homebrew package includes ffprobe:

```sh
brew install ffmpeg
```

For URL imports as well:

```sh
brew install ffmpeg yt-dlp
```

If `brew` is not installed, first install [Homebrew](https://brew.sh/). Then return to the app and click **Check again**. You do not need to restart the app or configure a Python environment. Imports stay disabled until their required tools are found; live recording remains available.

### 3. Download the transcription model once

Open **Settings** and wait for the existing-model check to finish. On a fresh Mac with no compatible model, click **Download model (1.6 GB)**. Wait until Settings shows **Model downloaded**. The app verifies the complete model checksum before using it.

The model is saved at `~/Library/Application Support/Kian/models/ggml-large-v3-turbo.bin`, outside the app bundle, and survives updates and reinstalls. If a compatible model already exists, Tala shows **Existing model verified** and uses it without another download or copy. You never need to install the original Kian first.

### 4. Make your first transcript

Leave **Settings → Language → Automatic / multilingual** selected for mixed-language speech, including Taglish. Optional English and Filipino/Tagalog settings are available for recordings in one language. The same language settings apply to live, file, and URL transcription.

- **Live:** open **New transcription**, select your audio source in the live transcription bar, and click **Start recording**. Grant microphone/audio capture access when requested, then click **Stop recording** when finished.
- **File:** on **New transcription**, drop an audio/video file into the upload area or click **choose files**, review its details, optionally set **Start** and **End**, and click **Add to queue**.
- **URL:** paste a supported media URL into the link field on **New transcription**, click **Fetch details**, optionally set **Start** and **End**, then **Add to queue**. For links requiring an account, expand **URL sign-in helper**.

The home screen shows the processing queue and recent sessions in lists that expand with the window height. Scroll either list to see more entries. Use **Sessions** or **Queue** in the top navigation to view the full lists; Settings is alongside them in the top navigation. Open a session to edit, copy, or export it. **Copy text**, **Format**, and **Export file…** stay at the bottom of the window while the transcript scrolls. Transcripts are saved in `~/Documents/Kian/`; allow Documents access if macOS asks. After initial setup, live recording and local-file transcription work offline. URL imports require internet access.

For a Terminal dependency check without opening the dashboard or downloading anything:

```sh
"$HOME/Applications/Tala.app/Contents/MacOS/tala" --check-dependencies
```

This prints the resolved tool paths and readiness for local-file and URL imports. It exits with status `1` if any media tool is missing; that does not prevent live recording.

## Build from source

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

## Linux and command-line usage

Linux supports headless file/URL transcription, microphone recording through PortAudio/ALSA, saved-session listing, and TXT/Markdown/SRT/VTT/JSON exports. The AppKit dashboard, menu bar, and CoreAudio system-audio taps require macOS. Linux currently uses CPU inference; there is no Linux desktop UI or CUDA setup in this build.

### Install the prebuilt Linux CLI

#### 1. Download and extract Tala

Download [tala-linux-x86_64.tar.gz](https://github.com/Ajtn05/Tala/releases/latest/download/tala-linux-x86_64.tar.gz) from the [latest release](https://github.com/Ajtn05/Tala/releases/latest), then extract it:

```sh
tar -xzf tala-linux-x86_64.tar.gz
./tala --help
```

#### 2. Download and verify the model

Before recording or transcribing, download Whisper Large v3 Turbo once. You need an internet connection and at least **2 GB of free space** for Tala and the approximately **1.6 GB model**:

```sh
./tala --ensure-model
./tala --check-model
```

Wait for both commands to finish successfully. Tala checks for a compatible existing model before downloading and verifies the full SHA-256 checksum before using it. `--check-model` prints the verified model path without downloading anything.

On Linux, the default download location is `~/.local/share/tala/models/ggml-large-v3-turbo.bin`; XDG and `TALA_DATA_DIR` overrides are described below. The model stays outside the extracted program and survives CLI updates. Local-file and microphone transcription can then work offline.

#### 3. Transcribe your first file

```sh
./tala --transcribe lecture.mp4
```

The package is built and tested on Ubuntu 24.04 x86_64. Install compatible ALSA, C++ and OpenMP runtime libraries through your distribution if needed. File imports require FFmpeg and ffprobe; URL imports also require yt-dlp. Go and CMake are only needed to build from source.

### Build the Linux CLI from source

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

`make setup` builds the same pinned Whisper and PortAudio versions, with Metal disabled on Linux. It does not download a model. Linux CI builds the native libraries, runs Go tests, race checks, UI regression tests and CLI smoke checks, and packages the CLI. Real microphone hardware still needs manual validation; see the [verification notes](docs/VERIFICATION.md).

After building, complete [Step 2: Download and verify the model](#2-download-and-verify-the-model) before your first transcription.

To reuse a compatible model stored elsewhere, include its path in each command instead of downloading another copy:

```sh
./tala --model /path/to/ggml-large-v3-turbo.bin --check-model
./tala --model /path/to/ggml-large-v3-turbo.bin --transcribe lecture.mp4
```

Linux stores verified downloads in `$XDG_DATA_HOME/tala/models` (default `~/.local/share/tala/models`), settings in `$XDG_CONFIG_HOME/tala/settings.json` (default `~/.config/tala/settings.json`), and transcripts in `$XDG_DATA_HOME/tala/transcripts`. `TALA_DATA_DIR` isolates settings/models and `TALA_OUTPUT_DIR` or `--output` selects transcript storage. Relative XDG environment paths are ignored.

The same CLI commands work on macOS. Put options before positional inputs; quote paths and URLs:

```sh
# Print text to stdout and save a structured local session.
./tala --transcribe lecture.mp4
./tala --single-line --transcribe lecture.mp4 > lecture.txt
./tala --format srt --transcribe lecture.mp4 > lecture.srt
./tala --language auto --prompt 'Ateneo, Kubernetes, barangay' --transcribe lecture.mp4
./tala --transcribe 'https://example.org/media'

# Transcribe only the selected part; timestamps remain on the original timeline.
./tala --start 1:30 --end 5:00 --transcribe lecture.mp4
./tala --cookies-from-browser firefox --start 90 --end 300 --transcribe 'https://example.org/media'
# Optional profile selection; these options do not change saved settings.
./tala --cookies-from-browser chrome --browser-profile 'Profile 1' --transcribe 'https://example.org/media'
./tala --cookies-from-browser zen --transcribe 'https://example.org/media'
# Dia or another unlisted browser: use a Netscape cookie export.
./tala --cookies /path/to/cookies.txt --transcribe 'https://example.org/media'
# Disable saved browser sign-in for this invocation.
./tala --cookies-from-browser= --transcribe 'https://example.org/media'

# Process multiple inputs sequentially with one model.
./tala --format json first.wav second.mp4 > sessions.json
# Multiple JSON results form a stream of JSON documents, not one JSON array.

# Record the default microphone for one minute, or omit duration and stop with Ctrl+C.
./tala --record --duration 1m --format txt

# List and export existing sessions without loading a model or starting queued work.
./tala --list
./tala --export SESSION_ID --single-line > transcript.txt
./tala --export SESSION_ID --format vtt > transcript.vtt
./tala --help
```

Recognition flags apply only to the invocation; `--prompt=''` clears a saved vocabulary hint for that run. `--format` accepts `txt`, `md`, `srt`, `vtt`, or `json`. `--text-mode` accepts `clean`, `continuous`, `timestamps`, or `sources`; `--single-line` is an alias for `continuous`. Subtitle output requires one input per invocation and always preserves segment timings.

Progress and errors go to stderr; stdout contains the selected result format. Sessions and partial transcripts are saved locally. Ctrl+C cancels file/URL work and exits with status 130; for live recording, it stops capture, drains buffered speech, saves, and exports the result. `--duration` begins when recording starts. A headless invocation does not resume earlier waiting jobs from the dashboard. Close an app using the same data directory before starting a recording or transcription CLI command; use separate `TALA_DATA_DIR` and `--output` directories for isolated workspaces.

On Linux, microphone recording uses an available PortAudio input, including ALSA devices named “default” or “hw:0.” System audio and combined capture are macOS-only; on Linux, record your monitor source using your audio tools, then transcribe the saved file.

## Reuse your existing model

Tala verifies the GGML header and the full SHA-256 checksum before using a model. The pinned unquantized multilingual Large v3 Turbo model has SHA-256:

```text
1fc70f774d38eb169993ac391eea357ef47c88757ef72ee5943879b7e8e2bc69
```

Discovery checks, in preference order:

1. `TALA_MODEL_PATH` (or legacy `KIAN_MODEL_PATH`) and the path saved in Settings.
2. `TALA_MODEL_DIR` (or legacy `KIAN_MODEL_DIR`) and the additional model directory saved in Settings.
3. `~/Applications/Kian.app/Contents/Resources/models/ggml-large-v3-turbo.bin`.
4. `/Applications/Kian.app/Contents/Resources/models/ggml-large-v3-turbo.bin`.
5. `~/Library/Application Support/Kian/models/ggml-large-v3-turbo.bin`.
6. The running bundle's Resources/models directory and the development models directory.
7. Other `Kian*.app` or `Tala*.app` installations directly under `~/Applications` and `/Applications`.

A valid existing model is opened **at its current location**, including through a symlink. No copy is made. In Settings, the status explicitly says **Existing model reused** or **Model downloaded**, and displays the resolved path. Other models, formats, or quantized variants are deliberately rejected by the pinned checksum.

If none is valid, use Settings → Choose model, enter an existing model path/directory, or explicitly choose Download model. The download action searches and verifies all candidates again before opening a network connection. Downloads go to Application Support, outside the app bundle. They use a temporary file and are published only after verification. Existing files, including invalid ones, are never overwritten or deleted. Cancellation and download errors remove only the download's own temporary file.

Read-only verification is also available from Terminal:

```sh
./tala --check-model
# Explicitly permit a download only when discovery finds no valid model:
./tala --ensure-model
```

Keep the original Kian installation while reusing a model inside it. To move away from that installation later, first copy its model to a durable directory yourself and choose that path in Settings; verify it successfully before changing anything else. Tala never removes the original app or model.

## Install and update safely

Tala uses a separate application name and bundle ID (`io.github.ajtn05.tala`). It can coexist with the original Kian. Stop recording in the original app before recording in Tala to avoid competing audio capture and inference.

```sh
bash scripts/install.sh 'dist/Tala.app'
```

The installer checks model reuse first, installs the small app, and opens it. It never replaces an existing app: repeated installs receive a dated destination name. It never downloads a model by default. For unattended setup, explicitly set `TALA_DOWNLOAD_IF_MISSING=1`; the same verified discovery still runs before any download. `TALA_NO_LAUNCH=1` suppresses launch. An optional second argument selects the installation directory.

Rebuilds preserve previous bundles in `dist/previous-builds/`, including any files someone may have placed inside them. Installation and packaging scripts have no app/model deletion path. Application Support model files and settings, and Documents transcripts, survive rebuilds, updates, and reinstallations. No model is included in the 13 MB application bundle.

## Live recording

Open New transcription, choose Microphone, System audio, or both in the recording bar, and start recording. The menu bar and dashboard use the same controller, source selection, and start/stop state. Changing sources requires stopping first. Closing the dashboard hides its window; capture continues. Reopen it from the Dock, or the menu bar if enabled.

Grant microphone and audio capture permissions when macOS asks. Settings → Privacy & Security → Microphone / Audio Capture can restore denied access. A rebuilt ad-hoc signed app may need fresh permission. Documents access may also be requested. All capture and inference stay on the Mac.

Finalized engine chunks appear immediately. There is no simulated word streaming. Stop ends capture and drains buffered chunks before marking the session complete. The existing CoreAudio taps, PortAudio microphone capture, echo suppression, and legacy text output remain in use.

## Local files and URLs

Install optional media tools once:

```sh
brew install ffmpeg yt-dlp
```

They are external tools, not bundled. Tala finds them in PATH, `/opt/homebrew/bin`, `/usr/local/bin`, or a bundle Resources/tools directory. No manual Python environment configuration is needed. Live recording works without these tools. Settings reports each tool's availability, location, and readiness for local files versus URLs. The home screen links to Settings when an import tool is missing. Use **Check again** after installing tools; the app also checks again when you open New transcription and before enqueueing or processing an import. Missing dependencies are reported before any media download or model loading. The Terminal equivalent is `./tala --check-dependencies`.

On New transcription, select multiple files or drag them from Finder into the upload area, review their actual audio format and duration, then add them to the queue. FFmpeg decodes the first audio stream from common audio/video containers. Audio is streamed as 16 kHz mono float PCM in non-overlapping 30-second windows, without playback or deliberate delays. Decoding reads one window ahead while Whisper processes the current window. Windows containing only exact zero samples skip inference while preserving their place in the timeline; any nonzero sample, however quiet, still reaches Whisper. Memory use for decoding is bounded; a whole long video is never loaded into RAM. Recognition near a window boundary may be less accurate than within a window.

For a URL, fetch its metadata, review the title and duration, and add it to the queue. Accessible YouTube videos, Shorts, direct media URLs, and sites supported by the installed yt-dlp are supported. Playlists are limited to one selected video; multi-video playlist expansion is not implemented. Restricted, private, DRM-protected, unavailable, and network-failed media report tool errors. Browser sign-in is optional and off by default. It uses your existing browser session through yt-dlp and does not bypass site access controls. Only import content you are authorized to access.

### Select a time range

Each reviewed file and fetched link has optional **Start** and **End** fields. Enter seconds (`90`), minutes and seconds (`1:30`), or hours, minutes, and seconds (`1:02:30`), with up to three decimal places for milliseconds. Blank Start means the beginning; blank End means the end of the media. End must be after Start and within the known media duration. Each file can have its own range.

Only the selected audio is decoded and transcribed. Progress and session duration count the selected audio, while transcript timestamps, playback seeks, and SRT/VTT exports retain the original media times. A `1:30`–`5:00` selection processes 3 minutes 30 seconds and begins its transcript at or after `1:30`. Ranges survive retries and app restarts. Original files remain unchanged. URL imports currently download the full source before decoding the selection; retained media also stays complete for accurate playback.

The CLI supports the same selection with `--start` and `--end`, applied to every input in that invocation. These flags apply only to imports; `--duration` remains the live-recording limit.

### URL sign-in helper

Expand **URL sign-in helper** below the link field, or in **Settings → Media tools**:

1. Choose the browser where the media plays while signed in. Optionally enter a profile name or path, such as `Default` or `Profile 1`; leaving it blank lets yt-dlp choose the most recently used profile. Safari does not support profile selection here.
2. Click **Open site to sign in**, or **Copy URL** and paste it into any browser. Switch to the intended browser profile if necessary, and sign in on the website.
3. Click **Save sign-in settings**, then **Fetch details** again. Retry failed queue items after correcting sign-in settings.

The selection applies to metadata lookup and new URL downloads. Active downloads keep their current selection. Choose **Off — public links** and save to disable it. Tala saves only the browser/profile selection or a selected cookie-file path; it does not collect passwords or export browser cookies. The [yt-dlp browser-cookie option](https://github.com/yt-dlp/yt-dlp#filesystem-options) reads the browser session locally when needed. If extraction fails, close the browser and retry, and check macOS Keychain or file-access prompts. Some restrictions still require an updated yt-dlp or remain unsupported. See the [upstream cookie guidance](https://github.com/yt-dlp/yt-dlp/wiki/FAQ#how-do-i-pass-cookies-to-yt-dlp).


**Zen:** select **Zen** for automatic discovery of its own profile directory on macOS or Linux. Tala uses yt-dlp's Firefox-compatible reader with that directory; it does not read your Firefox profile instead. For a specific or nonstandard installation, open `about:support` in Zen and paste its **Profile Folder** path into Tala's profile field.

**Dia and other browsers:** select **Dia (cookies.txt)** or **Other browser (cookies.txt)**. Dia opens with the native browser action; Other uses your default browser. **Copy URL** also works with any browser. After signing in, export that site's cookies in Netscape `cookies.txt` format and use **Choose file…** (or enter its local path). **How to export cookies** opens the upstream instructions. Then save and fetch details again. This is cookie-file support for Dia; automatic extraction from Dia's encrypted cookie store is not implemented. Pasting a page URL alone cannot share a signed-in session.

Cookie exports grant account access: keep them private. Tala passes a private temporary copy to yt-dlp because yt-dlp may update its cookie jar. The copy is removed on completion, failure, or cancellation, and the selected original is never modified. A forced process kill can leave a private OS temporary directory. Expired exports must be refreshed in the browser. The CLI's `--cookies PATH` uses the same behavior, without changing saved settings.

### Downloads and queue

URL audio is downloaded into a private temporary directory. Temporary downloads are removed on success, error, and cancellation. An explicit Keep downloaded media option retains one durable copy under Documents/Kian/media for review. Original local files are referenced in place and never deleted. A crash or forced kill can leave an OS temporary directory; it does not affect original media.

The queue processes one job at a time and retains partial transcripts on cancellation/error. Cancel and Retry are available. A restart restores waiting and interrupted imports from the beginning in a new session; the earlier partial session remains marked Interrupted. Completed sessions remain available. **Clear queue** removes the full queue list and cancels active/waiting imports after confirmation; saved sessions, partial transcripts, and original media remain available. Progress reflects decoded audio that has actually finished inference. Elapsed time includes preparation and any live-recording pause. Processing speed depends on the Mac and media; no fixed faster-than-real-time speed is promised.

When live recording starts, the current import finishes its active inference window, then waits at the next checkpoint until live recording stops. There is only one loaded model and inference is serialized. Stopping a batch job cancels its decoder/downloader process group and prevents subsequent inference windows. The pinned Whisper binding checks cancellation at encoder checkpoints, so stopping an active native inference call may take time.

## Review, edit, and export

Live, file, and URL sessions share one viewer. Choose Text only · paragraphs, Text only · single line, Timestamps, or Sources + timestamps without rerunning inference. Single line joins segments and collapses whitespace when copying or exporting plain text; the viewer still wraps to the window width. Subtitle timings stay unchanged. Adjust font size, auto-scroll, or search with Cmd+F. Cmd+N starts a new transcription. Standard Cmd+C / Cmd+A / Cmd+V editing shortcuts are available. The pin button keeps the dashboard above other windows. **Settings → Appearance** offers Slate, Graphite, Evergreen, Iris, Tangerine, and Lagoon color schemes, with system, light, or dark appearance. Choices persist across launches. The bundled Manrope variable font works offline.

Hover or focus a segment and choose Edit. Save correction stores it separately from the original text. Restore original removes the correction. Incoming segments do not replace saved corrections. History searches titles and corrected transcript contents. Sort by date, title, or duration, and group by day, month, source, or status. Day groups use your local calendar date. Sort and group choices persist across launches; sessions can be renamed or deleted with a confirmation. Deletion removes only structured session files, preserving original media, legacy TXT files, and models. A small deletion record prevents a preserved legacy file from reappearing in history on the next launch.

Copy transcript uses the selected display mode. Text selection and Cmd+C copy a selection. Export through the native save dialog to TXT, Markdown, SRT, or VTT. Subtitle formats use actual segment times, without invented speaker identities or word timings. Untimed legacy transcripts cannot be exported as subtitles.

For retained/imported local media, Review media opens native AVPlayer controls. In timestamped/source-aware view, clicking a timestamp seeks without automatically starting playback. URL sessions with discarded media retain their original source URL. Finder playback requires a codec supported by AVFoundation; FFmpeg may transcribe additional formats that AVFoundation cannot play.

Automatic multilingual detection is the default for English, Filipino, and Taglish. Optional English/Filipino settings and vocabulary hints are available. List expected names, acronyms, or specialist words, or give a short description, such as “A Taglish lecture about Kubernetes at Ateneo de Manila.” Whisper uses the hint to guide recognition of new audio; it is not a replacement dictionary or a rewriting instruction, and spelling is not guaranteed. Leave it blank for general recordings. Translation is always disabled; there is no cleanup or rewriting model.

## Local data and architecture

- `~/Documents/Kian/sessions/<id>.json`: atomically written session metadata and corrections.
- `~/Documents/Kian/sessions/<id>.jsonl`: append-only finalized segment journal, synced on append. Complete records survive a truncated final record.
- `~/Documents/Kian/transcript_*.txt`: compatible live text output; existing files are imported without modification or fabricated timestamps.
- `~/Documents/Kian/queue.json`: atomically written queue metadata.
- `~/Library/Application Support/Kian/settings.json`: model path, model directory, recognition options, optional menu bar icon, and URL browser/profile selection or cookie-file path (browser sign-in is off by default).
- `~/Library/Application Support/Kian/models/`: verified downloads only.
- macOS preferences: window geometry, selected session, display mode, font size.

`TALA_DATA_DIR` overrides Application Support storage for isolated testing. `--output /path` or `TALA_OUTPUT_DIR` overrides transcript storage. Legacy `KIAN_DATA_DIR`, `KIAN_OUTPUT_DIR`, `KIAN_MODEL_PATH`, and `KIAN_MODEL_DIR` remain supported; their `TALA_` equivalents take precedence. The installer also accepts the legacy `KIAN_DOWNLOAD_IF_MISSING` and `KIAN_NO_LAUNCH` names.

The `Kian` storage folder names above are intentionally retained so existing model settings, downloads, transcripts, and pending jobs remain available after the rename. Tala's native window/session preferences also read the previous Kian Studio preferences on first use.

The WKWebView loads only bundled assets and uses a narrow native script bridge, without a listening HTTP server. Navigation is blocked; the content security policy prevents external assets/network calls. Transcript text is inserted as text, not HTML. Event delivery is bounded and recovers by requesting a snapshot after overflow. Slow UI delivery does not block audio capture/inference. The controller serializes GPU access, and session persistence is independent of the window lifecycle.

## Verification

The [Build and test workflow](https://github.com/Ajtn05/Tala/actions/workflows/build.yml) builds and tests macOS and Linux packages on pushes to `main`, pull requests, and manual runs. Each platform uploads its package as a workflow artifact. Version tags such as `v2.1.0` publish a release with both packages and SHA-256 checksums after both platform jobs pass. The macOS bundle version matches the release tag; routine CI builds do not create releases.

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

See [verification notes](docs/VERIFICATION.md) for exercised paths and remaining manual checks, and [import performance experiments](docs/IMPORT_PERFORMANCE.md) for timing and transcript-parity results. Floating captions, playlist expansion, and current-playback segment highlighting are not implemented.

Application code is MIT; upstream license and attribution are preserved. Manrope is bundled under the [SIL Open Font License](internal/dashboard/assets/Manrope-OFL.txt); its license is also included in the app’s Resources folder. Tala is an independent extension of Kian; the original project is credited above.
