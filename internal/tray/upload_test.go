package tray

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"github.com/sundeiii/yeet-client/internal/config"
	"github.com/sundeiii/yeet-client/pkg/puush"
)

func newTestManager(t *testing.T) *TrayManager {
	t.Helper()
	test.NewTempApp(t)
	m := NewTrayManager(config.DefaultConfig(), puush.NewClientFromApiKey("", ""))
	m.pendingDir = t.TempDir()
	t.Cleanup(m.StopUploadQueue)
	return m
}

func TestHistoryLabel(t *testing.T) {
	now := time.Date(2026, 9, 25, 18, 0, 0, 0, time.UTC)
	today := &puush.HistoryItem{FileName: "my_file.png", Time: time.Date(2026, 9, 25, 14, 32, 0, 0, time.UTC)}
	if got := historyLabel(today, now); got != "my__file.png   ·   14:32" {
		t.Errorf("today's upload: %q", got)
	}

	older := &puush.HistoryItem{FileName: "a very long file name that goes on and on and on.png", Time: time.Date(2026, 9, 2, 9, 0, 0, 0, time.UTC)}
	got := historyLabel(older, now)
	if got != "a very long file name tha…n and on.png   ·   Sep 2" {
		t.Errorf("older upload: %q", got)
	}
}

func TestHumanSize(t *testing.T) {
	cases := map[int64]string{512: "512 B", 1536: "1.5 KB", 27 << 20: "27.0 MB"}
	for size, want := range cases {
		if got := humanSize(size); got != want {
			t.Errorf("humanSize(%d) = %q, want %q", size, got, want)
		}
	}
}

func TestGatherBatch(t *testing.T) {
	m := newTestManager(t)

	// Explorer starts the app once per selected file
	m.enqueue(uploadBatch{jobs: []*uploadJob{newFileJob("b.png")}, gather: true})
	m.enqueue(uploadBatch{jobs: []*uploadJob{{Name: "screenshot.png"}}})
	m.enqueue(uploadBatch{jobs: []*uploadJob{newFileJob("c.png")}, gather: true})

	batches := m.gatherBatch(uploadBatch{jobs: []*uploadJob{newFileJob("a.png")}, gather: true})
	if len(batches) != 2 {
		t.Fatalf("expected the files and the screenshot as 2 batches, got %d", len(batches))
	}
	var names []string
	for _, job := range batches[0].jobs {
		names = append(names, job.Name)
	}
	if len(names) != 3 || names[0] != "a.png" || names[2] != "c.png" {
		t.Errorf("files should be gathered in order, got %v", names)
	}
	if batches[1].jobs[0].Name != "screenshot.png" {
		t.Error("the screenshot should come after the gathered files")
	}
}

func TestFailedUploadsSurviveRestarts(t *testing.T) {
	m := newTestManager(t)

	onDisk := filepath.Join(t.TempDir(), "video.mp4")
	os.WriteFile(onDisk, []byte("video"), 0644)

	screenshot := &uploadJob{Name: "ss (today).png", Data: []byte("png")}
	m.keepFailed(screenshot)
	m.keepFailed(newFileJob(onDisk))
	m.keepFailed(screenshot) // kept once

	if names := m.FailedUploads(); len(names) != 2 {
		t.Fatalf("expected 2 failed uploads, got %v", names)
	}
	if screenshot.Data != nil || screenshot.Pending == "" {
		t.Fatal("the screenshot should have moved to the pending folder")
	}

	// A new session finds both again
	again := newTestManager(t)
	again.pendingDir = m.pendingDir
	again.loadFailed()
	names := again.FailedUploads()
	if len(names) != 2 {
		t.Fatalf("expected 2 failed uploads after a restart, got %v", names)
	}
	for _, job := range again.failed {
		data, err := os.ReadFile(job.Path)
		if err != nil || (job.Name == "ss (today).png") != (string(data) == "png") {
			t.Errorf("%s points at the wrong file: %q %v", job.Name, data, err)
		}
	}

	// Discarding removes the kept copies
	again.DiscardFailedUploads()
	if _, err := os.Stat(screenshot.Pending); !os.IsNotExist(err) {
		t.Error("discarding should delete the kept screenshot")
	}
	if _, err := os.Stat(onDisk); err != nil {
		t.Error("discarding must never delete the user's own files")
	}
	third := newTestManager(t)
	third.pendingDir = m.pendingDir
	third.loadFailed()
	if len(third.FailedUploads()) != 0 {
		t.Error("nothing should be left after discarding")
	}
}
