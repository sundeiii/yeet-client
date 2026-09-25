package notifications

import "os/exec"

func playFile(path string) {
	exec.Command("afplay", path).Run()
}
