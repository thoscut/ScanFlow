package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/thoscut/scanflow/server/internal/config"
	"github.com/thoscut/scanflow/server/internal/jobs"
)

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// Health check
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": "0.1.0",
	})
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

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "ok",
		"version":     "0.1.0",
		"scanner":     s.scanner.IsConnected(),
		"devices":     len(devices),
		"active_jobs": activeJobs,
		"total_jobs":  len(jobList),
	})
}

// Scanner management
func (s *Server) handleListDevices(w http.ResponseWriter, r *http.Request) {
	devices := s.scanner.ListDevices()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"devices": devices,
	})
}

func (s *Server) handleGetDevice(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	device, ok := s.scanner.GetDevice(id)
	if !ok {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	writeJSON(w, http.StatusOK, device)
}

func (s *Server) handleOpenDevice(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.scanner.Open(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "opened"})
}

func (s *Server) handleCloseDevice(w http.ResponseWriter, r *http.Request) {
	if err := s.scanner.Close(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "closed"})
}

// Scan operations
func (s *Server) handleStartScan(w http.ResponseWriter, r *http.Request) {
	var req jobs.ScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	profile := req.Profile
	if profile == "" {
		profile = "standard"
	}

	if _, ok := s.profiles.Get(profile); !ok {
		writeError(w, http.StatusBadRequest, "unknown profile: "+profile)
		return
	}

	outputCfg := jobs.OutputConfig{Target: "paperless"}
	if req.Output != nil {
		outputCfg = *req.Output
	}

	job := jobs.NewJob(profile, outputCfg, req.Metadata)

	if err := s.jobQueue.Submit(job); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	slog.Info("scan started via API", "job_id", job.ID, "profile", profile)
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) handleGetJobStatus(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	job, ok := s.jobQueue.Get(jobID)
	if !ok {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	if err := s.jobQueue.Cancel(jobID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

func (s *Server) handleGetPreview(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	job, ok := s.jobQueue.Get(jobID)
	if !ok {
		writeError(w, http.StatusNotFound, "job not found")
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

	writeJSON(w, http.StatusOK, map[string]interface{}{"previews": previews})
}

func (s *Server) handleContinueScan(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	job, ok := s.jobQueue.Get(jobID)
	if !ok {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}

	if job.Status != jobs.StatusScanning && job.Status != jobs.StatusPending {
		writeError(w, http.StatusBadRequest, "job is not in scanning state")
		return
	}

	// Re-submit for continuation scanning
	job.SetStatus(jobs.StatusScanning)
	writeJSON(w, http.StatusOK, map[string]string{"status": "continuing"})
}

func (s *Server) handleFinishScan(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	job, ok := s.jobQueue.Get(jobID)
	if !ok {
		writeError(w, http.StatusNotFound, "job not found")
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
	writeJSON(w, http.StatusOK, map[string]string{"status": "finishing"})
}

// Page management
func (s *Server) handleListPages(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	job, ok := s.jobQueue.Get(jobID)
	if !ok {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"pages": job.Pages})
}

func (s *Server) handleDeletePage(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	pageNum, err := strconv.Atoi(chi.URLParam(r, "pageNum"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid page number")
		return
	}

	job, ok := s.jobQueue.Get(jobID)
	if !ok {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}

	if !job.DeletePage(pageNum) {
		writeError(w, http.StatusNotFound, "page not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleReorderPages(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	_, ok := s.jobQueue.Get(jobID)
	if !ok {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}

	var req struct {
		Order []int `json:"order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "reordered"})
}

// Output targets
func (s *Server) handleListOutputs(w http.ResponseWriter, r *http.Request) {
	outputs := s.outputs.ListTargets()
	writeJSON(w, http.StatusOK, map[string]interface{}{"outputs": outputs})
}

func (s *Server) handleSendOutput(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	_, ok := s.jobQueue.Get(jobID)
	if !ok {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}

	var req struct {
		Target string `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "sending"})
}

// Profiles
func (s *Server) handleListProfiles(w http.ResponseWriter, r *http.Request) {
	profiles := s.profiles.List()
	writeJSON(w, http.StatusOK, map[string]interface{}{"profiles": profiles})
}

func (s *Server) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	profile, ok := s.profiles.Get(name)
	if !ok {
		writeError(w, http.StatusNotFound, "profile not found")
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (s *Server) handleCreateProfile(w http.ResponseWriter, r *http.Request) {
	var profile config.Profile
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	name := profile.Profile.Name
	if name == "" {
		writeError(w, http.StatusBadRequest, "profile name is required")
		return
	}

	s.profiles.Set(name, &profile)
	writeJSON(w, http.StatusCreated, profile)
}

func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if _, ok := s.profiles.Get(name); !ok {
		writeError(w, http.StatusNotFound, "profile not found")
		return
	}

	var profile config.Profile
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	s.profiles.Set(name, &profile)
	writeJSON(w, http.StatusOK, profile)
}
