package processor

import (
	"context"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thoscut/scanflow/server/internal/config"
	"github.com/thoscut/scanflow/server/internal/jobs"
)

func TestSaveImages(t *testing.T) {
	dir := t.TempDir()

	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.White)
		}
	}

	pages := []*jobs.Page{
		{Number: 1, Image: img},
		{Number: 2, Image: img},
	}

	paths, err := saveImages(dir, pages)
	if err != nil {
		t.Fatalf("saveImages failed: %v", err)
	}

	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(paths))
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("saved image not found: %v", err)
		}
	}

	// Verify page paths were updated
	for _, page := range pages {
		if page.Path == "" {
			t.Fatalf("page %d path not set", page.Number)
		}
	}
}

func TestSaveImagesSkipsNilImages(t *testing.T) {
	dir := t.TempDir()

	pages := []*jobs.Page{
		{Number: 1, Image: nil},
	}

	paths, err := saveImages(dir, pages)
	if err != nil {
		t.Fatalf("saveImages failed: %v", err)
	}

	if len(paths) != 0 {
		t.Fatalf("expected 0 paths for nil image, got %d", len(paths))
	}
}

func TestSaveImageAsJPEG(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jpg")

	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}

	if err := saveImageAsJPEG(path, img, 85); err != nil {
		t.Fatalf("saveImageAsJPEG failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("saved JPEG not found: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("saved JPEG is empty")
	}
}

func TestLoadImage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jpg")

	original := image.NewRGBA(image.Rect(0, 0, 50, 50))
	if err := saveImageAsJPEG(path, original, 85); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := loadImage(path)
	if err != nil {
		t.Fatalf("loadImage failed: %v", err)
	}

	bounds := loaded.Bounds()
	if bounds.Dx() != 50 || bounds.Dy() != 50 {
		t.Fatalf("expected 50x50, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestLoadImageFileNotFound(t *testing.T) {
	_, err := loadImage("/nonexistent/path.jpg")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestDeskewImagesPassThrough(t *testing.T) {
	paths := []string{"/a.png", "/b.png"}
	result, err := deskewImages(context.Background(), paths)
	if err != nil {
		t.Fatalf("deskew failed: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(result))
	}
}

func TestRemoveBlankPages(t *testing.T) {
	dir := t.TempDir()

	// Create a white (blank) image
	whiteImg := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			whiteImg.Set(x, y, color.White)
		}
	}
	whitePath := filepath.Join(dir, "white.jpg")
	saveImageAsJPEG(whitePath, whiteImg, 85)

	// Create a non-blank image
	blackImg := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			blackImg.Set(x, y, color.Black)
		}
	}
	blackPath := filepath.Join(dir, "black.jpg")
	saveImageAsJPEG(blackPath, blackImg, 85)

	paths := []string{whitePath, blackPath}
	result, err := removeBlankPages(paths, 0.99)
	if err != nil {
		t.Fatalf("removeBlankPages failed: %v", err)
	}

	// Only the black image should remain
	if len(result) != 1 {
		t.Fatalf("expected 1 non-blank page, got %d", len(result))
	}
	if result[0] != blackPath {
		t.Fatalf("expected black image to survive, got %s", result[0])
	}
}

func TestRemoveBlankPagesDefaultThreshold(t *testing.T) {
	dir := t.TempDir()

	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.Black)
		}
	}
	path := filepath.Join(dir, "black.jpg")
	saveImageAsJPEG(path, img, 85)

	// threshold=0 should use default 0.99
	result, err := removeBlankPages([]string{path}, 0)
	if err != nil {
		t.Fatalf("removeBlankPages failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatal("non-blank page should survive with default threshold")
	}
}

func TestIsBlankPageZeroSize(t *testing.T) {
	// Zero-size image should be considered blank
	img := image.NewRGBA(image.Rect(0, 0, 0, 0))
	if !isBlankPage(img, 0.99) {
		t.Fatal("zero-size image should be blank")
	}
}

func TestCreatePDFNoImages(t *testing.T) {
	err := createPDF(context.Background(), nil, "/tmp/out.pdf", config.PDFConfig{})
	if err == nil {
		t.Fatal("expected error for empty image list")
	}
}

func TestCreatePDFSingleImage(t *testing.T) {
	dir := t.TempDir()

	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.White)
		}
	}
	imgPath := filepath.Join(dir, "page.jpg")
	saveImageAsJPEG(imgPath, img, 85)

	pdfPath := filepath.Join(dir, "output.pdf")
	err := createPDF(context.Background(), []string{imgPath}, pdfPath, config.PDFConfig{JPEGQuality: 85})
	if err != nil {
		t.Fatalf("createPDF failed: %v", err)
	}

	data, err := os.ReadFile(pdfPath)
	if err != nil {
		t.Fatalf("read PDF failed: %v", err)
	}

	if !strings.HasPrefix(string(data), "%PDF-1.4") {
		t.Fatal("PDF should start with %PDF-1.4 header")
	}
	if !strings.Contains(string(data), "%%EOF") {
		t.Fatal("PDF should contain EOF marker")
	}
}

func TestCreatePDFMultipleImages(t *testing.T) {
	dir := t.TempDir()

	for i := range 3 {
		img := image.NewRGBA(image.Rect(0, 0, 50, 50))
		path := filepath.Join(dir, strings.Replace("page_000X.jpg", "X", string(rune('1'+i)), 1))
		saveImageAsJPEG(path, img, 85)
	}

	entries, _ := os.ReadDir(dir)
	paths := make([]string, 0, len(entries))
	for _, e := range entries {
		paths = append(paths, filepath.Join(dir, e.Name()))
	}

	pdfPath := filepath.Join(dir, "output.pdf")
	err := createPDF(context.Background(), paths, pdfPath, config.PDFConfig{JPEGQuality: 85})
	if err != nil {
		t.Fatalf("createPDF failed: %v", err)
	}

	data, err := os.ReadFile(pdfPath)
	if err != nil {
		t.Fatalf("read PDF failed: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "/Count 3") {
		t.Fatal("PDF should have 3 pages")
	}
}

func TestCreatePDFDefaultQuality(t *testing.T) {
	dir := t.TempDir()

	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	imgPath := filepath.Join(dir, "page.jpg")
	saveImageAsJPEG(imgPath, img, 85)

	pdfPath := filepath.Join(dir, "output.pdf")
	// quality 0 should use default (85)
	err := createPDF(context.Background(), []string{imgPath}, pdfPath, config.PDFConfig{JPEGQuality: 0})
	if err != nil {
		t.Fatalf("createPDF with default quality failed: %v", err)
	}

	if _, err := os.Stat(pdfPath); err != nil {
		t.Fatalf("PDF not created: %v", err)
	}
}

func TestGenerateFilename(t *testing.T) {
	tests := []struct {
		name     string
		job      *jobs.Job
		contains string
	}{
		{
			name:     "with title",
			job:      &jobs.Job{Metadata: &jobs.DocumentMetadata{Title: "Invoice"}},
			contains: "Invoice_",
		},
		{
			name:     "without metadata",
			job:      &jobs.Job{},
			contains: "scan_",
		},
		{
			name:     "with empty title",
			job:      &jobs.Job{Metadata: &jobs.DocumentMetadata{Title: ""}},
			contains: "scan_",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := generateFilename(tt.job)
			if !strings.Contains(filename, tt.contains) {
				t.Fatalf("expected filename containing %q, got %q", tt.contains, filename)
			}
			if !strings.HasSuffix(filename, ".pdf") {
				t.Fatalf("expected .pdf suffix, got %q", filename)
			}
		})
	}
}

func TestSanitizeFilenameSpecialChars(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Normal Name", "Normal_Name"},
		{"file.pdf", "file.pdf"},
		{"Hello World!", "Hello_World"},
		{"", "document"},
		{"test-file_123", "test-file_123"},
		{"über/scharf", "berscharf"},
		{"path/../traversal", "path..traversal"},
		{"a\tb\nc", "abc"},
		{"日本語", "document"}, // non-ASCII only
		{"mixed_日本語_test", "mixed__test"},
	}

	for _, tt := range tests {
		result := sanitizeFilename(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestRunOCRInvalidLanguage(t *testing.T) {
	err := runOCR(context.Background(), "in.pdf", "out.pdf", "eng; rm -rf /", "")
	if err == nil {
		t.Fatal("expected error for invalid language")
	}
	if !strings.Contains(err.Error(), "invalid OCR language") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunOCREmptyLanguage(t *testing.T) {
	err := runOCR(context.Background(), "in.pdf", "out.pdf", "", "")
	if err == nil {
		t.Fatal("expected error for empty language")
	}
}

func TestPipelineNewPipeline(t *testing.T) {
	cfg := config.ProcessingConfig{
		TempDirectory:     "/tmp/test",
		MaxConcurrentJobs: 2,
		PDF:               config.PDFConfig{JPEGQuality: 85},
		OCR: config.OCRConfig{
			Enabled:  true,
			Language: "eng",
		},
	}

	p := NewPipeline(cfg)
	if p == nil {
		t.Fatal("pipeline should not be nil")
	}
	if p.tempDir != "/tmp/test" {
		t.Fatalf("unexpected temp dir: %s", p.tempDir)
	}
	if !p.ocrEnabled {
		t.Fatal("OCR should be enabled")
	}
}

func TestPipelineSetOCR(t *testing.T) {
	p := NewPipeline(config.ProcessingConfig{
		OCR: config.OCRConfig{Enabled: true, Language: "eng"},
	})

	p.SetOCR(false, "deu")
	if p.ocrEnabled {
		t.Fatal("OCR should be disabled after SetOCR")
	}
	if p.ocrLanguage != "deu" {
		t.Fatalf("expected language 'deu', got %s", p.ocrLanguage)
	}

	// Empty language should not change it
	p.SetOCR(true, "")
	if p.ocrLanguage != "deu" {
		t.Fatalf("expected language to remain 'deu', got %s", p.ocrLanguage)
	}
}
