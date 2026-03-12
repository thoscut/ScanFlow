package api

import (
	"testing"
	"time"

	"github.com/thoscut/scanflow/server/internal/jobs"
)

func TestWebSocketHubBroadcast(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()

	// Give hub goroutine time to start.
	time.Sleep(10 * time.Millisecond)

	// Broadcast should not block when there are no clients.
	hub.Broadcast(jobs.ProgressUpdate{
		Type:    "test",
		Message: "hello",
	})

	// Verify the hub channel doesn't deadlock.
	time.Sleep(10 * time.Millisecond)
}

func TestWebSocketHubBroadcastChannelFull(t *testing.T) {
	hub := NewWebSocketHub()
	// Don't start Run() to simulate a full channel scenario.

	// Fill the broadcast channel.
	for range 256 {
		hub.Broadcast(jobs.ProgressUpdate{Type: "fill"})
	}

	// This should not block; it should be dropped gracefully.
	hub.Broadcast(jobs.ProgressUpdate{Type: "overflow"})
}
