package updater

import "testing"

func TestSemanticVersionFromTag(t *testing.T) {
	for _, tag := range []string{"1.2.3", "v1.2.3"} {
		version, err := NewSemanticVersionFromString(tag)
		if err != nil || version.String() != "1.2.3" {
			t.Errorf("%q parsed as %v (%v)", tag, version, err)
		}
	}

	older, _ := NewSemanticVersionFromString("1.1.1")
	newer, _ := NewSemanticVersionFromString("1.1.2")
	if !newer.IsNewerThan(older) || older.IsNewerThan(newer) {
		t.Error("1.1.2 should be newer than 1.1.1")
	}

	// Untagged builds are 0.0.0, so they update to the latest release
	untagged, err := NewSemanticVersionFromString("0.0.0")
	if err != nil || !older.IsNewerThan(untagged) {
		t.Error("any release should be newer than an untagged build")
	}
}
