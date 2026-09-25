package updater

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const updateCheckTimeout = 8 * time.Second

func CanUpdate() bool {
	executable, err := os.Executable()
	if err != nil {
		return false
	}
	return canUpdate(executable)
}

func Check(current Version, branch Branch, force bool) (ReleaseCandidate, error) {
	ctx, cancel := context.WithTimeout(context.Background(), updateCheckTimeout)
	defer cancel()

	release, err := FetchGitHubCandidate(ctx, branch)
	if err != nil {
		return nil, err
	}
	if release == nil {
		// No release / download found
		return nil, nil
	}

	if !current.CanCompare(release.Version()) {
		// We'll assume that we have switched update branches
		return release, nil
	}

	if current.IsOlderThan(release.Version()) || force {
		// We have found a newer version on github or the update was forced
		return release, nil
	}

	// No newer version found
	return nil, nil
}

func Perform(candidate ReleaseCandidate) (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("get executable path: %w", err)
	}
	return performUpdate(candidate.DownloadUrl(), executable)
}

func Cleanup() bool {
	executable, err := os.Executable()
	if err != nil {
		return false
	}
	return cleanupUpdate(executable)
}

func downloadFile(url string, destination string) error {
	out, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer out.Close()

	// TODO: add some context timeout here too, i'm literally too lazy right now
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("unexpected HTTP status: %s", resp.Status)
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}
	return nil
}

func hasWritePermission(path string) bool {
	file, err := os.CreateTemp(path, ".puush-update-test-*")
	if err != nil {
		return false
	}
	originalPath := file.Name()
	renamedPath := originalPath + ".renamed"
	defer os.Remove(originalPath)
	defer os.Remove(renamedPath)

	if err := file.Close(); err != nil {
		return false
	}
	if err := os.Rename(originalPath, renamedPath); err != nil {
		return false
	}
	return os.Remove(renamedPath) == nil
}

func removePathWithBackoff(path string, remove func(string) error, maxAttempts int, backoff time.Duration) (err error) {
	delay := backoff

	for i := 0; i < maxAttempts; i++ {
		err = remove(path)
		if err == nil || os.IsNotExist(err) {
			return nil
		}

		time.Sleep(delay)
		delay *= 2 // Exponential backoff
	}

	return fmt.Errorf("failed to remove %s after %d attempts: %w", path, maxAttempts, err)
}
