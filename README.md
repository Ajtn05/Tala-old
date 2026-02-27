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
3. Follow the Terminal prompts — it will install BlackHole and download the Whisper model (~1.5 GB)
4. On first launch, click **Setup Audio Devices...** in the menu bar to complete audio setup

## Audio setup

To capture system audio, macOS needs two virtual audio devices (Multi-Output and Aggregate). Kian's built-in setup wizard creates these automatically — just click **Setup Audio Devices...** from the menu bar.

After the wizard finishes, you need to do one manual step:

1. Go to **System Settings → Sound → Output**
2. Select **Kian Multi-Output**

You'll still hear audio through your speakers/headphones, and Kian will capture it via BlackHole.

> **Tip:** If you switch between speakers and headphones, recreate the Multi-Output Device by running the setup wizard again.

<details>
<summary>Manual setup (if you prefer)</summary>

### 1. Create a Multi-Output Device

- Open **Audio MIDI Setup** (Spotlight → "Audio MIDI Setup")
- Click **+** at the bottom-left → **Create Multi-Output Device**
- Check **your output device** (e.g. MacBook Air Speakers or your headphones) **+ BlackHole 2ch**

### 2. Create an Aggregate Device

- Click **+** → **Create Aggregate Device**
- Check **BlackHole 2ch + Built-in Microphone**
- Set **Clock Source** to **BlackHole 2ch**

### 3. Select Multi-Output Device as system output

- Go to **System Settings → Sound → Output**
- Select **Multi-Output Device**

</details>

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
