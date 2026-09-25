//go:build !windows && !darwin

package tray

// Unix systems do not support tray tooltips unfortunately :(

func setTrayTooltip(string) {}
