# Verification — November 7, 2026

Environment: Apple Silicon M5, macOS 27.0.1, Go 1.27.3 (module supports Go 1.25+), pinned whisper.cpp v1.8.3, Metal enabled. The deployment target remains macOS 14.2.

## Automated checks

- `make test`: passes all Go package tests (58 test functions; the full installed-model test is opt-in and was also run successfully).
- `make race`: passes the Go race detector across all packages. An upstream test read a bytes.Buffer concurrently with the silence flush writer; that test now observes completed writes over a channel.
- Real installed-model integration: the model in `~/Applications/Kian.app/Contents/Resources/models/ggml-large-v3-turbo.bin` passed format and full SHA-256 verification. `Ensure(download=true)` reused that same path without downloading or changing its size or modification time.
- Session tests cover sorting, source labels, duration formatting/rounding, corrected text persistence, restoring originals, all four export formats, untimed legacy handling, idempotent import, deletion tombstones, deletion boundaries, concurrent appends/corrections, traversal rejection, and recovery after a truncated journal record.
- Queue/controller tests cover source synchronization, rejecting changes during recording, duplicate recording rejection, live priority/checkpoint resume, cancellation while paused, active/waiting job cancellation, retry persistence, restart recovery, preserving completed results, concurrent snapshots/events, and nonblocking event overflow recovery.
- Media tests use synthetic 63-second WAV audio: bounded 30/30/3-second windows at offsets 0/30/60, actual metadata, decoder cancellation, invalid media rejection, URL validation, and argument isolation for URLs with shell-like characters. Processing has no playback timer.
- Model tests cover discovery order, configured directories, original bundle paths, corrupt/incompatible model rejection without modification, cancelled discovery, and durable settings.
- `bash scripts/test-install.sh`: repeated synthetic installations preserve both pre-existing app bundles and their model files; no download action is requested.
- `node --check internal/dashboard/assets/app.js`, shell syntax validation, `git diff --check`, app signature/plist checks, and `otool -L` pass. Dynamic links are only to system frameworks/libraries.

## Real inference

The upstream JFK MP3 (11 seconds) was decoded and transcribed through the production batch queue using the existing model and Metal on Apple M5. It produced:

> And so, my fellow Americans, ask not what your country can do for you, ask what you can do for your country.

The finalized segment timing was 0–10.4 seconds. The session and queue reached Completed with 11 seconds processed. A cold CLI run took about 18 seconds including model initialization; a native dashboard run under concurrent development load took about 23 seconds. These short samples do not demonstrate faster-than-real-time throughput or long-session performance.

A sandboxed test could not allocate Metal buffers and crashed inside the upstream C library. Running the same build with normal GPU access succeeded. Metal has not been disabled or replaced.

## Native UI exercise

An isolated QA bundle with the same production code and separate test data directories was launched through macOS:

- Dashboard renders in the system dark appearance and navigation works.
- Settings displays **Existing model reused** and the original full model path.
- Tool detection finds FFmpeg, ffprobe, and yt-dlp.
- Native file picker selects JFK MP3; the interface displays its actual duration and format.
- Add to queue performs real inference, with model loading/ready states and completed progress.
- Open transcript shows the real output in the shared viewer.
- Edit saves a separate correction while preserving the model-generated text in the journal.
- Native TXT save dialog exports the corrected text, verified from the resulting file.

During validation, WKWebView denied localStorage for the opaque local page origin. UI preferences now use NSUserDefaults through a restricted bridge. Native startup now initializes transcript storage after the AppKit event loop starts, so macOS can present Documents permissions. Earlier pre-event-loop launches stalled on Documents access; production-directory first-launch permission handling still needs a manual check on the final build.

## Manual checks still required

These have not been claimed as verified:

- Permission-granted microphone, system audio, and combined live sessions, including stopping/flushing and menu/dashboard synchronization with real hardware.
- English/Filipino/Taglish recognition quality on representative recordings.
- Actual YouTube/site downloads against current platform behavior (unit tests verify integration, metadata parsing, URL validation, and safe arguments).
- Long MP4 lectures, hour-long sessions, resource use, boundary accuracy, and measured sustained throughput.
- Native AVPlayer seeking for each supported playback codec; Finder drag-and-drop; light mode, minimum window size, keyboard accessibility, and restored window/session state after restart.
- Starting live recording during a real GPU-heavy import; tests exercise controller scheduling, but the combined hardware flow remains manual.
- Clean-machine installation, Documents/audio permission prompts, and Intel/macOS 14.2 compatibility.

Floating captions, playlist expansion, and playback-driven segment highlighting are deferred. Imported windows are non-overlapping; recognition at a hard boundary may be less accurate. The pinned Whisper binding may take until an encoder checkpoint to honor inference cancellation. FFmpeg and yt-dlp are external optional dependencies, not bundled. The app is ad-hoc signed, not notarized.
