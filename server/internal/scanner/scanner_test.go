package scanner

import (
	"context"
	"testing"
)

func TestNewScanner(t *testing.T) {
	opts := ScanOptions{Resolution: 300, Mode: "color"}
	sc := New("", true, opts)

	if sc == nil {
		t.Fatal("scanner should not be nil")
	}
	if sc.IsConnected() {
		t.Fatal("scanner should not be connected initially")
	}
}

func TestScannerInit(t *testing.T) {
	opts := ScanOptions{Resolution: 300, Mode: "color"}
	sc := New("", true, opts)

	if err := sc.Init(); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// With autoOpen=true and stub backend, should be connected
	if !sc.IsConnected() {
		t.Fatal("scanner should be connected after init with auto_open")
	}
}

func TestScannerDiscover(t *testing.T) {
	sc := New("", false, ScanOptions{})
	sc.Init()

	devices, err := sc.Discover()
	if err != nil {
		t.Fatalf("discover failed: %v", err)
	}

	if len(devices) == 0 {
		t.Fatal("expected at least one device from stub backend")
	}

	if devices[0].Name != "test:0" {
		t.Fatalf("expected device 'test:0', got %s", devices[0].Name)
	}
}

func TestScannerListDevices(t *testing.T) {
	sc := New("", false, ScanOptions{})
	sc.Init()

	devices := sc.ListDevices()
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
}

func TestScannerGetDevice(t *testing.T) {
	sc := New("", false, ScanOptions{})
	sc.Init()

	device, ok := sc.GetDevice("test:0")
	if !ok {
		t.Fatal("expected to find device")
	}
	if device.Name != "test:0" {
		t.Fatalf("expected 'test:0', got %s", device.Name)
	}

	_, ok = sc.GetDevice("nonexistent")
	if ok {
		t.Fatal("should not find nonexistent device")
	}
}

func TestScannerOpenClose(t *testing.T) {
	sc := New("", false, ScanOptions{})
	sc.Init()

	if err := sc.Open("test:0"); err != nil {
		t.Fatalf("open failed: %v", err)
	}
	if !sc.IsConnected() {
		t.Fatal("should be connected after open")
	}

	if err := sc.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	if sc.IsConnected() {
		t.Fatal("should not be connected after close")
	}
}

func TestScannerSetOptions(t *testing.T) {
	sc := New("", true, ScanOptions{})
	sc.Init()

	opts := ScanOptions{
		Resolution: 600,
		Mode:       "gray",
		Source:     "flatbed",
		PageWidth:  210.0,
		PageHeight: 297.0,
	}

	if err := sc.SetOptions(opts); err != nil {
		t.Fatalf("set options failed: %v", err)
	}
}

func TestScannerSetOptionsNotConnected(t *testing.T) {
	sc := New("", false, ScanOptions{})
	sc.Init()

	err := sc.SetOptions(ScanOptions{Resolution: 300})
	if err != ErrNotConnected {
		t.Fatalf("expected ErrNotConnected, got: %v", err)
	}
}

func TestScannerScanBatch(t *testing.T) {
	sc := New("", true, ScanOptions{})
	sc.SetBackend(NewTestBackend(3))
	sc.Init()
	sc.Open("test:0")

	ctx := context.Background()
	pages, err := sc.ScanBatch(ctx, ScanOptions{Resolution: 300})
	if err != nil {
		t.Fatalf("scan batch failed: %v", err)
	}

	count := 0
	for page := range pages {
		if page.Err != nil {
			t.Fatalf("page error: %v", page.Err)
		}
		count++
		if page.Number != count {
			t.Fatalf("expected page %d, got %d", count, page.Number)
		}
	}

	if count != 3 {
		t.Fatalf("expected 3 pages, got %d", count)
	}
}

func TestScannerScanBatchNotConnected(t *testing.T) {
	sc := New("", false, ScanOptions{})
	sc.Init()

	_, err := sc.ScanBatch(context.Background(), ScanOptions{})
	if err != ErrNotConnected {
		t.Fatalf("expected ErrNotConnected, got: %v", err)
	}
}

func TestScannerGetButtonState(t *testing.T) {
	sc := New("", true, ScanOptions{})
	sc.Init()

	pressed, err := sc.GetButtonState("scan")
	if err != nil {
		t.Fatalf("get button state failed: %v", err)
	}
	if pressed {
		t.Fatal("expected button not pressed")
	}
}

func TestScannerGetButtonStateNotConnected(t *testing.T) {
	sc := New("", false, ScanOptions{})
	sc.Init()

	_, err := sc.GetButtonState("scan")
	if err != ErrNotConnected {
		t.Fatalf("expected ErrNotConnected, got: %v", err)
	}
}
