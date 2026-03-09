package api

import (
	"net/http"

	"github.com/gogrlx/grlx/v2/internal/api/handlers"

	"github.com/taigrr/log-socket/browser"
	"github.com/taigrr/log-socket/ws"

	cmd "github.com/gogrlx/grlx/v2/internal/api/handlers/ingredients/cmd"
	test "github.com/gogrlx/grlx/v2/internal/api/handlers/ingredients/test"
	"github.com/gogrlx/grlx/v2/internal/config"
)

// BuildInfoStruct holds the current build version information.
var BuildInfoStruct config.Version

// Route holds the method and pattern for a named API route.
// Used by the CLI client to construct request URLs.
type Route struct {
	Method  string
	Pattern string
}

// Routes maps route names to their method and URL pattern.
// This is consumed by the API client for URL construction.
var Routes = map[string]Route{
	"GetVersion":        {Method: http.MethodGet, Pattern: "/version"},
	"GetLogSocket":      {Method: http.MethodGet, Pattern: "/logs/ws"},
	"GetLogPage":        {Method: http.MethodGet, Pattern: "/logs"},
	"GetCertificate":    {Method: http.MethodGet, Pattern: "/auth/cert/"},
	"PutNKey":           {Method: http.MethodPut, Pattern: "/pki/putnkey"},
	"GetID":             {Method: http.MethodPost, Pattern: "/pki/getnkey"},
	"AcceptID":          {Method: http.MethodPost, Pattern: "/pki/acceptnkey"},
	"RejectID":          {Method: http.MethodPost, Pattern: "/pki/rejectnkey"},
	"ListID":            {Method: http.MethodPost, Pattern: "/pki/listnkey"},
	"DenyID":            {Method: http.MethodPost, Pattern: "/pki/denynkey"},
	"UnacceptID":        {Method: http.MethodPost, Pattern: "/pki/unacceptnkey"},
	"DeleteID":          {Method: http.MethodPost, Pattern: "/pki/deletenkey"},
	"TestPing":          {Method: http.MethodPost, Pattern: "/test/ping"},
	"Cook":              {Method: http.MethodPost, Pattern: "/cook"},
	"CmdRun":            {Method: http.MethodPost, Pattern: "/cmd/run"},
	"ListSprouts":       {Method: http.MethodGet, Pattern: "/sprouts"},
	"GetSprout":         {Method: http.MethodPost, Pattern: "/sprouts/get"},
	"ListJobs":          {Method: http.MethodGet, Pattern: "/jobs"},
	"GetJob":            {Method: http.MethodGet, Pattern: "/jobs/{jid}"},
	"CancelJob":         {Method: http.MethodDelete, Pattern: "/jobs/{jid}"},
	"ListJobsForSprout": {Method: http.MethodGet, Pattern: "/jobs/sprout/{sproutID}"},
	"GetAllProps":       {Method: http.MethodGet, Pattern: "/props/{sproutID}"},
	"GetProp":           {Method: http.MethodGet, Pattern: "/props/{sproutID}/{name}"},
	"SetProp":           {Method: http.MethodPut, Pattern: "/props/{sproutID}/{name}"},
	"DeleteProp":        {Method: http.MethodDelete, Pattern: "/props/{sproutID}/{name}"},
}

// NewRouter creates an http.ServeMux with all API routes registered.
// Uses Go 1.22+ method and path parameter patterns.
func NewRouter(v config.Version, certificate string) *http.ServeMux {
	handlers.SetBuildVersion(v)
	BuildInfoStruct = v
	_ = certificate // reserved for future TLS configuration
	mux := http.NewServeMux()

	// Public routes (no auth required)
	mux.Handle("GET /auth/cert/", Logger(http.HandlerFunc(handlers.GetCertificate), "GetCertificate"))
	mux.Handle("PUT /pki/putnkey", Logger(http.HandlerFunc(handlers.PutNKey), "PutNKey"))

	// Auth-required routes
	register := func(pattern string, name string, h http.HandlerFunc) {
		mux.Handle(pattern, Logger(Auth(http.HandlerFunc(h), name), name))
	}

	// Version
	register("GET /version", "GetVersion", handlers.GetVersion)

	// Logs
	register("GET /logs/ws", "GetLogSocket", ws.LogSocketHandler)
	register("GET /logs", "GetLogPage", browser.LogSocketViewHandler)

	// PKI
	register("POST /pki/getnkey", "GetID", handlers.GetNKey)
	register("POST /pki/acceptnkey", "AcceptID", handlers.AcceptNKey)
	register("POST /pki/rejectnkey", "RejectID", handlers.RejectNKey)
	register("POST /pki/listnkey", "ListID", handlers.ListNKey)
	register("POST /pki/denynkey", "DenyID", handlers.DenyNKey)
	register("POST /pki/unacceptnkey", "UnacceptID", handlers.UnacceptNKey)
	register("POST /pki/deletenkey", "DeleteID", handlers.DeleteNKey)

	// Test
	register("POST /test/ping", "TestPing", test.HTestPing)

	// Cook
	register("POST /cook", "Cook", handlers.Cook)

	// Cmd
	register("POST /cmd/run", "CmdRun", cmd.HCmdRun)

	// Sprouts
	register("GET /sprouts", "ListSprouts", handlers.ListSprouts)
	register("POST /sprouts/get", "GetSprout", handlers.GetSprout)

	// Jobs
	register("GET /jobs", "ListJobs", handlers.ListJobs)
	register("GET /jobs/{jid}", "GetJob", handlers.GetJob)
	register("DELETE /jobs/{jid}", "CancelJob", handlers.CancelJob)
	register("GET /jobs/sprout/{sproutID}", "ListJobsForSprout", handlers.ListJobsForSprout)

	// Props
	register("GET /props/{sproutID}", "GetAllProps", handlers.GetAllProps)
	register("GET /props/{sproutID}/{name}", "GetProp", handlers.GetProp)
	register("PUT /props/{sproutID}/{name}", "SetProp", handlers.SetProp)
	register("DELETE /props/{sproutID}/{name}", "DeleteProp", handlers.DeleteProp)

	return mux
}
