package ui

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/energye/systray"

	"kian/internal/audio"
	"kian/internal/config"
	"kian/internal/pipeline"
)

type App struct {
	cfg pipeline.Config

	mu        sync.Mutex
	recording bool
	cancel    context.CancelFunc
	done      chan struct{}
	audioMode audio.Mode

	mStart      *systray.MenuItem
	mStop       *systray.MenuItem
	mOpenFolder *systray.MenuItem
	mMic        *systray.MenuItem
	mSystem     *systray.MenuItem
}

func NewApp(cfg pipeline.Config) *App {
	return &App{cfg: cfg}
}

func (a *App) Run() {
	systray.Run(a.onReady, a.onExit)
}

func (a *App) onReady() {
	systray.SetTemplateIcon(iconData, iconData)
	systray.SetTooltip("Kian")
	systray.SetOnClick(func(menu systray.IMenu) { menu.ShowMenu() })

	a.mStart = systray.AddMenuItem("▶ Start Recording", "Start audio capture and transcription")
	a.mStart.Click(func() { a.start() })

	a.mStop = systray.AddMenuItem("■ Stop Recording", "Stop audio capture")
	a.mStop.Disable()
	a.mStop.Click(func() { a.handleStopClick() })

	systray.AddSeparator()

	a.mMic = systray.AddMenuItemCheckbox("Microphone", "Capture from microphone", true)
	a.mMic.Click(func() { a.toggleAudioSource(a.mMic) })

	a.mSystem = systray.AddMenuItemCheckbox("System Audio", "Capture system audio", true)
	a.mSystem.Click(func() { a.toggleAudioSource(a.mSystem) })

	systray.AddSeparator()

	a.mOpenFolder = systray.AddMenuItem("Open Output Folder", "Open the transcript output directory")
	a.mOpenFolder.Click(func() { a.openOutputFolder() })

	systray.AddSeparator()

	mAbout := systray.AddMenuItem("About", "About Kian")
	mAbout.Click(func() { showAboutDialog() })

	systray.AddSeparator()

	mQuit := systray.AddMenuItem("Quit", "Quit Kian")
	mQuit.Click(func() {
		a.stop()
		systray.Quit()
	})
}

func (a *App) handleStopClick() {
	a.mu.Lock()
	recording := a.recording
	a.mu.Unlock()

	if !recording {
		return
	}

	a.stop()
}

func showErrorDialog(err error) {
	script := fmt.Sprintf(`display dialog "Recording stopped unexpectedly:\n\n%s" buttons {"OK"} default button "OK" with title "Kian Error" with icon stop`, err)
	exec.Command("osascript", "-e", script).Start()
}

func showAboutDialog() {
	script := `set result to button returned of (display dialog "Kian — Audio Transcription



https://getkian.app" buttons {"Website", "OK"} default button "OK" with title "About Kian" with icon note)
if result is "Website" then
	open location "https://getkian.app"
end if`
	exec.Command("osascript", "-e", script).Start()
}

func (a *App) start() {
	a.mu.Lock()
	if a.recording {
		a.mu.Unlock()
		return
	}
	a.done = make(chan struct{})
	a.recording = true
	a.mu.Unlock()

	a.mStart.Disable()
	a.mStop.Enable()
	a.mMic.Disable()
	a.mSystem.Disable()
	systray.SetIcon(iconRecordingData)

	ctx, cancel := context.WithCancel(context.Background())
	a.mu.Lock()
	a.cancel = cancel
	a.mu.Unlock()

	cfg := a.cfg
	cfg.AudioMode = a.audioMode
	cfg.OnSegment = a.onSegment

	go func() {
		defer func() {
			close(a.done)

			a.mu.Lock()
			a.recording = false
			a.cancel = nil
			a.mu.Unlock()

			a.mStop.Disable()
			a.mStart.Enable()
			a.mMic.Enable()
			a.mSystem.Enable()
			systray.SetTemplateIcon(iconData, iconData)
		}()

		if err := pipeline.Run(ctx, cfg); err != nil {
			log.Printf("pipeline error: %v", err)
			showErrorDialog(err)
		}
	}()
}

func (a *App) stop() {
	a.mu.Lock()
	cancel := a.cancel
	done := a.done
	a.mu.Unlock()

	if cancel == nil {
		return
	}

	cancel()
	<-done
}

func (a *App) toggleAudioSource(clicked *systray.MenuItem) {
	a.mu.Lock()
	recording := a.recording
	a.mu.Unlock()

	if recording {
		return
	}

	mic := a.mMic.Checked()
	sys := a.mSystem.Checked()
	if clicked == a.mMic {
		mic = !mic
	} else {
		sys = !sys
	}

	if !mic && !sys {
		return
	}

	switch {
	case mic && sys:
		a.audioMode = audio.ModeBoth
	case mic:
		a.audioMode = audio.ModeMicrophone
	default:
		a.audioMode = audio.ModeSystem
	}

	a.syncCheckmarks()
}

func (a *App) syncCheckmarks() {
	switch a.audioMode {
	case audio.ModeBoth:
		a.mMic.Check()
		a.mSystem.Check()
	case audio.ModeMicrophone:
		a.mMic.Check()
		a.mSystem.Uncheck()
	case audio.ModeSystem:
		a.mMic.Uncheck()
		a.mSystem.Check()
	}
}

func (a *App) onSegment(_ string) {}

func (a *App) openOutputFolder() {
	dir := a.cfg.OutputDir
	if dir == "" {
		dir = config.DefaultOutputDir
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(filepath.Dir(execPath()), dir)
	}

	os.MkdirAll(dir, 0o755)

	if runtime.GOOS == "darwin" {
		exec.Command("open", dir).Run()
	}
}

func (a *App) onExit() {}

func execPath() string {
	p, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(p)
}
