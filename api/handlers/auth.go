package handlers

import (
	"encoding/json"
	"net/http"

	. "github.com/gogrlx/grlx/config"
	"github.com/gogrlx/grlx/pki"
	. "github.com/gogrlx/grlx/types"
)

func GetCertificate(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, RootCA)
}

//TODO: enable client authentication
func AcceptNKey(w http.ResponseWriter, r *http.Request) {
	var km KeyManager
	// Auth first as user
	// 401 for unauthorized
	err := json.NewDecoder(r.Body).Decode(&km)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = pki.AcceptNKey(km.SproutID)
	// 404 for key not found
	if err == ErrSproutIDNotFound {
		w.WriteHeader(http.StatusNotFound)
		jw, _ := json.Marshal(Inline200{Success: false})
		w.Write(jw)
		return
	}
	// 200 for success
	if err == nil {
		w.WriteHeader(http.StatusOK)
		jw, _ := json.Marshal(Inline200{Success: true})
		w.Write(jw)
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
	jw, _ := json.Marshal(Inline200{Success: false})
	w.Write(jw)
	return
}
