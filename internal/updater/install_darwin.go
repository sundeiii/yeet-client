//go:build darwin

package updater

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	bundleExtension  = ".app"
	bundleIdentifier = "ee.tupsujumal.yeet"
)

func canUpdate(executable string) bool {
	bundlePath, err := puushBundlePath(executable)
	if err != nil {
		return false
	}
	return hasWritePermission(filepath.Dir(bundlePath))
}

func performUpdate(downloadURL, executable string) (string, error) {
	bundlePath, err := puushBundlePath(executable)
	if err != nil {
		return "", err
	}

	bundleParent := filepath.Dir(bundlePath) // most likely /Applications or ~/Applications
	stagingDirectory, err := os.MkdirTemp(bundleParent, ".puush-update-*")
	if err != nil {
		return "", fmt.Errorf("create update staging directory: %w", err)
	}
	defer os.RemoveAll(stagingDirectory)

	// Download latest bundle from release candidate
	archivePath := filepath.Join(stagingDirectory, "update.app.zip")
	if err := downloadFile(downloadURL, archivePath); err != nil {
		return "", fmt.Errorf("download app bundle update: %w", err)
	}

	// Extract bundle to the staging directory
	extractedPath := filepath.Join(stagingDirectory, "extracted")
	if err := os.Mkdir(extractedPath, 0o700); err != nil {
		return "", fmt.Errorf("create app bundle extraction directory: %w", err)
	}
	if output, err := exec.Command("/usr/bin/ditto", "-x", "-k", archivePath, extractedPath).CombinedOutput(); err != nil {
		return "", fmt.Errorf("extract app bundle update: %w: %s", err, strings.TrimSpace(string(output)))
	}

	replacementBundle, err := findExtractedAppBundle(extractedPath)
	if err != nil {
		return "", err
	}
	replacementExecutable := filepath.Join(
		replacementBundle,
		"Contents",
		"MacOS",
		filepath.Base(executable),
	)
	if err := validateBundle(replacementBundle, replacementExecutable); err != nil {
		return "", err
	}

	// The bundle has been downloaded & verified
	// We can proceed with the update by replacing the current bundle with the new one

	backupBundle := bundlePath + ".old"
	if _, err := os.Stat(backupBundle); err == nil {
		// The backup bundle already exists, which means the previous update attempt failed to clean up
		return "", fmt.Errorf("old app bundle backup still exists at %s (please restart puush)", backupBundle)
	} else if !os.IsNotExist(err) {
		// Some other error occurred while trying to stat the backup bundle
		return "", fmt.Errorf("inspect old app bundle backup: %w", err)
	}

	// Rename the current bundle to .old
	if err := os.Rename(bundlePath, backupBundle); err != nil {
		return "", fmt.Errorf("back up current app bundle: %w", err)
	}

	// Finally, rename the replacement bundle to the original bundle path
	if err := os.Rename(replacementBundle, bundlePath); err != nil {
		installErr := fmt.Errorf("install replacement app bundle: %w", err)
		if restoreErr := os.Rename(backupBundle, bundlePath); restoreErr != nil {
			return "", errors.Join(installErr, fmt.Errorf("restore current app bundle: %w", restoreErr))
		}
		return "", installErr
	}

	// Return the path to the new executable
	return filepath.Join(bundlePath, "Contents", "MacOS", filepath.Base(executable)), nil
}

func cleanupUpdate(executable string) (hasUpdated bool) {
	bundlePath, err := puushBundlePath(executable)
	if err != nil {
		return false
	}

	backupBundle := bundlePath + ".old"
	if _, err := os.Stat(backupBundle); err == nil {
		go removePathWithBackoff(backupBundle, os.RemoveAll, 5, time.Second)
		go resetScreenCapturePermission()
		return true
	}
	return false
}

func puushBundlePath(executable string) (string, error) {
	executable, err := filepath.Abs(executable)
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}

	// Bundle path: /puush.app/Contents/MacOS/puush
	macOSDirectory := filepath.Dir(executable)
	contentsDirectory := filepath.Dir(macOSDirectory)
	bundlePath := filepath.Dir(contentsDirectory)

	isValidBundle := filepath.Base(macOSDirectory) != "MacOS" ||
		filepath.Base(contentsDirectory) != "Contents" ||
		!strings.EqualFold(filepath.Ext(bundlePath), bundleExtension)

	if isValidBundle {
		return "", fmt.Errorf("executable is not inside a puush app bundle: %s", executable)
	}
	return bundlePath, nil
}

func findExtractedAppBundle(extractedPath string) (string, error) {
	entries, err := os.ReadDir(extractedPath)
	if err != nil {
		return "", fmt.Errorf("inspect extracted app bundle: %w", err)
	}

	// Find the first folder in the directory that has a .app extension
	// If there are multiple, something has definitely gone wrong...
	var bundlePath string
	for _, entry := range entries {
		if !entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), bundleExtension) {
			continue
		}
		if bundlePath != "" {
			return "", errors.New("update archive contains multiple app bundles")
		}
		bundlePath = filepath.Join(extractedPath, entry.Name())
	}
	if bundlePath == "" {
		return "", errors.New("update archive does not contain an app bundle")
	}
	return bundlePath, nil
}

func validateBundle(bundlePath, executable string) error {
	// The "puush" executable should be a regular file with execute permissions
	info, err := os.Stat(executable)
	if err != nil {
		return fmt.Errorf("inspect replacement executable: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("replacement executable is not executable: %s", executable)
	}

	// The bundle should contain an Info.plist file
	if _, err := os.Stat(filepath.Join(bundlePath, "Contents", "Info.plist")); err != nil {
		return fmt.Errorf("inspect replacement app metadata: %w", err)
	}

	// The bundle should be signed with the correct identifier
	requirement := fmt.Sprintf("identifier %q", bundleIdentifier)
	if output, err := exec.Command("/usr/bin/codesign", "--verify", "--strict", "-R="+requirement, bundlePath).CombinedOutput(); err != nil {
		return fmt.Errorf("verify replacement app bundle: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func resetScreenCapturePermission() error {
	// On every update of the puush app, the user has to re-grant screen capture permission in System Preferences
	// This is because we use ad-hoc signing for the app bundle, which means the signature changes on every update
	// TODO: Find a way to have a consistent signature without paying apple a bagillion dollars a year
	if output, err := exec.Command("/usr/bin/tccutil", "reset", "ScreenCapture", bundleIdentifier).CombinedOutput(); err != nil {
		return fmt.Errorf("reset screen capture permission: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
