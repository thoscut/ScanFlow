package jobs

import (
	"context"
	"image"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"
)

// JobStatus represents the current state of a scan job.
type JobStatus string

const (
	StatusPending    JobStatus = "pending"
	StatusScanning   JobStatus = "scanning"
	StatusProcessing JobStatus = "processing"
	StatusCompleted  JobStatus = "completed"
	StatusFailed     JobStatus = "failed"
	StatusCancelled  JobStatus = "cancelled"
)

// Job represents a scan job with all its data and state.
type Job struct {
	ID         string           `json:"id"`
	Status     JobStatus        `json:"status"`
	Profile    string           `json:"profile"`
	Pages      []*Page          `json:"pages"`
	Progress   int              `json:"progress"`
	Error      string           `json:"error,omitempty"`
	Output     OutputConfig     `json:"output"`
	Metadata   *DocumentMetadata `json:"metadata,omitempty"`
	OcrEnabled *bool            `json:"ocr_enabled,omitempty"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at"`

	mu       sync.RWMutex
	cancel   context.CancelFunc
	progress chan ProgressUpdate
}

// Page represents a single scanned page.
type Page struct {
	Number    int       `json:"number"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	Format    string    `json:"format"`
	Size      int64     `json:"size"`
	Path      string    `json:"path"`
	Image     image.Image `json:"-"`
	Err       error     `json:"-"`
}

// OutputConfig defines where to send the finished document.
type OutputConfig struct {
	Target   string `json:"target"`
	Filename string `json:"filename,omitempty"`
}

// DocumentMetadata holds metadata for the output document.
type DocumentMetadata struct {
	Title              string `json:"title,omitempty"`
	Created            string `json:"created,omitempty"`
	Correspondent      int    `json:"correspondent,omitempty"`
	DocumentType       int    `json:"document_type,omitempty"`
	Tags               []int  `json:"tags,omitempty"`
	ArchiveSerialNumber string `json:"archive_serial_number,omitempty"`
}

// ScanOptions configures scanner settings for a job.
type ScanOptions struct {
	Resolution int     `json:"resolution"`
	Mode       string  `json:"mode"`
	Source     string  `json:"source"`
	PageWidth  float64 `json:"page_width"`
	PageHeight float64 `json:"page_height"`
	Brightness int     `json:"brightness"`
	Contrast   int     `json:"contrast"`
}

// ScanRequest represents an incoming scan request from the API.
type ScanRequest struct {
	Profile    string            `json:"profile,omitempty"`
	DeviceID   string            `json:"device_id,omitempty"`
	Options    *ScanOptions      `json:"options,omitempty"`
	Output     *OutputConfig     `json:"output,omitempty"`
	Metadata   *DocumentMetadata `json:"metadata,omitempty"`
	OcrEnabled *bool             `json:"ocr_enabled,omitempty"`
}

// ProgressUpdate is sent via WebSocket to report job progress.
type ProgressUpdate struct {
	Type       string `json:"type"`
	JobID      string `json:"job_id"`
	Status     string `json:"status,omitempty"`
	Page       int    `json:"page,omitempty"`
	Progress   int    `json:"progress,omitempty"`
	Message    string `json:"message,omitempty"`
	PreviewURL string `json:"preview_url,omitempty"`
	Error      string `json:"error,omitempty"`
}

// Document represents a finished document ready for output.
type Document struct {
	Filename      string
	Title         string
	Created       string
	Correspondent int
	DocumentType  int
	Tags          []int
	ArchiveSerial string
	Reader        io.Reader
	Size          int64
}

// NewJob creates a new job with default values.
func NewJob(profile string, output OutputConfig, metadata *DocumentMetadata, ocrEnabled *bool) *Job {
	now := time.Now()
	return &Job{
		ID:         uuid.New().String(),
		Status:     StatusPending,
		Profile:    profile,
		Pages:      make([]*Page, 0),
		Output:     output,
		Metadata:   metadata,
		OcrEnabled: ocrEnabled,
		CreatedAt:  now,
		UpdatedAt:  now,
		progress:   make(chan ProgressUpdate, 100),
	}
}

// SetStatus updates the job status thread-safely.
func (j *Job) SetStatus(status JobStatus) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = status
	j.UpdatedAt = time.Now()
}

// SetError marks the job as failed with an error message.
func (j *Job) SetError(err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = StatusFailed
	j.Error = err.Error()
	j.UpdatedAt = time.Now()
}

// AddPage adds a scanned page to the job.
func (j *Job) AddPage(page *Page) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Pages = append(j.Pages, page)
	j.UpdatedAt = time.Now()
}

// DeletePage removes a page by number (1-indexed).
func (j *Job) DeletePage(pageNum int) bool {
	j.mu.Lock()
	defer j.mu.Unlock()

	for i, p := range j.Pages {
		if p.Number == pageNum {
			j.Pages = append(j.Pages[:i], j.Pages[i+1:]...)
			// Renumber remaining pages
			for k := i; k < len(j.Pages); k++ {
				j.Pages[k].Number = k + 1
			}
			j.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// PageCount returns the number of scanned pages.
func (j *Job) PageCount() int {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return len(j.Pages)
}

// SetCancel stores the cancel function for the job context.
func (j *Job) SetCancel(cancel context.CancelFunc) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.cancel = cancel
}

// Cancel cancels the job.
func (j *Job) Cancel() {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.cancel != nil {
		j.cancel()
	}
	j.Status = StatusCancelled
	j.UpdatedAt = time.Now()
}

// SendProgress sends a progress update for this job.
func (j *Job) SendProgress(update ProgressUpdate) {
	update.JobID = j.ID
	select {
	case j.progress <- update:
	default:
		// Channel full, drop update
	}
}

// Progress returns the progress channel for this job.
func (j *Job) ProgressChan() <-chan ProgressUpdate {
	return j.progress
}
