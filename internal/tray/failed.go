package tray

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Uploads that failed are kept until they're retried or discarded, even
// across restarts. Screenshots and clipboard contents are written to the
// pending folder; files from disk are remembered by their path.

const failedListName = "failed.json"

type failedEntry struct {
	Name      string
	Path      string
	LocalCopy string
}

// FailedUploads returns the names of the uploads waiting to be retried.
func (m *TrayManager) FailedUploads() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	names := make([]string, 0, len(m.failed))
	for _, job := range m.failed {
		names = append(names, job.Name)
	}
	return names
}

// RetryFailedUploads queues all kept uploads again.
func (m *TrayManager) RetryFailedUploads() {
	m.mu.Lock()
	jobs := m.failed
	m.failed = nil
	m.mu.Unlock()

	if len(jobs) == 0 {
		return
	}
	for _, job := range jobs {
		job.Attempts = 0
	}
	m.saveFailed()
	m.stateChanged()

	if err := m.enqueue(uploadBatch{jobs: jobs}); err != nil {
		for _, job := range jobs {
			m.keepFailed(job)
		}
		m.OnUploadError(err)
	}
}

// DiscardFailedUploads throws away all kept uploads.
func (m *TrayManager) DiscardFailedUploads() {
	m.mu.Lock()
	jobs := m.failed
	m.failed = nil
	m.mu.Unlock()

	for _, job := range jobs {
		if job.Pending != "" {
			os.Remove(job.Pending)
		}
	}
	m.saveFailed()
	m.stateChanged()
}

// keepFailed adds a job to the failed uploads, writing its contents to the
// pending folder if it only exists in memory.
func (m *TrayManager) keepFailed(job *uploadJob) {
	if job.Data != nil && job.Pending == "" {
		if path, err := m.writePending(job); err == nil {
			job.Pending = path
			job.Path = path
			job.Data = nil
		} else {
			log.Printf("Could not keep %s for a retry: %v", job.Name, err)
		}
	}

	m.mu.Lock()
	if !slices.Contains(m.failed, job) {
		m.failed = append(m.failed, job)
	}
	m.mu.Unlock()

	m.saveFailed()
	m.stateChanged()
}

// forgetFailed removes a job from the failed uploads once it's done with.
func (m *TrayManager) forgetFailed(job *uploadJob) {
	if job.Pending != "" {
		os.Remove(job.Pending)
		job.Pending = ""
	}

	m.mu.Lock()
	index := slices.Index(m.failed, job)
	if index >= 0 {
		m.failed = slices.Delete(m.failed, index, index+1)
	}
	m.mu.Unlock()

	if index >= 0 {
		m.saveFailed()
		m.stateChanged()
	}
}

func (m *TrayManager) writePending(job *uploadJob) (string, error) {
	if err := os.MkdirAll(m.pendingDir, 0755); err != nil {
		return "", err
	}
	// The prefix keeps names unique; the real name follows the first "_"
	path := filepath.Join(m.pendingDir, fmt.Sprintf("%d_%s", time.Now().UnixNano(), job.Name))
	return path, os.WriteFile(path, job.Data, 0644)
}

// saveFailed writes the list of failed files from disk. Kept screenshots are
// found again by looking in the pending folder.
func (m *TrayManager) saveFailed() {
	m.mu.Lock()
	var entries []failedEntry
	for _, job := range m.failed {
		if job.Pending == "" {
			entries = append(entries, failedEntry{Name: job.Name, Path: job.Path, LocalCopy: job.LocalCopy})
		}
	}
	m.mu.Unlock()

	listPath := filepath.Join(m.pendingDir, failedListName)
	if len(entries) == 0 {
		os.Remove(listPath)
		return
	}
	data, err := json.Marshal(entries)
	if err == nil {
		os.MkdirAll(m.pendingDir, 0755)
		err = os.WriteFile(listPath, data, 0644)
	}
	if err != nil {
		log.Printf("Could not save the failed uploads: %v", err)
	}
}

// loadFailed finds uploads that failed in an earlier session.
func (m *TrayManager) loadFailed() {
	var jobs []*uploadJob

	if data, err := os.ReadFile(filepath.Join(m.pendingDir, failedListName)); err == nil {
		var entries []failedEntry
		json.Unmarshal(data, &entries)
		for _, entry := range entries {
			if _, err := os.Stat(entry.Path); err == nil {
				jobs = append(jobs, &uploadJob{Name: entry.Name, Path: entry.Path, LocalCopy: entry.LocalCopy})
			}
		}
	}

	files, _ := os.ReadDir(m.pendingDir)
	for _, file := range files {
		if file.IsDir() || file.Name() == failedListName {
			continue
		}
		_, name, ok := strings.Cut(file.Name(), "_")
		if !ok || name == "" {
			continue
		}
		path := filepath.Join(m.pendingDir, file.Name())
		jobs = append(jobs, &uploadJob{Name: name, Path: path, Pending: path})
	}

	m.mu.Lock()
	m.failed = jobs
	m.mu.Unlock()
	if len(jobs) > 0 {
		log.Printf("%d failed uploads are waiting to be retried", len(jobs))
	}
}
