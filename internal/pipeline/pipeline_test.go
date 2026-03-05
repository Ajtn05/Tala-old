package pipeline

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kian/internal/audio"
	"kian/internal/config"
	"kian/internal/transcriber"
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

func TestWriteSegments_mergesConsecutiveFromSameSource(t *testing.T) {
	var buf bytes.Buffer
	segments := make(chan transcriber.Segment, 10)

	segments <- transcriber.Segment{Source: "Mic", Start: 0, End: 2 * time.Second, Text: "hello"}
	segments <- transcriber.Segment{Source: "Mic", Start: 2 * time.Second, End: 4 * time.Second, Text: "world"}
	close(segments)

	err := writeSegments(Config{}, &buf, segments)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "hello world")
	assert.Equal(t, 1, countLines(output), "should produce a single merged line")
}

func TestWriteSegments_flushesOnSourceChange(t *testing.T) {
	var buf bytes.Buffer
	segments := make(chan transcriber.Segment, 10)

	segments <- transcriber.Segment{Source: "Mic", Start: 0, End: 2 * time.Second, Text: "from mic"}
	segments <- transcriber.Segment{Source: "System Audio", Start: 2 * time.Second, End: 4 * time.Second, Text: "from system"}
	close(segments)

	err := writeSegments(Config{}, &buf, segments)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "[Mic]")
	assert.Contains(t, output, "[System Audio]")
	assert.Equal(t, 2, countLines(output))
}

func TestWriteSegments_emptyChannel(t *testing.T) {
	var buf bytes.Buffer
	segments := make(chan transcriber.Segment)
	close(segments)

	err := writeSegments(Config{}, &buf, segments)
	require.NoError(t, err)
	assert.Empty(t, buf.String())
}

func TestWriteSegments_silenceTimerFlush(t *testing.T) {
	var buf bytes.Buffer
	segments := make(chan transcriber.Segment, 10)

	segments <- transcriber.Segment{Source: "Mic", Start: 0, End: 1 * time.Second, Text: "hello"}

	cfg := Config{SilenceFlushSec: 0.05}

	done := make(chan error, 1)
	go func() {
		done <- writeSegments(cfg, &buf, segments)
	}()

	// Wait for silence timer to fire
	time.Sleep(150 * time.Millisecond)
	assert.Contains(t, buf.String(), "hello", "silence timer should have flushed pending segment")

	close(segments)
	require.NoError(t, <-done)
}

func TestWriteSegments_mergesTextWithSpacing(t *testing.T) {
	tests := []struct {
		name  string
		text1 string
		text2 string
		want  string
	}{
		{
			name:  "no trailing or leading space",
			text1: "hello",
			text2: "world",
			want:  "hello world",
		},
		{
			name:  "first has trailing space",
			text1: "hello ",
			text2: "world",
			want:  "hello world",
		},
		{
			name:  "second has leading space",
			text1: "hello",
			text2: " world",
			want:  "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			segments := make(chan transcriber.Segment, 10)
			segments <- transcriber.Segment{Source: "Mic", Start: 0, End: 1 * time.Second, Text: tt.text1}
			segments <- transcriber.Segment{Source: "Mic", Start: 1 * time.Second, End: 2 * time.Second, Text: tt.text2}
			close(segments)

			err := writeSegments(Config{}, &buf, segments)
			require.NoError(t, err)
			assert.Contains(t, buf.String(), tt.want)
		})
	}
}

func TestTranscribeChunks_propagatesSourceAndOffset(t *testing.T) {
	chunks := make(chan audio.Chunk, 1)
	segments := make(chan transcriber.Segment, 10)

	chunks <- audio.Chunk{
		Source:      "TestSource",
		Samples:    []float32{0.1, 0.2},
		StartOffset: 30 * time.Second,
	}
	close(chunks)

	fakeTranscribe := func(_ context.Context, _ []float32) ([]transcriber.Segment, error) {
		return []transcriber.Segment{
			{Start: 0, End: 2 * time.Second, Text: "hello"},
			{Start: 2 * time.Second, End: 5 * time.Second, Text: "world"},
		}, nil
	}

	err := transcribeChunksFunc(context.Background(), fakeTranscribe, chunks, segments)
	close(segments)
	require.NoError(t, err)

	var result []transcriber.Segment
	for seg := range segments {
		result = append(result, seg)
	}

	require.Len(t, result, 2)
	assert.Equal(t, "TestSource", result[0].Source)
	assert.Equal(t, 30*time.Second, result[0].Start)
	assert.Equal(t, 32*time.Second, result[0].End)
	assert.Equal(t, "TestSource", result[1].Source)
	assert.Equal(t, 32*time.Second, result[1].Start)
	assert.Equal(t, 35*time.Second, result[1].End)
}

func TestTranscribeChunks_cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	chunks := make(chan audio.Chunk, 1)
	segments := make(chan transcriber.Segment, 10)

	cancel()

	chunks <- audio.Chunk{
		Source:  "Test",
		Samples: []float32{0.1},
	}
	close(chunks)

	fakeTranscribe := func(ctx context.Context, _ []float32) ([]transcriber.Segment, error) {
		return nil, ctx.Err()
	}

	err := transcribeChunksFunc(ctx, fakeTranscribe, chunks, segments)
	close(segments)
	require.NoError(t, err, "should return nil on cancellation")
	assert.Empty(t, segments)
}

// countLines counts non-empty lines in output (segments are separated by double newlines)
func countLines(s string) int {
	n := 0
	for _, line := range bytes.Split([]byte(s), []byte("\n\n")) {
		if len(bytes.TrimSpace(line)) > 0 {
			n++
		}
	}
	return n
}
