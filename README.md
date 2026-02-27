# Kian

A macOS menu bar app that transcribes audio in real time. It captures both your microphone and system audio (e.g. Google Meet, YouTube, Zoom) and writes timestamped transcripts to text files.

Built with [whisper.cpp](https://github.com/ggerganov/whisper.cpp) for fast, local transcription — nothing is sent to the cloud.

## How it works

1. Click the Kian icon in the menu bar
2. Choose **Microphone**, **System Audio**, or both
3. Click **Start Recording**
4. Speak or play audio — Kian transcribes in real time
5. Click **Stop Recording** — the transcript is saved to `~/Documents/Kian/`

Each transcript line is labeled with the source (`[Microphone]` or `[System Audio]`) and a timestamp.

## Prerequisites

- macOS 13.0+
- [Homebrew](https://brew.sh)

The installer handles the rest automatically, but under the hood Kian needs:

- **PortAudio** — audio capture library
- **BlackHole 2ch** — virtual audio driver that routes system audio to Kian
- **whisper.cpp model** (~1.5 GB, downloaded on first install)

## Installation

1. Download the latest `Kian-Installer.dmg` from [Releases](../../releases)
2. Open the DMG, right-click **Install Kian** and select **Open**
3. Follow the prompts — it will install dependencies and download the model
4. Complete the one-time audio setup (see below)

## Audio setup

To capture system audio, macOS needs two virtual audio devices. The installer opens Audio MIDI Setup and walks you through this, but here's the summary:

### 1. Create a Multi-Output Device

This sends audio to both your speakers (or headphones) and BlackHole simultaneously.

- Open **Audio MIDI Setup** (Spotlight → "Audio MIDI Setup")
- Click **+** at the bottom-left → **Create Multi-Output Device**
- Check **your output device** (e.g. MacBook Air Speakers or your headphones) **+ BlackHole 2ch**

### 2. Create an Aggregate Device

This combines BlackHole (system audio) and your microphone into a single input that Kian can capture from.

- Click **+** → **Create Aggregate Device**
- Check **BlackHole 2ch + Built-in Microphone**
- Set **Clock Source** to **BlackHole 2ch**

### 3. Select Multi-Output Device as system output

- Go to **System Settings → Sound → Output**
- Select **Multi-Output Device**

You'll still hear audio through your speakers/headphones, and Kian will capture it via BlackHole.

> **Tip:** If you switch between speakers and headphones, update the Multi-Output Device to include the current output device.

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
  audio/           Audio capture, device selection, silence detection
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
