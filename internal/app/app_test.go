package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"tala/internal/audio"
	"tala/internal/models"
	"tala/internal/session"
	"testing"
	"time"
)

func controller(t *testing.T) *Controller {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	c := &Controller{Store: &session.Store{Dir: t.TempDir()}, ctx: ctx, cancel: cancel, wake: make(chan struct{}, 1), Events: make(chan Event, 2)}
	c.cond = sync.NewCond(&c.mu)
	return c
}
func TestRecordingSourceSynchronization(t *testing.T) {
	c := controller(t)
	if e := c.SetMode(audio.ModeMicrophone); e != nil {
		t.Fatal(e)
	}
	if c.LiveState().Mode != audio.ModeMicrophone {
		t.Fatal("controller not updated")
	}
	c.mu.Lock()
	c.live.Active = true
	c.mu.Unlock()
	if c.SetMode(audio.ModeSystem) == nil {
		t.Fatal("changed audio source during recording")
	}
	if c.LiveState().Mode != audio.ModeMicrophone {
		t.Fatal("source unexpectedly changed")
	}
	if c.Start() == nil {
		t.Fatal("second recording was accepted")
	}
}
func TestMenuBarPreferenceWithoutModelAndWhileRecording(t *testing.T) {
	t.Setenv("TALA_DATA_DIR", t.TempDir())
	c := controller(t)
	c.Models = models.Manager{}
	want := models.Settings{Language: "auto", Prompt: "Ateneo", ModelDir: "/existing/models"}
	if err := c.Models.Save(want); err != nil {
		t.Fatal(err)
	}
	c.live.Active = true
	if err := c.SetMenuBarIcon(true); err != nil {
		t.Fatal(err)
	}
	want.MenuBarIcon = true
	if got := c.Models.Settings(); got != want {
		t.Fatalf("other settings changed: %+v", got)
	}
	if err := c.SetMenuBarIcon(false); err != nil {
		t.Fatal(err)
	}
	if c.Models.Settings().MenuBarIcon {
		t.Fatal("icon was not disabled")
	}
}
func TestHeadlessLeavesEarlierWaitingJobsAlone(t *testing.T) {
	t.Setenv("TALA_DATA_DIR", t.TempDir())
	dir := t.TempDir()
	jobs := []Job{{ID: "previous", State: "waiting", Source: "/do/not/open.wav", Kind: "file"}}
	b, _ := json.Marshal(jobs)
	if err := os.WriteFile(filepath.Join(dir, "queue.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	settings := models.Settings{Language: "tl", Prompt: "barangay"}
	manager := models.Manager{Home: t.TempDir(), WorkingDir: t.TempDir(), Exe: "/nonexistent/tala"}
	c, err := newControllerWithOptions(dir, manager, &settings)
	if err != nil {
		t.Fatal(err)
	}
	// Closing waits for the worker, including the initial wake-up checkpoint.
	c.Close()
	if got := c.Jobs(); len(got) != 1 || got[0].State != "waiting" {
		t.Fatalf("headless run consumed earlier queue: %+v", got)
	}
	if got := c.recognitionSettings(); got.Prompt != "barangay" || got.Language != "tl" {
		t.Fatal("invocation overrides were ignored")
	}
	if got := manager.Settings(); got.Prompt != "" || got.Language != "auto" {
		t.Fatal("invocation overrides changed saved settings")
	}
}
func TestBatchWaitsForLiveAndResumes(t *testing.T) {
	c := controller(t)
	c.live.Active = true
	done := make(chan error, 1)
	go func() { done <- c.waitBatch(c.ctx, "job") }()
	select {
	case <-done:
		t.Fatal("batch ran during live recording")
	case <-time.After(25 * time.Millisecond):
	}
	c.mu.Lock()
	c.live.Active = false
	c.cond.Broadcast()
	c.mu.Unlock()
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("batch did not resume")
	}
}
func TestPausedBatchCanBeCancelled(t *testing.T) {
	c := controller(t)
	c.live.Active = true
	ctx, cancel := context.WithCancel(c.ctx)
	done := make(chan error, 1)
	go func() { done <- c.waitBatch(ctx, "job") }()
	cancel()
	select {
	case e := <-done:
		if e != context.Canceled {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled job stayed blocked")
	}
}
func TestQueueCancellationRetryAndPersistence(t *testing.T) {
	c := controller(t)
	c.jobs = []Job{{ID: "job", State: "waiting"}}
	if e := c.CancelJob("job"); e != nil {
		t.Fatal(e)
	}
	if c.jobs[0].State != "cancelled" {
		t.Fatal("waiting job not cancelled")
	}
	if e := c.Retry("job"); e != nil {
		t.Fatal(e)
	}
	if c.jobs[0].State != "waiting" {
		t.Fatal("retry failed")
	}
	b, e := os.ReadFile(c.queuePath())
	if e != nil {
		t.Fatal(e)
	}
	var jobs []Job
	_ = json.Unmarshal(b, &jobs)
	if jobs[0].State != "waiting" {
		t.Fatal("state not persisted")
	}
	ctx, cancel := context.WithCancel(c.ctx)
	c.cancelJob = cancel
	c.activeJob = "job"
	if e = c.CancelJob("job"); e != nil {
		t.Fatal(e)
	}
	if ctx.Err() != context.Canceled {
		t.Fatal("running job context not cancelled")
	}
}
func TestSlowSubscriberDoesNotBlockAndRecoversLastEvent(t *testing.T) {
	c := controller(t)
	for i := 0; i < 1000; i++ {
		c.Emit("segment", i)
	}
	if !c.resync {
		t.Fatal("overflow not detected")
	}
	<-c.Events
	c.AfterEvent()
	found := false
	for len(c.Events) > 0 {
		if (<-c.Events).Type == "resync" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing snapshot recovery after final event")
	}
}
func TestConcurrentEventsAndSnapshots(t *testing.T) {
	c := controller(t)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				c.Emit("activity", "working")
				_ = c.Snapshot()
				_ = c.LiveState()
			}
		}()
	}
	wg.Wait()
}
func TestRestartKeepsCompletedResultsAndMarksInterruptedSessions(t *testing.T) {
	t.Setenv("KIAN_DATA_DIR", t.TempDir())
	home := t.TempDir()
	output := t.TempDir()
	store := &session.Store{Dir: output}
	v, e := store.Create("live", "Unfinished", "Microphone")
	if e != nil {
		t.Fatal(e)
	}
	jobs := []Job{{ID: "finished", State: "completed", Progress: 1}, {ID: "pending", State: "transcribing", Source: filepath.Join(output, "missing.wav"), Kind: "file"}}
	b, _ := json.Marshal(jobs)
	_ = os.WriteFile(filepath.Join(output, "queue.json"), b, 0600)
	c, e := newController(output, models.Manager{Home: home, Exe: filepath.Join(home, "kian"), WorkingDir: home})
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	got, e := c.Store.Get(v.ID)
	if e != nil || got.Status != "interrupted" {
		t.Fatalf("partial session not preserved: %s %v", got.Status, e)
	}
	deadline := time.After(2 * time.Second)
	for {
		c.mu.Lock()
		done := c.jobs[1].State == "failed"
		kept := c.jobs[0].State == "completed"
		c.mu.Unlock()
		if !kept {
			t.Fatal("completed result changed")
		}
		if done {
			break
		}
		select {
		case <-deadline:
			t.Fatal("pending job was not restored")
		case <-time.After(10 * time.Millisecond):
		}
	}
}
func TestRetainedCopyCancellationRemovesOnlyNewFile(t *testing.T) {
	src := filepath.Join(t.TempDir(), "source.wav")
	dest := filepath.Join(t.TempDir(), "copy.wav")
	_ = os.WriteFile(src, []byte("original"), 0600)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if copyFile(ctx, src, dest) != context.Canceled {
		t.Fatal("copy did not cancel")
	}
	if _, e := os.Stat(dest); !os.IsNotExist(e) {
		t.Fatal("partial copy retained")
	}
	b, _ := os.ReadFile(src)
	if string(b) != "original" {
		t.Fatal("source removed")
	}
}
