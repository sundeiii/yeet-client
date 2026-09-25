package updater

import (
	"context"

	"github.com/google/go-github/v89/github"
)

func FetchGitHubRelease(ctx context.Context) (ReleaseCandidate, error) {
	client, err := github.NewClient()
	if err != nil {
		return nil, err
	}

	// NOTE: This does not include pre-releases, so we'll always consider this as stable updates
	release, response, err := client.Repositories.GetLatestRelease(ctx, GitHubUser, GitHubRepository)
	if response != nil && response.StatusCode == 404 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	tag := release.GetTagName()
	version, err := NewVersionFromString(tag)
	if err != nil {
		return nil, err
	}

	return &GitHubReleaseCandidate{release: release, version: version, branch: BranchStable}, nil
}
