package audio

import (
	"context"
	"math"
	"time"

	"tala/internal/config"
)

// TapCaptureConfig holds parameters for capturing from a CoreAudio process tap.
type TapCaptureConfig struct {
	DrainContext     context.Context
	SampleRate       float64
	SilenceThreshold float64
	SilenceDuration  float64
	MaxChunkDuration float64
	TapActive        *TapActive
}

func (c TapCaptureConfig) silenceThreshold() float64 {
	if c.SilenceThreshold > 0 {
		return c.SilenceThreshold
	}
	return config.DefaultSilenceThreshold
}

func (c TapCaptureConfig) silenceDuration() float64 {
	if c.SilenceDuration > 0 {
		return c.SilenceDuration
	}
	return config.DefaultSilenceDuration
}

func (c TapCaptureConfig) maxChunkDuration() float64 {
	if c.MaxChunkDuration > 0 {
		return c.MaxChunkDuration
	}
	return config.DefaultMaxChunkDuration
}

type tapCapturer struct {
	channels     int
	sampleRate   float64
	silThresh    float64
	silFrames    int
	maxFrames    int
	needResample bool
	ratio        float64
	tapActive    *TapActive

	accumulator   []float32
	silentCount   int
	totalFrames   int
	elapsedFrames int
	hasSpeech     bool
}

// CaptureTap reads audio from the given TapDevice, chunks it, and sends
// Chunks to the output channel. It blocks until ctx is cancelled.
func CaptureTap(ctx context.Context, tapDev *TapDevice, cfg TapCaptureConfig, out chan<- Chunk) error {
	sampleRate := cfg.SampleRate
	if sampleRate == 0 {
		sampleRate = 48000
	}

	channels := tapDev.Channels()
	if channels == 0 {
		channels = 2
	}

	tc := &tapCapturer{
		channels:     channels,
		sampleRate:   sampleRate,
		silThresh:    cfg.silenceThreshold(),
		silFrames:    int(sampleRate * cfg.silenceDuration()),
		maxFrames:    int(sampleRate * cfg.maxChunkDuration()),
		needResample: math.Abs(sampleRate-TargetSampleRate) > 1.0,
		ratio:        TargetSampleRate / sampleRate,
		tapActive:    cfg.TapActive,
	}

	buf := make([]float32, config.DefaultFramesPerBuffer*channels)

	for {
		select {
		case <-ctx.Done():
			if tc.hasSpeech && len(tc.accumulator) > 0 {
				if cfg.DrainContext != nil {
					tc.emit(cfg.DrainContext, out)
				} else {
					flushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
					tc.emit(flushCtx, out)
					cancel()
				}
			}
			return nil
		default:
		}

		n := tapDev.ReadSamples(buf)
		if n == 0 {
			time.Sleep(5 * time.Millisecond)
			continue
		}

		tc.processBuffer(ctx, buf[:n], out)
	}
}

func (tc *tapCapturer) processBuffer(ctx context.Context, samples []float32, out chan<- Chunk) {
	framesRead := len(samples) / tc.channels

	mono := MixToMono(samples, tc.channels)
	hasSpeechInBuf := RMS(mono) >= tc.silThresh

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

	tc.emit(ctx, out)
}

func (tc *tapCapturer) emit(ctx context.Context, out chan<- Chunk) {
	startOffset := time.Duration(float64(tc.elapsedFrames-tc.totalFrames)/tc.sampleRate*1000) * time.Millisecond

	mono := MixToMono(tc.accumulator, tc.channels)
	if tc.needResample {
		mono = Resample(mono, tc.ratio)
	}

	tc.accumulator = tc.accumulator[:0]
	tc.silentCount = 0
	tc.totalFrames = 0
	tc.hasSpeech = false

	if RMS(mono) < tc.silThresh {
		return
	}

	select {
	case out <- Chunk{Source: config.LabelSystemAudio, Samples: mono, StartOffset: startOffset}:
	case <-ctx.Done():
	}
}
