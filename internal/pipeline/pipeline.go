package pipeline

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/gordonklaus/portaudio"

	"kian/internal/audio"
	"kian/internal/config"
	"kian/internal/transcriber"
)

type Config struct {
	AudioMode        audio.Mode
	ModelPath        string
	Language         string
	OutputDir        string
	Threads          int
	DeviceSampleRate float64
	OnSegment        func(line string)

	SilenceFlushSec    float64
	SilenceThreshold   float64
	SilenceDuration    float64
	MaxChunkDuration   float64
	DominanceRatio     float64
	CrosstalkThreshold float64
}

func (c Config) silenceFlush() time.Duration {
	if c.SilenceFlushSec > 0 {
		return time.Duration(c.SilenceFlushSec * float64(time.Second))
	}
	return config.DefaultSilenceFlush
}

func Run(ctx context.Context, cfg Config) error {
	device, err := audio.FindDeviceForMode(cfg.AudioMode)
	if err != nil {
		return err
	}

	trans, err := transcriber.New(transcriber.Config{
		ModelPath: cfg.ModelPath,
		Language:  cfg.Language,
		Threads:   cfg.Threads,
	})
	if err != nil {
		return fmt.Errorf("init transcriber: %w", err)
	}
	defer trans.Close()

	output, cleanup, err := createOutput(cfg.OutputDir)
	if err != nil {
		return err
	}
	defer cleanup()

	chunks := make(chan audio.Chunk, 3)
	segments := make(chan transcriber.Segment, 10)

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		defer close(chunks)
		return captureAudio(gctx, cfg, device, chunks)
	})
	g.Go(func() error {
		defer close(segments)
		return transcribeChunks(gctx, trans, chunks, segments)
	})
	g.Go(func() error {
		return writeSegments(cfg, output, segments)
	})

	return g.Wait()
}

func captureAudio(ctx context.Context, cfg Config, device *portaudio.DeviceInfo, chunks chan<- audio.Chunk) error {
	return audio.Capture(ctx, audio.CaptureConfig{
		Device:           device,
		SampleRate:       cfg.DeviceSampleRate,
		SourceLabel:      sourceLabelForMode(cfg.AudioMode),
		SilenceThreshold: cfg.SilenceThreshold,
		SilenceDuration:  cfg.SilenceDuration,
		MaxChunkDuration: cfg.MaxChunkDuration,
		DominanceRatio:     cfg.DominanceRatio,
		CrosstalkThreshold: cfg.CrosstalkThreshold,
	}, chunks)
}

func transcribeChunks(ctx context.Context, trans *transcriber.Transcriber, chunks <-chan audio.Chunk, segments chan<- transcriber.Segment) error {
	for chunk := range chunks {
		segs, err := trans.Transcribe(ctx, chunk.Samples)
		if err != nil {
			return fmt.Errorf("transcribe: %w", err)
		}
		for _, seg := range segs {
			seg.Source = chunk.Source
			seg.Start += chunk.StartOffset
			seg.End += chunk.StartOffset
			segments <- seg
		}
	}
	return nil
}

func writeSegments(cfg Config, output io.Writer, segments <-chan transcriber.Segment) error {
	var pending *mergedSegment
	silenceTimer := time.NewTimer(0)
	silenceTimer.Stop()
	defer silenceTimer.Stop()

	silenceFlush := cfg.silenceFlush()

	for {
		select {
		case seg, ok := <-segments:
			if !ok {
				return flushPending(cfg, output, &pending)
			}
			if pending != nil && pending.source != seg.Source {
				if err := flushPending(cfg, output, &pending); err != nil {
					return err
				}
			}
			if pending == nil {
				pending = &mergedSegment{
					source: seg.Source,
					start:  seg.Start,
					end:    seg.End,
					text:   seg.Text,
				}
			} else {
				pending.end = seg.End
				if !strings.HasSuffix(pending.text, " ") && !strings.HasPrefix(seg.Text, " ") {
					pending.text += " "
				}
				pending.text += seg.Text
			}
			silenceTimer.Reset(silenceFlush)
		case <-silenceTimer.C:
			if err := flushPending(cfg, output, &pending); err != nil {
				return err
			}
		}
	}
}

func flushPending(cfg Config, output io.Writer, pending **mergedSegment) error {
	if *pending == nil {
		return nil
	}
	line := fmt.Sprintf("[%s] [%s → %s] %s",
		(*pending).source, formatDuration((*pending).start), formatDuration((*pending).end), strings.TrimSpace((*pending).text))
	if cfg.OnSegment != nil {
		cfg.OnSegment(line)
	}
	_, err := fmt.Fprintf(output, "%s\n\n", line)
	if err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	*pending = nil
	return nil
}

func createOutput(dir string) (io.Writer, func(), error) {
	if dir == "" {
		dir = config.DefaultOutputDir
	}

	outDir := dir
	if !filepath.IsAbs(outDir) {
		execPath, err := os.Executable()
		if err != nil {
			return nil, nil, fmt.Errorf("resolve executable path: %w", err)
		}
		outDir = filepath.Join(filepath.Dir(execPath), dir)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create output directory %s: %w", outDir, err)
	}

	filename := fmt.Sprintf("transcript_%s.txt", time.Now().Format("2006-01-02_15-04-05"))
	path := filepath.Join(outDir, filename)

	f, err := os.Create(path)
	if err != nil {
		return nil, nil, fmt.Errorf("create output file %s: %w", path, err)
	}

	cleanup := func() { f.Close() }
	return f, cleanup, nil
}

func sourceLabelForMode(mode audio.Mode) string {
	switch mode {
	case audio.ModeMicrophone:
		return config.LabelMicrophone
	case audio.ModeSystem:
		return config.LabelSystemAudio
	default:
		return ""
	}
}

type mergedSegment struct {
	source string
	start  time.Duration
	end    time.Duration
	text   string
}

func formatDuration(d time.Duration) string {
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d", m, s)
}
