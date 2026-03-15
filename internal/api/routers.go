package api

import (
	"net/http"

	"github.com/gogrlx/grlx/v2/internal/api/handlers"
	"github.com/gogrlx/grlx/v2/internal/config"
)

// NewRouter creates an http.ServeMux with HTTPS-only API routes.
// PKI management and file serving are the only authenticated routes.
// All command-and-control operations (sprouts, jobs, props, cohorts,
// auth, and version) are handled exclusively over the NATS bus.
// The /health endpoint is unauthenticated for load-balancer probes.
func NewRouter(_ config.Version, certificate string) *http.ServeMux {
	_ = certificate // reserved for future TLS configuration
	mux := http.NewServeMux()

	// Health check (unauthenticated, for load-balancer/monitoring probes)
	mux.Handle("GET /health", Logger(http.HandlerFunc(handlers.GetHealth), "GetHealth"))

	// Public bootstrap routes (no auth required)
	mux.Handle("GET /auth/cert/", Logger(http.HandlerFunc(handlers.GetCertificate), "GetCertificate"))
	mux.Handle("PUT /pki/putnkey", Logger(http.HandlerFunc(handlers.PutNKey), "PutNKey"))

	// File server: serves files from the recipe directory.
	// Sprouts fetch files using farmer:// URLs which resolve to this endpoint.
	fileRoot := config.RecipeDir
	fileServer := http.StripPrefix("/files/", http.FileServer(http.Dir(fileRoot)))
	mux.Handle("GET /files/", Logger(Auth(fileServer, "FileServer"), "FileServer"))

	return mux
}
