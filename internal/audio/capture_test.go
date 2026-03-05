package audio

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kian/internal/config"
)

func TestRMS(t *testing.T) {
	tests := []struct {
		name    string
		samples []float32
		want    float64
	}{
		{
			name:    "empty",
			samples: nil,
			want:    0,
		},
		{
			name:    "silence",
			samples: []float32{0, 0, 0, 0},
			want:    0,
		},
		{
			name:    "constant signal",
			samples: []float32{0.5, 0.5, 0.5, 0.5},
			want:    0.5,
		},
		{
			name:    "symmetric signal",
			samples: []float32{1, -1, 1, -1},
			want:    1.0,
		},
		{
			name:    "single sample",
			samples: []float32{0.5},
			want:    0.5,
		},
		{
			name:    "mixed values",
			samples: []float32{0, 1, 0, 1},
			want:    math.Sqrt(0.5),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RMS(tt.samples)
			assert.InDelta(t, tt.want, got, 1e-9)
		})
	}
}

func TestMixToMono(t *testing.T) {
	tests := []struct {
		name        string
		interleaved []float32
		channels    int
		want        []float32
	}{
		{
			name:        "mono passthrough",
			interleaved: []float32{0.1, 0.2, 0.3},
			channels:    1,
			want:        []float32{0.1, 0.2, 0.3},
		},
		{
			name:        "stereo average",
			interleaved: []float32{0.2, 0.8, 0.4, 0.6},
			channels:    2,
			want:        []float32{0.5, 0.5},
		},
		{
			name:        "3 channels",
			interleaved: []float32{0.3, 0.6, 0.9},
			channels:    3,
			want:        []float32{0.6},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MixToMono(tt.interleaved, tt.channels)
			require.Len(t, got, len(tt.want))
			for i := range got {
				assert.InDelta(t, tt.want[i], got[i], 1e-6)
			}
		})
	}
}

func TestResample(t *testing.T) {
	tests := []struct {
		name    string
		samples []float32
		ratio   float64
		wantLen int
	}{
		{
			name:    "downsample 2:1",
			samples: []float32{0, 1, 0, 1, 0, 1},
			ratio:   0.5,
			wantLen: 3,
		},
		{
			name:    "upsample 2:1",
			samples: []float32{0, 1, 0},
			ratio:   2.0,
			wantLen: 6,
		},
		{
			name:    "identity",
			samples: []float32{0.1, 0.2, 0.3},
			ratio:   1.0,
			wantLen: 3,
		},
		{
			name:    "empty",
			samples: nil,
			ratio:   0.5,
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Resample(tt.samples, tt.ratio)
			assert.Len(t, got, tt.wantLen)
		})
	}
}

func TestResample_linearInterpolation(t *testing.T) {
	samples := []float32{0, 1, 0, 1}
	got := Resample(samples, 0.5)

	require.Len(t, got, 2)
	assert.InDelta(t, float32(0), got[0], 1e-6)
	assert.InDelta(t, float32(0), got[1], 1e-6)
}

func TestResample_preservesMagnitude(t *testing.T) {
	samples := make([]float32, 100)
	for i := range samples {
		samples[i] = 0.7
	}
	got := Resample(samples, 16000.0/44100.0)
	for i, v := range got {
		assert.InDelta(t, 0.7, v, 1e-5, "sample %d", i)
	}
}

func newTestCapturer(sampleRate float64, channels int, silThresh float64, silenceSec float64, maxSec float64) (*capturer, chan Chunk) {
	out := make(chan Chunk, 10)
	return &capturer{
		out:           out,
		sampleRate:    sampleRate,
		fpb:           int(sampleRate * 0.01), // 10ms buffers
		numChannels:   channels,
		needResample:  false,
		ratio:         1.0,
		silThresh:     silThresh,
		silenceFrames: int(sampleRate * silenceSec),
		maxFrames:     int(sampleRate * maxSec),
		sourceLabel:   "Test",
	}, out
}

func loudBuf(fpb, channels int) []float32 {
	buf := make([]float32, fpb*channels)
	for i := range buf {
		buf[i] = 0.5
	}
	return buf
}

func silentBuf(fpb, channels int) []float32 {
	return make([]float32, fpb*channels)
}

func TestCapturer_silenceFlushesChunk(t *testing.T) {
	// 1000 Hz sample rate, mono, threshold 0.01, silence duration 0.05s (50 frames), max 10s
	c, out := newTestCapturer(1000, 1, 0.01, 0.05, 10)
	ctx := context.Background()

	// Feed 3 loud buffers (30ms of speech)
	for range 3 {
		c.processBuffer(ctx, loudBuf(c.fpb, 1))
	}
	assert.Empty(t, out, "should not emit while speech is ongoing")

	// Feed enough silent buffers to exceed silence duration (50 frames = 5 buffers of 10 frames)
	for range 6 {
		c.processBuffer(ctx, silentBuf(c.fpb, 1))
	}

	require.Len(t, out, 1)
	chunk := <-out
	assert.Equal(t, "Test", chunk.Source)
	assert.NotEmpty(t, chunk.Samples)
}

func TestCapturer_maxDurationForceFlush(t *testing.T) {
	// max chunk 0.05s = 50 frames, 10 frame buffers → flush after 5 loud buffers
	c, out := newTestCapturer(1000, 1, 0.01, 1.0, 0.05)
	ctx := context.Background()

	for range 6 {
		c.processBuffer(ctx, loudBuf(c.fpb, 1))
	}

	require.Len(t, out, 1)
	chunk := <-out
	assert.Equal(t, "Test", chunk.Source)
}

func TestCapturer_silenceOnlyNoChunk(t *testing.T) {
	c, out := newTestCapturer(1000, 1, 0.01, 0.05, 10)
	ctx := context.Background()

	for range 20 {
		c.processBuffer(ctx, silentBuf(c.fpb, 1))
	}

	assert.Empty(t, out)
}

func TestCapturer_flushRemainingOnCancel(t *testing.T) {
	c, out := newTestCapturer(1000, 1, 0.01, 1.0, 10)
	ctx := context.Background()

	// Feed some speech but not enough silence to flush
	for range 3 {
		c.processBuffer(ctx, loudBuf(c.fpb, 1))
	}
	assert.Empty(t, out)

	c.flushRemaining(ctx)

	require.Len(t, out, 1)
	chunk := <-out
	assert.Equal(t, "Test", chunk.Source)
}

func TestCapturer_echoSuppression(t *testing.T) {
	tapActive := &TapActive{}
	c, out := newTestCapturer(1000, 1, 0.01, 0.05, 10)
	c.tapActive = tapActive
	ctx := context.Background()

	// Mark tap as active — mic should suppress
	tapActive.MarkActive()

	// Feed loud buffers while tap is active — should be treated as silence
	for range 3 {
		c.processBuffer(ctx, loudBuf(c.fpb, 1))
	}
	assert.Empty(t, out)
	assert.False(t, c.hasSpeech, "speech should not be detected while tap is active")
}

func TestCapturer_stereoMixedToMono(t *testing.T) {
	// Stereo input: left=0.4, right=0.6 → mono=0.5
	c, out := newTestCapturer(1000, 2, 0.01, 0.02, 10)
	ctx := context.Background()

	stereo := make([]float32, c.fpb*2)
	for i := 0; i < len(stereo); i += 2 {
		stereo[i] = 0.4
		stereo[i+1] = 0.6
	}

	c.processBuffer(ctx, stereo)
	// Feed silence to flush — trailing silence is included in the chunk
	for range 3 {
		c.processBuffer(ctx, silentBuf(c.fpb, 2))
	}

	require.Len(t, out, 1)
	chunk := <-out
	// First fpb mono samples should be the averaged stereo speech
	require.True(t, len(chunk.Samples) >= c.fpb)
	for i := range c.fpb {
		assert.InDelta(t, 0.5, chunk.Samples[i], 1e-6, "sample %d", i)
	}
}

func TestTapActive(t *testing.T) {
	ta := &TapActive{}

	t.Run("initially inactive", func(t *testing.T) {
		assert.False(t, ta.IsActive(time.Second))
	})

	t.Run("active after mark", func(t *testing.T) {
		ta.MarkActive()
		assert.True(t, ta.IsActive(time.Second))
	})

	t.Run("inactive after window expires", func(t *testing.T) {
		ta.MarkActive()
		time.Sleep(15 * time.Millisecond)
		assert.False(t, ta.IsActive(10*time.Millisecond))
	})
}

func TestCaptureConfig_defaults(t *testing.T) {
	t.Run("framesPerBuffer", func(t *testing.T) {
		assert.Equal(t, config.DefaultFramesPerBuffer, CaptureConfig{}.framesPerBuffer())
		assert.Equal(t, 512, CaptureConfig{FramesPerBuffer: 512}.framesPerBuffer())
	})

	t.Run("silenceThreshold", func(t *testing.T) {
		assert.Equal(t, config.DefaultSilenceThreshold, CaptureConfig{}.silenceThreshold())
		assert.Equal(t, 0.01, CaptureConfig{SilenceThreshold: 0.01}.silenceThreshold())
	})

	t.Run("silenceDuration", func(t *testing.T) {
		assert.Equal(t, config.DefaultSilenceDuration, CaptureConfig{}.silenceDuration())
		assert.Equal(t, 2.0, CaptureConfig{SilenceDuration: 2.0}.silenceDuration())
	})

	t.Run("maxChunkDuration", func(t *testing.T) {
		assert.Equal(t, config.DefaultMaxChunkDuration, CaptureConfig{}.maxChunkDuration())
		assert.Equal(t, 60.0, CaptureConfig{MaxChunkDuration: 60.0}.maxChunkDuration())
	})
}
