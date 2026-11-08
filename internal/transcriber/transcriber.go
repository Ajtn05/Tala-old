package transcriber

import (
	"context"
	"fmt"
	"io"
	"time"

	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"

	"tala/internal/config"
)

type Segment struct {
	Source string
	Start  time.Duration
	End    time.Duration
	Text   string
}

type Config struct {
	ModelPath string
	Language  string
	Threads   int
	Prompt    string
}

type Transcriber struct {
	model   whisper.Model
	lang    string
	threads int
	prompt  string
}

func New(cfg Config) (*Transcriber, error) {
	model, err := whisper.New(cfg.ModelPath)
	if err != nil {
		return nil, fmt.Errorf("load model %s: %w", cfg.ModelPath, err)
	}

	threads := cfg.Threads
	if threads <= 0 {
		threads = config.DefaultThreads
	}

	lang := cfg.Language
	if lang == "" {
		lang = config.DefaultLanguage
	}

	return &Transcriber{
		model:   model,
		lang:    lang,
		threads: threads,
		prompt:  cfg.Prompt,
	}, nil
}

func (t *Transcriber) Transcribe(ctx context.Context, samples []float32) ([]Segment, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Only exact digital silence is safe to omit without a speech detector.
	// A nonzero sample, however quiet, must still reach the unchanged decoder.
	if digitalSilence(samples) {
		return nil, nil
	}
	return t.transcribe(ctx, samples)
}

func digitalSilence(samples []float32) bool {
	if len(samples) == 0 {
		return false
	}
	for _, sample := range samples {
		if sample != 0 {
			return false
		}
	}
	return true
}

// transcribe is the unchanged native path, also used as the benchmark reference.
func (t *Transcriber) transcribe(ctx context.Context, samples []float32) ([]Segment, error) {
	wctx, err := t.model.NewContext()
	if err != nil {
		return nil, fmt.Errorf("create context: %w", err)
	}

	if err := wctx.SetLanguage(t.lang); err != nil {
		return nil, fmt.Errorf("set language %s: %w", t.lang, err)
	}
	wctx.SetThreads(uint(t.threads))
	wctx.SetTranslate(false)
	if t.prompt != "" {
		wctx.SetInitialPrompt(t.prompt)
	}

	abort := func() bool {
		return ctx.Err() == nil
	}
	if err := wctx.Process(samples, abort, nil, nil); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("process audio: %w", err)
	}

	var segments []Segment
	for {
		seg, err := wctx.NextSegment()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read segment: %w", err)
		}
		if seg.Text == "" {
			continue
		}
		segments = append(segments, Segment{
			Start: seg.Start,
			End:   seg.End,
			Text:  seg.Text,
		})
	}
	return segments, nil
}

func (t *Transcriber) Close() error {
	return t.model.Close()
}

// SetOptions is called by the shared engine while holding its inference lock.
func (t *Transcriber) SetOptions(language, prompt string) { t.lang = language; t.prompt = prompt }
