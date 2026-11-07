package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tala/internal/app"
	"tala/internal/models"
	"tala/internal/session"
)

func TestCLIOptions(t *testing.T) {
	for _, args := range [][]string{
		{"--record", "--transcribe", "file.wav"},
		{"--list", "--export", "id"},
		{"--duration", "3s"},
		{"--record", "--duration", "-1s"},
		{"--language", "invalid", "audio.wav"},
		{"--format", "exe", "audio.wav"},
		{"--text-mode", "invalid", "audio.wav"},
		{"--format", "srt", "one.wav", "two.wav"},
	} {
		if _, err := parseOptions(args, io.Discard); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
	o, err := parseOptions([]string{"--single-line", "--prompt=", "--transcribe", "first.wav", "--transcribe", "https://example.org/audio", "third.wav"}, io.Discard)
	if err != nil || len(o.input) != 3 || o.textMode != "continuous" || !o.promptSet {
		t.Fatalf("options: %+v %v", o, err)
	}
}

func TestCLIListAndExportWithoutModel(t *testing.T) {
	dir := t.TempDir()
	store := &session.Store{Dir: dir}
	v, err := store.Create("file", "CLI example", "original.wav")
	if err != nil {
		t.Fatal(err)
	}
	seg, err := store.Append(v.ID, session.Segment{Text: "Original", Start: 0, End: 1})
	if err != nil {
		t.Fatal(err)
	}
	correction := "Hello\nTaglish"
	if err := store.Correct(v.ID, seg.ID, &correction); err != nil {
		t.Fatal(err)
	}
	_, err = store.Append(v.ID, session.Segment{Text: "world.", Start: 1, End: 2})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := execute(context.Background(), options{output: dir, list: true}, models.Manager{}, &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	var items []session.Session
	if err := json.Unmarshal(out.Bytes(), &items); err != nil || len(items) != 1 || items[0].ID != v.ID {
		t.Fatalf("list: %s %v", out.String(), err)
	}
	out.Reset()
	o := options{output: dir, export: v.ID, format: "txt", textMode: "continuous"}
	if err := execute(context.Background(), o, models.Manager{}, &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	if out.String() != "Hello Taglish world." {
		t.Fatalf("export: %q", out.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "queue.json")); !os.IsNotExist(err) {
		t.Fatal("read-only CLI initialized a queue")
	}
}

func TestCLIWaitHandlesTerminalSnapshotAndMissingModel(t *testing.T) {
	t.Setenv("TALA_DATA_DIR", t.TempDir())
	manager := models.Manager{Home: t.TempDir(), Exe: "/nonexistent/tala", WorkingDir: t.TempDir()}
	// A model-free headless run must terminate instead of waiting forever.
	c, err := app.NewHeadless(t.TempDir(), models.Settings{Language: "auto"})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitModel(ctx, c); err == nil {
		t.Fatal("cancelled verification succeeded")
	}
	var out bytes.Buffer
	if err := execute(context.Background(), options{checkModel: true}, manager, &out, io.Discard); err == nil || !strings.Contains(out.String(), "missing") {
		t.Fatalf("diagnostic: %s %v", out.String(), err)
	}
}
