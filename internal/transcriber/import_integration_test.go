package transcriber

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"tala/internal/config"
	"tala/internal/media"
	"tala/internal/models"
)

// TestImportOptimizationParity is opt-in because it loads the full model and
// requires normal GPU access on macOS. It never downloads or modifies a model.
// Supply short representative recordings with TALA_TEST_AUDIO (a path list).
func TestImportOptimizationParity(t *testing.T) {
	modelPath := models.Environment("TEST_MODEL")
	if modelPath == "" {
		t.Skip("set TALA_TEST_MODEL to run real inference and thread comparisons")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	if err := models.Verify(ctx, modelPath); err != nil {
		t.Fatal(err)
	}
	language := os.Getenv("TALA_TEST_LANGUAGE")
	if language == "" {
		language = "auto"
	}
	trans, err := New(Config{ModelPath: modelPath, Language: language, Prompt: os.Getenv("TALA_TEST_PROMPT"), Threads: config.DefaultThreads})
	if err != nil {
		t.Fatal(err)
	}
	defer trans.Close()
	paths := []string{filepath.Join("..", "..", "third_party", "whisper.cpp", "samples", "jfk.mp3")}
	if value := os.Getenv("TALA_TEST_AUDIO"); value != "" {
		paths = filepath.SplitList(value)
	}
	type fixture struct {
		name    string
		samples []float32
	}
	var fixtures []fixture
	for _, path := range paths {
		err := media.Stream(ctx, path, func(samples []float32, offset float64) error {
			fixtures = append(fixtures, fixture{path + "@" + time.Duration(offset*float64(time.Second)).String(), samples})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	// Quiet speech and speech with surrounding silence must use the same native
	// decoder, rather than an amplitude threshold that can discard spoken words.
	padded := make([]float32, 30*16000)
	copy(padded[5*16000:], fixtures[0].samples)
	quiet := append([]float32(nil), padded...)
	for i := range quiet {
		quiet[i] *= 0.001
	}
	fixtures = append(fixtures, fixture{"padded-speech", padded}, fixture{"quiet-speech", quiet})
	if _, err := trans.transcribe(ctx, fixtures[0].samples); err != nil {
		t.Fatal(err) // Warm up Metal without including model/graph startup in timings.
	}
	type measurement struct {
		Fixture  string    `json:"fixture"`
		Round    int       `json:"round"`
		Threads  int       `json:"threads"`
		Seconds  float64   `json:"seconds"`
		Exact    bool      `json:"exact_text_and_timestamps"`
		Segments []Segment `json:"segments"`
	}
	logResult := func(v measurement) {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("IMPORT_MEASUREMENT %s", b)
	}
	reference := make([][]Segment, len(fixtures))
	for i, f := range fixtures {
		reference[i], err = trans.transcribe(ctx, f.samples)
		if err != nil {
			t.Fatal(err)
		}
	}
	// Reverse/rotate the order to reduce startup and ordering bias. Candidates
	// are measured only; the application's four-thread default is not changed.
	for round, order := range [][]int{{4, 2, 6, 8}, {8, 6, 2, 4}, {2, 4, 8, 6}} {
		for _, threads := range order {
			trans.threads = threads
			for i, f := range fixtures {
				started := time.Now()
				got, err := trans.Transcribe(ctx, f.samples)
				elapsed := time.Since(started).Seconds()
				if err != nil {
					t.Fatal(err)
				}
				exact := reflect.DeepEqual(reference[i], got)
				logResult(measurement{f.name, round, threads, elapsed, exact, got})
				if threads == config.DefaultThreads && !exact {
					t.Errorf("default-thread output changed for %s: got %+v; reference %+v", f.name, got, reference[i])
				}
			}
		}
	}
	trans.threads = config.DefaultThreads
	silence := make([]float32, 30*16000)
	started := time.Now()
	baseline, err := trans.transcribe(ctx, silence)
	if err != nil {
		t.Fatal(err)
	}
	logResult(measurement{"digital-silence-native", 0, trans.threads, time.Since(started).Seconds(), true, baseline})
	started = time.Now()
	got, err := trans.Transcribe(ctx, silence)
	if err != nil || len(got) != 0 {
		t.Fatalf("digital silence produced speech: %+v, %v", got, err)
	}
	logResult(measurement{"digital-silence-skipped", 0, trans.threads, time.Since(started).Seconds(), reflect.DeepEqual(baseline, got), got})
}
