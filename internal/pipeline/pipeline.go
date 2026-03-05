package pipeline

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

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

	switch cfg.AudioMode {
	case audio.ModeMicrophone:
		g.Go(func() error {
			defer close(chunks)
			return captureMicrophoneWithTapActive(gctx, cfg, chunks, nil)
		})

	case audio.ModeSystem:
		g.Go(func() error {
			defer close(chunks)
			return captureSystemAudio(gctx, cfg, chunks, nil)
		})

	case audio.ModeBoth:
		tapActive := &audio.TapActive{}
		g.Go(func() error {
			defer close(chunks)
			inner, innerCtx := errgroup.WithContext(gctx)
			inner.Go(func() error {
				return captureSystemAudio(innerCtx, cfg, chunks, tapActive)
			})
			inner.Go(func() error {
				return captureMicrophoneWithTapActive(innerCtx, cfg, chunks, tapActive)
			})
			return inner.Wait()
		})
	}

	g.Go(func() error {
		defer close(segments)
		return transcribeChunks(gctx, trans, chunks, segments)
	})
	g.Go(func() error {
		return writeSegments(cfg, output, segments)
	})

	return g.Wait()
}

func captureMicrophoneWithTapActive(ctx context.Context, cfg Config, chunks chan<- audio.Chunk, tapActive *audio.TapActive) error {
	device, err := audio.FindMicrophone()
	if err != nil {
		return err
	}

	return audio.Capture(ctx, audio.CaptureConfig{
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

	tc := newTapCapturer(tapDev, cfg, tapActive)
	return tc.readLoop(ctx, chunks)
}

type tapCapturer struct {
	tapDev   *audio.TapDevice
	channels int
	sampleRate   float64
	silThresh    float64
	silFrames    int
	maxFrames    int
	needResample bool
	ratio        float64
	buf       []float32
	tapActive *audio.TapActive

	accumulator   []float32
	silentCount   int
	totalFrames   int
	elapsedFrames int
	hasSpeech     bool
}

func newTapCapturer(tapDev *audio.TapDevice, cfg Config, tapActive *audio.TapActive) *tapCapturer {
	sampleRate := cfg.DeviceSampleRate
	if sampleRate == 0 {
		sampleRate = 48000
	}

	channels := tapDev.Channels()
	if channels == 0 {
		channels = 2
	}

	silThresh := cfg.SilenceThreshold
	if silThresh == 0 {
		silThresh = config.DefaultSilenceThreshold
	}
	silFrames := int(sampleRate * cfg.SilenceDuration)
	if silFrames == 0 {
		silFrames = int(sampleRate * config.DefaultSilenceDuration)
	}
	maxFrames := int(sampleRate * cfg.MaxChunkDuration)
	if maxFrames == 0 {
		maxFrames = int(sampleRate * config.DefaultMaxChunkDuration)
	}

	return &tapCapturer{
		tapDev:   tapDev,
		channels: channels,
		sampleRate:   sampleRate,
		silThresh:    silThresh,
		silFrames:    silFrames,
		maxFrames:    maxFrames,
		needResample: math.Abs(sampleRate-audio.TargetSampleRate) > 1.0,
		ratio:        audio.TargetSampleRate / sampleRate,
		buf:          make([]float32, config.DefaultFramesPerBuffer*channels),
		tapActive:    tapActive,
	}
}

func (tc *tapCapturer) readLoop(ctx context.Context, chunks chan<- audio.Chunk) error {
	for {
		select {
		case <-ctx.Done():
			if tc.hasSpeech && len(tc.accumulator) > 0 {
				tc.emit(ctx, chunks)
			}
			return nil
		default:
		}

		n := tc.tapDev.ReadSamples(tc.buf)
		if n == 0 {
			time.Sleep(5 * time.Millisecond)
			continue
		}

		tc.processBuffer(ctx, tc.buf[:n], chunks)
	}
}

func (tc *tapCapturer) processBuffer(ctx context.Context, samples []float32, chunks chan<- audio.Chunk) {
	framesRead := len(samples) / tc.channels

	mono := mixToMono(samples, tc.channels)
	bufRMS := rms(mono)
	hasSpeechInBuf := bufRMS >= tc.silThresh

	if hasSpeechInBuf && tc.tapActive != nil {
		tc.tapActive.MarkActive()
	}

	tc.elapsedFrames += framesRead

	if hasSpeechInBuf {
		tc.silentCount = 0
		tc.hasSpeech = true
	} else {
		tc.silentCount += framesRead
	}

	if !tc.hasSpeech {
		return
	}

	tc.accumulator = append(tc.accumulator, samples...)
	tc.totalFrames += framesRead

	if tc.silentCount < tc.silFrames && tc.totalFrames < tc.maxFrames {
		return
	}

	tc.emit(ctx, chunks)
}

func (tc *tapCapturer) emit(ctx context.Context, chunks chan<- audio.Chunk) {
	startOffset := time.Duration(float64(tc.elapsedFrames-tc.totalFrames)/tc.sampleRate*1000) * time.Millisecond

	mono := mixToMono(tc.accumulator, tc.channels)
	if tc.needResample {
		mono = resample(mono, tc.ratio)
	}
	if rms(mono) >= tc.silThresh {
		select {
		case chunks <- audio.Chunk{Source: config.LabelSystemAudio, Samples: mono, StartOffset: startOffset}:
		case <-ctx.Done():
		}
	}

	tc.accumulator = tc.accumulator[:0]
	tc.silentCount = 0
	tc.totalFrames = 0
	tc.hasSpeech = false
}

// mixToMono and resample are local copies of the audio package helpers
// to avoid circular imports while keeping the tap capture self-contained.

func mixToMono(interleaved []float32, channels int) []float32 {
	if channels <= 1 {
		out := make([]float32, len(interleaved))
		copy(out, interleaved)
		return out
	}
	frames := len(interleaved) / channels
	mono := make([]float32, frames)
	for i := range frames {
		var sum float32
		for ch := range channels {
			sum += interleaved[i*channels+ch]
		}
		mono[i] = sum / float32(channels)
	}
	return mono
}

func resample(samples []float32, ratio float64) []float32 {
	outLen := int(float64(len(samples)) * ratio)
	out := make([]float32, outLen)
	for i := range out {
		srcIdx := float64(i) / ratio
		idx := int(srcIdx)
		frac := float32(srcIdx - float64(idx))
		if idx+1 < len(samples) {
			out[i] = samples[idx]*(1-frac) + samples[idx+1]*frac
		} else if idx < len(samples) {
			out[i] = samples[idx]
		}
	}
	return out
}

func rms(samples []float32) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sumSq float64
	for _, s := range samples {
		sumSq += float64(s) * float64(s)
	}
	return math.Sqrt(sumSq / float64(len(samples)))
}

func transcribeChunks(ctx context.Context, trans *transcriber.Transcriber, chunks <-chan audio.Chunk, segments chan<- transcriber.Segment) error {
	for chunk := range chunks {
		segs, err := trans.Transcribe(ctx, chunk.Samples)
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
