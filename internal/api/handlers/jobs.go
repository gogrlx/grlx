package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	log "github.com/gogrlx/grlx/v2/internal/log"

	"github.com/gogrlx/grlx/v2/internal/jobs"
)

var jobStore *jobs.Store

func init() {
	jobStore = jobs.NewStore()
}

// ListJobs returns all recent jobs across all sprouts.
// Supports an optional ?limit=N query parameter (default 50).
func ListJobs(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil || parsed < 1 {
			http.Error(w, "invalid limit parameter", http.StatusBadRequest)
			return
		}
		limit = parsed
	}

	summaries, err := jobStore.ListAllJobs(limit)
	if err != nil {
		log.Errorf("error listing jobs: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if summaries == nil {
		summaries = []jobs.JobSummary{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(summaries); err != nil {
		log.Errorf("error encoding jobs response: %v", err)
	}
}

// GetJob returns a specific job by JID.
// The JID is extracted from the URL path parameter {jid}.
func GetJob(w http.ResponseWriter, r *http.Request) {
	jid := r.PathValue("jid")
	if jid == "" {
		http.Error(w, "missing jid parameter", http.StatusBadRequest)
		return
	}

	summary, err := jobStore.FindJob(jid)
	if err != nil {
		if errors.Is(err, jobs.ErrJobNotFound) {
			http.Error(w, "job not found", http.StatusNotFound)
			return
		}
		log.Errorf("error finding job %s: %v", jid, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(summary); err != nil {
		log.Errorf("error encoding job response: %v", err)
	}
}

// ListJobsForSprout returns all jobs for a specific sprout.
// The sprout ID is extracted from the URL path parameter {sproutID}.
func ListJobsForSprout(w http.ResponseWriter, r *http.Request) {
	sproutID := r.PathValue("sproutID")
	if sproutID == "" {
		http.Error(w, "missing sproutID parameter", http.StatusBadRequest)
		return
	}

	summaries, err := jobStore.ListJobsForSprout(sproutID)
	if err != nil {
		if errors.Is(err, jobs.ErrSproutNoJobs) {
			// Return empty list, not an error
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("[]"))
			return
		}
		log.Errorf("error listing jobs for sprout %s: %v", sproutID, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if summaries == nil {
		summaries = []jobs.JobSummary{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(summaries); err != nil {
		log.Errorf("error encoding jobs response: %v", err)
	}
}

// CancelJob publishes a cancel request for a job via NATS.
// The sprout must be subscribed to the cancel subject to act on it.
// Returns 202 Accepted if the cancel message was published successfully.
func CancelJob(w http.ResponseWriter, r *http.Request) {
	jid := r.PathValue("jid")
	if jid == "" {
		http.Error(w, "missing jid parameter", http.StatusBadRequest)
		return
	}

	// Find the job to get the sprout ID for targeted cancel
	summary, err := jobStore.FindJob(jid)
	if err != nil {
		if errors.Is(err, jobs.ErrJobNotFound) {
			http.Error(w, "job not found", http.StatusNotFound)
			return
		}
		log.Errorf("error finding job %s for cancel: %v", jid, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Only running or pending jobs can be cancelled
	if summary.Status != jobs.JobRunning && summary.Status != jobs.JobPending {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		resp := map[string]string{
			"error":  "job cannot be cancelled",
			"status": summary.Status.String(),
		}
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Publish cancel event to NATS
	subject := fmt.Sprintf("grlx.sprouts.%s.cancel", summary.SproutID)
	cancelMsg, _ := json.Marshal(map[string]string{"jid": jid})

	if conn == nil {
		log.Error("NATS connection not available for job cancel")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if err := conn.Publish(subject, cancelMsg); err != nil {
		log.Errorf("error publishing cancel for job %s: %v", jid, err)
		http.Error(w, "failed to publish cancel request", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	resp := map[string]string{
		"jid":     jid,
		"sprout":  summary.SproutID,
		"message": "cancel request published",
	}
	json.NewEncoder(w).Encode(resp)
}
