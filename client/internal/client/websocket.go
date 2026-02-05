package client

import (
	"context"
	"log/slog"
	"strings"

	"github.com/gorilla/websocket"
)

// ConnectWebSocket establishes a WebSocket connection for live job updates.
func (c *Client) ConnectWebSocket(ctx context.Context) (<-chan ProgressUpdate, error) {
	wsURL := strings.Replace(c.baseURL, "http", "ws", 1) + "/api/v1/ws?api_key=" + c.apiKey

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return nil, err
	}

	updates := make(chan ProgressUpdate, 100)

	go func() {
		defer close(updates)
		defer conn.Close()

		// Close connection when context is done
		go func() {
			<-ctx.Done()
			conn.Close()
		}()

		for {
			var update ProgressUpdate
			if err := conn.ReadJSON(&update); err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					slog.Debug("websocket error", "error", err)
				}
				return
			}
			select {
			case updates <- update:
			case <-ctx.Done():
				return
			}
		}
	}()

	return updates, nil
}

// WaitForJob polls the job status until it reaches a terminal state.
func (c *Client) WaitForJob(ctx context.Context, jobID string, onUpdate func(ScanJob)) (*ScanJob, error) {
	// Try WebSocket first
	updates, err := c.ConnectWebSocket(ctx)
	if err == nil {
		for update := range updates {
			if update.JobID != jobID {
				continue
			}

			job, err := c.GetJobStatus(ctx, jobID)
			if err != nil {
				continue
			}

			if onUpdate != nil {
				onUpdate(*job)
			}

			switch job.Status {
			case "completed", "failed", "cancelled":
				return job, nil
			}
		}
	}

	// Fallback to polling
	return c.pollJobStatus(ctx, jobID, onUpdate)
}

func (c *Client) pollJobStatus(ctx context.Context, jobID string, onUpdate func(ScanJob)) (*ScanJob, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		job, err := c.GetJobStatus(ctx, jobID)
		if err != nil {
			return nil, err
		}

		if onUpdate != nil {
			onUpdate(*job)
		}

		switch job.Status {
		case "completed", "failed", "cancelled":
			return job, nil
		}

		// Wait before next poll
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}
}
