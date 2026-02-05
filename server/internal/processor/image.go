package processor

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"

	"github.com/thoscut/scanflow/server/internal/jobs"
)

// saveImages writes scanned page images to disk as PNG files.
func saveImages(dir string, pages []*jobs.Page) ([]string, error) {
	paths := make([]string, 0, len(pages))

	for _, page := range pages {
		if page.Image == nil {
			continue
		}

		filename := fmt.Sprintf("page_%04d.png", page.Number)
		path := filepath.Join(dir, filename)

		f, err := os.Create(path)
		if err != nil {
			return nil, fmt.Errorf("create image file: %w", err)
		}

		if err := png.Encode(f, page.Image); err != nil {
			f.Close()
			return nil, fmt.Errorf("encode image: %w", err)
		}
		f.Close()

		page.Path = path
		paths = append(paths, path)
	}

	return paths, nil
}

// saveImageAsJPEG writes an image to disk as a JPEG file.
func saveImageAsJPEG(path string, img image.Image, quality int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return jpeg.Encode(f, img, &jpeg.Options{Quality: quality})
}

// loadImage reads an image from disk.
func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	return img, err
}

// deskewImages attempts to straighten tilted scans.
// This is a simplified implementation; a production version would use
// more sophisticated algorithms (Hough transform, projection profiles).
func deskewImages(_ context.Context, paths []string) ([]string, error) {
	// In a production implementation, this would:
	// 1. Detect skew angle using projection profile or Hough transform
	// 2. Rotate the image to correct the skew
	// For now, we pass through the images unchanged.
	return paths, nil
}

// removeBlankPages filters out pages that are mostly white (blank).
func removeBlankPages(paths []string, threshold float64) ([]string, error) {
	if threshold <= 0 {
		threshold = 0.99
	}

	result := make([]string, 0, len(paths))

	for _, path := range paths {
		img, err := loadImage(path)
		if err != nil {
			return nil, fmt.Errorf("load image %s: %w", path, err)
		}

		if !isBlankPage(img, threshold) {
			result = append(result, path)
		} else {
			os.Remove(path)
		}
	}

	return result, nil
}

// isBlankPage checks if an image is mostly white (blank).
func isBlankPage(img image.Image, threshold float64) bool {
	bounds := img.Bounds()
	totalPixels := bounds.Dx() * bounds.Dy()
	if totalPixels == 0 {
		return true
	}

	whitePixels := 0
	// Sample every 4th pixel for performance
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 4 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 4 {
			r, g, b, _ := img.At(x, y).RGBA()
			// Check if pixel is "white-ish" (high luminance)
			gray := color.GrayModel.Convert(img.At(x, y)).(color.Gray)
			_ = r
			_ = g
			_ = b
			if gray.Y > 240 {
				whitePixels++
			}
		}
	}

	sampledPixels := (bounds.Dx() / 4) * (bounds.Dy() / 4)
	if sampledPixels == 0 {
		return true
	}

	ratio := float64(whitePixels) / float64(sampledPixels)
	return ratio >= threshold
}
