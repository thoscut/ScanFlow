//go:build !linux && !windows

package scanner

func (w *ButtonWatcher) playBeep() {
	// No system beep available on this platform.
}
