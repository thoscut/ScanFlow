//go:build linux

package scanner

import "os/exec"

func (w *ButtonWatcher) playBeep() {
	// Try system beep command (available on most Linux systems).
	exec.Command("beep", "-f", "1000", "-l", "100").Run()
}
