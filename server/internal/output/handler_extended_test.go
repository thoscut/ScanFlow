package output

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thoscut/scanflow/server/internal/config"
	"github.com/thoscut/scanflow/server/internal/jobs"
)

func TestFilesystemHandlerName(t *testing.T) {
	h := NewFilesystemHandler("/tmp")
	if h.Name() != "filesystem" {
		t.Fatalf("expected 'filesystem', got %s", h.Name())
	}
}

func TestFilesystemHandlerAvailable(t *testing.T) {
	h := NewFilesystemHandler("/tmp")
	if !h.Available() {
		t.Fatal("expected available with non-empty directory")
	}

	h2 := NewFilesystemHandler("")
	if h2.Available() {
		t.Fatal("expected unavailable with empty directory")
	}
}

func TestFilesystemHandlerSanitizesFilename(t *testing.T) {
	dir := t.TempDir()
	h := NewFilesystemHandler(dir)

	doc := &jobs.Document{
		Filename: ".",
		Reader:   strings.NewReader("data"),
	}

	if err := h.Send(context.Background(), doc); err != nil {
		t.Fatalf("send failed: %v", err)
	}

	// Should fall back to "document.pdf"
	if _, err := os.Stat(filepath.Join(dir, "document.pdf")); err != nil {
		t.Fatalf("expected document.pdf: %v", err)
	}
}

func TestPaperlessHandlerName(t *testing.T) {
	h := NewPaperlessHandler(config.PaperlessConfig{})
	if h.Name() != "paperless" {
		t.Fatalf("expected 'paperless', got %s", h.Name())
	}
}

func TestPaperlessHandlerAvailable(t *testing.T) {
	h := NewPaperlessHandler(config.PaperlessConfig{URL: "http://localhost", Token: "tok"})
	if !h.Available() {
		t.Fatal("expected available")
	}

	h2 := NewPaperlessHandler(config.PaperlessConfig{})
	if h2.Available() {
		t.Fatal("expected unavailable without URL/token")
	}
}

func TestPaperlessConsumeHandlerName(t *testing.T) {
	h := NewPaperlessConsumeHandler(config.PaperlessConsumeConfig{})
	if h.Name() != "paperless_consume" {
		t.Fatalf("expected 'paperless_consume', got %s", h.Name())
	}
}

func TestPaperlessConsumeHandlerAvailable(t *testing.T) {
	h := NewPaperlessConsumeHandler(config.PaperlessConsumeConfig{Path: "/nonexistent"})
	if h.Available() {
		t.Fatal("expected unavailable for nonexistent path")
	}

	dir := t.TempDir()
	h2 := NewPaperlessConsumeHandler(config.PaperlessConsumeConfig{Path: dir})
	if !h2.Available() {
		t.Fatal("expected available for existing path")
	}

	h3 := NewPaperlessConsumeHandler(config.PaperlessConsumeConfig{Path: ""})
	if h3.Available() {
		t.Fatal("expected unavailable for empty path")
	}
}

func TestPaperlessConsumeHandlerSend(t *testing.T) {
	dir := t.TempDir()
	h := NewPaperlessConsumeHandler(config.PaperlessConsumeConfig{Path: dir})

	doc := &jobs.Document{
		Title:   "Invoice",
		Created: "2026-01-15",
		Reader:  strings.NewReader("pdf-data"),
	}

	if err := h.Send(context.Background(), doc); err != nil {
		t.Fatalf("send failed: %v", err)
	}

	// Check that a file was created in the consume directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}

	name := entries[0].Name()
	if !strings.HasSuffix(name, ".pdf") {
		t.Fatalf("expected .pdf suffix, got %s", name)
	}
	if !strings.Contains(name, "Invoice") {
		t.Fatalf("expected filename to contain 'Invoice', got %s", name)
	}
}

func TestPaperlessConsumeHandlerPathTraversal(t *testing.T) {
	dir := t.TempDir()
	h := NewPaperlessConsumeHandler(config.PaperlessConsumeConfig{Path: dir})

	doc := &jobs.Document{
		Title:  "../../etc/passwd",
		Reader: strings.NewReader("data"),
	}

	if err := h.Send(context.Background(), doc); err != nil {
		t.Fatalf("send failed: %v", err)
	}

	// File should be inside dir, not escaped
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		full := filepath.Join(dir, e.Name())
		if !strings.HasPrefix(full, dir) {
			t.Fatalf("file escaped consume directory: %s", full)
		}
	}
}

func TestSMBHandlerName(t *testing.T) {
	h := NewSMBHandler(config.SMBConfig{})
	if h.Name() != "smb" {
		t.Fatalf("expected 'smb', got %s", h.Name())
	}
}

func TestSMBHandlerAvailable(t *testing.T) {
	h := NewSMBHandler(config.SMBConfig{Server: "nas.local", Share: "scans"})
	if !h.Available() {
		t.Fatal("expected available")
	}

	h2 := NewSMBHandler(config.SMBConfig{})
	if h2.Available() {
		t.Fatal("expected unavailable without server/share")
	}
}

func TestEmailHandlerName(t *testing.T) {
	h := NewEmailHandler(config.EmailConfig{})
	if h.Name() != "email" {
		t.Fatalf("expected 'email', got %s", h.Name())
	}
}

func TestEmailHandlerAvailable(t *testing.T) {
	h := NewEmailHandler(config.EmailConfig{
		SMTPHost:         "smtp.example.com",
		FromAddress:      "scan@example.com",
		DefaultRecipient: "user@example.com",
	})
	if !h.Available() {
		t.Fatal("expected available")
	}

	h2 := NewEmailHandler(config.EmailConfig{})
	if h2.Available() {
		t.Fatal("expected unavailable without required fields")
	}
}

func TestManagerListTargets(t *testing.T) {
	m := NewManager(config.OutputConfig{
		Paperless: config.PaperlessConfig{Enabled: true, URL: "http://localhost", Token: "tok"},
	})

	targets := m.ListTargets()
	if len(targets) < 2 {
		t.Fatalf("expected at least 2 targets (filesystem + paperless), got %d", len(targets))
	}

	names := make(map[string]bool)
	for _, tgt := range targets {
		names[tgt.Name] = true
	}

	if !names["filesystem"] {
		t.Fatal("expected filesystem target")
	}
	if !names["paperless"] {
		t.Fatal("expected paperless target")
	}
}

func TestManagerSendToFilesystem(t *testing.T) {
	dir := t.TempDir()
	m := &Manager{handlers: map[string]Handler{
		"filesystem": NewFilesystemHandler(dir),
	}}

	doc := &jobs.Document{
		Filename: "test.pdf",
		Reader:   strings.NewReader("pdf-content"),
	}

	if err := m.Send(context.Background(), "filesystem", doc); err != nil {
		t.Fatalf("send failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "test.pdf"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "pdf-content" {
		t.Fatalf("unexpected content: %s", string(data))
	}
}
