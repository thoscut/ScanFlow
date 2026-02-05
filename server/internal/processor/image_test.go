package processor

import (
	"image"
	"image/color"
	"testing"
)

func TestIsBlankPage(t *testing.T) {
	// Create a white image
	whiteImg := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			whiteImg.Set(x, y, color.White)
		}
	}

	if !isBlankPage(whiteImg, 0.99) {
		t.Fatal("white image should be detected as blank")
	}

	// Create a non-blank image (black)
	blackImg := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			blackImg.Set(x, y, color.Black)
		}
	}

	if isBlankPage(blackImg, 0.99) {
		t.Fatal("black image should not be detected as blank")
	}

	// Create a mixed image (50% white)
	mixedImg := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			if y < 50 {
				mixedImg.Set(x, y, color.White)
			} else {
				mixedImg.Set(x, y, color.Black)
			}
		}
	}

	if isBlankPage(mixedImg, 0.99) {
		t.Fatal("mixed image should not be detected as blank at 99% threshold")
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Normal Name", "Normal_Name"},
		{"file.pdf", "file.pdf"},
		{"Hello World!", "Hello_World"},
		{"", "document"},
		{"test-file_123", "test-file_123"},
	}

	for _, tt := range tests {
		result := sanitizeFilename(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
