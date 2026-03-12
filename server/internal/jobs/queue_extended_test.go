package jobs

import (
	"testing"
	"time"
)

func TestJobSendProgress(t *testing.T) {
	job := NewJob("standard", OutputConfig{}, nil, nil)

	update := ProgressUpdate{
		Type:    "test",
		Message: "hello",
	}
	job.SendProgress(update)

	select {
	case got := <-job.ProgressChan():
		if got.Type != "test" {
			t.Fatalf("expected type 'test', got %s", got.Type)
		}
		if got.JobID != job.ID {
			t.Fatalf("expected job ID %s, got %s", job.ID, got.JobID)
		}
	case <-time.After(time.Second):
		t.Fatal("expected progress update")
	}
}

func TestJobSendProgressChannelFull(t *testing.T) {
	job := NewJob("standard", OutputConfig{}, nil, nil)

	// Fill the channel
	for range 100 {
		job.SendProgress(ProgressUpdate{Type: "fill"})
	}

	// This should not block
	job.SendProgress(ProgressUpdate{Type: "overflow"})
}

func TestJobCancelWithoutCancelFunc(t *testing.T) {
	job := NewJob("standard", OutputConfig{}, nil, nil)
	// Cancel without setting cancel func should not panic
	job.Cancel()
	if job.Status != StatusCancelled {
		t.Fatalf("expected cancelled, got %s", job.Status)
	}
}

func TestJobSetStatusUpdatesTimestamp(t *testing.T) {
	job := NewJob("standard", OutputConfig{}, nil, nil)
	original := job.UpdatedAt

	time.Sleep(time.Millisecond)
	job.SetStatus(StatusScanning)

	if !job.UpdatedAt.After(original) {
		t.Fatal("expected updated_at to be after original")
	}
}

func TestJobSetErrorUpdatesTimestamp(t *testing.T) {
	job := NewJob("standard", OutputConfig{}, nil, nil)
	original := job.UpdatedAt

	time.Sleep(time.Millisecond)
	job.SetError(&testError{msg: "fail"})

	if !job.UpdatedAt.After(original) {
		t.Fatal("expected updated_at to be after original")
	}
}

func TestJobAddPageUpdatesTimestamp(t *testing.T) {
	job := NewJob("standard", OutputConfig{}, nil, nil)
	original := job.UpdatedAt

	time.Sleep(time.Millisecond)
	job.AddPage(&Page{Number: 1})

	if !job.UpdatedAt.After(original) {
		t.Fatal("expected updated_at to be after original")
	}
}

func TestJobDeleteFirstPage(t *testing.T) {
	job := NewJob("standard", OutputConfig{}, nil, nil)
	job.AddPage(&Page{Number: 1})
	job.AddPage(&Page{Number: 2})
	job.AddPage(&Page{Number: 3})

	job.DeletePage(1)
	if job.PageCount() != 2 {
		t.Fatalf("expected 2 pages, got %d", job.PageCount())
	}
	// Check renumbering
	if job.Pages[0].Number != 1 {
		t.Fatalf("first page should be renumbered to 1, got %d", job.Pages[0].Number)
	}
	if job.Pages[1].Number != 2 {
		t.Fatalf("second page should be renumbered to 2, got %d", job.Pages[1].Number)
	}
}

func TestJobDeleteLastPage(t *testing.T) {
	job := NewJob("standard", OutputConfig{}, nil, nil)
	job.AddPage(&Page{Number: 1})
	job.AddPage(&Page{Number: 2})

	job.DeletePage(2)
	if job.PageCount() != 1 {
		t.Fatalf("expected 1 page, got %d", job.PageCount())
	}
	if job.Pages[0].Number != 1 {
		t.Fatalf("remaining page should be 1, got %d", job.Pages[0].Number)
	}
}

func TestNewJobWithOcrEnabled(t *testing.T) {
	ocrOn := true
	job := NewJob("standard", OutputConfig{}, nil, &ocrOn)

	if job.OcrEnabled == nil {
		t.Fatal("expected OcrEnabled to be set")
	}
	if !*job.OcrEnabled {
		t.Fatal("expected OcrEnabled to be true")
	}
}

func TestQueueSubscribeUnsubscribe(t *testing.T) {
	q := NewQueue()
	job := NewJob("standard", OutputConfig{}, nil, nil)
	q.Submit(job)

	ch := q.Subscribe(job.ID)
	if ch == nil {
		t.Fatal("expected non-nil channel")
	}

	q.Unsubscribe(job.ID, ch)
}

func TestQueueRemoveClosesSubscribers(t *testing.T) {
	q := NewQueue()
	job := NewJob("standard", OutputConfig{}, nil, nil)
	q.Submit(job)

	ch := q.Subscribe(job.ID)
	q.Remove(job.ID)

	// Channel should be closed
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected channel to be closed")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for channel close")
	}
}

func TestQueuePending(t *testing.T) {
	q := NewQueue()
	job := NewJob("standard", OutputConfig{}, nil, nil)
	q.Submit(job)

	pending := q.Pending()
	select {
	case got := <-pending:
		if got.ID != job.ID {
			t.Fatalf("expected job %s, got %s", job.ID, got.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("expected pending job")
	}
}

func TestQueueGetNotFound(t *testing.T) {
	q := NewQueue()
	_, ok := q.Get("nonexistent")
	if ok {
		t.Fatal("expected not found")
	}
}

func TestQueueListEmpty(t *testing.T) {
	q := NewQueue()
	list := q.List()
	if len(list) != 0 {
		t.Fatalf("expected empty list, got %d", len(list))
	}
}
