//go:build !windows

package tray

func showInExplorer(path string) error {
	return nil
}
