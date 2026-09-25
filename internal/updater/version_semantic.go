package updater

import "fmt"

type VersionSemantic struct {
	Major int
	Minor int
	Patch int
}

func (v VersionSemantic) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func (v VersionSemantic) CanCompare(other Version) bool {
	_, ok := other.(VersionSemantic)
	return ok
}

func (v VersionSemantic) Compare(anyOther Version) int {
	other, ok := anyOther.(VersionSemantic)
	if !ok {
		panic(fmt.Sprintf("Cannot compare VersionSemantic with %T", anyOther))
	}

	if v.Major != other.Major {
		return v.Major - other.Major
	}
	if v.Minor != other.Minor {
		return v.Minor - other.Minor
	}
	return v.Patch - other.Patch
}

func (v VersionSemantic) IsNewerThan(other Version) bool {
	return v.Compare(other) > 0
}

func (v VersionSemantic) IsOlderThan(other Version) bool {
	return v.Compare(other) < 0
}

func (v VersionSemantic) IsEqualTo(other Version) bool {
	return v.Compare(other) == 0
}

func NewSemanticVersion(major, minor, patch int) VersionSemantic {
	return VersionSemantic{Major: major, Minor: minor, Patch: patch}
}

func NewSemanticVersionFromString(versionString string) (VersionSemantic, error) {
	var major, minor, patch int
	n, err := fmt.Sscanf(versionString, "%d.%d.%d", &major, &minor, &patch)
	if err != nil {
		return VersionSemantic{}, err
	}
	if n != 3 {
		return VersionSemantic{}, fmt.Errorf("invalid version string: %s", versionString)
	}
	return NewSemanticVersion(major, minor, patch), nil
}
