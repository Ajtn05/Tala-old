# Verification — November 7, 2026

Environment: Apple Silicon M5, macOS 27.0.1, Go 1.27.3 (module supports Go 1.25+), pinned whisper.cpp v1.8.3, Metal enabled. The deployment target remains macOS 14.2.

## Home layout overhaul — November 8, 2026

The home screen now contains live recording controls, file and URL import review, a processing queue preview, and recent sessions in independently scrollable lists that grow with the window height. The appearance refinement moves Settings into the tabs, keeps media diagnostics in Settings, removes repeated page headings, and uses the same rounded surfaces for history, queue, and settings. Manrope is bundled locally. Six color schemes and system/light/dark appearance are saved in native preferences, along with session sort/group choices. Compact top navigation replaces the sidebar. The existing light/dark colors, native bridge, transcript viewer, and pinned export controls are retained.

- `make test`, `make race`, `make test-ui` (twelve checks), `make bundle`, JavaScript syntax validation, and diff whitespace checks pass.
- UI regression checks cover file drops and deduplication, source selection and recording requests, enqueueing reviewed files, preserving link drafts during background updates, invalid-link feedback, discarding stale metadata, session sorting/grouping (including local calendar days around midnight), preference persistence, and queue-clear confirmation. Controller tests verify durable clearing, active cancellation, preserved sessions/live recording, late worker callbacks, and rollback when persistence fails.
- The bundled HTML/CSS/JavaScript was exercised in WebKit and Chromium with a mocked native bridge and explicitly synthetic sessions/jobs. Both appearances render at 1100 × 760; smaller layouts were checked at 680 × 480 and 390 × 844. No horizontal overflow or JavaScript errors were observed. The desktop home screen uses the available height for its queue and session lists; taller windows show more entries. Checks with 25 sessions and 12 queued imports confirmed growth, scrolling, preserved scroll positions during updates, and long-title containment.
- Browser checks also exercise all six schemes in both appearances, font loading under the production content security policy, sorting/grouping, and queue clearing. Browser interaction checks cover URL metadata and retained-media queueing, file review/queueing, recording start/stop requests, session navigation, copy/export requests, keyboard navigation, transcript search focus, empty states, and missing model/media-tool states. These checks validate the UI and bridge requests, not real capture, downloads, or inference.
- An isolated native preview with separate application identity and data/output directories rendered the new home screen in WKWebView. Settings navigation and Command-N worked. The existing model was verified in place and installed media tools were detected. The bundled Manrope font and new appearance controls also rendered in the native preview. Iris, Title A–Z sorting, and Source grouping remained selected after quitting and relaunching the isolated app. No recording or real import was started for this UI check.

## Automated checks

- `make test`: passes all Go package tests (64 test functions; the full installed-model test is opt-in and was also run successfully).
- `make race`: passes the Go race detector across all packages. An upstream test read a bytes.Buffer concurrently with the silence flush writer; that test now observes completed writes over a channel.
- Real installed-model integration: the model in `~/Applications/Kian.app/Contents/Resources/models/ggml-large-v3-turbo.bin` passed format and full SHA-256 verification. `Ensure(download=true)` reused that same path without downloading or changing its size or modification time.
- Session tests cover sorting, source labels, duration formatting/rounding, corrected text persistence, restoring originals, all four export formats, untimed legacy handling, idempotent import, deletion tombstones, deletion boundaries, concurrent appends/corrections, traversal rejection, and recovery after a truncated journal record.
- Queue/controller tests cover source synchronization, rejecting changes during recording, duplicate recording rejection, live priority/checkpoint resume, cancellation while paused, active/waiting job cancellation, retry persistence, restart recovery, preserving completed results, concurrent snapshots/events, and nonblocking event overflow recovery.
- Media tests use synthetic 63-second WAV audio: bounded 30/30/3-second windows at offsets 0/30/60, actual metadata, decoder cancellation, invalid media rejection, URL validation, and argument isolation for URLs with shell-like characters. Processing has no playback timer.
- Model tests cover discovery order, configured directories, original bundle paths, corrupt/incompatible model rejection without modification, cancelled discovery, and durable settings.
- `bash scripts/test-install.sh`: repeated synthetic installations preserve both pre-existing app bundles and their model files; no download action is requested.
- `node --check internal/dashboard/assets/app.js`, shell syntax validation, `git diff --check`, app signature/plist checks, and `otool -L` pass. Dynamic links are only to system frameworks/libraries.

## Tala rename and dependency checks

The application is now Tala (`io.github.ajtn05.tala`), with the `tala` executable and Go module. Build/install scripts, release artifact names, menu/window branding, the quickstart, and the documentation page use Tala. The inherited upstream custom-domain file was removed; no domain or hosted repository was renamed.

Additional checks cover missing/all-installed media tools, local imports without yt-dlp, ffprobe supplied by the FFmpeg package, non-executable paths and directories, tools discovered after installation, and new `TALA_` environment overrides alongside legacy `KIAN_` compatibility. `./tala --check-dependencies` reports the installed Homebrew paths and readiness for both file and URL imports without loading Whisper. Both the UI bridge and queue enforce requirements before downloading or processing media. The installer reports optional missing tools without blocking live recording.

Native UI verification with an isolated Tala QA bundle confirmed the Tala name/window, prominent English/Filipino/Taglish multilingual text, Import media tool readiness, the Check again action, resolved tool paths in Settings, Automatic / multilingual recognition, and reuse of the original Kian model. Existing Kian storage locations and predecessor preferences are retained for compatibility. Renaming has not changed inference or translation settings; real language-quality testing remains in the manual list below.

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

## Navigation, drops, settings, text layout, and Linux CLI revision

- The sidebar now starts recordings/imports through New transcription. Live transcription and Import media are no longer permanent sidebar tabs. A recording shortcut appears only during capture. Page headers use workspace/navigation context instead of repeating the page heading.
- External file drops are accepted in the web content and resolve paths from the native drag pasteboard. A native destination remains available for the outer view. Selection is additive and duplicate paths are ignored. Node regression tests execute the production drag handlers with a mocked native bridge, covering file drags without File.path, path delivery/probing, repeated drops, and ordinary text drags. This does not establish a successful physical Finder-to-window drag; that remains a manual check.
- Vocabulary settings explain recognition hints with examples and distinguish them from substitutions or rewriting instructions. The optional menu bar item defaults to off and can be changed immediately, including without a model or during recording. It is created/removed natively, with AppKit startup, Dock reopening, and quitting independent of a tray library. Startup keeps the original macOS thread locked before main can migrate.
- Native QA in a separate app/data directory confirmed the simplified navigation, a single live page heading with navigation context above it, vocabulary guidance, the off/on/off settings checkbox, and continuous transcript display with the footer still visible. Settings persisted menuBarIcon=false after toggling. Command-Q closed the QA app; the user's running Tala instance was left running.
- Single-line plain text joins corrected segments and collapses whitespace without a terminal newline. Paragraph output remains available; SRT/VTT retain their timings. A real 80-segment saved-session CLI export contained 7,478 characters and no line breaks. Session tests cover corrected/multiline/empty segments and unchanged subtitle timing.
- A real headless JFK MP3 transcription used the existing verified model and Metal, completed all 11 decoded seconds, and printed only the expected transcript on stdout as a single line. No model download/copy was created in the isolated data directory.
- The CLI supports file/URL inputs, sequential multi-input processing, invocation-only language/vocabulary/model options, timed or Ctrl+C microphone recording, list/export commands, JSON, and the text layouts. Completion uses locked job snapshots instead of relying on bounded UI events. Ctrl+C cancels file/URL work; live stop drains buffered capture. Headless runs leave earlier waiting dashboard jobs alone. Tests cover argument validation, model-free listing/export, missing/cancelled model checks, preference persistence during recording, and preservation of earlier queued jobs.
- Linux excludes AppKit/CoreAudio host code, builds CPU Whisper plus static PortAudio/ALSA, uses XDG data/config paths, supplies apt-based dependency guidance, and packages a CLI tarball. A dedicated Ubuntu 24.04 CI job is configured for builds, tests, race checks, and artifact upload. The Linux package graph selects desktop_linux.go without dashboard/ui dependencies. Pure Go packages cross-compile for Linux. No Linux runtime, ALSA hardware recording, full native Linux link, or Linux CI run was available in this macOS session; those results are not claimed as verified.
- For this revision: make test, make race, make test-ui, make bundle, JavaScript/shell syntax checks, and diff whitespace checks pass. The real CLI inference and native UI checks described above used isolated data directories. Hardware recording, including timed recording and stop/drain on Linux, remains unverified end to end.

## Manual checks still required

### Utility interface revision

The bundle now uses `LSUIElement=false` and regular AppKit activation for Dock presence. The separate title strip is removed using full-size content and a transparent, hidden title. Native window controls, a drag surface, and standard Close/Minimize/Zoom menus remain. Dock/menu quit requests use the controller's shutdown path, and reopening deminiaturizes the window.

The interface uses slate surfaces with blue actions in both appearances. Promotional copy and decorative language sections were removed; language choices remain in Recognition settings. Media diagnostics are expandable below import controls. The waveform SVG is the shared source for the dashboard mark, Dock icon, and menu bar icons; `python3 scripts/generate-icons.py` regenerates the icon assets with ImageMagick and macOS sips.

Checks for this revision:

- `make test`, `make bundle`, JavaScript syntax, shell syntax, formatting, diff whitespace, bundle plist, and strict code-signature verification pass.
- Native QA uses separate support/transcript directories and an 80-segment synthetic transcript. Copy/format/export remain on screen when scrolling deep into the transcript. The native TXT export dialog saved the test transcript and the resulting file content was inspected.
- Native empty-session controls disable copy/export. Search displays “No matching text.” Recording, import, and settings pages were inspected in dark appearance. Window zoom, close/reopen, and Command-Q were exercised. The final app menu has one application menu, Edit, and Window.
- The production HTML/CSS/JavaScript also rendered in an isolated browser fixture at 680 × 480 in light appearance. DOM measurements confirmed no horizontal overflow and an export button within the window bounds. This checks layout, not native media integration.
- `make package` succeeds with normal access to macOS disk-image services; sandboxed disk-image creation fails with “Device not configured.” The generated ICNS is recognized as a 1024 × 1024 icon by sips.

Native minimum-size dragging, a direct Dock-icon interaction, and quitting during an active hardware recording have not been verified in this revision.

### Remaining end-to-end checks

These have not been claimed as verified:

- Permission-granted microphone, system audio, and combined live sessions, including stopping/flushing and menu/dashboard synchronization with real hardware.
- English/Filipino/Taglish recognition quality on representative recordings.
- Actual YouTube/site downloads against current platform behavior (unit tests verify integration, metadata parsing, URL validation, and safe arguments).
- Long MP4 lectures, hour-long sessions, resource use, boundary accuracy, and measured sustained throughput.
- Native AVPlayer seeking for each supported playback codec; Finder drag-and-drop; light mode, minimum window size, keyboard accessibility, and restored window/session state after restart.
- Starting live recording during a real GPU-heavy import; tests exercise controller scheduling, but the combined hardware flow remains manual.
- Clean-machine installation, Documents/audio permission prompts, and Intel/macOS 14.2 compatibility.

Floating captions, playlist expansion, and playback-driven segment highlighting are deferred. Imported windows are non-overlapping; recognition at a hard boundary may be less accurate. The pinned Whisper binding may take until an encoder checkpoint to honor inference cancellation. FFmpeg and yt-dlp are external optional dependencies, not bundled. The app is ad-hoc signed, not notarized.
