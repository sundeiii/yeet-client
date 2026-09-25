//go:build !darwin

package updater

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func canUpdate(executable string) bool {
	return hasWritePermission(filepath.Dir(executable))
}

func needsUpdateRepair(_ Version) bool {
	return false
}

func performUpdate(downloadURL, executable string) (string, error) {
	executableInfo, err := os.Stat(executable)
	if err != nil {
		return "", fmt.Errorf("stat current executable: %w", err)
	}

	replacementExecutable := executable + ".new"
	defer os.Remove(replacementExecutable)

	if err := downloadFile(downloadURL, replacementExecutable); err != nil {
		return "", fmt.Errorf("download update: %w", err)
	}

	// Preserve the mode of the current executable so the replacement remains executable on unix
	if err := os.Chmod(replacementExecutable, executableInfo.Mode().Perm()); err != nil {
		return "", fmt.Errorf("set replacement executable permissions: %w", err)
	}

	// Rename current executable to a backup name
	backupExecutable := executable + ".old"
	if err := os.Rename(executable, backupExecutable); err != nil {
		return "", fmt.Errorf("rename current executable: %w", err)
	}

	// Rename the new executable to the original name
	if err := os.Rename(replacementExecutable, executable); err != nil {
		// We're in a really bad state right now, since no executable would be available to run
		// Attempt to restore the original executable
		os.Rename(backupExecutable, executable)
		return "", fmt.Errorf("replace executable with new version: %w", err)
	}
	return executable, nil
}

func cleanupUpdate(executable string) bool {
	newExecutable := executable + ".new"
	backupExecutable := executable + ".old"

	// Remove any incomplete replacement left by an interrupted update
	os.Remove(newExecutable)

	if _, err := os.Stat(backupExecutable); err == nil {
		// Windows may keep the old executable open briefly after the replacement starts
		go removePathWithBackoff(backupExecutable, os.Remove, 5, time.Second)
		return true
	}
	return false
}
