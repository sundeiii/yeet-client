package updater

// Branch identifies the source to download updates from.
type Branch string

const (
	// Regular github releases
	BranchStable Branch = "stable"
	// Latest github actions builds
	BranchNightly Branch = "nightly"
)

func (branch Branch) String() string {
	switch branch {
	case BranchStable:
		return "Stable"
	case BranchNightly:
		return "Nightly"
	default:
		return string(branch)
	}
}

func NewBranchFromString(s string) Branch {
	switch s {
	case "Stable":
		return BranchStable
	case "Nightly":
		return BranchNightly
	default:
		return Branch(s)
	}
}
