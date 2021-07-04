package api

import (
	"net/http"
)

func GetCertificate(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, certFile)
}

func SubmitNKey(w http.ResponseWriter, r *http.Request) {
	// Post key and save to storage

}
func AcceptNKey(w http.ResponseWriter, r *http.Request) {

	// Auth first as user
	// 404 for key not found
	// 401 for unauthorized
	// 200 for success
}
