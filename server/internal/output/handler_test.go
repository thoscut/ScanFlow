package output

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/thoscut/scanflow/server/internal/config"
	"github.com/thoscut/scanflow/server/internal/jobs"
)

// failNTimesHandler is a mock output handler that fails a configurable number
// of times before succeeding.
type failNTimesHandler struct {
	name     string
	failures int
	calls    int
}

func (h *failNTimesHandler) Name() string      { return h.name }
func (h *failNTimesHandler) Available() bool    { return true }
func (h *failNTimesHandler) Send(_ context.Context, _ *jobs.Document) error {
	h.calls++
	if h.calls <= h.failures {
		return fmt.Errorf("temporary failure")
	}
	return nil
}

func TestFilesystemHandlerSend(t *testing.T) {
	dir := t.TempDir()
	handler := NewFilesystemHandler(dir)

	doc := &jobs.Document{
		Filename: "scan.pdf",
		Reader:   strings.NewReader("pdf-data"),
	}

	if err := handler.Send(context.Background(), doc); err != nil {
		t.Fatalf("send failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "scan.pdf"))
	if err != nil {
		t.Fatalf("read written file failed: %v", err)
	}
	if string(data) != "pdf-data" {
		t.Fatalf("unexpected file content: %s", string(data))
	}
}

func TestPaperlessHandlerSendIncludesMetadata(t *testing.T) {
	var (
		authHeader  string
		contentType string
		fields      = map[string][]string{}
		document    []byte
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		contentType = r.Header.Get("Content-Type")

		mediaType, params, err := mime.ParseMediaType(contentType)
		if err != nil {
			t.Fatalf("parse media type failed: %v", err)
		}
		if mediaType != "multipart/form-data" {
			t.Fatalf("unexpected media type: %s", mediaType)
		}

		reader := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("read multipart failed: %v", err)
			}

			data, err := io.ReadAll(part)
			if err != nil {
				t.Fatalf("read part failed: %v", err)
			}

			if part.FormName() == "document" {
				document = data
				continue
			}
			fields[part.FormName()] = append(fields[part.FormName()], string(data))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"task_id":"task-123"}`))
	}))
	defer server.Close()

	handler := NewPaperlessHandler(config.PaperlessConfig{
		URL:   server.URL,
		Token: "secret-token",
	})
	handler.client = server.Client()

	doc := &jobs.Document{
		Filename:      "scan.pdf",
		Title:         "Invoice",
		Created:       "2026-03-10",
		Correspondent: 7,
		DocumentType:  3,
		Tags:          []int{1, 2},
		ArchiveSerial: "ARC-42",
		Reader:        bytes.NewBufferString("document-bytes"),
	}

	if err := handler.Send(context.Background(), doc); err != nil {
		t.Fatalf("send failed: %v", err)
	}

	if authHeader != "Token secret-token" {
		t.Fatalf("unexpected auth header: %s", authHeader)
	}
	if string(document) != "document-bytes" {
		t.Fatalf("unexpected uploaded document: %s", string(document))
	}
	if got := fields["title"]; len(got) != 1 || got[0] != "Invoice" {
		t.Fatalf("unexpected title field: %#v", got)
	}
	if got := fields["tags"]; len(got) != 2 || got[0] != "1" || got[1] != "2" {
		t.Fatalf("unexpected tags field: %#v", got)
	}
	if got := fields["archive_serial_number"]; len(got) != 1 || got[0] != "ARC-42" {
		t.Fatalf("unexpected archive serial field: %#v", got)
	}
}

func TestManagerSendUnknownTarget(t *testing.T) {
	manager := NewManager(config.DefaultConfig().Output)

	err := manager.Send(context.Background(), "missing", &jobs.Document{})
	if err == nil {
		t.Fatal("expected error for unknown target")
	}
	if !strings.Contains(err.Error(), "unknown output target") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFilesystemHandlerPathTraversal(t *testing.T) {
	dir := t.TempDir()
	handler := NewFilesystemHandler(dir)

	doc := &jobs.Document{
		Filename: "../../etc/passwd",
		Reader:   strings.NewReader("should-not-escape"),
	}

	if err := handler.Send(context.Background(), doc); err != nil {
		t.Fatalf("send failed: %v", err)
	}

	// The file must end up inside dir, not in ../../etc
	if _, err := os.Stat(filepath.Join(dir, "passwd")); err != nil {
		t.Fatalf("expected file inside dir: %v", err)
	}
}

func TestSanitizeMIMEValue(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{"normal.pdf", "normal.pdf"},
		{"file\r\nInjected-Header: evil", "fileInjected-Header: evil"},
		{"file\x00name.pdf", "filename.pdf"},
		{`file"quote.pdf`, "filequote.pdf"},
	}
	for _, tc := range cases {
		got := sanitizeMIMEValue(tc.input)
		if got != tc.want {
			t.Errorf("sanitizeMIMEValue(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestManagerSendRetries(t *testing.T) {
	mock := &failNTimesHandler{name: "test", failures: 2}
	m := &Manager{handlers: map[string]Handler{"test": mock}}

	err := m.Send(context.Background(), "test", &jobs.Document{Filename: "doc.pdf"})
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if mock.calls != 3 {
		t.Fatalf("expected 3 calls, got %d", mock.calls)
	}
}

func TestManagerSendRetriesExhausted(t *testing.T) {
	mock := &failNTimesHandler{name: "test", failures: 10}
	m := &Manager{handlers: map[string]Handler{"test": mock}}

	err := m.Send(context.Background(), "test", &jobs.Document{Filename: "doc.pdf"})
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if !strings.Contains(err.Error(), "all retries exhausted") {
		t.Fatalf("unexpected error message: %v", err)
	}
	// 1 initial + 3 retries = 4 calls
	if mock.calls != 4 {
		t.Fatalf("expected 4 calls, got %d", mock.calls)
	}
}

// --- Integration tests with mock HTTP services ---

func TestPaperlessHandlerSendSuccess(t *testing.T) {
	// Create a mock Paperless-NGX server that accepts uploads
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/documents/post_document/" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Token test-token" {
			t.Errorf("missing or wrong auth header")
		}
		// Verify multipart form
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		_, _, err := r.FormFile("document")
		if err != nil {
			t.Fatalf("missing document field: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"task_id": "abc-123"})
	}))
	defer srv.Close()

	handler := NewPaperlessHandler(config.PaperlessConfig{
		URL:   srv.URL,
		Token: "test-token",
	})

	doc := &jobs.Document{
		Filename: "test.pdf",
		Title:    "Test Document",
		Reader:   strings.NewReader("fake pdf content"),
		Size:     16,
	}
	err := handler.Send(context.Background(), doc)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestPaperlessHandlerSendServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer srv.Close()

	handler := NewPaperlessHandler(config.PaperlessConfig{
		URL:   srv.URL,
		Token: "test-token",
	})

	doc := &jobs.Document{
		Filename: "test.pdf",
		Reader:   strings.NewReader("fake pdf"),
		Size:     8,
	}
	err := handler.Send(context.Background(), doc)
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestPaperlessHandlerGetTaskStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		taskID := r.URL.Query().Get("task_id")
		if taskID == "" {
			t.Error("missing task_id parameter")
		}
		json.NewEncoder(w).Encode([]map[string]string{{"status": "SUCCESS"}})
	}))
	defer srv.Close()

	handler := NewPaperlessHandler(config.PaperlessConfig{
		URL:   srv.URL,
		Token: "test-token",
	})

	status, err := handler.GetTaskStatus(context.Background(), "task-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "SUCCESS" {
		t.Fatalf("expected SUCCESS, got %s", status)
	}
}

func TestEmailHandlerSanitizesHeaders(t *testing.T) {
	// Test that MIME header injection is prevented
	tests := []struct {
		input    string
		expected string
	}{
		{"normal.pdf", "normal.pdf"},
		{"file\r\nBcc: evil@attacker.com", "fileBcc: evil@attacker.com"},
		{"file\x00.pdf", "file.pdf"},
		{`file"name.pdf`, "filename.pdf"},
	}
	for _, tt := range tests {
		result := sanitizeMIMEValue(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeMIMEValue(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestFilesystemHandlerConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	handler := NewFilesystemHandler(dir)

	// Write multiple documents concurrently
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			doc := &jobs.Document{
				Filename: fmt.Sprintf("doc_%d.pdf", n),
				Reader:   strings.NewReader(fmt.Sprintf("content %d", n)),
				Size:     9,
			}
			if err := handler.Send(context.Background(), doc); err != nil {
				t.Errorf("concurrent write %d failed: %v", n, err)
			}
		}(i)
	}
	wg.Wait()

	// Verify all files exist
	entries, _ := os.ReadDir(dir)
	if len(entries) != 5 {
		t.Fatalf("expected 5 files, got %d", len(entries))
	}
}
