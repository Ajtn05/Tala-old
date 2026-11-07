package ui

import (
	"github.com/energye/systray"
	"kian/internal/app"
	"kian/internal/audio"
	"kian/internal/dashboard"
	"log"
	"os/exec"
)

type App struct {
	initialize               func() (*app.Controller, error)
	controller               *app.Controller
	host                     *dashboard.Host
	start, stop, mic, system *systray.MenuItem
}

func NewApp(c *app.Controller) *App                              { return &App{controller: c, host: dashboard.New(c)} }
func NewLazyApp(initialize func() (*app.Controller, error)) *App { return &App{initialize: initialize} }
func (a *App) Run() {
	systray.Run(func() {
		if a.initialize != nil {
			systray.SetTemplateIcon(iconData, iconData)
			systray.SetTooltip("Opening Kian Studio…")
			c, e := a.initialize()
			if e != nil {
				log.Printf("open workspace: %v", e)
				item := systray.AddMenuItem("Cannot open transcript storage: "+e.Error(), "Check Documents access in Privacy & Security")
				item.Disable()
				quit := systray.AddMenuItem("Quit", "Quit Kian Studio")
				quit.Click(systray.Quit)
				return
			}
			a.controller = c
			a.host = dashboard.New(c)
		}
		a.onReady()
	}, a.onExit)
}
func (a *App) onReady() {
	systray.SetTemplateIcon(iconData, iconData)
	systray.SetTooltip("Kian Studio")
	systray.SetOnClick(func(menu systray.IMenu) { menu.ShowMenu() })
	open := systray.AddMenuItem("Open Dashboard", "Review transcripts, import media, and manage settings")
	open.Click(a.host.Show)
	systray.AddSeparator()
	a.start = systray.AddMenuItem("Start Recording", "Record with the selected sources")
	a.start.Click(func() {
		if e := a.controller.Start(); e != nil {
			a.controller.Emit("error", e.Error())
			a.host.Show()
		}
	})
	a.stop = systray.AddMenuItem("Stop Recording", "Finish and save the recording")
	a.stop.Click(a.controller.Stop)
	a.mic = systray.AddMenuItemCheckbox("Microphone", "Capture microphone audio", true)
	a.mic.Click(func() { a.toggle(true) })
	a.system = systray.AddMenuItemCheckbox("System Audio", "Capture system audio", true)
	a.system.Click(func() { a.toggle(false) })
	systray.AddSeparator()
	folder := systray.AddMenuItem("Open Output Folder", "Open local transcript storage")
	folder.Click(func() { _ = exec.Command("open", a.controller.Store.Dir).Start() })
	systray.AddSeparator()
	quit := systray.AddMenuItem("Quit Kian Studio", "Save and close")
	quit.Click(func() { go func() { a.controller.Close(); systray.Quit() }() })
	a.host.OnQuit = func() { a.controller.Close(); systray.Quit() }
	a.host.OnEvent = func(e app.Event) {
		if e.Type == "live" {
			a.sync()
		}
	}
	a.host.Init()
	a.sync()
}
func (a *App) sync() {
	v := a.controller.LiveState()
	if v.Active {
		a.start.Disable()
		a.stop.Enable()
		a.mic.Disable()
		a.system.Disable()
		systray.SetIcon(iconRecordingData)
	} else {
		a.start.Enable()
		a.stop.Disable()
		a.mic.Enable()
		a.system.Enable()
		systray.SetTemplateIcon(iconData, iconData)
	}
	if v.Mode != audio.ModeSystem {
		a.mic.Check()
	} else {
		a.mic.Uncheck()
	}
	if v.Mode != audio.ModeMicrophone {
		a.system.Check()
	} else {
		a.system.Uncheck()
	}
}
func (a *App) toggle(mic bool) {
	v := a.controller.LiveState()
	m, s := v.Mode != audio.ModeSystem, v.Mode != audio.ModeMicrophone
	if mic {
		m = !m
	} else {
		s = !s
	}
	if !m && !s {
		return
	}
	mode := audio.ModeBoth
	if !m {
		mode = audio.ModeSystem
	} else if !s {
		mode = audio.ModeMicrophone
	}
	_ = a.controller.SetMode(mode)
}
func (a *App) onExit() {}
