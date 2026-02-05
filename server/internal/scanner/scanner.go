package scanner

import (
	"context"
	"errors"
	"image"
	"log/slog"
	"sync"

	"github.com/thoscut/scanflow/server/internal/jobs"
)

var (
	ErrNotConnected = errors.New("scanner not connected")
	ErrBusy         = errors.New("scanner is busy")
	ErrNoDevice     = errors.New("no scanner device found")
)

// Device represents a detected scanner device.
type Device struct {
	Name   string `json:"name"`
	Vendor string `json:"vendor"`
	Model  string `json:"model"`
	Type   string `json:"type"`
}

// ScanOptions configures scanner settings.
type ScanOptions struct {
	Resolution int
	Mode       string
	Source     string
	PageWidth  float64
	PageHeight float64
	Brightness int
	Contrast   int
}

// Scanner manages scanner hardware access via SANE.
type Scanner struct {
	deviceName string
	autoOpen   bool
	defaults   ScanOptions

	devices    []Device
	connected  bool
	scanning   bool
	mu         sync.RWMutex

	// SANE backend interface for testability
	backend ScannerBackend
}

// ScannerBackend abstracts the SANE library for testing.
type ScannerBackend interface {
	Init() error
	Close()
	ListDevices() ([]Device, error)
	Open(deviceName string) error
	CloseDevice()
	SetOption(name string, value interface{}) error
	GetOption(name string) (interface{}, error)
	ReadImage() (image.Image, error)
	IsOpen() bool
}

// New creates a new Scanner instance.
func New(deviceName string, autoOpen bool, defaults ScanOptions) *Scanner {
	return &Scanner{
		deviceName: deviceName,
		autoOpen:   autoOpen,
		defaults:   defaults,
		backend:    &stubBackend{},
	}
}

// SetBackend sets the scanner backend (SANE or stub).
func (s *Scanner) SetBackend(b ScannerBackend) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.backend = b
}

// Init initializes the scanner subsystem and optionally discovers/opens devices.
func (s *Scanner) Init() error {
	if err := s.backend.Init(); err != nil {
		return err
	}

	devices, err := s.backend.ListDevices()
	if err != nil {
		slog.Warn("failed to list scanner devices", "error", err)
	} else {
		s.mu.Lock()
		s.devices = devices
		s.mu.Unlock()
		slog.Info("scanner devices found", "count", len(devices))
	}

	if s.autoOpen && len(devices) > 0 {
		deviceName := s.deviceName
		if deviceName == "" {
			deviceName = devices[0].Name
		}
		if err := s.Open(deviceName); err != nil {
			slog.Warn("failed to auto-open scanner", "device", deviceName, "error", err)
		}
	}

	return nil
}

// Discover rescans for available scanner devices.
func (s *Scanner) Discover() ([]Device, error) {
	devices, err := s.backend.ListDevices()
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.devices = devices
	s.mu.Unlock()

	return devices, nil
}

// ListDevices returns the last known list of devices.
func (s *Scanner) ListDevices() []Device {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.devices
}

// GetDevice returns a device by name.
func (s *Scanner) GetDevice(name string) (Device, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, d := range s.devices {
		if d.Name == name {
			return d, true
		}
	}
	return Device{}, false
}

// Open connects to a scanner device.
func (s *Scanner) Open(deviceName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.backend.Open(deviceName); err != nil {
		return err
	}

	s.connected = true
	s.deviceName = deviceName
	slog.Info("scanner opened", "device", deviceName)
	return nil
}

// Close disconnects from the scanner.
func (s *Scanner) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.backend.CloseDevice()
	s.connected = false
	slog.Info("scanner closed")
	return nil
}

// IsConnected returns whether a scanner is currently connected.
func (s *Scanner) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connected
}

// SetOptions configures the scanner with the given options.
func (s *Scanner) SetOptions(opts ScanOptions) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.connected {
		return ErrNotConnected
	}

	if opts.Resolution > 0 {
		if err := s.backend.SetOption("resolution", opts.Resolution); err != nil {
			return err
		}
	}
	if opts.Mode != "" {
		if err := s.backend.SetOption("mode", opts.Mode); err != nil {
			return err
		}
	}
	if opts.Source != "" {
		if err := s.backend.SetOption("source", opts.Source); err != nil {
			return err
		}
	}
	if opts.PageHeight == 0 {
		s.backend.SetOption("page-height", 0) // Unlimited
	} else if opts.PageHeight > 0 {
		s.backend.SetOption("page-height", opts.PageHeight)
	}
	if opts.PageWidth > 0 {
		s.backend.SetOption("page-width", opts.PageWidth)
	}

	return nil
}

// ScanBatch performs a batch scan, returning pages over a channel.
func (s *Scanner) ScanBatch(ctx context.Context, opts ScanOptions) (<-chan *jobs.Page, error) {
	s.mu.Lock()
	if !s.connected {
		s.mu.Unlock()
		return nil, ErrNotConnected
	}
	if s.scanning {
		s.mu.Unlock()
		return nil, ErrBusy
	}
	s.scanning = true
	s.mu.Unlock()

	if err := s.SetOptions(opts); err != nil {
		s.mu.Lock()
		s.scanning = false
		s.mu.Unlock()
		return nil, err
	}

	pages := make(chan *jobs.Page)

	go func() {
		defer close(pages)
		defer func() {
			s.mu.Lock()
			s.scanning = false
			s.mu.Unlock()
		}()

		pageNum := 0
		for {
			select {
			case <-ctx.Done():
				return
			default:
				img, err := s.backend.ReadImage()
				if err != nil {
					// Check for ADF empty / end of batch
					if isEndOfFeed(err) {
						slog.Info("ADF empty, batch scan complete", "pages", pageNum)
						return
					}
					pages <- &jobs.Page{Err: err}
					return
				}

				pageNum++
				bounds := img.Bounds()
				pages <- &jobs.Page{
					Number: pageNum,
					Width:  bounds.Dx(),
					Height: bounds.Dy(),
					Image:  img,
				}
				slog.Debug("page scanned", "page", pageNum,
					"width", bounds.Dx(), "height", bounds.Dy())
			}
		}
	}()

	return pages, nil
}

// GetButtonState reads the current state of a scanner button.
func (s *Scanner) GetButtonState(buttonName string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.connected {
		return false, ErrNotConnected
	}

	val, err := s.backend.GetOption(buttonName)
	if err != nil {
		return false, err
	}

	if b, ok := val.(bool); ok {
		return b, nil
	}
	return false, nil
}

// Shutdown cleans up scanner resources.
func (s *Scanner) Shutdown() {
	s.Close()
	s.backend.Close()
}

func isEndOfFeed(err error) bool {
	// SANE returns specific error codes for empty ADF
	return err.Error() == "document feeder out of documents" ||
		err.Error() == "no more data available" ||
		err.Error() == "end of file"
}
