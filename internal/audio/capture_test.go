package audio

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kian/internal/config"
)

func TestRms(t *testing.T) {
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
			got := rms(tt.samples)
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
			got := mixToMono(tt.interleaved, tt.channels)
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
			got := resample(tt.samples, tt.ratio)
			assert.Len(t, got, tt.wantLen)
		})
	}
}

func TestResample_linearInterpolation(t *testing.T) {
	// 4 samples at source rate, downsample to ~half
	samples := []float32{0, 1, 0, 1}
	got := resample(samples, 0.5)

	// Output should have 2 samples, interpolated from source
	require.Len(t, got, 2)
	// First output maps to srcIdx=0 → samples[0]=0
	assert.InDelta(t, float32(0), got[0], 1e-6)
	// Second output maps to srcIdx=2 → samples[2]=0
	assert.InDelta(t, float32(0), got[1], 1e-6)
}

func TestResample_preservesMagnitude(t *testing.T) {
	// Constant signal should remain constant after resampling
	samples := make([]float32, 100)
	for i := range samples {
		samples[i] = 0.7
	}
	got := resample(samples, 16000.0/44100.0)
	for i, v := range got {
		assert.InDelta(t, 0.7, v, 1e-5, "sample %d", i)
	}
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
