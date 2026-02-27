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

func TestHasSourceSpeech(t *testing.T) {
	const thresh = 0.01

	tests := []struct {
		name      string
		buf       []float32
		channels  int
		aggregate bool
		want      bool
	}{
		{
			name:      "single source silence",
			buf:       []float32{0, 0, 0, 0},
			channels:  1,
			aggregate: false,
			want:      false,
		},
		{
			name:      "single source speech",
			buf:       []float32{0.5, 0.5, 0.5, 0.5},
			channels:  1,
			aggregate: false,
			want:      true,
		},
		{
			name:      "aggregate both silent",
			buf:       []float32{0, 0, 0, 0, 0, 0}, // 2 frames x 3 channels
			channels:  3,
			aggregate: true,
			want:      false,
		},
		{
			name:      "aggregate remote has speech",
			buf:       []float32{0.5, 0.5, 0, 0.5, 0.5, 0}, // remote loud, local silent
			channels:  3,
			aggregate: true,
			want:      true,
		},
		{
			name:      "aggregate local has speech",
			buf:       []float32{0, 0, 0.5, 0, 0, 0.5}, // remote silent, local loud
			channels:  3,
			aggregate: true,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasSourceSpeech(tt.buf, tt.channels, tt.aggregate, thresh)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSplitBySource(t *testing.T) {
	tests := []struct {
		name        string
		interleaved []float32
		channels    int
		wantRemote  []float32
		wantLocal   []float32
	}{
		{
			name:        "3 channels 2 frames",
			interleaved: []float32{0.2, 0.4, 0.9, 0.6, 0.8, 0.1},
			channels:    3,
			wantRemote:  []float32{0.3, 0.7}, // avg of first 2 channels
			wantLocal:   []float32{0.9, 0.1}, // last channel
		},
		{
			name:        "4 channels 1 frame",
			interleaved: []float32{0.1, 0.2, 0.3, 0.8},
			channels:    4,
			wantRemote:  []float32{0.2}, // avg of first 3 channels
			wantLocal:   []float32{0.8},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			remote, local := splitBySource(tt.interleaved, tt.channels)
			require.Len(t, remote, len(tt.wantRemote))
			require.Len(t, local, len(tt.wantLocal))
			for i := range remote {
				assert.InDelta(t, tt.wantRemote[i], remote[i], 1e-6)
			}
			for i := range local {
				assert.InDelta(t, tt.wantLocal[i], local[i], 1e-6)
			}
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

	t.Run("dominanceRatio", func(t *testing.T) {
		assert.Equal(t, config.DefaultDominanceRatio, CaptureConfig{}.dominanceRatio())
		assert.Equal(t, 3.0, CaptureConfig{DominanceRatio: 3.0}.dominanceRatio())
	})

	t.Run("crosstalkThreshold", func(t *testing.T) {
		assert.Equal(t, config.DefaultCrosstalkThreshold, CaptureConfig{}.crosstalkThreshold())
		assert.Equal(t, 0.9, CaptureConfig{CrosstalkThreshold: 0.9}.crosstalkThreshold())
	})
}

func TestCrossCorrelation(t *testing.T) {
	t.Run("empty signals", func(t *testing.T) {
		assert.InDelta(t, 0.0, crossCorrelation(nil, nil), 1e-6)
	})

	t.Run("different length returns zero", func(t *testing.T) {
		assert.InDelta(t, 0.0, crossCorrelation(
			[]float32{0.1, 0.2},
			[]float32{0.1, 0.2, 0.3},
		), 1e-6)
	})

	t.Run("too short for envelope", func(t *testing.T) {
		assert.InDelta(t, 0.0, crossCorrelation(
			[]float32{0.1, 0.5, -0.3},
			[]float32{0.1, 0.5, -0.3},
		), 1e-6)
	})
}

func TestCrossCorrelation_envelopeDetection(t *testing.T) {
	// Generate a signal long enough for multiple envelope windows.
	// envelopeWindowSamples = 160, so we need at least a few hundred samples.
	n := 3200 // 20 envelope frames
	signal := make([]float32, n)
	for i := range n {
		signal[i] = float32(math.Sin(float64(i)*0.05)) * float32(1+math.Sin(float64(i)*0.001))
	}

	t.Run("identical signals", func(t *testing.T) {
		got := crossCorrelation(signal, signal)
		assert.Greater(t, got, 0.99)
	})

	t.Run("scaled copy (crosstalk)", func(t *testing.T) {
		scaled := make([]float32, n)
		for i := range n {
			scaled[i] = signal[i] * 0.4
		}
		got := crossCorrelation(signal, scaled)
		assert.Greater(t, got, 0.99)
	})

	t.Run("delayed and attenuated (speaker to mic)", func(t *testing.T) {
		delayed := make([]float32, n)
		delay := 50
		for i := delay; i < n; i++ {
			delayed[i] = signal[i-delay] * 0.5
		}
		got := crossCorrelation(signal, delayed)
		assert.Greater(t, got, 0.9, "delayed signal should still have high envelope correlation")
	})

	t.Run("uncorrelated signals", func(t *testing.T) {
		other := make([]float32, n)
		for i := range n {
			other[i] = float32(math.Sin(float64(i) * 0.17))
		}
		got := crossCorrelation(signal, other)
		assert.Less(t, got, 0.5, "different signals should have low envelope correlation")
	})
}

func TestPearson(t *testing.T) {
	tests := []struct {
		name string
		a    []float64
		b    []float64
		want float64
	}{
		{
			name: "identical",
			a:    []float64{1, 2, 3, 4, 5},
			b:    []float64{1, 2, 3, 4, 5},
			want: 1.0,
		},
		{
			name: "perfectly anti-correlated",
			a:    []float64{1, 2, 3, 4, 5},
			b:    []float64{5, 4, 3, 2, 1},
			want: -1.0,
		},
		{
			name: "scaled copy",
			a:    []float64{1, 2, 3, 4, 5},
			b:    []float64{2, 4, 6, 8, 10},
			want: 1.0,
		},
		{
			name: "empty",
			a:    nil,
			b:    nil,
			want: 0.0,
		},
		{
			name: "constant (zero variance)",
			a:    []float64{3, 3, 3},
			b:    []float64{3, 3, 3},
			want: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pearson(tt.a, tt.b)
			assert.InDelta(t, tt.want, got, 1e-6)
		})
	}
}

func TestEnvelope(t *testing.T) {
	t.Run("too short", func(t *testing.T) {
		assert.Nil(t, envelope(make([]float32, 100)))
	})

	t.Run("single window", func(t *testing.T) {
		samples := make([]float32, envelopeWindowSamples)
		for i := range samples {
			samples[i] = 0.5
		}
		env := envelope(samples)
		require.Len(t, env, 1)
		assert.InDelta(t, 0.5, env[0], 1e-6)
	})

	t.Run("two windows with different levels", func(t *testing.T) {
		samples := make([]float32, envelopeWindowSamples*2)
		for i := range envelopeWindowSamples {
			samples[i] = 0.2
		}
		for i := envelopeWindowSamples; i < envelopeWindowSamples*2; i++ {
			samples[i] = 0.8
		}
		env := envelope(samples)
		require.Len(t, env, 2)
		assert.InDelta(t, 0.2, env[0], 1e-6)
		assert.InDelta(t, 0.8, env[1], 1e-6)
	})
}
