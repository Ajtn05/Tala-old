package audio

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tala/internal/config"
)

func newTestTapCapturer(sampleRate float64, channels int, silThresh float64, silenceSec float64, maxSec float64) *tapCapturer {
	return &tapCapturer{
		channels:   channels,
		sampleRate: sampleRate,
		silThresh:  silThresh,
		silFrames:  int(sampleRate * silenceSec),
		maxFrames:  int(sampleRate * maxSec),
	}
}

func TestTapCapturer_silenceFlushesChunk(t *testing.T) {
	tc := newTestTapCapturer(1000, 1, 0.01, 0.05, 10)
	out := make(chan Chunk, 10)
	ctx := context.Background()
	fpb := 10

	for range 3 {
		tc.processBuffer(ctx, loudBuf(fpb, 1), out)
	}
	assert.Empty(t, out)

	for range 6 {
		tc.processBuffer(ctx, silentBuf(fpb, 1), out)
	}

	require.Len(t, out, 1)
	chunk := <-out
	assert.Equal(t, config.LabelSystemAudio, chunk.Source)
}

func TestTapCapturer_maxDurationForceFlush(t *testing.T) {
	tc := newTestTapCapturer(1000, 1, 0.01, 1.0, 0.05)
	out := make(chan Chunk, 10)
	ctx := context.Background()
	fpb := 10

	for range 6 {
		tc.processBuffer(ctx, loudBuf(fpb, 1), out)
	}

	require.Len(t, out, 1)
}

func TestTapCapturer_marksTapActive(t *testing.T) {
	ta := &TapActive{}
	tc := newTestTapCapturer(1000, 1, 0.01, 1.0, 10)
	tc.tapActive = ta
	out := make(chan Chunk, 10)
	ctx := context.Background()

	assert.False(t, ta.IsActive(100*1e6))

	tc.processBuffer(ctx, loudBuf(10, 1), out)

	assert.True(t, ta.IsActive(1e9))
}

func TestTapCapturer_silenceOnlyNoChunk(t *testing.T) {
	tc := newTestTapCapturer(1000, 1, 0.01, 0.05, 10)
	out := make(chan Chunk, 10)
	ctx := context.Background()

	for range 20 {
		tc.processBuffer(ctx, silentBuf(10, 1), out)
	}

	assert.Empty(t, out)
}

func TestTapCapturer_stereoInput(t *testing.T) {
	tc := newTestTapCapturer(1000, 2, 0.01, 0.02, 10)
	out := make(chan Chunk, 10)
	ctx := context.Background()
	fpb := 10

	stereo := make([]float32, fpb*2)
	for i := 0; i < len(stereo); i += 2 {
		stereo[i] = 0.3
		stereo[i+1] = 0.7
	}

	tc.processBuffer(ctx, stereo, out)

	for range 3 {
		tc.processBuffer(ctx, silentBuf(fpb, 2), out)
	}

	require.Len(t, out, 1)
	chunk := <-out
	// First fpb mono samples should be the averaged stereo speech
	require.True(t, len(chunk.Samples) >= fpb)
	for i := range fpb {
		assert.InDelta(t, 0.5, chunk.Samples[i], 1e-6, "sample %d", i)
	}
}

func TestTapCapturer_emitOnCancel(t *testing.T) {
	tc := newTestTapCapturer(1000, 1, 0.01, 1.0, 10)
	out := make(chan Chunk, 10)
	ctx := context.Background()

	for range 3 {
		tc.processBuffer(ctx, loudBuf(10, 1), out)
	}
	assert.Empty(t, out)

	// Simulate what CaptureTap does on ctx.Done
	if tc.hasSpeech && len(tc.accumulator) > 0 {
		tc.emit(ctx, out)
	}

	require.Len(t, out, 1)
}

func TestTapCaptureConfig_defaults(t *testing.T) {
	t.Run("silenceThreshold", func(t *testing.T) {
		assert.Equal(t, config.DefaultSilenceThreshold, TapCaptureConfig{}.silenceThreshold())
		assert.Equal(t, 0.01, TapCaptureConfig{SilenceThreshold: 0.01}.silenceThreshold())
	})

	t.Run("silenceDuration", func(t *testing.T) {
		assert.Equal(t, config.DefaultSilenceDuration, TapCaptureConfig{}.silenceDuration())
		assert.Equal(t, 2.0, TapCaptureConfig{SilenceDuration: 2.0}.silenceDuration())
	})

	t.Run("maxChunkDuration", func(t *testing.T) {
		assert.Equal(t, config.DefaultMaxChunkDuration, TapCaptureConfig{}.maxChunkDuration())
		assert.Equal(t, 60.0, TapCaptureConfig{MaxChunkDuration: 60.0}.maxChunkDuration())
	})
}
