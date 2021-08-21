package api

import (
	"net/http"

	"github.com/gogrlx/grlx/api/handlers"
	. "github.com/gogrlx/grlx/types"
	"github.com/gorilla/mux"
	"github.com/taigrr/log-socket/browser"
	"github.com/taigrr/log-socket/ws"
)

var BuildInfoStruct Version
var certFile string

func NewRouter(v Version, certificate string) *mux.Router {
	BuildInfoStruct = v
	certFile = certificate
	router := mux.NewRouter().StrictSlash(true)
	for _, route := range routes {
		var handler http.Handler
		handler = route.HandlerFunc
		handler = Logger(handler, route.Name)
		router.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(handler)

	}
	return router
}

var routes = Routes{
	Route{
		Name:        "GetLogSocket",
		Method:      http.MethodGet,
		Pattern:     "/logs/ws",
		HandlerFunc: ws.LogSocketHandler,
	},
	Route{
		Name:        "GetLogPage",
		Method:      http.MethodGet,
		Pattern:     "/logs",
		HandlerFunc: browser.LogSocketViewHandler,
	},
	Route{
		Name:        "GetCertificate",
		Method:      http.MethodGet,
		Pattern:     "/auth/cert/",
		HandlerFunc: handlers.GetCertificate,
	},
	Route{
		Name:        "PutNKey",
		Method:      http.MethodPut,
		Pattern:     "/pki/putnkey",
		HandlerFunc: handlers.PutNKey,
	},
	Route{
		Name: "AcceptID",
		Method: http.MethodPost,
		Pattern: "/pki/acceptnkey",
		HandlerFunc: handlers.AcceptNKey,
	}
}
