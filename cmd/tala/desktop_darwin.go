package main

import (
	"github.com/gordonklaus/portaudio"
	"tala/internal/app"
	"tala/internal/dashboard"
	"tala/internal/ui"
)

func launchDesktop(output string) error {
	ui.NewLazyApp(func() (*app.Controller, error) {
		setupLogging(output)
		// File imports and settings work even when no microphone is available.
		_ = portaudio.Initialize()
		return app.New(output)
	}).Run()
	_ = portaudio.Terminate()
	return nil
}
func notifyExisting() { dashboard.NotifyExisting() }
