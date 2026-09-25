package updater

// Branch identifies the source to download updates from.
type Branch string

// BranchStable is the only channel: releases published on GitHub.
const BranchStable Branch = "stable"

func (branch Branch) String() string {
	if branch == BranchStable {
		return "Stable"
	}
	return string(branch)
}
