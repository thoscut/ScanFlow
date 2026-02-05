package jobs

import (
	"fmt"
	"log/slog"
	"sync"
)

// Queue manages scan jobs with a concurrent-safe map and processing channel.
type Queue struct {
	jobs    map[string]*Job
	pending chan *Job
	mu      sync.RWMutex

	subscribers map[string][]chan ProgressUpdate
	subMu       sync.RWMutex
}

// NewQueue creates a new job queue.
func NewQueue() *Queue {
	return &Queue{
		jobs:        make(map[string]*Job),
		pending:     make(chan *Job, 100),
		subscribers: make(map[string][]chan ProgressUpdate),
	}
}

// Submit adds a new job to the queue.
func (q *Queue) Submit(job *Job) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if _, exists := q.jobs[job.ID]; exists {
		return fmt.Errorf("job %s already exists", job.ID)
	}

	q.jobs[job.ID] = job
	slog.Info("job submitted", "job_id", job.ID, "profile", job.Profile)

	// Forward progress updates to subscribers
	go q.forwardProgress(job)

	select {
	case q.pending <- job:
		return nil
	default:
		return fmt.Errorf("job queue is full")
	}
}

// Get returns a job by ID.
func (q *Queue) Get(id string) (*Job, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	job, ok := q.jobs[id]
	return job, ok
}

// List returns all jobs.
func (q *Queue) List() []*Job {
	q.mu.RLock()
	defer q.mu.RUnlock()

	result := make([]*Job, 0, len(q.jobs))
	for _, job := range q.jobs {
		result = append(result, job)
	}
	return result
}

// Pending returns the channel of pending jobs for workers.
func (q *Queue) Pending() <-chan *Job {
	return q.pending
}

// Cancel cancels a job by ID.
func (q *Queue) Cancel(id string) error {
	q.mu.RLock()
	job, ok := q.jobs[id]
	q.mu.RUnlock()

	if !ok {
		return fmt.Errorf("job %s not found", id)
	}

	job.Cancel()
	slog.Info("job cancelled", "job_id", id)
	return nil
}

// Subscribe creates a channel to receive progress updates for a specific job.
func (q *Queue) Subscribe(jobID string) chan ProgressUpdate {
	q.subMu.Lock()
	defer q.subMu.Unlock()

	ch := make(chan ProgressUpdate, 50)
	q.subscribers[jobID] = append(q.subscribers[jobID], ch)
	return ch
}

// Unsubscribe removes a subscriber channel.
func (q *Queue) Unsubscribe(jobID string, ch chan ProgressUpdate) {
	q.subMu.Lock()
	defer q.subMu.Unlock()

	subs := q.subscribers[jobID]
	for i, sub := range subs {
		if sub == ch {
			q.subscribers[jobID] = append(subs[:i], subs[i+1:]...)
			close(ch)
			return
		}
	}
}

func (q *Queue) forwardProgress(job *Job) {
	for update := range job.ProgressChan() {
		q.subMu.RLock()
		subs := q.subscribers[job.ID]
		for _, ch := range subs {
			select {
			case ch <- update:
			default:
				// Subscriber slow, drop update
			}
		}
		q.subMu.RUnlock()
	}
}

// Remove deletes a job from the queue.
func (q *Queue) Remove(id string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.jobs, id)

	q.subMu.Lock()
	defer q.subMu.Unlock()
	if subs, ok := q.subscribers[id]; ok {
		for _, ch := range subs {
			close(ch)
		}
		delete(q.subscribers, id)
	}
}
