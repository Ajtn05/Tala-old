//go:build darwin

package ui

import (
	"log"
	"tala/internal/app"
	"tala/internal/dashboard"
)

type App struct {
	initialize func() (*app.Controller, error)
	controller *app.Controller
	host       *dashboard.Host
}

func NewApp(c *app.Controller) *App                              { return &App{controller: c} }
func NewLazyApp(initialize func() (*app.Controller, error)) *App { return &App{initialize: initialize} }
func (a *App) Run() {
	dashboard.Run(func() {
		if a.initialize != nil {
			c, err := a.initialize()
			if err != nil {
				log.Printf("open workspace: %v", err)
				dashboard.StartupError(err.Error())
				return
			}
			a.controller = c
		}
		a.host = dashboard.New(a.controller)
		a.host.OnQuit = func() { a.controller.Close(); dashboard.Stop() }
		a.host.OnEvent = func(e app.Event) {
			if e.Type == "live" || e.Type == "settings" {
				a.host.SyncMenuBar()
			}
		}
		a.host.Init()
	})
}
