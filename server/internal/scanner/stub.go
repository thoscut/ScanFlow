package scanner

import (
	"errors"
	"image"
	"image/color"
)

// stubBackend provides a no-op SANE backend for development and testing
// without actual scanner hardware or SANE libraries installed.
type stubBackend struct {
	open       bool
	deviceName string
	options    map[string]interface{}
}

func (s *stubBackend) Init() error {
	s.options = make(map[string]interface{})
	return nil
}

func (s *stubBackend) Close() {
	s.open = false
}

func (s *stubBackend) ListDevices() ([]Device, error) {
	return []Device{
		{
			Name:   "test:0",
			Vendor: "Test",
			Model:  "Virtual Scanner",
			Type:   "virtual device",
		},
	}, nil
}

func (s *stubBackend) Open(deviceName string) error {
	s.open = true
	s.deviceName = deviceName
	return nil
}

func (s *stubBackend) CloseDevice() {
	s.open = false
}

func (s *stubBackend) SetOption(name string, value interface{}) error {
	if !s.open {
		return errors.New("device not open")
	}
	s.options[name] = value
	return nil
}

func (s *stubBackend) GetOption(name string) (interface{}, error) {
	if !s.open {
		return nil, errors.New("device not open")
	}
	if val, ok := s.options[name]; ok {
		return val, nil
	}
	// Return default values for known options
	switch name {
	case "scan":
		return false, nil
	case "resolution":
		return 300, nil
	case "mode":
		return "color", nil
	}
	return nil, nil
}

func (s *stubBackend) ReadImage() (image.Image, error) {
	if !s.open {
		return nil, errors.New("device not open")
	}
	// Return end of feed after first call to simulate single page
	return nil, errors.New("document feeder out of documents")
}

func (s *stubBackend) IsOpen() bool {
	return s.open
}

// testBackend provides a scanner backend that generates test images.
type testBackend struct {
	stubBackend
	pagesRemaining int
}

// NewTestBackend creates a backend that generates N test pages.
func NewTestBackend(pages int) ScannerBackend {
	return &testBackend{
		pagesRemaining: pages,
	}
}

func (t *testBackend) ReadImage() (image.Image, error) {
	if !t.open {
		return nil, errors.New("device not open")
	}
	if t.pagesRemaining <= 0 {
		return nil, errors.New("document feeder out of documents")
	}
	t.pagesRemaining--

	// Generate a simple test image (A4 at 100 DPI = ~827x1169 pixels)
	width, height := 827, 1169
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	// Fill with white
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.White)
		}
	}
	// Draw a border
	for x := 0; x < width; x++ {
		img.Set(x, 0, color.Black)
		img.Set(x, height-1, color.Black)
	}
	for y := 0; y < height; y++ {
		img.Set(0, y, color.Black)
		img.Set(width-1, y, color.Black)
	}

	return img, nil
}
