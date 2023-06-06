package api

import (
	"net/http"

	"github.com/gogrlx/grlx/api/handlers"

	"github.com/gorilla/mux"
	"github.com/taigrr/log-socket/browser"
	"github.com/taigrr/log-socket/ws"

	cmd "github.com/gogrlx/grlx/api/handlers/ingredients/cmd"
	test "github.com/gogrlx/grlx/api/handlers/ingredients/test"
	"github.com/gogrlx/grlx/types"
)

type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
}

var (
	BuildInfoStruct types.Version
	certFile        string
)

func NewRouter(v types.Version, certificate string) *mux.Router {
	handlers.SetBuildVersion(v)
	BuildInfoStruct = v
	certFile = certificate
	router := mux.NewRouter().StrictSlash(true)
	for _, route := range routes {
		var handler http.Handler
		handler = route.HandlerFunc
		handler = Logger(handler, route.Name)
		handler = Auth(handler, route.Name)
		router.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(handler)

	}
	return router
}

// TODO start using subrouters
var routes = []Route{
	{
		Name:        "GetVersion",
		Method:      http.MethodGet,
		Pattern:     "/version",
		HandlerFunc: handlers.GetVersion,
	},

	{
		Name:        "GetLogSocket",
		Method:      http.MethodGet,
		Pattern:     "/logs/ws",
		HandlerFunc: ws.LogSocketHandler,
	},
	{
		Name:        "GetLogPage",
		Method:      http.MethodGet,
		Pattern:     "/logs",
		HandlerFunc: browser.LogSocketViewHandler,
	},
	{
		Name:        "GetCertificate",
		Method:      http.MethodGet,
		Pattern:     "/auth/cert/",
		HandlerFunc: handlers.GetCertificate,
	},
	{
		Name:        "PutNKey",
		Method:      http.MethodPut,
		Pattern:     "/pki/putnkey",
		HandlerFunc: handlers.PutNKey,
	},
	{
		Name:        "GetID",
		Method:      http.MethodPost,
		Pattern:     "/pki/getnkey",
		HandlerFunc: handlers.GetNKey,
	},
	{
		Name:        "AcceptID",
		Method:      http.MethodPost,
		Pattern:     "/pki/acceptnkey",
		HandlerFunc: handlers.AcceptNKey,
	},
	{
		Name:        "RejectID",
		Method:      http.MethodPost,
		Pattern:     "/pki/rejectnkey",
		HandlerFunc: handlers.RejectNKey,
	},
	{
		Name:        "ListID",
		Method:      http.MethodPost,
		Pattern:     "/pki/listnkey",
		HandlerFunc: handlers.ListNKey,
	},
	{
		Name:        "DenyID",
		Method:      http.MethodPost,
		Pattern:     "/pki/denynkey",
		HandlerFunc: handlers.DenyNKey,
	},
	{
		Name:        "UnacceptID",
		Method:      http.MethodPost,
		Pattern:     "/pki/unacceptnkey",
		HandlerFunc: handlers.UnacceptNKey,
	},
	{
		Name:        "DeleteID",
		Method:      http.MethodPost,
		Pattern:     "/pki/deletenkey",
		HandlerFunc: handlers.DeleteNKey,
	},
	{
		Name:        "TestPing",
		Method:      http.MethodPost,
		Pattern:     "/test/ping",
		HandlerFunc: test.HTestPing,
	},
	{
		Name:        "CmdRun",
		Method:      http.MethodPost,
		Pattern:     "/cmd/run",
		HandlerFunc: cmd.HCmdRun,
	},
}
