//go:build !windows

package screenshots

// ActiveWindowTitle is the title of the window in front, for screenshot
// names. Only Windows says; elsewhere the name leaves it out.
func ActiveWindowTitle() string {
	return ""
}
