package jobs

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreCreateDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "store")
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if store == nil {
		t.Fatal("store is nil")
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("path is not a directory")
	}
}

func TestStoreSaveAndLoad(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	ocrEnabled := true
	job := NewJob("standard", OutputConfig{Target: "paperless", Filename: "test.pdf"}, &DocumentMetadata{
		Title: "Test Document",
		Tags:  []int{1, 2, 3},
	}, &ocrEnabled)
	job.SetStatus(StatusScanning)
	job.AddPage(&Page{Number: 1, Width: 100, Height: 200, Format: "png", Size: 1024})
	job.AddPage(&Page{Number: 2, Width: 100, Height: 200, Format: "png", Size: 2048})

	if err := store.Save(job); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filepath.Join(store.dir, job.ID+".json")); err != nil {
		t.Fatalf("json file not found: %v", err)
	}

	loaded, err := store.Load(job.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.ID != job.ID {
		t.Errorf("ID: got %q, want %q", loaded.ID, job.ID)
	}
	if loaded.Status != StatusScanning {
		t.Errorf("Status: got %q, want %q", loaded.Status, StatusScanning)
	}
	if loaded.Profile != "standard" {
		t.Errorf("Profile: got %q, want %q", loaded.Profile, "standard")
	}
	if loaded.Output.Target != "paperless" {
		t.Errorf("Output.Target: got %q, want %q", loaded.Output.Target, "paperless")
	}
	if loaded.Metadata == nil {
		t.Fatal("Metadata is nil")
	}
	if loaded.Metadata.Title != "Test Document" {
		t.Errorf("Metadata.Title: got %q, want %q", loaded.Metadata.Title, "Test Document")
	}
	if len(loaded.Metadata.Tags) != 3 {
		t.Errorf("Metadata.Tags: got %d tags, want 3", len(loaded.Metadata.Tags))
	}
	if loaded.OcrEnabled == nil || !*loaded.OcrEnabled {
		t.Error("OcrEnabled should be true")
	}
	if loaded.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
	// Pages are not restored (only metadata), but progress channel should exist
	if loaded.progress == nil {
		t.Error("progress channel should be initialized")
	}
}

func TestStoreSaveOverwrite(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	job := NewJob("standard", OutputConfig{Target: "fs"}, nil, nil)
	if err := store.Save(job); err != nil {
		t.Fatalf("Save: %v", err)
	}

	job.SetStatus(StatusCompleted)
	if err := store.Save(job); err != nil {
		t.Fatalf("Save (overwrite): %v", err)
	}

	loaded, err := store.Load(job.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Status != StatusCompleted {
		t.Errorf("Status: got %q, want %q", loaded.Status, StatusCompleted)
	}
}

func TestStoreLoadAll(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// Save 3 jobs
	ids := make(map[string]bool)
	for i := 0; i < 3; i++ {
		job := NewJob("standard", OutputConfig{Target: "paperless"}, nil, nil)
		ids[job.ID] = true
		if err := store.Save(job); err != nil {
			t.Fatalf("Save job %d: %v", i, err)
		}
	}

	loaded, err := store.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(loaded) != 3 {
		t.Fatalf("LoadAll: got %d jobs, want 3", len(loaded))
	}
	for _, job := range loaded {
		if !ids[job.ID] {
			t.Errorf("unexpected job ID: %s", job.ID)
		}
	}
}

func TestStoreLoadAllEmpty(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	loaded, err := store.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("LoadAll: got %d jobs, want 0", len(loaded))
	}
}

func TestStoreRemove(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	job := NewJob("standard", OutputConfig{Target: "paperless"}, nil, nil)
	if err := store.Save(job); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := store.Remove(job.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// File should be gone
	if _, err := os.Stat(filepath.Join(store.dir, job.ID+".json")); !os.IsNotExist(err) {
		t.Fatal("file still exists after Remove")
	}

	// Load should fail
	if _, err := store.Load(job.ID); err == nil {
		t.Fatal("Load should fail after Remove")
	}
}

func TestStoreRemoveNonexistent(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// Removing a nonexistent job should not error
	if err := store.Remove("does-not-exist"); err != nil {
		t.Fatalf("Remove nonexistent: %v", err)
	}
}

func TestNewQueueWithStore(t *testing.T) {
	dir := t.TempDir()

	// Create a store, submit a job through a queue, then verify a new queue loads it.
	store1, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	q1, err := NewQueueWithStore(store1)
	if err != nil {
		t.Fatalf("NewQueueWithStore: %v", err)
	}

	job := NewJob("standard", OutputConfig{Target: "paperless"}, &DocumentMetadata{Title: "Persist Test"}, nil)
	if err := q1.Submit(job); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	// Simulate status change and persist
	job.SetStatus(StatusCompleted)
	q1.SaveJob(job.ID)

	// Create a new queue from the same store directory (simulates restart)
	store2, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore (2): %v", err)
	}
	q2, err := NewQueueWithStore(store2)
	if err != nil {
		t.Fatalf("NewQueueWithStore (2): %v", err)
	}

	loaded, ok := q2.Get(job.ID)
	if !ok {
		t.Fatal("job not found in reloaded queue")
	}
	if loaded.Status != StatusCompleted {
		t.Errorf("Status: got %q, want %q", loaded.Status, StatusCompleted)
	}
	if loaded.Metadata == nil || loaded.Metadata.Title != "Persist Test" {
		t.Error("metadata not preserved")
	}
}

func TestQueueRemoveCleansStore(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	q, err := NewQueueWithStore(store)
	if err != nil {
		t.Fatalf("NewQueueWithStore: %v", err)
	}

	job := NewJob("standard", OutputConfig{Target: "fs"}, nil, nil)
	if err := q.Submit(job); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	// File should exist
	if _, err := os.Stat(filepath.Join(dir, job.ID+".json")); err != nil {
		t.Fatalf("file should exist after submit: %v", err)
	}

	q.Remove(job.ID)

	// File should be gone
	if _, err := os.Stat(filepath.Join(dir, job.ID+".json")); !os.IsNotExist(err) {
		t.Fatal("file should not exist after Remove")
	}
}

func TestQueueCancelPersists(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	q, err := NewQueueWithStore(store)
	if err != nil {
		t.Fatalf("NewQueueWithStore: %v", err)
	}

	job := NewJob("standard", OutputConfig{Target: "fs"}, nil, nil)
	if err := q.Submit(job); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if err := q.Cancel(job.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	// Reload from store
	loaded, err := store.Load(job.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Status != StatusCancelled {
		t.Errorf("Status: got %q, want %q", loaded.Status, StatusCancelled)
	}
}

func TestStoreCompletedAtPersisted(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	job := NewJob("standard", OutputConfig{Target: "fs"}, nil, nil)
	job.SetStatus(StatusCompleted)

	if err := store.Save(job); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load(job.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.CompletedAt.IsZero() {
		t.Error("CompletedAt should be set for completed job")
	}
	if loaded.CompletedAt.Before(loaded.CreatedAt) {
		t.Error("CompletedAt should be after CreatedAt")
	}
}

func TestQueueWithoutStoreStillWorks(t *testing.T) {
	q := NewQueue()
	job := NewJob("standard", OutputConfig{Target: "fs"}, nil, nil)
	if err := q.Submit(job); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	got, ok := q.Get(job.ID)
	if !ok {
		t.Fatal("job not found")
	}
	if got.ID != job.ID {
		t.Errorf("ID mismatch: got %q, want %q", got.ID, job.ID)
	}
	q.SaveJob(job.ID) // should be a no-op, no panic
	q.Remove(job.ID)  // should work without store
	_, ok = q.Get(job.ID)
	if ok {
		t.Fatal("job should be removed")
	}
}

func TestStoreIgnoresNonJSONFiles(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// Save a real job
	job := NewJob("standard", OutputConfig{Target: "fs"}, nil, nil)
	if err := store.Save(job); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Create non-JSON file and a subdirectory
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hello"), 0o644)
	os.Mkdir(filepath.Join(dir, "subdir"), 0o755)

	loaded, err := store.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("LoadAll: got %d jobs, want 1", len(loaded))
	}
}

func TestStoreTimestampPrecision(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	job := NewJob("standard", OutputConfig{Target: "fs"}, nil, nil)

	if err := store.Save(job); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load(job.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Timestamps should survive round-trip within a microsecond
	diff := job.CreatedAt.Sub(loaded.CreatedAt)
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Microsecond {
		t.Errorf("CreatedAt drift: %v", diff)
	}
}
