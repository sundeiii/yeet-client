//go:build !windows && !darwin && !linux

package contextmenu

func applyPlatform(_ string, _ bool) error {
	return ErrUnsupportedPlatform
}
