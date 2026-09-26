//go:build !windows

package screenshots

import "image"

// RecordingSupported reports whether the screen can be recorded here.
func RecordingSupported() bool { return false }

// SelectRecordingArea lets the user pick the area to record.
func SelectRecordingArea() (image.Rectangle, error) {
	return image.Rectangle{}, ErrRecordingNotSupported
}

// StartRecording records an area of the screen into a file at path.
func StartRecording(area image.Rectangle, format RecordingFormat, path string) (*Recording, error) {
	return nil, ErrRecordingNotSupported
}
