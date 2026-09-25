package updater

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v89/github"
)

const GitHubNightlyTagPrefix = "nightly-"

func FetchGitHubNightly(ctx context.Context) (ReleaseCandidate, error) {
	client, err := github.NewClient()
	if err != nil {
		return nil, err
	}

	releases, _, err := client.Repositories.ListReleases(
		ctx,
		GitHubUser,
		GitHubRepository,
		&github.ListOptions{PerPage: 100},
	)
	if err != nil {
		return nil, err
	}

	for _, release := range releases {
		tag := release.GetTagName()
		if release.GetDraft() || !release.GetPrerelease() || !strings.HasPrefix(tag, GitHubNightlyTagPrefix) {
			continue
		}

		version, err := NewTimestampVersionFromString(strings.TrimPrefix(tag, GitHubNightlyTagPrefix))
		if err != nil {
			return nil, fmt.Errorf("parse nightly release tag %q: %w", tag, err)
		}

		return &GitHubReleaseCandidate{
			release: release,
			version: version,
			branch:  BranchNightly,
		}, nil
	}
	return nil, nil
}
