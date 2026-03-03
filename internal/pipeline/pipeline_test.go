package pipeline

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kian/internal/config"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{
			name: "zero",
			d:    0,
			want: "00:00",
		},
		{
			name: "30 seconds",
			d:    30 * time.Second,
			want: "00:30",
		},
		{
			name: "1 minute 5 seconds",
			d:    65 * time.Second,
			want: "01:05",
		},
		{
			name: "10 minutes",
			d:    10 * time.Minute,
			want: "10:00",
		},
		{
			name: "over an hour",
			d:    61*time.Minute + 30*time.Second,
			want: "61:30",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatDuration(tt.d))
		})
	}
}

func TestConfig_silenceFlush(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		assert.Equal(t, config.DefaultSilenceFlush, Config{}.silenceFlush())
	})

	t.Run("custom", func(t *testing.T) {
		assert.Equal(t, 3*time.Second, Config{SilenceFlushSec: 3.0}.silenceFlush())
	})
}

func TestFlushPending(t *testing.T) {
	t.Run("nil pending is noop", func(t *testing.T) {
		var buf bytes.Buffer
		var pending *mergedSegment
		err := flushPending(Config{}, &buf, &pending)
		require.NoError(t, err)
		assert.Empty(t, buf.String())
		assert.Nil(t, pending)
	})

	t.Run("writes formatted line and clears pending", func(t *testing.T) {
		var buf bytes.Buffer
		pending := &mergedSegment{
			source: "Microphone",
			start:  30 * time.Second,
			end:    35 * time.Second,
			text:   "hello world",
		}
		err := flushPending(Config{}, &buf, &pending)
		require.NoError(t, err)
		assert.Equal(t, "[Microphone] [00:30 → 00:35] hello world\n\n", buf.String())
		assert.Nil(t, pending)
	})

	t.Run("trims whitespace from text", func(t *testing.T) {
		var buf bytes.Buffer
		pending := &mergedSegment{
			source: "System Audio",
			start:  0,
			end:    5 * time.Second,
			text:   "  some text  ",
		}
		err := flushPending(Config{}, &buf, &pending)
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "some text")
		assert.NotContains(t, buf.String(), "  some text  ")
	})

	t.Run("calls OnSegment callback", func(t *testing.T) {
		var buf bytes.Buffer
		var captured string
		cfg := Config{
			OnSegment: func(line string) { captured = line },
		}
		pending := &mergedSegment{
			source: "Microphone",
			start:  0,
			end:    1 * time.Second,
			text:   "test",
		}
		err := flushPending(cfg, &buf, &pending)
		require.NoError(t, err)
		assert.Equal(t, "[Microphone] [00:00 → 00:01] test", captured)
	})
}
