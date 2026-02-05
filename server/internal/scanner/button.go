package scanner

import (
	"context"
	"log/slog"
	"os/exec"
	"time"
)

// ButtonWatcher monitors the scanner hardware button via SANE polling
// and distinguishes between short press (< threshold) and long press (>= threshold).
type ButtonWatcher struct {
	scanner       *Scanner
	pollInterval  time.Duration
	longPressDur  time.Duration
	onShortPress  func()
	onLongPress   func()
	beepEnabled   bool

	pressStart    time.Time
	isPressed     bool
	longPressBeep bool
}

// ButtonConfig holds configuration for the button watcher.
type ButtonConfig struct {
	Enabled           bool
	PollInterval      time.Duration
	LongPressDuration time.Duration
	ShortPressProfile string
	LongPressProfile  string
	Output            string
	BeepOnLongPress   bool
}

// NewButtonWatcher creates a new button watcher with the given callbacks.
func NewButtonWatcher(scanner *Scanner, cfg ButtonConfig, onShort, onLong func()) *ButtonWatcher {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 50 * time.Millisecond
	}
	if cfg.LongPressDuration == 0 {
		cfg.LongPressDuration = 1 * time.Second
	}

	return &ButtonWatcher{
		scanner:      scanner,
		pollInterval: cfg.PollInterval,
		longPressDur: cfg.LongPressDuration,
		onShortPress: onShort,
		onLongPress:  onLong,
		beepEnabled:  cfg.BeepOnLongPress,
	}
}

// Start begins polling the scanner button. Blocks until context is cancelled.
func (w *ButtonWatcher) Start(ctx context.Context) {
	slog.Info("button watcher started",
		"poll_interval", w.pollInterval,
		"long_press_threshold", w.longPressDur)

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("button watcher stopped")
			return
		case <-ticker.C:
			w.poll()
		}
	}
}

func (w *ButtonWatcher) poll() {
	pressed, err := w.scanner.GetButtonState("scan")
	if err != nil {
		return // Scanner busy during scan
	}

	now := time.Now()

	if pressed && !w.isPressed {
		// Button just pressed - start timing
		w.pressStart = now
		w.isPressed = true
		w.longPressBeep = false
		slog.Debug("button pressed, measuring duration...")

	} else if pressed && w.isPressed {
		// Button held - check if long press threshold reached
		if w.beepEnabled && !w.longPressBeep &&
			now.Sub(w.pressStart) >= w.longPressDur {
			w.playBeep()
			w.longPressBeep = true
			slog.Debug("long press threshold reached")
		}

	} else if !pressed && w.isPressed {
		// Button released - determine action
		w.isPressed = false
		duration := now.Sub(w.pressStart)

		if duration >= w.longPressDur {
			slog.Info("long press detected", "duration", duration)
			if w.onLongPress != nil {
				go w.onLongPress()
			}
		} else {
			slog.Info("short press detected", "duration", duration)
			if w.onShortPress != nil {
				go w.onShortPress()
			}
		}
	}
}

func (w *ButtonWatcher) playBeep() {
	// Try system beep command (available on most Linux systems)
	exec.Command("beep", "-f", "1000", "-l", "100").Run()
}
