package jobs

import (
	"testing"
)

func TestNewJob(t *testing.T) {
	output := OutputConfig{Target: "paperless"}
	metadata := &DocumentMetadata{Title: "Test Document"}

	job := NewJob("standard", output, metadata, nil)

	if job.ID == "" {
		t.Fatal("job ID should not be empty")
	}
	if job.Status != StatusPending {
		t.Fatalf("expected status %s, got %s", StatusPending, job.Status)
	}
	if job.Profile != "standard" {
		t.Fatalf("expected profile standard, got %s", job.Profile)
	}
	if job.Metadata.Title != "Test Document" {
		t.Fatalf("expected title 'Test Document', got %s", job.Metadata.Title)
	}
	if len(job.Pages) != 0 {
		t.Fatalf("expected 0 pages, got %d", len(job.Pages))
	}
}

func TestJobSetStatus(t *testing.T) {
	job := NewJob("standard", OutputConfig{}, nil, nil)

	job.SetStatus(StatusScanning)
	if job.Status != StatusScanning {
		t.Fatalf("expected %s, got %s", StatusScanning, job.Status)
	}

	job.SetStatus(StatusProcessing)
	if job.Status != StatusProcessing {
		t.Fatalf("expected %s, got %s", StatusProcessing, job.Status)
	}
}

func TestJobSetError(t *testing.T) {
	job := NewJob("standard", OutputConfig{}, nil, nil)

	testErr := &testError{msg: "scan failed"}
	job.SetError(testErr)

	if job.Status != StatusFailed {
		t.Fatalf("expected %s, got %s", StatusFailed, job.Status)
	}
	if job.Error != "scan failed" {
		t.Fatalf("expected error 'scan failed', got %s", job.Error)
	}
}

func TestJobAddDeletePage(t *testing.T) {
	job := NewJob("standard", OutputConfig{}, nil, nil)

	job.AddPage(&Page{Number: 1, Width: 100, Height: 200})
	job.AddPage(&Page{Number: 2, Width: 100, Height: 200})
	job.AddPage(&Page{Number: 3, Width: 100, Height: 200})

	if job.PageCount() != 3 {
		t.Fatalf("expected 3 pages, got %d", job.PageCount())
	}

	// Delete middle page
	if !job.DeletePage(2) {
		t.Fatal("expected delete to succeed")
	}

	if job.PageCount() != 2 {
		t.Fatalf("expected 2 pages after delete, got %d", job.PageCount())
	}

	// Check renumbering
	if job.Pages[0].Number != 1 {
		t.Fatalf("expected page 1, got %d", job.Pages[0].Number)
	}
	if job.Pages[1].Number != 2 {
		t.Fatalf("expected page 2, got %d", job.Pages[1].Number)
	}

	// Delete non-existent page
	if job.DeletePage(5) {
		t.Fatal("expected delete to fail for non-existent page")
	}
}

func TestJobCancel(t *testing.T) {
	job := NewJob("standard", OutputConfig{}, nil, nil)

	cancelled := false
	job.SetCancel(func() { cancelled = true })
	job.Cancel()

	if !cancelled {
		t.Fatal("cancel function was not called")
	}
	if job.Status != StatusCancelled {
		t.Fatalf("expected %s, got %s", StatusCancelled, job.Status)
	}
}

func TestQueueSubmitAndGet(t *testing.T) {
	q := NewQueue()
	job := NewJob("standard", OutputConfig{Target: "paperless"}, nil, nil)

	if err := q.Submit(job); err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	got, ok := q.Get(job.ID)
	if !ok {
		t.Fatal("job not found")
	}
	if got.ID != job.ID {
		t.Fatalf("expected job %s, got %s", job.ID, got.ID)
	}

	// Duplicate submit should fail
	if err := q.Submit(job); err == nil {
		t.Fatal("expected error for duplicate submit")
	}
}

func TestQueueList(t *testing.T) {
	q := NewQueue()

	job1 := NewJob("standard", OutputConfig{}, nil, nil)
	job2 := NewJob("oversize", OutputConfig{}, nil, nil)

	q.Submit(job1)
	q.Submit(job2)

	jobs := q.List()
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
}

func TestQueueCancel(t *testing.T) {
	q := NewQueue()
	job := NewJob("standard", OutputConfig{}, nil, nil)

	q.Submit(job)

	if err := q.Cancel(job.ID); err != nil {
		t.Fatalf("cancel failed: %v", err)
	}

	if err := q.Cancel("nonexistent"); err == nil {
		t.Fatal("expected error for nonexistent job")
	}
}

func TestQueueRemove(t *testing.T) {
	q := NewQueue()
	job := NewJob("standard", OutputConfig{}, nil, nil)

	q.Submit(job)
	q.Remove(job.ID)

	_, ok := q.Get(job.ID)
	if ok {
		t.Fatal("job should have been removed")
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string { return e.msg }
