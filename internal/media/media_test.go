package media

import (
	"context"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestURLValidation(t *testing.T) {
	for _, s := range []string{"file:///etc/passwd", "ftp://example.com/a", "https://user:password@example.com/a", "javascript:alert(1)", "--exec=bad", "https://"} {
		if ValidateURL(s) == nil {
			t.Errorf("accepted %q", s)
		}
	}
	for _, s := range []string{"https://youtube.com/shorts/example", "https://example.org/a.mp3?q=a&b=1", "http://localhost:8000/audio"} {
		if e := ValidateURL(s); e != nil {
			t.Error(e)
		}
	}
}
func synthetic(t *testing.T, seconds int) string {
	t.Helper()
	tool, e := Tool("ffmpeg")
	if e != nil {
		t.Skip(e)
	}
	p := filepath.Join(t.TempDir(), "lecture with spaces.wav")
	if out, e := exec.Command(tool, "-v", "error", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=16000", "-t", strconv.Itoa(seconds), "-ac", "1", p).CombinedOutput(); e != nil {
		t.Fatalf("synthetic fixture: %v %s", e, out)
	}
	return p
}

func TestProbeAndStreamingChunksPreserveTime(t *testing.T) {
	p := synthetic(t, 63)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	m, e := Probe(ctx, p)
	if e != nil || math.Abs(m.Duration-63) > .01 {
		t.Fatalf("probe: %+v %v", m, e)
	}
	var offsets []float64
	var sizes []int
	e = Stream(ctx, p, func(samples []float32, offset float64) error {
		offsets = append(offsets, offset)
		sizes = append(sizes, len(samples))
		if len(samples) > 30*16000 {
			t.Fatal("unbounded PCM chunk")
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	if len(offsets) != 3 || offsets[1] != 30 || offsets[2] != 60 || sizes[2] != 3*16000 {
		t.Fatalf("incorrect chunk offsets: %v %v", offsets, sizes)
	}
}
func TestStreamCancellationStopsDecoder(t *testing.T) {
	p := synthetic(t, 63)
	ctx, cancel := context.WithCancel(context.Background())
	e := Stream(ctx, p, func(_ []float32, _ float64) error { cancel(); return ctx.Err() })
	if !errors.Is(e, context.Canceled) {
		t.Fatalf("did not cancel: %v", e)
	}
}
func TestProbeRejectsNonAudioAndDirectories(t *testing.T) {
	p := filepath.Join(t.TempDir(), "not audio.txt")
	_ = os.WriteFile(p, []byte("test"), 0600)
	if _, e := Probe(context.Background(), p); e == nil {
		t.Fatal("accepted invalid media")
	}
	if _, e := Probe(context.Background(), filepath.Dir(p)); e == nil {
		t.Fatal("accepted directory")
	}
}
func TestMetadataAndDownloadArgumentsAreNotShellCommands(t *testing.T) {
	dir := t.TempDir()
	args := filepath.Join(dir, "args")
	t.Setenv("KIAN_TEST_ARGS", args)
	script := filepath.Join(dir, "yt-dlp")
	_ = os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$KIAN_TEST_ARGS\"\nprintf '%s' '{\"title\":\"Lecture\",\"duration\":12}'\n"), 0700)
	t.Setenv("PATH", dir)
	raw := "https://example.com/audio?title=$(touch%20bad)&a=1"
	m, e := Fetch(context.Background(), raw)
	if e != nil || m.Title != "Lecture" || m.Duration != 12 {
		t.Fatalf("metadata: %+v %v", m, e)
	}
	b, _ := os.ReadFile(args)
	if !strings.Contains(string(b), "--\n"+raw+"\n") {
		t.Fatalf("URL was not an isolated positional argument: %s", b)
	}
}
