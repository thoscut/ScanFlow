package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	toml "github.com/pelletier/go-toml/v2"
	"github.com/thoscut/scanflow/server/internal/config"
	"github.com/thoscut/scanflow/server/internal/jobs"
)

// validOCRLangAPI matches Tesseract language codes for API input validation.
var validOCRLangAPI = regexp.MustCompile(`^[a-zA-Z0-9_]+(\+[a-zA-Z0-9_]+)*$`)

func writeJSON(w http.ResponseWriter, status int, v any, r *http.Request) {
	if reqID := middleware.GetReqID(r.Context()); reqID != "" {
		w.Header().Set("X-Request-ID", reqID)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string, r *http.Request) {
	resp := map[string]string{"error": msg}
	if reqID := middleware.GetReqID(r.Context()); reqID != "" {
		resp["request_id"] = reqID
	}
	writeJSON(w, status, resp, r)
}

// Health check
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": "0.1.0",
	}, r)
}

// Readiness probe
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if !s.scanner.IsConnected() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "not_ready",
			"reason": "scanner not connected",
		}, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ready",
	}, r)
}

// Server status
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	devices := s.scanner.ListDevices()
	jobList := s.jobQueue.List()

	activeJobs := 0
	for _, j := range jobList {
		if j.Status == jobs.StatusScanning || j.Status == jobs.StatusProcessing {
			activeJobs++
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "ok",
		"version":     "0.1.0",
		"scanner":     s.scanner.IsConnected(),
		"devices":     len(devices),
		"active_jobs": activeJobs,
		"total_jobs":  len(jobList),
	}, r)
}

// Scanner management
func (s *Server) handleListDevices(w http.ResponseWriter, r *http.Request) {
	devices := s.scanner.ListDevices()
	writeJSON(w, http.StatusOK, map[string]any{
		"devices": devices,
	}, r)
}

func (s *Server) handleGetDevice(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	device, ok := s.scanner.GetDevice(id)
	if !ok {
		writeError(w, http.StatusNotFound, "device not found", r)
		return
	}
	writeJSON(w, http.StatusOK, device, r)
}

func (s *Server) handleOpenDevice(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.scanner.Open(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "opened"}, r)
}

func (s *Server) handleCloseDevice(w http.ResponseWriter, r *http.Request) {
	if err := s.scanner.Close(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "closed"}, r)
}

// Scanner capabilities
func (s *Server) handleGetCapabilities(w http.ResponseWriter, r *http.Request) {
	caps := s.scanner.GetCapabilities()
	writeJSON(w, http.StatusOK, caps, r)
}

// Scan operations
func (s *Server) handleStartScan(w http.ResponseWriter, r *http.Request) {
	var req jobs.ScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", r)
		return
	}

	profile := req.Profile
	if profile == "" {
		profile = "standard"
	}

	if _, ok := s.profiles.Get(profile); !ok {
		writeError(w, http.StatusBadRequest, "unknown profile: "+profile, r)
		return
	}

	outputCfg := jobs.OutputConfig{Target: "paperless"}
	if req.Output != nil {
		outputCfg = *req.Output
	}

	job := jobs.NewJob(profile, outputCfg, req.Metadata, req.OcrEnabled)

	if err := s.jobQueue.Submit(job); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), r)
		return
	}

	slog.Info("scan started via API", "job_id", job.ID, "profile", profile)
	writeJSON(w, http.StatusAccepted, job, r)
}

func (s *Server) handleGetJobStatus(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	job, ok := s.jobQueue.Get(jobID)
	if !ok {
		writeError(w, http.StatusNotFound, "job not found", r)
		return
	}
	writeJSON(w, http.StatusOK, job, r)
}

func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	if err := s.jobQueue.Cancel(jobID); err != nil {
		writeError(w, http.StatusNotFound, err.Error(), r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"}, r)
}

func (s *Server) handleGetPreview(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	job, ok := s.jobQueue.Get(jobID)
	if !ok {
		writeError(w, http.StatusNotFound, "job not found", r)
		return
	}

	type pagePreview struct {
		Number int    `json:"number"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
		URL    string `json:"url"`
	}

	previews := make([]pagePreview, 0, len(job.Pages))
	for _, p := range job.Pages {
		previews = append(previews, pagePreview{
			Number: p.Number,
			Width:  p.Width,
			Height: p.Height,
			URL:    "/api/v1/scan/" + jobID + "/pages/" + strconv.Itoa(p.Number) + "/preview",
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"previews": previews}, r)
}

func (s *Server) handleContinueScan(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	job, ok := s.jobQueue.Get(jobID)
	if !ok {
		writeError(w, http.StatusNotFound, "job not found", r)
		return
	}

	if job.Status != jobs.StatusScanning && job.Status != jobs.StatusPending {
		writeError(w, http.StatusBadRequest, "job is not in scanning state", r)
		return
	}

	// Re-submit for continuation scanning
	job.SetStatus(jobs.StatusScanning)
	writeJSON(w, http.StatusOK, map[string]string{"status": "continuing"}, r)
}

func (s *Server) handleFinishScan(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	job, ok := s.jobQueue.Get(jobID)
	if !ok {
		writeError(w, http.StatusNotFound, "job not found", r)
		return
	}

	// Parse optional output/metadata overrides
	var req struct {
		Output   *jobs.OutputConfig     `json:"output,omitempty"`
		Metadata *jobs.DocumentMetadata `json:"metadata,omitempty"`
	}
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req)
	}

	if req.Output != nil {
		job.Output = *req.Output
	}
	if req.Metadata != nil {
		job.Metadata = req.Metadata
	}

	job.SetStatus(jobs.StatusProcessing)
	writeJSON(w, http.StatusOK, map[string]string{"status": "finishing"}, r)
}

// Page management
func (s *Server) handleListPages(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	job, ok := s.jobQueue.Get(jobID)
	if !ok {
		writeError(w, http.StatusNotFound, "job not found", r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pages": job.Pages}, r)
}

func (s *Server) handleDeletePage(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	pageNum, err := strconv.Atoi(chi.URLParam(r, "pageNum"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid page number", r)
		return
	}

	job, ok := s.jobQueue.Get(jobID)
	if !ok {
		writeError(w, http.StatusNotFound, "job not found", r)
		return
	}

	if !job.DeletePage(pageNum) {
		writeError(w, http.StatusNotFound, "page not found", r)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"}, r)
}

func (s *Server) handleReorderPages(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	_, ok := s.jobQueue.Get(jobID)
	if !ok {
		writeError(w, http.StatusNotFound, "job not found", r)
		return
	}

	var req struct {
		Order []int `json:"order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", r)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "reordered"}, r)
}

// Output targets
func (s *Server) handleListOutputs(w http.ResponseWriter, r *http.Request) {
	outputs := s.outputs.ListTargets()
	writeJSON(w, http.StatusOK, map[string]any{"outputs": outputs}, r)
}

func (s *Server) handleSendOutput(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	_, ok := s.jobQueue.Get(jobID)
	if !ok {
		writeError(w, http.StatusNotFound, "job not found", r)
		return
	}

	var req struct {
		Target string `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", r)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "sending"}, r)
}

// Profiles
func (s *Server) handleListProfiles(w http.ResponseWriter, r *http.Request) {
	profiles := s.profiles.List()
	writeJSON(w, http.StatusOK, map[string]any{"profiles": profiles}, r)
}

func (s *Server) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	profile, ok := s.profiles.Get(name)
	if !ok {
		writeError(w, http.StatusNotFound, "profile not found", r)
		return
	}
	writeJSON(w, http.StatusOK, profile, r)
}

func (s *Server) handleCreateProfile(w http.ResponseWriter, r *http.Request) {
	var profile config.Profile
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", r)
		return
	}

	name := profile.Profile.Name
	if name == "" {
		writeError(w, http.StatusBadRequest, "profile name is required", r)
		return
	}

	s.profiles.Set(name, &profile)
	writeJSON(w, http.StatusCreated, profile, r)
}

func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if _, ok := s.profiles.Get(name); !ok {
		writeError(w, http.StatusNotFound, "profile not found", r)
		return
	}

	var profile config.Profile
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", r)
		return
	}

	s.profiles.Set(name, &profile)
	writeJSON(w, http.StatusOK, profile, r)
}

// Settings

type settingsResponse struct {
	OcrEnabled  bool   `json:"ocr_enabled"`
	OcrLanguage string `json:"ocr_language"`
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, settingsResponse{
		OcrEnabled:  s.cfg.Processing.OCR.Enabled,
		OcrLanguage: s.cfg.Processing.OCR.Language,
	}, r)
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req settingsResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", r)
		return
	}

	// Validate OCR language to prevent command injection through Tesseract args.
	if req.OcrLanguage != "" && !validOCRLangAPI.MatchString(req.OcrLanguage) {
		writeError(w, http.StatusBadRequest, "invalid OCR language", r)
		return
	}

	s.cfg.Processing.OCR.Enabled = req.OcrEnabled
	if req.OcrLanguage != "" {
		s.cfg.Processing.OCR.Language = req.OcrLanguage
	}

	// Update the pipeline with new settings
	s.processor.SetOCR(req.OcrEnabled, s.cfg.Processing.OCR.Language)

	writeJSON(w, http.StatusOK, settingsResponse{
		OcrEnabled:  s.cfg.Processing.OCR.Enabled,
		OcrLanguage: s.cfg.Processing.OCR.Language,
	}, r)
}

// Profile export - returns profile as downloadable TOML
func (s *Server) handleExportProfile(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	profile, ok := s.profiles.Get(name)
	if !ok {
		writeError(w, http.StatusNotFound, "profile not found", r)
		return
	}
	// Sanitize the profile name for use in the Content-Disposition header
	// to prevent header injection via special characters.
	safeName := sanitizeHeaderValue(name)
	w.Header().Set("Content-Type", "application/toml")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.toml"`, safeName))
	if err := toml.NewEncoder(w).Encode(profile); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encode profile", r)
		return
	}
}

// sanitizeHeaderValue removes characters that could cause HTTP header injection.
func sanitizeHeaderValue(v string) string {
	result := make([]byte, 0, len(v))
	for i := 0; i < len(v); i++ {
		c := v[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' {
			result = append(result, c)
		}
	}
	if len(result) == 0 {
		return "profile"
	}
	return string(result)
}

// Profile import - accepts TOML body
func (s *Server) handleImportProfile(w http.ResponseWriter, r *http.Request) {
	var profile config.Profile
	if err := toml.NewDecoder(r.Body).Decode(&profile); err != nil {
		writeError(w, http.StatusBadRequest, "invalid TOML: "+err.Error(), r)
		return
	}

	name := profile.Profile.Name
	if name == "" {
		writeError(w, http.StatusBadRequest, "profile name is required", r)
		return
	}

	s.profiles.Set(name, &profile)
	writeJSON(w, http.StatusCreated, profile, r)
}
