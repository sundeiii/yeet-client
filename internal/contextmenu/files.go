package contextmenu

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

var ErrFileConflict = errors.New("an unmanaged file already exists at the context-menu path")

func writeOwnedFile(path string, content []byte, mode fs.FileMode) error {
	existing, err := os.ReadFile(path)
	if err == nil && !bytes.Contains(existing, []byte(ownershipMarker)) {
		return fmt.Errorf("%w: %s", ErrFileConflict, path)
	}
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("inspect %s: %w", path, err)
	}
	return writeFileAtomically(path, content, mode)
}

func removeOwnedFile(path string) error {
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if !bytes.Contains(content, []byte(ownershipMarker)) {
		return fmt.Errorf("%w: %s", ErrFileConflict, path)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

func writeFileAtomically(path string, content []byte, mode fs.FileMode) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0755); err != nil {
		return fmt.Errorf("create %s: %w", directory, err)
	}

	temporary, err := os.CreateTemp(directory, ".puush-context-menu-*")
	if err != nil {
		return fmt.Errorf("create temporary context-menu file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return fmt.Errorf("set permissions on %s: %w", temporaryPath, err)
	}
	if _, err := temporary.Write(content); err != nil {
		temporary.Close()
		return fmt.Errorf("write %s: %w", temporaryPath, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close %s: %w", temporaryPath, err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}
