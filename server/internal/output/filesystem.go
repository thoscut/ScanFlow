package output

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/thoscut/scanflow/server/internal/jobs"
)

// FilesystemHandler saves documents to the local filesystem.
type FilesystemHandler struct {
	directory string
}

// NewFilesystemHandler creates a new filesystem output handler.
func NewFilesystemHandler(dir string) *FilesystemHandler {
	return &FilesystemHandler{directory: dir}
}

func (h *FilesystemHandler) Name() string { return "filesystem" }

func (h *FilesystemHandler) Available() bool {
	return h.directory != ""
}

// Send saves a document to the local filesystem.
func (h *FilesystemHandler) Send(_ context.Context, doc *jobs.Document) error {
	if err := os.MkdirAll(h.directory, 0o755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	path := filepath.Join(h.directory, doc.Filename)

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, doc.Reader); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}
