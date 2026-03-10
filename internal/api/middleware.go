package api

import (
	"net/http"
	"time"

	"github.com/gogrlx/grlx/v2/internal/auth"
	log "github.com/gogrlx/grlx/v2/internal/log"
)

func Logger(inner http.Handler, name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		inner.ServeHTTP(w, r)

		log.Tracef("%s %s %s %s",
			r.Method, r.RequestURI,
			name, time.Since(start),
		)
	})
}

// Auth wraps a handler with authentication and role-based access control.
// The name parameter must match a key from the Routes map so that
// role permissions can be checked against the route.
//
// Public routes (GetCertificate, PutNKey) are allowed without a token.
// If dangerously_allow_root is set in the farmer config, all requests
// are allowed without authentication.
func Auth(inner http.Handler, name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch name {
		case "GetCertificate", "PutNKey":
			inner.ServeHTTP(w, r)
			return
		}

		// Development bypass — no auth required.
		if auth.DangerouslyAllowRoot() {
			log.Tracef("dangerously_allow_root: bypassing auth for %s", name)
			inner.ServeHTTP(w, r)
			return
		}

		authToken := r.Header.Get("Authorization")
		if authToken == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if auth.TokenHasRouteAccess(authToken, name) {
			inner.ServeHTTP(w, r)
		} else {
			w.WriteHeader(http.StatusForbidden)
		}
	})
}
