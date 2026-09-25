package updater

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/google/go-github/v89/github"
)

const GitHubUser = "sundeiii"
const GitHubRepository = "yeet-client"

type GitHubReleaseCandidate struct {
	release *github.RepositoryRelease
	version Version
	branch  Branch
}

func (c *GitHubReleaseCandidate) Version() Version {
	return c.version
}

func (c *GitHubReleaseCandidate) Branch() Branch {
	return c.branch
}

func (c *GitHubReleaseCandidate) Description() string {
	return c.release.GetBody()
}

func (c *GitHubReleaseCandidate) CreatedAt() time.Time {
	return c.release.GetCreatedAt().Time
}

func (c *GitHubReleaseCandidate) IsPrerelease() bool {
	return c.release.GetPrerelease()
}

func (c *GitHubReleaseCandidate) DownloadUrl() string {
	targetFilename := releaseAssetFilename(
		runtime.GOOS,
		runtime.GOARCH,
	)
	for _, asset := range c.release.Assets {
		if asset.GetName() == targetFilename {
			return asset.GetBrowserDownloadURL()
		}
	}
	return ""
}

func FetchGitHubCandidate(ctx context.Context, branch Branch) (ReleaseCandidate, error) {
	var release ReleaseCandidate
	var err error
	switch branch {
	case BranchStable:
		release, err = FetchGitHubRelease(ctx)
	default:
		return nil, fmt.Errorf("unknown update branch %q", branch)
	}
	if err != nil {
		return nil, err
	}
	if release == nil {
		// No release found
		return nil, nil
	}
	if release.DownloadUrl() == "" {
		// No download url found
		return nil, nil
	}
	return release, nil
}

func releaseAssetFilename(goos, goarch string) string {
	switch goos {
	case "darwin":
		return fmt.Sprintf("yeet-macos-%s.app.zip", goarch)
	case "windows":
		return fmt.Sprintf("yeet-windows-%s.exe", goarch)
	default:
		return fmt.Sprintf("yeet-%s-%s", goos, goarch)
	}
}
