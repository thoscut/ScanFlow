package output

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thoscut/scanflow/server/internal/config"
	"github.com/thoscut/scanflow/server/internal/jobs"
)

// PaperlessConsumeHandler places documents in the Paperless-NGX consume folder.
// Paperless watches this folder and automatically imports new files.
type PaperlessConsumeHandler struct {
	consumePath string
}

// NewPaperlessConsumeHandler creates a new consume folder output handler.
func NewPaperlessConsumeHandler(cfg config.PaperlessConsumeConfig) *PaperlessConsumeHandler {
	return &PaperlessConsumeHandler{
		consumePath: cfg.Path,
	}
}

func (h *PaperlessConsumeHandler) Name() string { return "paperless_consume" }

func (h *PaperlessConsumeHandler) Available() bool {
	if h.consumePath == "" {
		return false
	}
	_, err := os.Stat(h.consumePath)
	return err == nil
}

// Send places a document in the Paperless consume folder.
func (h *PaperlessConsumeHandler) Send(_ context.Context, doc *jobs.Document) error {
	if err := os.MkdirAll(h.consumePath, 0o755); err != nil {
		return fmt.Errorf("create consume directory: %w", err)
	}

	// Build filename using Paperless naming convention:
	// [correspondent] - [date] - [title] - [tags].pdf
	filename := h.buildFilename(doc)
	targetPath := filepath.Join(h.consumePath, filename)

	f, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("create file in consume folder: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, doc.Reader); err != nil {
		return fmt.Errorf("write to consume folder: %w", err)
	}

	return nil
}

func (h *PaperlessConsumeHandler) buildFilename(doc *jobs.Document) string {
	parts := make([]string, 0)

	if doc.Created != "" {
		parts = append(parts, doc.Created)
	}
	if doc.Title != "" {
		parts = append(parts, doc.Title)
	}

	name := strings.Join(parts, " - ")
	if name == "" {
		name = fmt.Sprintf("scan_%s", time.Now().Format("20060102_150405"))
	}

	// Sanitize
	result := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == ' ' || c == '.' {
			result = append(result, c)
		}
	}

	return string(result) + ".pdf"
}
