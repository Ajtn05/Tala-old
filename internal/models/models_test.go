package models

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCandidatesIncludeConfiguredAndOriginal(t *testing.T) {
	t.Setenv("KIAN_DATA_DIR", t.TempDir())
	t.Setenv("KIAN_MODEL_PATH", "/external/model.bin")
	t.Setenv("KIAN_MODEL_DIR", "/external/models")
	m := Manager{Home: "/Users/test", Exe: "/Applications/Kian Studio.app/Contents/MacOS/kian", WorkingDir: "/workspace"}
	paths := m.Candidates(Settings{ModelPath: "~/my-model.bin", ModelDir: "/configured"})
	joined := strings.Join(paths, "\n")
	for _, p := range []string{"/external/model.bin", "/external/models/" + Name, "/Users/test/my-model.bin", "/configured/" + Name, "/Users/test/Applications/Kian.app/Contents/Resources/models/" + Name, "/Applications/Kian.app/Contents/Resources/models/" + Name} {
		if !strings.Contains(joined, p) {
			t.Errorf("missing candidate %s", p)
		}
	}
	if paths[0] != "/external/model.bin" {
		t.Fatal("explicit override must be preferred")
	}
}
func TestRejectIncompatibleAndCorruptWithoutModification(t *testing.T) {
	for _, data := range []string{"GGUFbad", "lmggcorrupt", ""} {
		p := filepath.Join(t.TempDir(), "model.bin")
		if e := os.WriteFile(p, []byte(data), 0600); e != nil {
			t.Fatal(e)
		}
		if Verify(context.Background(), p) == nil {
			t.Fatal("accepted invalid model")
		}
		after, _ := os.ReadFile(p)
		if string(after) != data {
			t.Fatal("original model modified")
		}
	}
}
func TestCancelledVerification(t *testing.T) {
	p := filepath.Join(t.TempDir(), "model.bin")
	_ = os.WriteFile(p, []byte("lmggcorrupt"), 0600)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if Verify(ctx, p) != context.Canceled {
		t.Fatal("verification did not cancel")
	}
}
func TestEnsureWithoutDownloadPreservesAllFiles(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("KIAN_DATA_DIR", root)
	t.Setenv("KIAN_MODEL_PATH", filepath.Join(home, "invalid.bin"))
	_ = os.WriteFile(filepath.Join(home, "invalid.bin"), []byte("lmggbad"), 0600)
	m := Manager{Home: home, Exe: "/nonexistent", WorkingDir: home}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := m.Ensure(ctx, true, nil); e != context.Canceled {
		t.Fatalf("unexpected error %v", e)
	}
	b, _ := os.ReadFile(filepath.Join(home, "invalid.bin"))
	if string(b) != "lmggbad" {
		t.Fatal("original changed")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatal("cancelled discovery created download files")
	}
}
func TestSettingsSurviveNewManager(t *testing.T) {
	t.Setenv("KIAN_DATA_DIR", t.TempDir())
	m := Manager{}
	want := Settings{ModelPath: "/original/Kian.app/model.bin", ModelDir: "/models", Language: "auto", Prompt: "Taglish"}
	if e := m.Save(want); e != nil {
		t.Fatal(e)
	}
	if got := (Manager{}).Settings(); got != want {
		t.Fatalf("got %+v", got)
	}
}
func TestInstalledModelIntegration(t *testing.T) {
	path := os.Getenv("KIAN_TEST_MODEL")
	if path == "" {
		t.Skip("set KIAN_TEST_MODEL for a full checksum and reuse integration test")
	}
	t.Setenv("KIAN_MODEL_PATH", path)
	m := Default()
	before, e := os.Stat(path)
	if e != nil {
		t.Fatal(e)
	}
	status, e := m.Ensure(context.Background(), true, nil)
	if e != nil {
		t.Fatal(e)
	}
	if status.State != "reused" || status.Path != path {
		t.Fatalf("unexpected reuse: %+v", status)
	}
	after, _ := os.Stat(path)
	if before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("model was changed")
	}
}
