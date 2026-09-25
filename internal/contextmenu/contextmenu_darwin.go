//go:build darwin

package contextmenu

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// For macOS we install a Finder Quick Action, which is a special type of
// Automator workflow that can be invoked from the context menu in Finder

type macOSContextMenu struct {
	path string
}

func applyPlatform(executable string, enabled bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("find home directory: %w", err)
	}
	workflow := macOSContextMenu{
		path: filepath.Join(home, "Library", "Services", menuLabel+".workflow"),
	}
	if enabled {
		return workflow.enable(executable)
	}
	return workflow.disable()
}

func (workflow macOSContextMenu) enable(executable string) error {
	documentPath := filepath.Join(workflow.path, "Contents", "document.wflow")

	// Check if the workflow already exists and if it is owned by us
	// If it exists but is not owned by us, return an error to avoid overwriting someone else's workflow
	if _, err := os.Stat(workflow.path); err == nil {
		document, readErr := os.ReadFile(documentPath)
		if readErr != nil || !bytes.Contains(document, []byte(ownershipMarker)) {
			return fmt.Errorf("%w: %s", ErrFileConflict, workflow.path)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect finder quick action: %w", err)
	}

	// We are able to create / overwrite the workflow
	// Use bundled templates to generate the workflow files

	data := newContextMenuTemplateData(executable, "")
	command, err := renderContextMenuTemplate("macos-command.sh.tmpl", data)
	if err != nil {
		return err
	}

	data.Command = string(command)
	document, err := renderContextMenuTemplate("macos-document.wflow.tmpl", data)
	if err != nil {
		return err
	}

	info, err := renderContextMenuTemplate("macos-info.plist.tmpl", data)
	if err != nil {
		return err
	}

	contentsPath := filepath.Join(workflow.path, "Contents")
	if err := writeOwnedFile(documentPath, document, 0644); err != nil {
		return err
	}
	return writeOwnedFile(filepath.Join(contentsPath, "Info.plist"), info, 0644)
}

func (workflow macOSContextMenu) disable() error {
	documentPath := filepath.Join(workflow.path, "Contents", "document.wflow")
	document, err := os.ReadFile(documentPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read finder quick action: %w", err)
	}
	if !bytes.Contains(document, []byte(ownershipMarker)) {
		return fmt.Errorf("%w: %s", ErrFileConflict, workflow.path)
	}
	if err := os.RemoveAll(workflow.path); err != nil {
		return fmt.Errorf("remove finder quick action: %w", err)
	}
	return nil
}
