package tray

import (
	"errors"
	"time"
)

var (
	ErrEmptyUploadQueueRequest = errors.New("no files were provided for upload")
	ErrUploadQueueFull         = errors.New("the upload queue is full")
	ErrUploadQueueStopped      = errors.New("the upload queue has stopped")
)

// uploadBatch is a group of uploads whose links are handed out together.
type uploadBatch struct {
	jobs []*uploadJob

	// gather waits a moment for more files to join the batch: Explorer starts
	// the app once per selected file, and those should become one batch
	gather bool
}

// How long a gathering batch waits for the next file
const gatherWindow = 400 * time.Millisecond

// StartUploadQueue starts the single worker used for all uploads.
// Queueing uploads keeps tray progress & completion state tied to one file.
func (m *TrayManager) StartUploadQueue() {
	m.uploadQueueStart.Do(func() {
		go func() {
			for {
				select {
				case batch := <-m.uploadQueue:
					batches := []uploadBatch{batch}
					if batch.gather {
						batches = m.gatherBatch(batch)
					}
					for _, batch := range batches {
						m.processBatch(batch.jobs)
					}
				case <-m.uploadQueueStop:
					return
				}
			}
		}()
	})
}

// gatherBatch collects files that arrive shortly after the first one into
// its batch. Anything else that arrives meanwhile is returned after it.
func (m *TrayManager) gatherBatch(first uploadBatch) []uploadBatch {
	gathered := first
	var others []uploadBatch

	timer := time.NewTimer(gatherWindow)
	defer timer.Stop()

	for {
		select {
		case next := <-m.uploadQueue:
			if next.gather {
				gathered.jobs = append(gathered.jobs, next.jobs...)
				timer.Reset(gatherWindow)
			} else {
				others = append(others, next)
			}
		case <-timer.C:
			return append([]uploadBatch{gathered}, others...)
		case <-m.uploadQueueStop:
			return nil
		}
	}
}

// StopUploadQueue stops accepting and processing queued uploads.
func (m *TrayManager) StopUploadQueue() {
	m.uploadQueueStopOnce.Do(func() {
		close(m.uploadQueueStop)
	})
}

// EnqueueFile adds a single file to the upload queue.
func (m *TrayManager) EnqueueFile(path string) error {
	return m.EnqueueFiles([]string{path})
}

// EnqueueFiles adds a batch of files to the upload queue.
func (m *TrayManager) EnqueueFiles(paths []string) error {
	if len(paths) == 0 {
		return ErrEmptyUploadQueueRequest
	}

	jobs := make([]*uploadJob, 0, len(paths))
	for _, path := range paths {
		jobs = append(jobs, newFileJob(path))
	}
	return m.enqueue(uploadBatch{jobs: jobs, gather: true})
}

// enqueueJob queues a single upload, e.g. a screenshot, without waiting for others.
func (m *TrayManager) enqueueJob(job *uploadJob) {
	if err := m.enqueue(uploadBatch{jobs: []*uploadJob{job}}); err != nil {
		m.OnUploadError(err)
	}
}

func (m *TrayManager) enqueue(batch uploadBatch) error {
	select {
	case <-m.uploadQueueStop:
		return ErrUploadQueueStopped
	default:
	}

	select {
	case m.uploadQueue <- batch:
		for _, job := range batch.jobs {
			m.track(job, func(entry *QueueEntry) {
				entry.Status = QueueWaiting
				entry.Error = ""
				entry.Progress = 0
			})
		}
		return nil
	case <-m.uploadQueueStop:
		return ErrUploadQueueStopped
	default:
		return ErrUploadQueueFull
	}
}
