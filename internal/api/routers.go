package api

import (
	"net/http"

	"github.com/gogrlx/grlx/v2/internal/api/handlers"

	"github.com/gorilla/mux"
	"github.com/taigrr/log-socket/browser"
	"github.com/taigrr/log-socket/ws"

	cmd "github.com/gogrlx/grlx/v2/internal/api/handlers/ingredients/cmd"
	test "github.com/gogrlx/grlx/v2/internal/api/handlers/ingredients/test"
	"github.com/gogrlx/grlx/v2/internal/config"
)

type Route struct {
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
}

var (
	BuildInfoStruct config.Version
	certFile        string
)

func NewRouter(v config.Version, certificate string) *mux.Router {
	handlers.SetBuildVersion(v)
	BuildInfoStruct = v
	certFile = certificate
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
}
