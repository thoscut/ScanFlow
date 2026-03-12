package scanner

import (
	"context"
	"testing"
	"time"
)

func TestScannerShutdown(t *testing.T) {
	sc := New("", true, ScanOptions{})
	sc.Init()

	if !sc.IsConnected() {
		t.Fatal("expected connected after init")
	}

	sc.Shutdown()
	if sc.IsConnected() {
		t.Fatal("expected disconnected after shutdown")
	}
}

func TestScannerScanBatchBusy(t *testing.T) {
	sc := New("", true, ScanOptions{})
	sc.SetBackend(NewTestBackend(100))
	sc.Init()
	sc.Open("test:0")

	ctx, cancel := context.WithCancel(context.Background())

	// Start first batch
	_, err := sc.ScanBatch(ctx, ScanOptions{})
	if err != nil {
		t.Fatalf("first scan batch failed: %v", err)
	}

	// Second batch should fail because scanner is busy
	_, err = sc.ScanBatch(ctx, ScanOptions{})
	if err != ErrBusy {
		t.Fatalf("expected ErrBusy, got: %v", err)
	}

	cancel()
	// Give the goroutine time to clean up
	time.Sleep(50 * time.Millisecond)
}

func TestScannerScanBatchContextCancel(t *testing.T) {
	sc := New("", true, ScanOptions{})
	sc.SetBackend(NewTestBackend(1000))
	sc.Init()
	sc.Open("test:0")

	ctx, cancel := context.WithCancel(context.Background())

	pages, err := sc.ScanBatch(ctx, ScanOptions{})
	if err != nil {
		t.Fatalf("scan batch failed: %v", err)
	}

	// Read one page
	<-pages

	// Cancel and drain
	cancel()

	count := 1
	for range pages {
		count++
	}

	// Should have stopped early due to cancellation
	if count >= 1000 {
		t.Fatalf("expected early termination, got %d pages", count)
	}
}

func TestScannerSetBackend(t *testing.T) {
	sc := New("", false, ScanOptions{})
	sc.Init()

	tb := NewTestBackend(5)
	sc.SetBackend(tb)

	// Init with new backend
	sc.Init()
	devices := sc.ListDevices()
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
}

func TestScannerSetOptionsUnlimitedHeight(t *testing.T) {
	sc := New("", true, ScanOptions{})
	sc.Init()

	// PageHeight=0 means unlimited
	opts := ScanOptions{PageHeight: 0}
	if err := sc.SetOptions(opts); err != nil {
		t.Fatalf("set options failed: %v", err)
	}
}

func TestScannerSetOptionsWithAllFields(t *testing.T) {
	sc := New("", true, ScanOptions{})
	sc.Init()

	opts := ScanOptions{
		Resolution: 600,
		Mode:       "gray",
		Source:     "flatbed",
		PageWidth:  210.0,
		PageHeight: 297.0,
		Brightness: 50,
		Contrast:   50,
	}

	if err := sc.SetOptions(opts); err != nil {
		t.Fatalf("set options failed: %v", err)
	}
}

func TestScannerGetButtonStateNonBoolValue(t *testing.T) {
	sc := New("", true, ScanOptions{})
	sc.Init()

	// Set a non-bool value for the button
	backend := sc.backend.(*stubBackend)
	backend.SetOption("scan", "not-a-bool")

	pressed, err := sc.GetButtonState("scan")
	if err != nil {
		t.Fatalf("get button state failed: %v", err)
	}
	// Non-bool values should return false
	if pressed {
		t.Fatal("expected false for non-bool value")
	}
}

func TestIsEndOfFeed(t *testing.T) {
	tests := []struct {
		err      string
		expected bool
	}{
		{"document feeder out of documents", true},
		{"no more data available", true},
		{"end of file", true},
		{"some other error", false},
		{"scanner jam", false},
	}

	for _, tt := range tests {
		err := &testErr{msg: tt.err}
		if got := isEndOfFeed(err); got != tt.expected {
			t.Errorf("isEndOfFeed(%q) = %v, want %v", tt.err, got, tt.expected)
		}
	}
}

type testErr struct {
	msg string
}

func (e *testErr) Error() string { return e.msg }

func TestNewButtonWatcherDefaults(t *testing.T) {
	sc := New("", true, ScanOptions{})
	sc.Init()

	bw := NewButtonWatcher(sc, ButtonConfig{}, nil, nil)

	if bw.pollInterval != 50*time.Millisecond {
		t.Fatalf("expected default poll interval 50ms, got %v", bw.pollInterval)
	}
	if bw.longPressDur != 1*time.Second {
		t.Fatalf("expected default long press duration 1s, got %v", bw.longPressDur)
	}
}

func TestButtonWatcherStartCancel(t *testing.T) {
	sc := New("", true, ScanOptions{})
	sc.Init()

	bw := NewButtonWatcher(sc, ButtonConfig{PollInterval: 10 * time.Millisecond}, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		bw.Start(ctx)
		close(done)
	}()

	// Let it run briefly
	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// OK - exited cleanly
	case <-time.After(time.Second):
		t.Fatal("button watcher did not stop after context cancel")
	}
}

func TestButtonWatcherNilCallbacks(t *testing.T) {
	sc := New("", true, ScanOptions{})
	sc.Init()

	// Both callbacks are nil - should not panic
	bw := NewButtonWatcher(sc, ButtonConfig{LongPressDuration: time.Second}, nil, nil)

	backend := sc.backend.(*stubBackend)

	// Simulate short press
	backend.SetOption("scan", true)
	bw.poll()
	bw.pressStart = time.Now().Add(-500 * time.Millisecond)
	backend.SetOption("scan", false)
	bw.poll() // Should not panic with nil callback
}
