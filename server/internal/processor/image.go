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

	"github.com/thoscut/scanflow/server/internal/config"
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

// convertToGrayscale converts a color image to grayscale.
func convertToGrayscale(img image.Image) *image.Gray {
	bounds := img.Bounds()
	gray := image.NewGray(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			gray.Set(x, y, img.At(x, y))
		}
	}
	return gray
}

// adjustBrightness shifts every pixel by delta (-1.0 to +1.0 mapped to 0-255).
// A value of 0 means no change, positive brightens, negative darkens.
func adjustBrightness(img image.Image, delta float64) *image.RGBA {
	bounds := img.Bounds()
	dst := image.NewRGBA(bounds)
	shift := delta * 255
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			dst.Set(x, y, color.RGBA{
				R: clampU8(float64(r>>8) + shift),
				G: clampU8(float64(g>>8) + shift),
				B: clampU8(float64(b>>8) + shift),
				A: uint8(a >> 8),
			})
		}
	}
	return dst
}

// adjustContrast scales pixel values around the midpoint (128).
// factor > 0 increases contrast, < 0 decreases it.
func adjustContrast(img image.Image, factor float64) *image.RGBA {
	bounds := img.Bounds()
	dst := image.NewRGBA(bounds)
	scale := 1.0 + factor
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			dst.Set(x, y, color.RGBA{
				R: clampU8(128 + scale*(float64(r>>8)-128)),
				G: clampU8(128 + scale*(float64(g>>8)-128)),
				B: clampU8(128 + scale*(float64(b>>8)-128)),
				A: uint8(a >> 8),
			})
		}
	}
	return dst
}

// normalizeExposure stretches the histogram so that the darkest pixel maps to
// 0 and the brightest maps to 255. This is a lightweight auto-levels.
func normalizeExposure(img image.Image) *image.RGBA {
	bounds := img.Bounds()
	minY := uint8(255)
	maxY := uint8(0)
	// Find luminance range by sampling every 4th pixel.
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 4 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 4 {
			g := color.GrayModel.Convert(img.At(x, y)).(color.Gray)
			if g.Y < minY {
				minY = g.Y
			}
			if g.Y > maxY {
				maxY = g.Y
			}
		}
	}
	span := float64(maxY) - float64(minY)
	if span < 1 {
		span = 1
	}
	dst := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			dst.Set(x, y, color.RGBA{
				R: clampU8((float64(r>>8) - float64(minY)) * 255 / span),
				G: clampU8((float64(g>>8) - float64(minY)) * 255 / span),
				B: clampU8((float64(b>>8) - float64(minY)) * 255 / span),
				A: uint8(a >> 8),
			})
		}
	}
	return dst
}

// clampU8 clamps a float to the 0–255 range and returns a uint8.
func clampU8(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// applyImageFilters runs the configured post-processing filters on a set of
// saved images, overwriting each file in place and returning the (possibly
// unchanged) list of paths.
func applyImageFilters(paths []string, cfg config.ImageFilterConfig, prof config.ProfileProcessing) ([]string, error) {
	// Merge global config with profile overrides – profile wins when set.
	grayscale := cfg.ColorToGrayscale || prof.ColorToGrayscale
	brightness := cfg.BrightnessAdjust
	if prof.BrightnessAdjust != 0 {
		brightness = prof.BrightnessAdjust
	}
	contrast := cfg.ContrastAdjust
	if prof.ContrastAdjust != 0 {
		contrast = prof.ContrastAdjust
	}
	normalize := cfg.NormalizeExposure || prof.NormalizeExposure

	// Quick exit when nothing to do.
	if !grayscale && brightness == 0 && contrast == 0 && !normalize {
		return paths, nil
	}

	for _, p := range paths {
		img, err := loadImage(p)
		if err != nil {
			return nil, fmt.Errorf("load %s for filters: %w", p, err)
		}

		var result image.Image = img

		if grayscale {
			result = convertToGrayscale(result)
		}
		if brightness != 0 {
			result = adjustBrightness(result, brightness)
		}
		if contrast != 0 {
			result = adjustContrast(result, contrast)
		}
		if normalize {
			result = normalizeExposure(result)
		}

		// Overwrite original file as PNG.
		f, err := os.Create(p)
		if err != nil {
			return nil, fmt.Errorf("create %s: %w", p, err)
		}
		if err := png.Encode(f, result); err != nil {
			f.Close()
			return nil, fmt.Errorf("encode %s: %w", p, err)
		}
		f.Close()
	}
	return paths, nil
}
