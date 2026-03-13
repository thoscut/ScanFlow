package output

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/thoscut/scanflow/server/internal/config"
	"github.com/thoscut/scanflow/server/internal/jobs"
)

const maxRetries = 3

// Handler is the interface for all output targets.
type Handler interface {
	Name() string
	Send(ctx context.Context, doc *jobs.Document) error
	Available() bool
}

// Target describes a configured output target.
type Target struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Enabled   bool   `json:"enabled"`
	Available bool   `json:"available"`
}

// Manager routes documents to the appropriate output handler.
type Manager struct {
	handlers map[string]Handler
}

// NewManager creates a new output manager from the server configuration.
func NewManager(cfg config.OutputConfig) *Manager {
	m := &Manager{
		handlers: make(map[string]Handler),
	}

	if cfg.Paperless.Enabled {
		m.handlers["paperless"] = NewPaperlessHandler(cfg.Paperless)
	}

	if cfg.SMB.Enabled {
		m.handlers["smb"] = NewSMBHandler(cfg.SMB)
	}

	if cfg.PaperlessConsume.Enabled {
		m.handlers["paperless_consume"] = NewPaperlessConsumeHandler(cfg.PaperlessConsume)
	}

	if cfg.Email.Enabled {
		m.handlers["email"] = NewEmailHandler(cfg.Email)
	}

	// Filesystem is always available
	m.handlers["filesystem"] = NewFilesystemHandler("/var/lib/scanflow/documents")

	slog.Info("output handlers initialized", "count", len(m.handlers))
	return m
}

// Send routes a document to the specified output target with retry logic.
func (m *Manager) Send(ctx context.Context, target string, doc *jobs.Document) error {
	handler, ok := m.handlers[target]
	if !ok {
		return fmt.Errorf("unknown output target: %s", target)
	}

	slog.Info("sending document to output",
		"target", target,
		"filename", doc.Filename,
		"size", doc.Size)

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 2^(attempt-1) seconds → 1s, 2s, 4s
			delay := time.Duration(1<<(attempt-1)) * time.Second
			slog.Warn("retrying output send",
				"target", target,
				"attempt", attempt,
				"delay", delay,
				"error", lastErr)
			select {
			case <-ctx.Done():
				return fmt.Errorf("output %s: context cancelled during retry: %w", target, ctx.Err())
			case <-time.After(delay):
			}
		}

		if err := handler.Send(ctx, doc); err != nil {
			lastErr = err
			continue
		}

		slog.Info("document sent successfully",
			"target", target,
			"attempts", attempt+1)
		return nil
	}

	return fmt.Errorf("output %s: all retries exhausted: %w", target, lastErr)
}

// ListTargets returns all configured output targets.
func (m *Manager) ListTargets() []Target {
	targets := make([]Target, 0, len(m.handlers))
	for name, h := range m.handlers {
		targets = append(targets, Target{
			Name:      name,
			Type:      name,
			Enabled:   true,
			Available: h.Available(),
		})
	}
	return targets
}
