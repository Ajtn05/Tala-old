package audio

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gordonklaus/portaudio"

	"kian/internal/config"
)

// TapActive tracks whether the system audio tap is currently producing
// sound. The tap capturer marks it active on each above-threshold buffer;
// the mic capturer checks whether the tap was recently active to suppress
// echo. Uses a monotonic nanosecond timestamp for lock-free sharing.
type TapActive struct {
	lastActiveNano atomic.Int64
}

// MarkActive records that the tap is producing audio right now.
func (t *TapActive) MarkActive() {
	t.lastActiveNano.Store(time.Now().UnixNano())
}

// IsActive returns true if the tap produced audio within the given window.
func (t *TapActive) IsActive(window time.Duration) bool {
	last := t.lastActiveNano.Load()
	if last == 0 {
		return false
	}
	return time.Since(time.Unix(0, last)) < window
}

// TargetSampleRate is the sample rate required by whisper.cpp. This is not
// configurable — the model always expects 16 kHz input.
const TargetSampleRate = 16000

type Chunk struct {
	Source      string
	Samples     []float32
	StartOffset time.Duration
}

type CaptureConfig struct {
	DrainContext context.Context
	Device       *portaudio.DeviceInfo
	SampleRate   float64
	SourceLabel  string

	FramesPerBuffer  int
	SilenceThreshold float64
	SilenceDuration  float64
	MaxChunkDuration float64
	TapActive        *TapActive
}

func (c CaptureConfig) framesPerBuffer() int {
	if c.FramesPerBuffer > 0 {
		return c.FramesPerBuffer
	}
	return config.DefaultFramesPerBuffer
}

func (c CaptureConfig) silenceThreshold() float64 {
	if c.SilenceThreshold > 0 {
		return c.SilenceThreshold
	}
	return config.DefaultSilenceThreshold
}

func (c CaptureConfig) silenceDuration() float64 {
	if c.SilenceDuration > 0 {
		return c.SilenceDuration
	}
	return config.DefaultSilenceDuration
}

func (c CaptureConfig) maxChunkDuration() float64 {
	if c.MaxChunkDuration > 0 {
		return c.MaxChunkDuration
	}
	return config.DefaultMaxChunkDuration
}

type capturer struct {
	drainContext context.Context
	out          chan<- Chunk

	sampleRate  float64
	fpb         int
	numChannels int

	needResample bool
	ratio        float64

	silThresh     float64
	silenceFrames int
	maxFrames     int
	sourceLabel   string
	tapActive     *TapActive

	accumulator   []float32
	silentCount   int
	totalFrames   int
	elapsedFrames int
	hasSpeech     bool
}

func Capture(ctx context.Context, cfg CaptureConfig, out chan<- Chunk) error {
	sampleRate := cfg.SampleRate
	if sampleRate == 0 {
		sampleRate = cfg.Device.DefaultSampleRate
	}

	fpb := cfg.framesPerBuffer()

	stream, buf, err := openStream(cfg.Device, sampleRate, fpb)
	if err != nil {
		return err
	}
	defer stream.Close()

	if err := stream.Start(); err != nil {
		return fmt.Errorf("start stream: %w", err)
	}
	defer stream.Stop()

	c := &capturer{
		drainContext:  cfg.DrainContext,
		out:           out,
		sampleRate:    sampleRate,
		fpb:           fpb,
		numChannels:   cfg.Device.MaxInputChannels,
		needResample:  math.Abs(sampleRate-TargetSampleRate) > 1.0,
		ratio:         TargetSampleRate / sampleRate,
		silThresh:     cfg.silenceThreshold(),
		silenceFrames: int(sampleRate * cfg.silenceDuration()),
		maxFrames:     int(sampleRate * cfg.maxChunkDuration()),
		sourceLabel:   cfg.SourceLabel,
		tapActive:     cfg.TapActive,
		accumulator:   make([]float32, 0, int(sampleRate*5)*cfg.Device.MaxInputChannels),
	}

	return c.readLoop(ctx, stream, buf)
}

func openStream(device *portaudio.DeviceInfo, sampleRate float64, fpb int) (*portaudio.Stream, []float32, error) {
	buf := make([]float32, fpb*device.MaxInputChannels)
	params := portaudio.StreamParameters{
		Input: portaudio.StreamDeviceParameters{
			Device:   device,
			Channels: device.MaxInputChannels,
			Latency:  device.DefaultHighInputLatency,
		},
		SampleRate:      sampleRate,
		FramesPerBuffer: fpb,
	}

	stream, err := portaudio.OpenStream(params, buf)
	if err != nil {
		return nil, nil, fmt.Errorf("open stream: %w", err)
	}
	return stream, buf, nil
}

func (c *capturer) readLoop(ctx context.Context, stream *portaudio.Stream, buf []float32) error {
	for {
		select {
		case <-ctx.Done():
			c.flushRemaining(ctx)
			return nil
		default:
		}

		if err := stream.Read(); err != nil {
			if strings.Contains(err.Error(), "Input overflowed") {
				log.Println("warning: input overflowed, skipping buffer")
				continue
			}
			return fmt.Errorf("read stream: %w", err)
		}

		c.processBuffer(ctx, buf)
	}
}

func (c *capturer) processBuffer(ctx context.Context, buf []float32) {
	suppressed := c.tapActive != nil && c.tapActive.IsActive(300*time.Millisecond)
	hasSpeechInBuf := !suppressed && RMS(buf) >= c.silThresh
	c.elapsedFrames += c.fpb

	if hasSpeechInBuf {
		c.silentCount = 0
		c.hasSpeech = true
	} else {
		c.silentCount += c.fpb
	}

	if !c.hasSpeech {
		return
	}

	if !suppressed {
		c.accumulator = append(c.accumulator, buf...)
		c.totalFrames += c.fpb
	}

	if c.silentCount < c.silenceFrames && c.totalFrames < c.maxFrames {
		return
	}

	c.flushAccumulator(ctx)
}

func (c *capturer) startTime() time.Duration {
	return time.Duration(float64(c.elapsedFrames-c.totalFrames)/c.sampleRate*1000) * time.Millisecond
}

func (c *capturer) flushRemaining(ctx context.Context) {
	if !c.hasSpeech {
		return
	}
	if c.drainContext != nil {
		c.emit(c.drainContext, c.accumulator, c.startTime())
		return
	}
	flushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	c.emit(flushCtx, c.accumulator, c.startTime())
}

func (c *capturer) flushAccumulator(ctx context.Context) {
	c.emit(ctx, c.accumulator, c.startTime())
	c.accumulator = c.accumulator[:0]
	c.silentCount = 0
	c.totalFrames = 0
	c.hasSpeech = false
}

func (c *capturer) emit(ctx context.Context, interleaved []float32, startOffset time.Duration) {
	mono := MixToMono(interleaved, c.numChannels)
	if c.needResample {
		mono = Resample(mono, c.ratio)
	}
	if RMS(mono) < c.silThresh {
		return
	}

	label := c.sourceLabel
	if label == "" {
		label = config.LabelUnknown
	}
	select {
	case c.out <- Chunk{Source: label, Samples: mono, StartOffset: startOffset}:
	case <-ctx.Done():
	}
}
