package output

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"

	"github.com/thoscut/scanflow/server/internal/config"
	"github.com/thoscut/scanflow/server/internal/jobs"
)

// PaperlessHandler uploads documents to Paperless-NGX via its REST API.
type PaperlessHandler struct {
	baseURL string
	token   string
	client  *http.Client
}

// UploadResult contains the response from a Paperless upload.
type UploadResult struct {
	TaskID string `json:"task_id"`
}

// NewPaperlessHandler creates a new Paperless-NGX output handler.
func NewPaperlessHandler(cfg config.PaperlessConfig) *PaperlessHandler {
	return &PaperlessHandler{
		baseURL: cfg.URL,
		token:   cfg.Token,
		client:  &http.Client{},
	}
}

func (h *PaperlessHandler) Name() string { return "paperless" }

func (h *PaperlessHandler) Available() bool {
	return h.baseURL != "" && h.token != ""
}

// Send uploads a document to Paperless-NGX.
func (h *PaperlessHandler) Send(ctx context.Context, doc *jobs.Document) error {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Document file
	part, err := writer.CreateFormFile("document", doc.Filename)
	if err != nil {
		return fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, doc.Reader); err != nil {
		return fmt.Errorf("copy document data: %w", err)
	}

	// Metadata fields
	if doc.Title != "" {
		writer.WriteField("title", doc.Title)
	}
	if doc.Correspondent > 0 {
		writer.WriteField("correspondent", strconv.Itoa(doc.Correspondent))
	}
	if doc.DocumentType > 0 {
		writer.WriteField("document_type", strconv.Itoa(doc.DocumentType))
	}
	for _, tag := range doc.Tags {
		writer.WriteField("tags", strconv.Itoa(tag))
	}
	if doc.Created != "" {
		writer.WriteField("created", doc.Created)
	}
	if doc.ArchiveSerial != "" {
		writer.WriteField("archive_serial_number", doc.ArchiveSerial)
	}

	writer.Close()

	req, err := http.NewRequestWithContext(ctx, "POST",
		h.baseURL+"/api/documents/post_document/", body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+h.token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("paperless upload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("paperless error %d: %s", resp.StatusCode, string(respBody))
	}

	var result UploadResult
	json.NewDecoder(resp.Body).Decode(&result)

	return nil
}

// GetTaskStatus queries the status of a Paperless background task.
func (h *PaperlessHandler) GetTaskStatus(ctx context.Context, taskID string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		h.baseURL+"/api/tasks/?task_id="+taskID, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Token "+h.token)

	resp, err := h.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var tasks []struct {
		Status string `json:"status"`
	}
	json.NewDecoder(resp.Body).Decode(&tasks)

	if len(tasks) > 0 {
		return tasks[0].Status, nil
	}
	return "unknown", nil
}
