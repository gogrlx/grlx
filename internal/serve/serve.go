// Package serve provides the local HTTP server for the grlx web UI.
// It serves static web UI assets and proxies API requests to the
// farmer over NATS via the CLI client.
package serve

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gogrlx/grlx/v2/internal/api/client"
	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/log"
)

// BuildInfo is set by the caller (typically the serve command) to provide
// version information to the /api/v1/version endpoint.
var BuildInfo config.Version

// NewMux returns an http.ServeMux with all API routes registered.
func NewMux() *http.ServeMux {
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /api/v1/health", HandleHealth)

	// Version
	mux.HandleFunc("GET /api/v1/version", HandleVersion)

	// Sprouts
	mux.HandleFunc("GET /api/v1/sprouts", HandleNATSProxy("sprouts.list"))
	mux.HandleFunc("GET /api/v1/sprouts/{id}", HandleNATSProxyWithID("sprouts.get"))

	// Jobs
	mux.HandleFunc("GET /api/v1/jobs", HandleNATSProxy("jobs.list"))
	mux.HandleFunc("GET /api/v1/jobs/{jid}", HandleNATSProxyWithID("jobs.get"))

	// Props
	mux.HandleFunc("GET /api/v1/props", HandleNATSProxy("props.list"))
	mux.HandleFunc("GET /api/v1/props/{id}", HandleNATSProxyWithID("props.get"))

	// Cohorts
	mux.HandleFunc("GET /api/v1/cohorts", HandleNATSProxy("cohorts.list"))

	// Web UI placeholder (will be replaced with embed.FS)
	mux.HandleFunc("GET /", HandleUIPlaceholder)

	return mux
}

// HandleHealth returns a simple health check response.
func HandleHealth(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// HandleVersion returns CLI and farmer version info.
func HandleVersion(w http.ResponseWriter, _ *http.Request) {
	farmerVersion, err := client.GetVersion()
	cv := config.CombinedVersion{
		CLI:    BuildInfo,
		Farmer: farmerVersion,
	}
	if err != nil {
		cv.Error = err.Error()
	}
	WriteJSON(w, http.StatusOK, cv)
}

// HandleNATSProxy returns a handler that forwards the request to a NATS subject.
func HandleNATSProxy(method string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		result, err := client.NatsRequest(method, nil)
		if err != nil {
			WriteJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(result)
	}
}

// HandleNATSProxyWithID returns a handler that forwards the request to a NATS
// subject with the path parameter as the payload.
func HandleNATSProxyWithID(method string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			id = r.PathValue("jid")
		}
		if id == "" {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "missing id parameter"})
			return
		}
		params := map[string]string{"id": id}
		result, err := client.NatsRequest(method, params)
		if err != nil {
			WriteJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(result)
	}
}

// HandleUIPlaceholder serves a placeholder page until the web UI is embedded.
func HandleUIPlaceholder(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>grlx</title></head>
<body>
<h1>grlx</h1>
<p>Web UI not yet embedded. Build the web UI and embed it to serve it here.</p>
<p>API available at <code>/api/v1/</code></p>
</body>
</html>`)
}

// WithCORS wraps a handler with permissive CORS headers for local development.
func WithCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// WriteJSON marshals v to JSON and writes it to the response.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("WriteJSON: %v", err)
	}
}
