package updater

import "fmt"

type VersionInteger struct {
	number int
}

func (v VersionInteger) String() string {
	return fmt.Sprintf("%d", v.number)
}

func (v VersionInteger) CanCompare(other Version) bool {
	_, ok := other.(VersionInteger)
	return ok
}

func (v VersionInteger) Compare(anyOther Version) int {
	other, ok := anyOther.(VersionInteger)
	if !ok {
		panic(fmt.Sprintf("Cannot compare VersionBuild with %T", anyOther))
	}
	return v.number - other.number
}

func (v VersionInteger) IsNewerThan(other Version) bool {
	return v.Compare(other) > 0
}

func (v VersionInteger) IsOlderThan(other Version) bool {
	return v.Compare(other) < 0
}

func (v VersionInteger) IsEqualTo(other Version) bool {
	return v.Compare(other) == 0
}

func NewIntegerVersion(commitNumber int) VersionInteger {
	return VersionInteger{number: commitNumber}
}

func NewIntegerVersionFromString(versionString string) (VersionInteger, error) {
	var commitNumber int
	n, err := fmt.Sscanf(versionString, "%d", &commitNumber)
	if err != nil {
		return VersionInteger{}, err
	}
	if n != 1 {
		return VersionInteger{}, fmt.Errorf("invalid version string: %s", versionString)
	}
	return NewIntegerVersion(commitNumber), nil
}
