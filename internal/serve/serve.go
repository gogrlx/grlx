// Package serve provides the local HTTP server for the grlx web UI.
// It serves static web UI assets and proxies API requests to the
// farmer over NATS via the CLI client.
package serve

import (
	"encoding/json"
	"io"
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
	mux.HandleFunc("GET /api/v1/jobs/sprout/{id}", HandleNATSProxyWithID("jobs.forsprout"))
	mux.HandleFunc("DELETE /api/v1/jobs/{jid}", HandleNATSProxyWithID("jobs.cancel"))

	// Cook
	mux.HandleFunc("POST /api/v1/cook", HandleNATSProxyWithBody("cook"))

	// Props
	mux.HandleFunc("GET /api/v1/props", HandleNATSProxy("props.list"))
	mux.HandleFunc("GET /api/v1/props/{id}", HandleNATSProxyWithID("props.get"))
	mux.HandleFunc("GET /api/v1/props/{id}/{key}", HandlePropsKeyProxy("props.getkey"))
	mux.HandleFunc("PUT /api/v1/props/{id}/{key}", HandlePropsSetProxy("props.set"))
	mux.HandleFunc("DELETE /api/v1/props/{id}/{key}", HandlePropsKeyProxy("props.delete"))

	// Cohorts
	mux.HandleFunc("GET /api/v1/cohorts", HandleNATSProxy("cohorts.list"))
	mux.HandleFunc("POST /api/v1/cohorts/resolve", HandleNATSProxyWithBody("cohorts.resolve"))

	// Keys (PKI)
	mux.HandleFunc("GET /api/v1/keys", HandleNATSProxy("pki.list"))
	mux.HandleFunc("POST /api/v1/keys/{id}/accept", HandleNATSProxyWithID("pki.accept"))
	mux.HandleFunc("POST /api/v1/keys/{id}/reject", HandleNATSProxyWithID("pki.reject"))
	mux.HandleFunc("POST /api/v1/keys/{id}/deny", HandleNATSProxyWithID("pki.deny"))
	mux.HandleFunc("POST /api/v1/keys/{id}/unaccept", HandleNATSProxyWithID("pki.unaccept"))
	mux.HandleFunc("DELETE /api/v1/keys/{id}", HandleNATSProxyWithID("pki.delete"))

	// Auth
	mux.HandleFunc("GET /api/v1/auth/whoami", HandleNATSProxy("auth.whoami"))
	mux.HandleFunc("GET /api/v1/auth/users", HandleNATSProxy("auth.users"))

	// Serve embedded web UI (SPA with index.html fallback)
	mux.Handle("GET /", UIHandler())

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

// HandleNATSProxyWithBody returns a handler that reads the request body as JSON
// and forwards it to a NATS subject.
func HandleNATSProxyWithBody(method string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read request body"})
			return
		}
		defer r.Body.Close()

		var params any
		if len(body) > 0 {
			if err := json.Unmarshal(body, &params); err != nil {
				WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
				return
			}
		}

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

// HandlePropsKeyProxy returns a handler that forwards a props request keyed
// by both sprout ID and property key to a NATS subject.
func HandlePropsKeyProxy(method string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		key := r.PathValue("key")
		if id == "" || key == "" {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "missing id or key parameter"})
			return
		}
		params := map[string]string{"id": id, "key": key}
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

// HandlePropsSetProxy returns a handler that forwards a props set request
// with the sprout ID, key, and the request body value to a NATS subject.
func HandlePropsSetProxy(method string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		key := r.PathValue("key")
		if id == "" || key == "" {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "missing id or key parameter"})
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read request body"})
			return
		}
		defer r.Body.Close()

		var value any
		if len(body) > 0 {
			if err := json.Unmarshal(body, &value); err != nil {
				WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
				return
			}
		}

		params := map[string]any{"id": id, "key": key, "value": value}
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
