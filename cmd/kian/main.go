package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gordonklaus/portaudio"
	"kian/internal/app"
	"kian/internal/dashboard"
	"kian/internal/media"
	"kian/internal/models"
	"kian/internal/ui"
)

func main() {
	checkDependencies := flag.Bool("check-dependencies", false, "Check FFmpeg, ffprobe, and yt-dlp without downloading or opening the app")
	check := flag.Bool("check-model", false, "Discover and verify existing models without downloading")
	ensure := flag.Bool("ensure-model", false, "Reuse a verified model; download only if none exists")
	input := flag.String("transcribe", "", "Transcribe a local file without opening the dashboard")
	output := flag.String("output", os.Getenv("KIAN_OUTPUT_DIR"), "Transcript directory (default ~/Documents/Kian)")
	flag.Parse()
	if *checkDependencies {
		report := media.Dependencies()
		b, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(b))
		if !report.URL.Ready {
			fmt.Fprintln(os.Stderr, "Live recording works without these media tools.")
			os.Exit(1)
		}
		return
	}
	manager := models.Default()
	if *check || *ensure {
		status, err := manager.Ensure(context.Background(), *ensure, nil)
		b, _ := json.MarshalIndent(status, "", "  ")
		fmt.Println(string(b))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if *output == "" {
		*output = filepath.Join(manager.Home, "Documents", "Kian")
	}
	if e := os.MkdirAll(manager.Root(), 0700); e != nil {
		log.Fatal(e)
	}
	lock, e := os.OpenFile(filepath.Join(manager.Root(), "studio.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		log.Fatal(e)
	}
	defer lock.Close()
	if e = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		dashboard.NotifyExisting()
		fmt.Fprintln(os.Stderr, "Opened the existing Kian Studio dashboard.")
		return
	}
	if *input == "" {
		ui.NewLazyApp(func() (*app.Controller, error) {
			setupLogging(*output)
			if e := portaudio.Initialize(); e != nil {
				log.Printf("initialize PortAudio: %v", e)
			}
			return app.New(*output)
		}).Run()
		_ = portaudio.Terminate()
		return
	}
	setupLogging(*output)
	controller, e := app.New(*output)
	if e != nil {
		log.Fatal(e)
	}
	if *input != "" {
		for controller.ModelState().State == "checking" || controller.ModelState().State == "" {
			time.Sleep(50 * time.Millisecond)
		}
		if e = controller.Add(*input, "file", false); e != nil {
			log.Fatal(e)
		}
		for event := range controller.Events {
			if event.Type == "jobs" {
				jobs := event.Data.([]app.Job)
				if len(jobs) > 0 {
					j := jobs[len(jobs)-1]
					if j.State == "completed" {
						fmt.Println(j.SessionID)
						controller.Close()
						return
					}
					if j.State == "failed" || j.State == "cancelled" {
						fmt.Fprintln(os.Stderr, j.Error)
						controller.Close()
						os.Exit(1)
					}
				}
			}
		}
	}
}
func setupLogging(output string) {
	if e := os.MkdirAll(output, 0700); e != nil {
		log.Printf("create log directory: %v", e)
		return
	}
	f, e := os.OpenFile(filepath.Join(output, "studio.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if e == nil {
		log.SetOutput(io.MultiWriter(os.Stderr, f))
	}
}
