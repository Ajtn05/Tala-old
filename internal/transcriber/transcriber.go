package transcriber

import (
	"context"
	"fmt"
	"io"
	"time"

	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"

	"kian/internal/config"
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
}

type Transcriber struct {
	model   whisper.Model
	lang    string
	threads int
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
	}, nil
}

func (t *Transcriber) Transcribe(ctx context.Context, samples []float32) ([]Segment, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	wctx, err := t.model.NewContext()
	if err != nil {
		return nil, fmt.Errorf("create context: %w", err)
	}

	if err := wctx.SetLanguage(t.lang); err != nil {
		return nil, fmt.Errorf("set language %s: %w", t.lang, err)
	}
	wctx.SetThreads(uint(t.threads))

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
