package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/thoscut/scanflow/server/internal/acme"
	"github.com/thoscut/scanflow/server/internal/config"
	"github.com/thoscut/scanflow/server/internal/jobs"
	"github.com/thoscut/scanflow/server/internal/output"
	"github.com/thoscut/scanflow/server/internal/processor"
	"github.com/thoscut/scanflow/server/internal/scanner"
)

// Server is the HTTP API server.
type Server struct {
	cfg         *config.Config
	router      chi.Router
	scanner     *scanner.Scanner
	jobQueue    *jobs.Queue
	profiles    *config.ProfileStore
	processor   *processor.Pipeline
	outputs     *output.Manager
	wsHub       *WebSocketHub
	metrics     *Metrics
	server      *http.Server
	acmeMgr     *acme.Manager
	acmeHTTPSrv *http.Server // port 80 listener for ACME HTTP-01 challenges
	jobTimeout  time.Duration
	done        chan struct{} // closed when jobWorker exits
}

// NewServer creates a new API server.
func NewServer(cfg *config.Config, sc *scanner.Scanner, q *jobs.Queue, profiles *config.ProfileStore, proc *processor.Pipeline, outputs *output.Manager) *Server {
	s := &Server{
		cfg:        cfg,
		scanner:    sc,
		jobQueue:   q,
		profiles:   profiles,
		processor:  proc,
		outputs:    outputs,
		wsHub:      NewWebSocketHub(),
		metrics:    NewMetrics(),
		jobTimeout: cfg.Processing.JobTimeout.Duration(),
		done:       make(chan struct{}),
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
	r.Use(SecurityHeadersMiddleware)
	r.Use(MaxBodyMiddleware)
	r.Use(CORSMiddleware())
	r.Use(RateLimitMiddleware(10, 20))
	r.Use(MetricsMiddleware(s.metrics))

	// Health check and metrics (no auth required)
	r.Get("/api/v1/health", s.handleHealth)
	r.Get("/api/v1/ready", s.handleReady)
	r.Get("/metrics", s.metrics.Handler())

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
func (s *Server) Start(ctx context.Context) error {
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

	// ACME / Let's Encrypt automatic certificates.
	if s.cfg.Server.TLS.ACME.Enabled {
		return s.startACME(ctx)
	}

	// Manual TLS with static certificate files.
	if s.cfg.Server.TLS.Enabled {
		return s.server.ListenAndServeTLS(
			s.cfg.Server.TLS.CertFile,
			s.cfg.Server.TLS.KeyFile,
		)
	}
	return s.server.ListenAndServe()
}

// startACME configures and starts the server with ACME-managed TLS.
func (s *Server) startACME(ctx context.Context) error {
	mgr, err := acme.New(s.cfg.Server.TLS.ACME)
	if err != nil {
		return fmt.Errorf("ACME setup: %w", err)
	}
	s.acmeMgr = mgr

	// For DNS challenges, obtain the certificate before starting TLS.
	if err := mgr.EnsureCertificate(ctx); err != nil {
		return fmt.Errorf("ACME certificate: %w", err)
	}

	// Start background renewal.
	mgr.StartRenewal(ctx)

	// Configure TLS.
	s.server.TLSConfig = mgr.TLSConfig()

	// If using HTTP-01, start a redirect handler on port 80.
	if handler := mgr.HTTPHandler(nil); handler != nil {
		httpAddr := fmt.Sprintf("%s:80", s.cfg.Server.Host)
		s.acmeHTTPSrv = &http.Server{
			Addr:         httpAddr,
			Handler:      handler,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		}
		go func() {
			slog.Info("ACME HTTP challenge listener starting", "addr", httpAddr)
			if err := s.acmeHTTPSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("ACME HTTP listener error", "error", err)
			}
		}()
	}

	// ListenAndServeTLS with empty cert/key uses the tls.Config.
	return s.server.ListenAndServeTLS("", "")
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	slog.Info("API server shutting down")

	// Stop accepting new jobs.
	s.jobQueue.Close()

	// Wait for the in-flight job worker to finish.
	select {
	case <-s.done:
		slog.Info("job worker finished")
	case <-ctx.Done():
		slog.Warn("timed out waiting for job worker to finish")
	}

	if s.acmeHTTPSrv != nil {
		if err := s.acmeHTTPSrv.Shutdown(ctx); err != nil {
			slog.Warn("ACME HTTP listener shutdown error", "error", err)
		}
	}
	return s.server.Shutdown(ctx)
}

// jobWorker processes jobs from the queue.
func (s *Server) jobWorker() {
	defer close(s.done)
	for job := range s.jobQueue.Pending() {
		s.processJob(job)
	}
}

func (s *Server) processJob(job *jobs.Job) {
	ctx, cancel := context.WithTimeout(context.Background(), s.jobTimeout)
	job.SetCancel(cancel)
	defer cancel()

	s.metrics.JobStarted()

	slog.Info("processing job", "job_id", job.ID, "profile", job.Profile)

	// Get profile
	profile, ok := s.profiles.Get(job.Profile)
	if !ok {
		job.SetError(fmt.Errorf("profile %q not found", job.Profile))
		s.jobQueue.SaveJob(job.ID)
		s.metrics.JobFailed()
		s.broadcastJobUpdate(job)
		return
	}

	// Set scanning status
	job.SetStatus(jobs.StatusScanning)
	s.jobQueue.SaveJob(job.ID)
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
		s.jobQueue.SaveJob(job.ID)
		s.metrics.JobFailed()
		s.broadcastJobUpdate(job)
		return
	}

	for page := range pages {
		if page.Err != nil {
			slog.Warn("page scan error", "error", page.Err, "job_id", job.ID)
			continue
		}
		job.AddPage(page)
		s.metrics.PageScanned()
		job.SendProgress(jobs.ProgressUpdate{
			Type:    "page_complete",
			Page:    page.Number,
			Message: fmt.Sprintf("Page %d scanned", page.Number),
		})
		s.broadcastJobUpdate(job)
	}

	if job.PageCount() == 0 {
		job.SetError(fmt.Errorf("no pages scanned"))
		s.jobQueue.SaveJob(job.ID)
		s.metrics.JobFailed()
		s.broadcastJobUpdate(job)
		return
	}

	// Process pages
	job.SetStatus(jobs.StatusProcessing)
	s.jobQueue.SaveJob(job.ID)
	s.broadcastJobUpdate(job)

	doc, err := s.processor.Process(ctx, job, profile)
	if err != nil {
		job.SetError(fmt.Errorf("processing failed: %w", err))
		s.jobQueue.SaveJob(job.ID)
		s.metrics.JobFailed()
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
		s.jobQueue.SaveJob(job.ID)
		s.metrics.JobFailed()
		s.broadcastJobUpdate(job)
		return
	}

	// Done
	job.SetStatus(jobs.StatusCompleted)
	s.jobQueue.SaveJob(job.ID)
	s.metrics.JobCompleted()
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
