package tray

import "errors"

var (
	ErrEmptyUploadQueueRequest = errors.New("no files were provided for upload")
	ErrUploadQueueFull         = errors.New("the upload queue is full")
	ErrUploadQueueStopped      = errors.New("the upload queue has stopped")
)

// StartUploadQueue starts the single worker used for file uploads.
// Queueing these uploads keeps tray progress & completion state tied to one file.
func (m *TrayManager) StartUploadQueue() {
	m.uploadQueueStart.Do(func() {
		go func() {
			for {
				select {
				case paths := <-m.uploadQueue:
					for _, path := range paths {
						m.PerformFileUpload(path)
					}
				case <-m.uploadQueueStop:
					return
				}
			}
		}()
	})
}

// StopUploadQueue stops accepting and processing queued file uploads.
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

	batch := append([]string(nil), paths...)
	select {
	case <-m.uploadQueueStop:
		return ErrUploadQueueStopped
	default:
	}

	select {
	case m.uploadQueue <- batch:
		return nil
	case <-m.uploadQueueStop:
		return ErrUploadQueueStopped
	default:
		return ErrUploadQueueFull
	}
}
