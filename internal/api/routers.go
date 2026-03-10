package api

import (
	"net/http"

	"github.com/gogrlx/grlx/v2/internal/api/handlers"

	"github.com/gorilla/mux"
	"github.com/taigrr/log-socket/v2/browser"
	"github.com/taigrr/log-socket/v2/ws"

	cmd "github.com/gogrlx/grlx/v2/internal/api/handlers/ingredients/cmd"
	test "github.com/gogrlx/grlx/v2/internal/api/handlers/ingredients/test"
	"github.com/gogrlx/grlx/v2/internal/config"
)

type Route struct {
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
}

// BuildInfoStruct holds the current build version information.
var BuildInfoStruct config.Version

func NewRouter(v config.Version, certificate string) *mux.Router {
	handlers.SetBuildVersion(v)
	BuildInfoStruct = v
	_ = certificate // reserved for future TLS configuration
	router := mux.NewRouter().StrictSlash(true)
	for name, route := range Routes {
		var handler http.Handler
		handler = route.HandlerFunc
		handler = Logger(handler, name)
		handler = Auth(handler, name)
		router.
			Methods(route.Method).
			Path(route.Pattern).
			Name(name).
			Handler(handler)

	}
	return router
}

// TODO start using subrouters
var Routes = map[string]Route{
	"GetVersion": {
		Method:      http.MethodGet,
		Pattern:     "/version",
		HandlerFunc: handlers.GetVersion,
	},
	"GetLogSocket": {
		Method:      http.MethodGet,
		Pattern:     "/logs/ws",
		HandlerFunc: ws.LogSocketHandler,
	},
	"GetLogPage": {
		Method:      http.MethodGet,
		Pattern:     "/logs",
		HandlerFunc: browser.LogSocketViewHandler,
	},
	"GetCertificate": {
		Method:      http.MethodGet,
		Pattern:     "/auth/cert/",
		HandlerFunc: handlers.GetCertificate,
	},
	"PutNKey": {
		Method:      http.MethodPut,
		Pattern:     "/pki/putnkey",
		HandlerFunc: handlers.PutNKey,
	},
	"GetID": {
		Method:      http.MethodPost,
		Pattern:     "/pki/getnkey",
		HandlerFunc: handlers.GetNKey,
	},
	"AcceptID": {
		Method:      http.MethodPost,
		Pattern:     "/pki/acceptnkey",
		HandlerFunc: handlers.AcceptNKey,
	},
	"RejectID": {
		Method:      http.MethodPost,
		Pattern:     "/pki/rejectnkey",
		HandlerFunc: handlers.RejectNKey,
	},
	"ListID": {
		Method:      http.MethodPost,
		Pattern:     "/pki/listnkey",
		HandlerFunc: handlers.ListNKey,
	},
	"DenyID": {
		Method:      http.MethodPost,
		Pattern:     "/pki/denynkey",
		HandlerFunc: handlers.DenyNKey,
	},
	"UnacceptID": {
		Method:      http.MethodPost,
		Pattern:     "/pki/unacceptnkey",
		HandlerFunc: handlers.UnacceptNKey,
	},
	"DeleteID": {
		Method:      http.MethodPost,
		Pattern:     "/pki/deletenkey",
		HandlerFunc: handlers.DeleteNKey,
	},
	"TestPing": {
		Method:      http.MethodPost,
		Pattern:     "/test/ping",
		HandlerFunc: test.HTestPing,
	},
	"Cook": {
		Method:      http.MethodPost,
		Pattern:     "/cook",
		HandlerFunc: handlers.Cook,
	},
	"CmdRun": {
		Method:      http.MethodPost,
		Pattern:     "/cmd/run",
		HandlerFunc: cmd.HCmdRun,
	},
	"ListSprouts": {
		Method:      http.MethodGet,
		Pattern:     "/sprouts",
		HandlerFunc: handlers.ListSprouts,
	},
	"GetSprout": {
		Method:      http.MethodPost,
		Pattern:     "/sprouts/get",
		HandlerFunc: handlers.GetSprout,
	},
	"ListJobs": {
		Method:      http.MethodGet,
		Pattern:     "/jobs",
		HandlerFunc: handlers.ListJobs,
	},
	"GetJob": {
		Method:      http.MethodGet,
		Pattern:     "/jobs/{jid}",
		HandlerFunc: handlers.GetJob,
	},
	"CancelJob": {
		Method:      http.MethodDelete,
		Pattern:     "/jobs/{jid}",
		HandlerFunc: handlers.CancelJob,
	},
	"ListJobsForSprout": {
		Method:      http.MethodGet,
		Pattern:     "/jobs/sprout/{sproutID}",
		HandlerFunc: handlers.ListJobsForSprout,
	},
	"GetAllProps": {
		Method:      http.MethodGet,
		Pattern:     "/props/{sproutID}",
		HandlerFunc: handlers.GetAllProps,
	},
	"GetProp": {
		Method:      http.MethodGet,
		Pattern:     "/props/{sproutID}/{name}",
		HandlerFunc: handlers.GetProp,
	},
	"SetProp": {
		Method:      http.MethodPut,
		Pattern:     "/props/{sproutID}/{name}",
		HandlerFunc: handlers.SetProp,
	},
	"DeleteProp": {
		Method:      http.MethodDelete,
		Pattern:     "/props/{sproutID}/{name}",
		HandlerFunc: handlers.DeleteProp,
	},
}
