package api

import (
	"net/http"

	"github.com/gogrlx/grlx/v2/internal/api/handlers"
	"github.com/gogrlx/grlx/v2/internal/config"
)

// BuildInfoStruct holds the current build version information.
var BuildInfoStruct config.Version

// NewRouter creates an http.ServeMux with HTTPS-only API routes.
// PKI management, sprout commands, jobs, props, cohorts, and auth
// are all handled over the NATS bus. Only certificate download,
// new sprout key submission, version info, and the file server
// remain on HTTPS.
func NewRouter(v config.Version, certificate string) *http.ServeMux {
	handlers.SetBuildVersion(v)
	BuildInfoStruct = v
	_ = certificate // reserved for future TLS configuration
	mux := http.NewServeMux()

	// Public routes (no auth required)
	mux.Handle("GET /auth/cert/", Logger(http.HandlerFunc(handlers.GetCertificate), "GetCertificate"))
	mux.Handle("PUT /pki/putnkey", Logger(http.HandlerFunc(handlers.PutNKey), "PutNKey"))

	// Version (auth required)
	mux.Handle("GET /version", Logger(Auth(http.HandlerFunc(handlers.GetVersion), "GetVersion"), "GetVersion"))

	// File server: serves files from the recipe directory.
	// Sprouts fetch files using farmer:// URLs which resolve to this endpoint.
	fileRoot := config.RecipeDir
	fileServer := http.StripPrefix("/files/", http.FileServer(http.Dir(fileRoot)))
	mux.Handle("GET /files/", Logger(Auth(fileServer, "FileServer"), "FileServer"))

	return mux
}
