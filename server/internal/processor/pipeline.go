package processor

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/thoscut/scanflow/server/internal/config"
	"github.com/thoscut/scanflow/server/internal/jobs"
)

// Pipeline orchestrates image processing, PDF creation, and OCR.
type Pipeline struct {
	tempDir     string
	ocrEnabled  bool
	ocrLanguage string
	ocrPath     string
	pdfConfig   config.PDFConfig
}

// NewPipeline creates a new processing pipeline.
func NewPipeline(cfg config.ProcessingConfig) *Pipeline {
	return &Pipeline{
		tempDir:     cfg.TempDirectory,
		ocrEnabled:  cfg.OCR.Enabled,
		ocrLanguage: cfg.OCR.Language,
		ocrPath:     cfg.OCR.TesseractPath,
		pdfConfig:   cfg.PDF,
	}
}

// Process takes a completed scan job and produces a Document ready for output.
func (p *Pipeline) Process(ctx context.Context, job *jobs.Job, profile *config.Profile) (*jobs.Document, error) {
	slog.Info("processing job", "job_id", job.ID, "pages", job.PageCount())

	// Create temp directory for this job
	jobDir := filepath.Join(p.tempDir, job.ID)
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(jobDir)

	// Step 1: Save images to disk
	job.SendProgress(jobs.ProgressUpdate{
		Type:     "processing",
		Progress: 10,
		Message:  "Saving scanned images...",
	})

	imagePaths, err := saveImages(jobDir, job.Pages)
	if err != nil {
		return nil, fmt.Errorf("save images: %w", err)
	}

	// Step 2: Image optimization
	if profile.Processing.OptimizeImages {
		job.SendProgress(jobs.ProgressUpdate{
			Type:     "processing",
			Progress: 20,
			Message:  "Optimizing images...",
		})

		if profile.Processing.Deskew {
			imagePaths, err = deskewImages(ctx, imagePaths)
			if err != nil {
				slog.Warn("deskew failed", "error", err)
			}
		}

		if profile.Processing.RemoveBlankPages {
			imagePaths, err = removeBlankPages(imagePaths, profile.Processing.BlankThreshold)
			if err != nil {
				slog.Warn("blank page removal failed", "error", err)
			}
		}
	}

	if len(imagePaths) == 0 {
		return nil, fmt.Errorf("no pages remaining after processing")
	}

	// Step 3: Create PDF
	job.SendProgress(jobs.ProgressUpdate{
		Type:     "processing",
		Progress: 50,
		Message:  "Creating PDF...",
	})

	pdfPath := filepath.Join(jobDir, "output.pdf")
	if err := createPDF(ctx, imagePaths, pdfPath, p.pdfConfig); err != nil {
		return nil, fmt.Errorf("create PDF: %w", err)
	}

	// Step 4: OCR
	ocrEnabled := p.ocrEnabled
	if profile.Processing.OCR.Enabled {
		ocrEnabled = true
	}

	if ocrEnabled {
		job.SendProgress(jobs.ProgressUpdate{
			Type:     "processing",
			Progress: 70,
			Message:  "Running OCR...",
		})

		lang := p.ocrLanguage
		if profile.Processing.OCR.Language != "" {
			lang = profile.Processing.OCR.Language
		}

		ocrPDFPath := filepath.Join(jobDir, "output_ocr.pdf")
		if err := runOCR(ctx, pdfPath, ocrPDFPath, lang, p.ocrPath); err != nil {
			slog.Warn("OCR failed, using PDF without OCR", "error", err)
		} else {
			pdfPath = ocrPDFPath
		}
	}

	// Step 5: Build document
	job.SendProgress(jobs.ProgressUpdate{
		Type:     "processing",
		Progress: 90,
		Message:  "Finalizing document...",
	})

	pdfFile, err := os.Open(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("open PDF: %w", err)
	}

	stat, err := pdfFile.Stat()
	if err != nil {
		pdfFile.Close()
		return nil, fmt.Errorf("stat PDF: %w", err)
	}

	doc := &jobs.Document{
		Reader: pdfFile,
		Size:   stat.Size(),
	}

	// Set filename
	doc.Filename = generateFilename(job)

	// Set metadata from job
	if job.Metadata != nil {
		doc.Title = job.Metadata.Title
		doc.Created = job.Metadata.Created
		doc.Correspondent = job.Metadata.Correspondent
		doc.DocumentType = job.Metadata.DocumentType
		doc.Tags = job.Metadata.Tags
		doc.ArchiveSerial = job.Metadata.ArchiveSerialNumber
	}

	job.SendProgress(jobs.ProgressUpdate{
		Type:     "processing",
		Progress: 100,
		Message:  "Document ready",
	})

	slog.Info("document processed",
		"job_id", job.ID,
		"pages", len(imagePaths),
		"size", stat.Size(),
		"filename", doc.Filename)

	return doc, nil
}

func generateFilename(job *jobs.Job) string {
	timestamp := time.Now().Format("20060102_150405")
	title := "scan"
	if job.Metadata != nil && job.Metadata.Title != "" {
		title = sanitizeFilename(job.Metadata.Title)
	}
	return fmt.Sprintf("%s_%s.pdf", title, timestamp)
}

func sanitizeFilename(name string) string {
	result := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' {
			result = append(result, c)
		} else if c == ' ' {
			result = append(result, '_')
		}
	}
	if len(result) == 0 {
		return "document"
	}
	return string(result)
}
