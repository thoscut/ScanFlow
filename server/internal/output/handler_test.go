package output

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
