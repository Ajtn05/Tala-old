package main

import (
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/gordonklaus/portaudio"

	"kian/internal/config"
	"kian/internal/paths"
	"kian/internal/pipeline"
	"kian/internal/ui"
)

func main() {
	modelPath, outputDir := paths.ResolveForBundle()
	setupLogging(outputDir)

	if err := portaudio.Initialize(); err != nil {
		log.Fatalf("initialize portaudio: %v", err)
	}
	defer portaudio.Terminate()

	cfg := pipeline.Config{
		ModelPath: modelPath,
		Language:  config.DefaultLanguage,
		OutputDir: outputDir,
		Threads:   config.DefaultThreads,
	}

	app := ui.NewApp(cfg)
	app.Run()
}

func setupLogging(outputDir string) {
	if outputDir == "" {
		outputDir = config.DefaultOutputDir
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		log.Printf("create log directory: %v", err)
		return
	}
	logPath := filepath.Join(outputDir, "kian.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		log.Printf("open log file: %v", err)
		return
	}
	log.SetOutput(io.MultiWriter(os.Stderr, f))
}
