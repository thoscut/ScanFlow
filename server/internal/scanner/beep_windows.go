//go:build windows

package scanner

import "os/exec"

func (w *ButtonWatcher) playBeep() {
	// Use PowerShell to emit a short beep on Windows.
	exec.Command("powershell", "-c", "[console]::Beep(1000,100)").Run()
}
