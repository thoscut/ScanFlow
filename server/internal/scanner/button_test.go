package scanner

import (
	"testing"
	"time"
)

func TestButtonWatcherShortPress(t *testing.T) {
	sc := New("", true, ScanOptions{})
	if err := sc.Init(); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	shortPressed := make(chan struct{}, 1)
	bw := NewButtonWatcher(sc, ButtonConfig{LongPressDuration: time.Second}, func() {
		shortPressed <- struct{}{}
	}, nil)

	backend := sc.backend.(*stubBackend)
	if err := backend.SetOption("scan", true); err != nil {
		t.Fatalf("set pressed state failed: %v", err)
	}
	bw.poll()

	bw.pressStart = time.Now().Add(-500 * time.Millisecond)
	if err := backend.SetOption("scan", false); err != nil {
		t.Fatalf("set released state failed: %v", err)
	}
	bw.poll()

	select {
	case <-shortPressed:
	case <-time.After(time.Second):
		t.Fatal("expected short press callback")
	}
}

func TestButtonWatcherLongPressThresholdBeep(t *testing.T) {
	sc := New("", true, ScanOptions{})
	if err := sc.Init(); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	longPressed := make(chan struct{}, 1)
	bw := NewButtonWatcher(sc, ButtonConfig{
		LongPressDuration: 100 * time.Millisecond,
		BeepOnLongPress:   true,
	}, nil, func() {
		longPressed <- struct{}{}
	})

	backend := sc.backend.(*stubBackend)
	if err := backend.SetOption("scan", true); err != nil {
		t.Fatalf("set pressed state failed: %v", err)
	}
	bw.poll()

	bw.pressStart = time.Now().Add(-200 * time.Millisecond)
	bw.poll()
	if !bw.longPressBeep {
		t.Fatal("expected long press beep marker to be set")
	}

	if err := backend.SetOption("scan", false); err != nil {
		t.Fatalf("set released state failed: %v", err)
	}
	bw.poll()

	select {
	case <-longPressed:
	case <-time.After(time.Second):
		t.Fatal("expected long press callback")
	}
}
