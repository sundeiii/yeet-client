package tray

import (
	"log"
	"time"

	"github.com/fsnotify/fsnotify"
)

// StartMonitor starts monitoring the specified directories for new files
func (m *TrayManager) StartMonitor(directories []string) error {
	m.StopMonitor()

	if len(directories) == 0 {
		// Nothing to monitor
		return nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	m.watcher = watcher

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Create) {
					// Trigger an upload when a new file is created in a watched directory
					log.Println("New file upload event:", event.Name)

					// Wait a little bit to ensure the file is fully written before attempting to upload
					time.AfterFunc(time.Second, func() {
						if err := m.EnqueueFile(event.Name); err != nil {
							log.Printf("Failed to queue monitored file %s: %v", event.Name, err)
						}
					})
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("Monitor error:", err)
			}
		}
	}()

	for _, dir := range directories {
		err = watcher.Add(dir)
		if err != nil {
			log.Printf("Failed to monitor directory %s: %v\n", dir, err)
			continue
		}
		log.Printf("Monitoring directory: %s\n", dir)
	}
	return nil
}

// StopMonitor stops the current file system watcher if one is running
func (m *TrayManager) StopMonitor() {
	if m.watcher != nil {
		m.watcher.Close()
		m.watcher = nil
	}
}
