package updater

import (
	"fmt"
	"strconv"
	"time"
)

type VersionTimestamp struct {
	timestamp time.Time
}

func (v VersionTimestamp) String() string {
	return v.timestamp.Format(time.DateTime)
}

func (v VersionTimestamp) CanCompare(other Version) bool {
	_, ok := other.(VersionTimestamp)
	return ok
}

func (v VersionTimestamp) Compare(anyOther Version) int {
	other, ok := anyOther.(VersionTimestamp)
	if !ok {
		panic(fmt.Sprintf("Cannot compare VersionTimestamp with %T", anyOther))
	}
	return v.timestamp.Compare(other.timestamp)
}

func (v VersionTimestamp) IsNewerThan(other Version) bool {
	return v.Compare(other) > 0
}

func (v VersionTimestamp) IsOlderThan(other Version) bool {
	return v.Compare(other) < 0
}

func (v VersionTimestamp) IsEqualTo(other Version) bool {
	return v.Compare(other) == 0
}

func NewTimestampVersion(timestamp time.Time) VersionTimestamp {
	return VersionTimestamp{timestamp: timestamp.UTC()}
}

func NewTimestampVersionFromString(versionString string) (VersionTimestamp, error) {
	unixTimestamp, err := strconv.ParseInt(versionString, 10, 64)
	if err != nil {
		return VersionTimestamp{}, fmt.Errorf("invalid timestamp version %q: %w", versionString, err)
	}
	return NewTimestampVersion(time.Unix(unixTimestamp, 0)), nil
}
