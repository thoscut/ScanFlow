package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/thoscut/scanflow/server/internal/config"
	"github.com/thoscut/scanflow/server/internal/jobs"
	"github.com/thoscut/scanflow/server/internal/output"
	"github.com/thoscut/scanflow/server/internal/processor"
	"github.com/thoscut/scanflow/server/internal/scanner"
)

// Server is the HTTP API server.
type Server struct {
	cfg       *config.Config
	router    chi.Router
	scanner   *scanner.Scanner
	jobQueue  *jobs.Queue
	profiles  *config.ProfileStore
	processor *processor.Pipeline
	outputs   *output.Manager
	wsHub     *WebSocketHub
	server    *http.Server
}

// NewServer creates a new API server.
func NewServer(cfg *config.Config, sc *scanner.Scanner, q *jobs.Queue, profiles *config.ProfileStore, proc *processor.Pipeline, outputs *output.Manager) *Server {
	s := &Server{
		cfg:       cfg,
		scanner:   sc,
		jobQueue:  q,
		profiles:  profiles,
		processor: proc,
		outputs:   outputs,
		wsHub:     NewWebSocketHub(),
	}

	s.setupRouter()
	return s
}

func (s *Server) setupRouter() {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(CORSMiddleware())

	// Health check (no auth required)
	r.Get("/api/v1/health", s.handleHealth)

	// API routes (with auth)
	r.Group(func(r chi.Router) {
		if s.cfg.Server.Auth.Enabled {
			r.Use(AuthMiddleware(s.cfg.Server.Auth.APIKeys))
		}

		// Scanner management
		r.Get("/api/v1/scanner/devices", s.handleListDevices)
		r.Get("/api/v1/scanner/devices/{id}", s.handleGetDevice)
		r.Post("/api/v1/scanner/devices/{id}/open", s.handleOpenDevice)
		r.Delete("/api/v1/scanner/devices/{id}/close", s.handleCloseDevice)

		// Scan operations
		r.Post("/api/v1/scan", s.handleStartScan)
		r.Get("/api/v1/scan/{jobID}", s.handleGetJobStatus)
		r.Delete("/api/v1/scan/{jobID}", s.handleCancelJob)
		r.Get("/api/v1/scan/{jobID}/preview", s.handleGetPreview)
		r.Post("/api/v1/scan/{jobID}/continue", s.handleContinueScan)
		r.Post("/api/v1/scan/{jobID}/finish", s.handleFinishScan)

		// Page management
		r.Get("/api/v1/scan/{jobID}/pages", s.handleListPages)
		r.Delete("/api/v1/scan/{jobID}/pages/{pageNum}", s.handleDeletePage)
		r.Post("/api/v1/scan/{jobID}/pages/reorder", s.handleReorderPages)

		// Output
		r.Get("/api/v1/outputs", s.handleListOutputs)
		r.Post("/api/v1/scan/{jobID}/send", s.handleSendOutput)

		// Profiles
		r.Get("/api/v1/profiles", s.handleListProfiles)
		r.Get("/api/v1/profiles/{name}", s.handleGetProfile)
		r.Post("/api/v1/profiles", s.handleCreateProfile)
		r.Put("/api/v1/profiles/{name}", s.handleUpdateProfile)

		// System
		r.Get("/api/v1/status", s.handleStatus)
		r.Get("/api/v1/settings", s.handleGetSettings)
		r.Put("/api/v1/settings", s.handleUpdateSettings)

		// WebSocket
		r.Get("/api/v1/ws", s.handleWebSocket)
	})

	s.router = r
}

// Start begins listening for HTTP connections.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port)

	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start WebSocket hub
	go s.wsHub.Run()

	// Start job worker
	go s.jobWorker()

	slog.Info("API server starting", "addr", addr)

	if s.cfg.Server.TLS.Enabled {
		return s.server.ListenAndServeTLS(
			s.cfg.Server.TLS.CertFile,
			s.cfg.Server.TLS.KeyFile,
		)
	}
	return s.server.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	slog.Info("API server shutting down")
	return s.server.Shutdown(ctx)
}

// jobWorker processes jobs from the queue.
func (s *Server) jobWorker() {
	for job := range s.jobQueue.Pending() {
		s.processJob(job)
	}
}

func (s *Server) processJob(job *jobs.Job) {
	ctx, cancel := context.WithCancel(context.Background())
	job.SetCancel(cancel)
	defer cancel()

	slog.Info("processing job", "job_id", job.ID, "profile", job.Profile)

	// Get profile
	profile, ok := s.profiles.Get(job.Profile)
	if !ok {
		job.SetError(fmt.Errorf("profile %q not found", job.Profile))
		s.broadcastJobUpdate(job)
		return
	}

	// Set scanning status
	job.SetStatus(jobs.StatusScanning)
	s.broadcastJobUpdate(job)

	// Perform scan
	opts := scanner.ScanOptions{
		Resolution: profile.Scanner.Resolution,
		Mode:       profile.Scanner.Mode,
		Source:     profile.Scanner.Source,
		PageWidth:  profile.Scanner.PageWidth,
		PageHeight: profile.Scanner.PageHeight,
	}

	pages, err := s.scanner.ScanBatch(ctx, opts)
	if err != nil {
		job.SetError(fmt.Errorf("scan failed: %w", err))
		s.broadcastJobUpdate(job)
		return
	}

	for page := range pages {
		if page.Err != nil {
			slog.Warn("page scan error", "error", page.Err, "job_id", job.ID)
			continue
		}
		job.AddPage(page)
		job.SendProgress(jobs.ProgressUpdate{
			Type:    "page_complete",
			Page:    page.Number,
			Message: fmt.Sprintf("Page %d scanned", page.Number),
		})
		s.broadcastJobUpdate(job)
	}

	if job.PageCount() == 0 {
		job.SetError(fmt.Errorf("no pages scanned"))
		s.broadcastJobUpdate(job)
		return
	}

	// Process pages
	job.SetStatus(jobs.StatusProcessing)
	s.broadcastJobUpdate(job)

	doc, err := s.processor.Process(ctx, job, profile)
	if err != nil {
		job.SetError(fmt.Errorf("processing failed: %w", err))
		s.broadcastJobUpdate(job)
		return
	}

	// Send to output
	target := job.Output.Target
	if target == "" {
		target = profile.Output.DefaultTarget
	}

	if err := s.outputs.Send(ctx, target, doc); err != nil {
		job.SetError(fmt.Errorf("output failed: %w", err))
		s.broadcastJobUpdate(job)
		return
	}

	// Done
	job.SetStatus(jobs.StatusCompleted)
	job.SendProgress(jobs.ProgressUpdate{
		Type:    "completed",
		Message: "Document processed and delivered",
	})
	s.broadcastJobUpdate(job)
	slog.Info("job completed", "job_id", job.ID, "pages", job.PageCount())
}

func (s *Server) broadcastJobUpdate(job *jobs.Job) {
	s.wsHub.Broadcast(jobs.ProgressUpdate{
		Type:     "job_update",
		JobID:    job.ID,
		Status:   string(job.Status),
		Progress: job.Progress,
		Message:  string(job.Status),
	})
}
