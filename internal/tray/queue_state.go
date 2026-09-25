package tray

import (
	"slices"
	"time"

	"fyne.io/fyne/v2"
)

// The upload queue window lists what's waiting, uploading, done or failed in
// this session.

type QueueStatus int

const (
	QueueWaiting QueueStatus = iota
	QueueUploading
	QueueDone
	QueueFailed
	QueueCancelled
)

// QueueEntry is one upload as the queue window shows it.
type QueueEntry struct {
	Id       int
	Name     string
	Status   QueueStatus
	Progress float64 // 0 to 100 while uploading
	Size     int64
	Link     string
	Error    string
	// Retryable: the upload failed and was kept, so it can be tried again
	Retryable bool
	Updated   time.Time

	job *uploadJob
}

// Finished entries kept for the window
const maxFinishedEntries = 50

// QueueEntries returns the uploads of this session, newest first.
func (m *TrayManager) QueueEntries() []QueueEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	entries := make([]QueueEntry, 0, len(m.queue))
	for i := len(m.queue) - 1; i >= 0; i-- {
		entry := *m.queue[i]
		entry.Retryable = entry.Status == QueueFailed && slices.Contains(m.failed, entry.job)
		entries = append(entries, entry)
	}
	return entries
}

// OnQueueChange registers a function that's called (on the main thread)
// whenever an upload in the queue changes.
func (m *TrayManager) OnQueueChange(listener func()) {
	m.mu.Lock()
	m.queueListeners = append(m.queueListeners, listener)
	m.mu.Unlock()
}

// ClearFinishedUploads removes done, cancelled and failed uploads from the list.
func (m *TrayManager) ClearFinishedUploads() {
	m.mu.Lock()
	m.queue = slices.DeleteFunc(m.queue, func(entry *QueueEntry) bool {
		return entry.Status == QueueDone || entry.Status == QueueCancelled || entry.Status == QueueFailed
	})
	m.mu.Unlock()
	m.queueChanged()
}

// RetryUpload tries one kept upload again.
func (m *TrayManager) RetryUpload(entryId int) {
	m.mu.Lock()
	var job *uploadJob
	for _, entry := range m.queue {
		if entry.Id == entryId {
			job = entry.job
		}
	}
	index := slices.Index(m.failed, job)
	if job == nil || index < 0 {
		m.mu.Unlock()
		return
	}
	m.failed = slices.Delete(m.failed, index, index+1)
	m.mu.Unlock()

	job.Attempts = 0
	m.saveFailed()
	m.stateChanged()
	if err := m.enqueue(uploadBatch{jobs: []*uploadJob{job}}); err != nil {
		m.keepFailed(job)
		m.OnUploadError(err)
	}
}

// track updates (or adds) the queue entry of a job.
func (m *TrayManager) track(job *uploadJob, update func(entry *QueueEntry)) {
	m.mu.Lock()
	var entry *QueueEntry
	for _, existing := range m.queue {
		if existing.job == job {
			entry = existing
		}
	}
	if entry == nil {
		m.queueSeq++
		entry = &QueueEntry{Id: m.queueSeq, Name: job.Name, job: job}
		m.queue = append(m.queue, entry)
	}
	update(entry)
	entry.Updated = time.Now()
	m.trimQueue()
	m.mu.Unlock()
	m.queueChanged()
}

// trimQueue forgets the oldest finished uploads. Call with m.mu held.
func (m *TrayManager) trimQueue() {
	finished := 0
	for i := len(m.queue) - 1; i >= 0; i-- {
		status := m.queue[i].Status
		if status == QueueWaiting || status == QueueUploading {
			continue
		}
		finished++
		if finished > maxFinishedEntries {
			m.queue = slices.Delete(m.queue, i, i+1)
		}
	}
}

func (m *TrayManager) queueChanged() {
	m.mu.Lock()
	listeners := append([]func(){}, m.queueListeners...)
	m.mu.Unlock()
	if len(listeners) == 0 {
		return
	}
	fyne.Do(func() {
		for _, listener := range listeners {
			listener()
		}
	})
}
