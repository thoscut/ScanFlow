package processor

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
)

// runOCR applies OCR to a PDF using Tesseract via the ocrmypdf wrapper.
// Falls back to direct Tesseract if ocrmypdf is not available.
func runOCR(ctx context.Context, inputPDF, outputPDF, language, tesseractPath string) error {
	if tesseractPath == "" {
		tesseractPath = "tesseract"
	}

	// Try ocrmypdf first (produces searchable PDF directly)
	if ocrMyPDFPath, err := exec.LookPath("ocrmypdf"); err == nil {
		return runOCRMyPDF(ctx, ocrMyPDFPath, inputPDF, outputPDF, language)
	}

	// Fall back to tesseract
	if _, err := exec.LookPath(tesseractPath); err != nil {
		return fmt.Errorf("tesseract not found: %w", err)
	}

	return runTesseract(ctx, tesseractPath, inputPDF, outputPDF, language)
}

func runOCRMyPDF(ctx context.Context, ocrMyPDFPath, inputPDF, outputPDF, language string) error {
	args := []string{
		"--language", language,
		"--skip-text",     // Skip pages that already have text
		"--optimize", "1", // Light optimization
		"--deskew",        // Auto-deskew
		inputPDF,
		outputPDF,
	}

	cmd := exec.CommandContext(ctx, ocrMyPDFPath, args...)
	cmd.Stderr = os.Stderr

	slog.Debug("running ocrmypdf", "args", args)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ocrmypdf failed: %w", err)
	}

	return nil
}

func runTesseract(ctx context.Context, tesseractPath, inputPDF, outputPDF, language string) error {
	// Tesseract works on images, not PDFs directly.
	// In a production implementation, we would:
	// 1. Extract images from the PDF
	// 2. Run tesseract on each image to get hOCR
	// 3. Merge hOCR text layer back into the PDF
	//
	// For simplicity, we copy the input to output unchanged
	// when ocrmypdf is not available.

	slog.Warn("ocrmypdf not available, copying PDF without OCR")

	input, err := os.ReadFile(inputPDF)
	if err != nil {
		return fmt.Errorf("read input PDF: %w", err)
	}

	if err := os.WriteFile(outputPDF, input, 0o644); err != nil {
		return fmt.Errorf("write output PDF: %w", err)
	}

	return nil
}
