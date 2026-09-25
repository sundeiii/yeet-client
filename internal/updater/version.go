package updater

import "fmt"

type Version interface {
	String() string
	CanCompare(other Version) bool
	Compare(other Version) int
	IsNewerThan(other Version) bool
	IsOlderThan(other Version) bool
	IsEqualTo(other Version) bool
}

func NewVersionFromString(versionString string) (Version, error) {
	semanticVersion, err := NewSemanticVersionFromString(versionString)
	if err == nil {
		return semanticVersion, nil
	}

	timestampVersion, err := NewTimestampVersionFromString(versionString)
	if err == nil {
		return timestampVersion, nil
	}

	integerVersion, err := NewIntegerVersionFromString(versionString)
	if err == nil {
		return integerVersion, nil
	}

	return nil, fmt.Errorf("invalid version string: %s", versionString)
}
