package processor

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/thoscut/scanflow/server/internal/config"
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

func TestValidOCRLang(t *testing.T) {
	valid := []string{"eng", "deu", "deu+eng", "chi_sim", "chi_sim+eng"}
	for _, lang := range valid {
		if !validOCRLang.MatchString(lang) {
			t.Errorf("expected %q to be valid", lang)
		}
	}

	invalid := []string{"eng; rm -rf /", "eng && cat /etc/passwd", "$(evil)", "eng\nfoo", ""}
	for _, lang := range invalid {
		if validOCRLang.MatchString(lang) {
			t.Errorf("expected %q to be invalid", lang)
		}
	}
}

func TestConvertToGrayscale(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	gray := convertToGrayscale(img)
	if gray.Bounds().Dx() != 10 || gray.Bounds().Dy() != 10 {
		t.Fatalf("unexpected bounds: %v", gray.Bounds())
	}
	// Red → gray should yield a specific luminance (~76)
	g := gray.GrayAt(5, 5)
	if g.Y < 50 || g.Y > 100 {
		t.Fatalf("unexpected gray value for red pixel: %d", g.Y)
	}
}

func TestAdjustBrightness(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: 100, G: 100, B: 100, A: 255})
		}
	}
	brighter := adjustBrightness(img, 0.1) // +25.5
	r, _, _, _ := brighter.At(2, 2).RGBA()
	if r>>8 < 120 {
		t.Fatalf("expected brighter pixel, got R=%d", r>>8)
	}

	darker := adjustBrightness(img, -0.2)
	r2, _, _, _ := darker.At(2, 2).RGBA()
	if r2>>8 > 60 {
		t.Fatalf("expected darker pixel, got R=%d", r2>>8)
	}
}

func TestAdjustContrast(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 200, B: 200, A: 255})
		}
	}
	out := adjustContrast(img, 0.5)
	r, _, _, _ := out.At(2, 2).RGBA()
	// 128 + 1.5*(200-128) = 128 + 108 = 236
	if r>>8 < 230 {
		t.Fatalf("expected increased contrast pixel, got R=%d", r>>8)
	}
}

func TestNormalizeExposure(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	// Half dark, half lighter
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if x < 4 {
				img.Set(x, y, color.RGBA{R: 50, G: 50, B: 50, A: 255})
			} else {
				img.Set(x, y, color.RGBA{R: 200, G: 200, B: 200, A: 255})
			}
		}
	}
	out := normalizeExposure(img)
	// Brightest pixels should be near 255
	r, _, _, _ := out.At(7, 0).RGBA()
	if r>>8 < 240 {
		t.Fatalf("expected normalized bright pixel near 255, got %d", r>>8)
	}
	// Darkest pixels should be near 0
	r2, _, _, _ := out.At(0, 0).RGBA()
	if r2>>8 > 15 {
		t.Fatalf("expected normalized dark pixel near 0, got %d", r2>>8)
	}
}

func TestClampU8(t *testing.T) {
	if clampU8(-10) != 0 {
		t.Fatal("negative should clamp to 0")
	}
	if clampU8(300) != 255 {
		t.Fatal("overflow should clamp to 255")
	}
	if clampU8(100) != 100 {
		t.Fatal("in-range should pass through")
	}
}

func TestApplyImageFiltersNoOp(t *testing.T) {
	dir := t.TempDir()
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	path := filepath.Join(dir, "page.jpg")
	saveImageAsJPEG(path, img, 85)

	paths, err := applyImageFilters([]string{path}, config.ImageFilterConfig{}, config.ProfileProcessing{})
	if err != nil {
		t.Fatalf("no-op filter failed: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(paths))
	}
}

func TestApplyImageFiltersGrayscale(t *testing.T) {
	dir := t.TempDir()
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	path := filepath.Join(dir, "page.png")
	f, _ := os.Create(path)
	_ = saveImageAsJPEG(path, img, 95)
	f.Close()

	paths, err := applyImageFilters([]string{path}, config.ImageFilterConfig{ColorToGrayscale: true}, config.ProfileProcessing{})
	if err != nil {
		t.Fatalf("grayscale filter failed: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(paths))
	}
	// Verify the file was rewritten
	info, _ := os.Stat(paths[0])
	if info.Size() == 0 {
		t.Fatal("rewritten file is empty")
	}
}
