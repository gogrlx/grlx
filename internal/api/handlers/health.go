package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	log "github.com/gogrlx/grlx/v2/internal/log"
)

var startTime time.Time

func init() {
	startTime = time.Now()
}

// HealthResponse is the JSON payload returned by the health endpoint.
type HealthResponse struct {
	Status string `json:"status"`
	Uptime string `json:"uptime"`
}

// GetHealth returns an unauthenticated health check with uptime.
func GetHealth(w http.ResponseWriter, _ *http.Request) {
	resp := HealthResponse{
		Status: "ok",
		Uptime: time.Since(startTime).Round(time.Second).String(),
	}
	jr, err := json.Marshal(resp)
	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jr)
}
