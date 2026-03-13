package jobs

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// jobRecord is the JSON-serializable representation of a Job.
// It captures metadata only — no images, readers, or channels.
type jobRecord struct {
	ID          string            `json:"id"`
	Status      JobStatus         `json:"status"`
	Profile     string            `json:"profile"`
	Progress    int               `json:"progress"`
	Error       string            `json:"error,omitempty"`
	Output      OutputConfig      `json:"output"`
	Metadata    *DocumentMetadata `json:"metadata,omitempty"`
	OcrEnabled  *bool             `json:"ocr_enabled,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	CompletedAt time.Time         `json:"completed_at,omitempty"`
	PageCount   int               `json:"page_count"`
}

func toRecord(job *Job) jobRecord {
	job.mu.RLock()
	defer job.mu.RUnlock()
	return jobRecord{
		ID:          job.ID,
		Status:      job.Status,
		Profile:     job.Profile,
		Progress:    job.Progress,
		Error:       job.Error,
		Output:      job.Output,
		Metadata:    job.Metadata,
		OcrEnabled:  job.OcrEnabled,
		CreatedAt:   job.CreatedAt,
		UpdatedAt:   job.UpdatedAt,
		CompletedAt: job.CompletedAt,
		PageCount:   len(job.Pages),
	}
}

func fromRecord(rec jobRecord) *Job {
	return &Job{
		ID:          rec.ID,
		Status:      rec.Status,
		Profile:     rec.Profile,
		Progress:    rec.Progress,
		Error:       rec.Error,
		Output:      rec.Output,
		Metadata:    rec.Metadata,
		OcrEnabled:  rec.OcrEnabled,
		CreatedAt:   rec.CreatedAt,
		UpdatedAt:   rec.UpdatedAt,
		CompletedAt: rec.CompletedAt,
		Pages:       make([]*Page, 0),
		progress:    make(chan ProgressUpdate, 100),
	}
}

// Store persists job metadata to JSON files on disk.
type Store struct {
	dir string
	mu  sync.Mutex
}

// NewStore creates a Store that writes job files to dir.
// The directory is created if it does not exist.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create store directory: %w", err)
	}
	return &Store{dir: dir}, nil
}

func (s *Store) path(id string) string {
	// Use only the base name to prevent path traversal even if the caller
	// passes a crafted ID like "../../etc/passwd".
	safe := filepath.Base(filepath.Clean(id))
	return filepath.Join(s.dir, safe+".json")
}

// Save writes the job's current state to a JSON file.
func (s *Store) Save(job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec := toRecord(job)
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal job %s: %w", job.ID, err)
	}

	tmp := s.path(job.ID) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write job %s: %w", job.ID, err)
	}
	if err := os.Rename(tmp, s.path(job.ID)); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename job %s: %w", job.ID, err)
	}
	return nil
}

// Load reads a single job from disk by ID.
func (s *Store) Load(id string) (*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path(id))
	if err != nil {
		return nil, fmt.Errorf("read job %s: %w", id, err)
	}

	var rec jobRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("unmarshal job %s: %w", id, err)
	}
	return fromRecord(rec), nil
}

// LoadAll reads every persisted job from the store directory.
func (s *Store) LoadAll() ([]*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("read store directory: %w", err)
	}

	var jobs []*Job
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}

		path := filepath.Join(s.dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("skipping unreadable job file", "path", path, "error", err)
			continue
		}
		var rec jobRecord
		if err := json.Unmarshal(data, &rec); err != nil {
			slog.Warn("skipping corrupted job file", "path", path, "error", err)
			continue
		}
		jobs = append(jobs, fromRecord(rec))
	}
	return jobs, nil
}

// Remove deletes the persisted file for a job.
func (s *Store) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := os.Remove(s.path(id))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove job %s: %w", id, err)
	}
	return nil
}
