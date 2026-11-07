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

	"kian/internal/audio"
	"kian/internal/config"
	"kian/internal/paths"
	"kian/internal/transcriber"
)

type Config struct {
	drainContext     context.Context
	AudioMode        audio.Mode
	ModelPath        string
	Language         string
	OutputDir        string
	Threads          int
	DeviceSampleRate float64
	OnSegment        func(line string)
	OnFinalSegment   func(transcriber.Segment) error
	OnOutputPath     func(string) error
	Engine           interface {
		Transcribe(context.Context, []float32) ([]transcriber.Segment, error)
	}

	SilenceFlushSec  float64
	SilenceThreshold float64
	SilenceDuration  float64
	MaxChunkDuration float64
}

func (c Config) silenceFlush() time.Duration {
	if c.SilenceFlushSec > 0 {
		return time.Duration(c.SilenceFlushSec * float64(time.Second))
	}
	return config.DefaultSilenceFlush
}

func Run(ctx context.Context, cfg Config) error {
	trans := cfg.Engine
	if trans == nil {
		owned, err := transcriber.New(transcriber.Config{ModelPath: cfg.ModelPath, Language: cfg.Language, Threads: cfg.Threads})
		if err != nil {
			return fmt.Errorf("init transcriber: %w", err)
		}
		defer owned.Close()
		trans = owned
	}

	output, cleanup, err := createOutput(cfg.OutputDir)
	if err != nil {
		return err
	}
	defer cleanup()
	if cfg.OnOutputPath != nil {
		if f, ok := output.(*os.File); ok {
			if err := cfg.OnOutputPath(f.Name()); err != nil {
				return err
			}
		}
	}

	chunks := make(chan audio.Chunk, 3)
	segments := make(chan transcriber.Segment, 10)

	// Stop capture immediately, then drain finalized audio through the same engine.
	captureCtx, captureCancel := context.WithCancel(ctx)
	defer captureCancel()
	g, gctx := errgroup.WithContext(context.WithoutCancel(ctx))
	cfg.drainContext = gctx
	go func() { <-gctx.Done(); captureCancel() }()

	switch cfg.AudioMode {
	case audio.ModeMicrophone:
		g.Go(func() error {
			defer close(chunks)
			return captureMicrophone(captureCtx, cfg, chunks, nil)
		})

	case audio.ModeSystem:
		g.Go(func() error {
			defer close(chunks)
			return captureSystemAudio(captureCtx, cfg, chunks, nil)
		})

	case audio.ModeBoth:
		tapActive := &audio.TapActive{}
		g.Go(func() error {
			defer close(chunks)
			inner, innerCtx := errgroup.WithContext(captureCtx)
			inner.Go(func() error {
				return captureSystemAudio(innerCtx, cfg, chunks, tapActive)
			})
			inner.Go(func() error {
				return captureMicrophone(innerCtx, cfg, chunks, tapActive)
			})
			return inner.Wait()
		})
	}

	g.Go(func() error {
		defer close(segments)
		return transcribeChunksFunc(gctx, trans.Transcribe, chunks, segments)
	})
	g.Go(func() error {
		return writeSegments(cfg, output, segments)
	})

	return g.Wait()
}

func captureMicrophone(ctx context.Context, cfg Config, chunks chan<- audio.Chunk, tapActive *audio.TapActive) error {
	device, err := audio.FindMicrophone()
	if err != nil {
		return err
	}

	return audio.Capture(ctx, audio.CaptureConfig{
		DrainContext:     cfg.drainContext,
		Device:           device,
		SampleRate:       cfg.DeviceSampleRate,
		SourceLabel:      config.LabelMicrophone,
		SilenceThreshold: cfg.SilenceThreshold,
		SilenceDuration:  cfg.SilenceDuration,
		MaxChunkDuration: cfg.MaxChunkDuration,
		TapActive:        tapActive,
	}, chunks)
}

func captureSystemAudio(ctx context.Context, cfg Config, chunks chan<- audio.Chunk, tapActive *audio.TapActive) error {
	tapDev, err := audio.CreateTapDevice()
	if err != nil {
		return fmt.Errorf("create tap device: %w", err)
	}
	defer tapDev.Destroy()

	if err := tapDev.StartIO(); err != nil {
		return fmt.Errorf("start tap IO: %w", err)
	}
	defer tapDev.StopIO()

	return audio.CaptureTap(ctx, tapDev, audio.TapCaptureConfig{
		DrainContext:     cfg.drainContext,
		SampleRate:       cfg.DeviceSampleRate,
		SilenceThreshold: cfg.SilenceThreshold,
		SilenceDuration:  cfg.SilenceDuration,
		MaxChunkDuration: cfg.MaxChunkDuration,
		TapActive:        tapActive,
	}, chunks)
}

type transcribeFunc func(ctx context.Context, samples []float32) ([]transcriber.Segment, error)

func transcribeChunks(ctx context.Context, trans *transcriber.Transcriber, chunks <-chan audio.Chunk, segments chan<- transcriber.Segment) error {
	return transcribeChunksFunc(ctx, trans.Transcribe, chunks, segments)
}

func transcribeChunksFunc(ctx context.Context, transcribe transcribeFunc, chunks <-chan audio.Chunk, segments chan<- transcriber.Segment) error {
	for chunk := range chunks {
		segs, err := transcribe(ctx, chunk.Samples)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("transcribe: %w", err)
		}
		for _, seg := range segs {
			seg.Source = chunk.Source
			seg.Start += chunk.StartOffset
			seg.End += chunk.StartOffset
			select {
			case segments <- seg:
			case <-ctx.Done():
				return nil
			}
		}
	}
	return nil
}

func writeSegments(cfg Config, output io.Writer, segments <-chan transcriber.Segment) error {
	var pending *mergedSegment
	silenceTimer := time.NewTimer(0)
	if !silenceTimer.Stop() {
		<-silenceTimer.C
	}
	defer silenceTimer.Stop()

	silenceFlush := cfg.silenceFlush()

	for {
		select {
		case seg, ok := <-segments:
			if !ok {
				return flushPending(cfg, output, &pending)
			}
			if cfg.OnFinalSegment != nil {
				if err := cfg.OnFinalSegment(seg); err != nil {
					return err
				}
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
			if !silenceTimer.Stop() {
				select {
				case <-silenceTimer.C:
				default:
				}
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
	seg := *pending
	*pending = nil

	line := fmt.Sprintf("[%s] [%s → %s] %s",
		seg.source, formatDuration(seg.start), formatDuration(seg.end), strings.TrimSpace(seg.text))
	if cfg.OnSegment != nil {
		cfg.OnSegment(line)
	}
	_, err := fmt.Fprintf(output, "%s\n\n", line)
	if err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

func createOutput(dir string) (io.Writer, func(), error) {
	outDir := paths.ResolveDir(dir)

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create output directory %s: %w", outDir, err)
	}

	filename := fmt.Sprintf("transcript_%s.txt", time.Now().Format("2006-01-02_15-04-05.000000000"))
	path := filepath.Join(outDir, filename)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, nil, fmt.Errorf("create output file %s: %w", path, err)
	}

	cleanup := func() { f.Close() }
	return f, cleanup, nil
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
