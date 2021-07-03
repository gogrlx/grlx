package api

import (
	"net/http"
)

func GetCertificate(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, certFile)
}
