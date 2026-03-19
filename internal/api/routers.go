package api

import (
	"net/http"

	"github.com/gogrlx/grlx/v2/internal/api/handlers"
	"github.com/gogrlx/grlx/v2/internal/config"
)

// NewBootstrapRouter creates an http.ServeMux for the farmer's PKI bootstrap
// server. This HTTPS server exists solely for initial sprout enrollment:
// sprouts that have no NATS credentials yet use it to fetch the CA certificate
// and register their NKey. Once a sprout is enrolled and connected to the NATS
// bus, all further communication (commands, jobs, props, files, etc.) happens
// exclusively over NATS.
//
// The file server endpoint is a legacy path kept for backward compatibility
// with sprouts that have not yet upgraded to NATS-based file fetching.
//
// No health endpoint is exposed here — the CLI's local HTTP server
// (internal/serve) provides /api/v1/health for monitoring and load balancers.
func NewBootstrapRouter(certificate string) *http.ServeMux {
	_ = certificate // reserved for future TLS configuration
	mux := http.NewServeMux()

	// PKI bootstrap routes (no auth required — pre-enrollment sprouts use these)
	mux.Handle("GET /auth/cert/", Logger(http.HandlerFunc(handlers.GetCertificate), "GetCertificate"))
	mux.Handle("PUT /pki/putnkey", Logger(http.HandlerFunc(handlers.PutNKey), "PutNKey"))

	// Legacy file server: serves files from the recipe directory.
	// New sprouts fetch files over NATS; this endpoint remains for
	// backward compatibility with older sprout versions.
	fileRoot := config.RecipeDir
	fileServer := http.StripPrefix("/files/", http.FileServer(http.Dir(fileRoot)))
	mux.Handle("GET /files/", Logger(Auth(fileServer, "FileServer"), "FileServer"))

	return mux
}
