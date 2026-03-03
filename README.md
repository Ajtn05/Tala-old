# Kian

Real-time audio transcription for macOS. Captures your microphone, system audio (Google Meet, Zoom, YouTube), or both — and writes timestamped transcripts to text files.

Built with [whisper.cpp](https://github.com/ggerganov/whisper.cpp) for fast, local transcription. Nothing leaves your machine.

## Features

- **Three capture modes** — Microphone only, System Audio only, or both simultaneously
- **100% local** — all processing happens on-device using whisper.cpp with Metal acceleration
- **No drivers needed** — system audio capture uses CoreAudio process taps (macOS 14.2+)
- **Menu bar app** — lives in your menu bar, no dock icon, no window clutter
- **Timestamped output** — each line is labeled with source and time range
- **Auto echo suppression** — in "Both" mode, mic echo is suppressed when system audio is active

## How it works

1. Click the Kian icon in the menu bar
2. Choose **Microphone**, **System Audio**, or both
3. Click **Start Recording**
4. Speak or play audio — Kian transcribes in real time
5. Click **Stop Recording** — the transcript is saved to `~/Documents/Kian/`

Each transcript line is labeled with the source (`[Microphone]` or `[System Audio]`) and a time range.

## Prerequisites

- macOS 14.2+ (Sonoma or later)

That's it. No additional drivers or dependencies needed. The installer handles everything automatically.

## Installation

### Step 1: Download

Download the latest `Kian-Installer.dmg` from [Releases](../../releases).

> **Is this safe?** The DMG is built from the open-source code on [GitHub](https://github.com/viktorminko/kian) by GitHub Actions CI. You can verify the build by checking the [workflow runs](https://github.com/viktorminko/kian/actions) or build it yourself from source.

### Step 2: Open the DMG

Double-click `Kian-Installer.dmg` to mount it. You'll see **Install Kian** inside.

### Step 3: Run the installer

The app is not signed with an Apple Developer certificate, so macOS Gatekeeper will block it by default. This is normal for open-source apps distributed outside the Mac App Store. You need to explicitly allow it.

**Option A — Right-click to open (recommended):**

1. Right-click (or Control-click) **Install Kian**
2. Select **Open** from the context menu
3. A dialog will say the app is from an unidentified developer — click **Open**

This is the standard macOS way to open apps from unidentified developers. It only bypasses Gatekeeper for this specific app — your system security settings remain unchanged.

**Option B — Allow in System Settings:**

1. Double-click **Install Kian** — macOS will block it
2. Go to **System Settings → Privacy & Security**
3. Scroll down to the Security section — you'll see a message about "Install Kian" being blocked
4. Click **Open Anyway**
5. Enter your password if prompted
6. Click **Open** in the confirmation dialog

This is equivalent to Option A — it creates a one-time exception for this app only.

**Option C — Remove quarantine via Terminal:**

```bash
xattr -d com.apple.quarantine "/Volumes/Kian Installer/Install Kian.app"
```

This removes the "downloaded from the internet" flag that triggers Gatekeeper. It does not disable any system-wide security. Then double-click **Install Kian** normally.

> **Why does macOS block it?** Gatekeeper blocks apps that aren't signed with an Apple Developer certificate ($99/year). Kian is free and open source, so it's distributed unsigned. None of these methods weaken your Mac's security — they only allow this specific app to run.

### Step 4: Follow the installer

A Terminal window will open and the installer will:

1. Copy **Kian.app** to `~/Applications/`
2. Download the Whisper model (~1.5 GB) from [Hugging Face](https://huggingface.co/ggerganov/whisper.cpp) on first install
3. Launch Kian automatically when done

> **What gets installed?** Kian.app is fully self-contained. The only file placed outside `~/Applications/` is the Whisper model bundled inside the app. Nothing runs in the background when Kian is not open. You can review the full [installer script](https://github.com/viktorminko/kian/blob/main/scripts/package.sh) on GitHub.

### Step 5: Grant permissions

On first launch, macOS will ask for **microphone access**. Click **Allow**. When you start recording with System Audio enabled, macOS will also ask for **audio capture** access. Click **Allow**.

Audio is processed locally by whisper.cpp — nothing is sent over the network.

If you accidentally denied either permission, go to **System Settings → Privacy & Security → Microphone** (or **Audio Capture**) and enable Kian.

### Updating

Run the installer again. It preserves the downloaded Whisper model (~1.5 GB) so you don't have to re-download it.

### Uninstalling

Delete `~/Applications/Kian.app`. That's it — no drivers or system modifications to clean up.

## How system audio capture works

Kian uses the CoreAudio process tap API (macOS 14.2+) to capture system audio directly — no virtual audio drivers needed. When you start recording with System Audio enabled, Kian creates a temporary audio tap that captures all system audio output. The tap is automatically destroyed when you stop recording.

You'll see a purple indicator dot in the menu bar while system audio is being captured. This is macOS indicating that an app is using audio capture.

## Development

### Setup

```bash
# Install dependencies and build whisper.cpp
make setup

# Build the binary
make build

# Build the installer DMG
bash scripts/package.sh
```

### Project structure

```
cmd/kian/          Entry point
internal/
  audio/           Audio capture, device selection, CoreAudio tap management
  config/          App-wide defaults and constants
  pipeline/        Capture → transcribe → write pipeline
  transcriber/     whisper.cpp Go bindings wrapper
  ui/              macOS menu bar (systray) interface
scripts/
  setup.sh         Dev environment setup
  package.sh       DMG packaging
icons/             App icon assets
```

## License

MIT
