package notifications

import "os/exec"

func playFile(path string) {
	// PulseAudio or PipeWire first, plain ALSA otherwise
	if err := exec.Command("paplay", path).Run(); err != nil {
		exec.Command("aplay", "-q", path).Run()
	}
}
