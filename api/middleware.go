package api

import (
	"net/http"
	"time"

	"github.com/gogrlx/grlx/auth"
	log "github.com/taigrr/log-socket/log"
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

func Auth(inner http.Handler, name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch name {
		case "GetCertificate", "PutNKey":
			inner.ServeHTTP(w, r)
		default:
			authToken := r.Header.Get("Authorization")
			if authToken == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if auth.TokenHasAccess(authToken, r.Method) {
				inner.ServeHTTP(w, r)
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		}
	})
}
