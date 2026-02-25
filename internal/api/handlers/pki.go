package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/nats-io/nkeys"
	log "github.com/taigrr/log-socket/log"

	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/pki"
	"github.com/gogrlx/grlx/v2/types"
)

// TODO: add callback event for when new key is PUT to the server
func PutNKey(w http.ResponseWriter, r *http.Request) {
	var submission types.KeySubmission
	// grab the body the req
	err := json.NewDecoder(r.Body).Decode(&submission)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Trace("An invalid NKey request was made. Ignoring.")
		return
	}

	// verify our sprout id is valid
	if !pki.IsValidSproutID(submission.SproutID) || strings.Contains(submission.SproutID, "_") {
		log.Trace("An invalid Sprout ID was submitted. Ignoring.")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// verify it's a valid NKey
	if !nkeys.IsValidPublicUserKey(submission.NKey) {
		log.Trace("An invalid NKey was submitted. Ignoring.")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// save it to the pki folder

	// check if the id exists in any of the folders
	// if it does, append a counter to the end, and check again
	// if we hit 100 sprouts with the same id, kick back a StatusBadRequest

	registered, matches := pki.NKeyExists(submission.SproutID, submission.NKey)
	if registered && matches {
		log.Trace("A previously known NKey was submitted. Ignoring.")
		jw, _ := json.Marshal(types.Inline{Success: true})
		w.WriteHeader(http.StatusOK)
		w.Write(jw)
		return
	}
	if !registered {
		log.Trace("A previously unknown NKey was submitted. Saving to Unaccepted.")
		pki.UnacceptNKey(submission.SproutID, submission.NKey)
		jw, _ := json.Marshal(types.Inline{Success: true})
		w.WriteHeader(http.StatusOK)
		w.Write(jw)
		return
	}
	// otherwise, it's registered and doesn't match
	for trailingIndex := 1; trailingIndex < 100; trailingIndex++ {
		registered, matches := pki.NKeyExists(submission.SproutID+"_"+strconv.Itoa(trailingIndex), submission.NKey)
		if registered && matches {
			log.Trace("A previously known NKey was submitted. Ignoring.")
			jw, _ := json.Marshal(types.Inline{Success: true})
			w.WriteHeader(http.StatusOK)
			w.Write(jw)
			return
		}
		if !registered {
			log.Trace("A previously accepted ID is presenting a new NKey. Saving to Rejected.")
			pki.RejectNKey(submission.SproutID+"_"+strconv.Itoa(trailingIndex), submission.NKey)
			jw, _ := json.Marshal(types.Inline{Success: true})
			w.WriteHeader(http.StatusOK)
			w.Write(jw)
			return
		}
	}
	// if there are more than 100 nkeys with the same id,
	// you're probably under attack
	log.Error("There are over 100 keys submitted with the same ID. Ignoring submission.")
	jw, _ := json.Marshal(types.Inline{Success: false})
	w.WriteHeader(http.StatusServiceUnavailable)
	w.Write(jw)
}

func GetCertificate(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, config.RootCA)
}

// TODO: enable client authentication
func AcceptNKey(w http.ResponseWriter, r *http.Request) {
	var km types.KeyManager
	err := json.NewDecoder(r.Body).Decode(&km)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = pki.AcceptNKey(km.SproutID)
	switch err {
	case types.ErrSproutIDNotFound:
		w.WriteHeader(http.StatusNotFound)
		jw, _ := json.Marshal(types.Inline{Success: false, Error: err})
		w.Write(jw)
		return
	case nil:
		w.WriteHeader(http.StatusOK)
		jw, _ := json.Marshal(types.Inline{Success: true, Error: err})
		w.Write(jw)
		return
	default:
		w.WriteHeader(http.StatusServiceUnavailable)
		jw, _ := json.Marshal(types.Inline{Success: false, Error: err})
		w.Write(jw)
		return
	}
}

func GetNKey(w http.ResponseWriter, r *http.Request) {
	var km types.KeyManager
	// Auth first as user
	// 401 for unauthorized
	err := json.NewDecoder(r.Body).Decode(&km)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	pubKey, err := pki.GetNKey(km.SproutID)
	// 404 for key not found
	switch err {
	case types.ErrSproutIDInvalid:
		w.WriteHeader(http.StatusBadRequest)
		jw, _ := json.Marshal(types.Inline{Success: false, Error: err})
		w.Write(jw)
		return
	case types.ErrSproutIDNotFound:
		w.WriteHeader(http.StatusNotFound)
		jw, _ := json.Marshal(types.Inline{Success: false, Error: err})
		w.Write(jw)
		return
	case nil:
		w.WriteHeader(http.StatusOK)
		jw, _ := json.Marshal(types.KeySubmission{NKey: pubKey, SproutID: km.SproutID})
		w.Write(jw)
		return
	default:
		w.WriteHeader(http.StatusServiceUnavailable)
		jw, _ := json.Marshal(types.Inline{Success: false, Error: err})
		w.Write(jw)
		return
	}
}

func RejectNKey(w http.ResponseWriter, r *http.Request) {
	var km types.KeyManager
	// Auth first as user
	// 401 for unauthorized
	err := json.NewDecoder(r.Body).Decode(&km)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = pki.RejectNKey(km.SproutID, "")
	switch err {
	case types.ErrSproutIDInvalid:
		w.WriteHeader(http.StatusBadRequest)
		jw, _ := json.Marshal(types.Inline{Success: false, Error: err})
		w.Write(jw)
		return
	case types.ErrSproutIDNotFound:
		w.WriteHeader(http.StatusNotFound)
		jw, _ := json.Marshal(types.Inline{Success: false, Error: err})
		w.Write(jw)
		return
	case nil:
		w.WriteHeader(http.StatusOK)
		jw, _ := json.Marshal(types.Inline{Success: true, Error: err})
		w.Write(jw)
		return
	default:
		w.WriteHeader(http.StatusServiceUnavailable)
		jw, _ := json.Marshal(types.Inline{Success: false, Error: err})
		w.Write(jw)
		return
	}
}

func ListNKey(w http.ResponseWriter, _ *http.Request) {
	keyList := pki.ListNKeysByType()
	jw, _ := json.Marshal(keyList)
	w.WriteHeader(http.StatusOK)
	w.Write(jw)
}

func DenyNKey(w http.ResponseWriter, r *http.Request) {
	var km types.KeyManager
	// Auth first as user
	// 401 for unauthorized
	err := json.NewDecoder(r.Body).Decode(&km)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = pki.DenyNKey(km.SproutID)
	switch err {
	case types.ErrSproutIDInvalid:
		w.WriteHeader(http.StatusBadRequest)
		jw, _ := json.Marshal(types.Inline{Success: false, Error: err})
		w.Write(jw)
		return
	case types.ErrSproutIDNotFound:
		w.WriteHeader(http.StatusNotFound)
		jw, _ := json.Marshal(types.Inline{Success: false, Error: err})
		w.Write(jw)
		return
	case nil:
		w.WriteHeader(http.StatusOK)
		jw, _ := json.Marshal(types.Inline{Success: true, Error: err})
		w.Write(jw)
		return
	default:
		w.WriteHeader(http.StatusServiceUnavailable)
		jw, _ := json.Marshal(types.Inline{Success: false, Error: err})
		w.Write(jw)
		return
	}
}

func UnacceptNKey(w http.ResponseWriter, r *http.Request) {
	var km types.KeyManager
	// Auth first as user
	// 401 for unauthorized
	err := json.NewDecoder(r.Body).Decode(&km)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = pki.UnacceptNKey(km.SproutID, "")
	switch err {
	case types.ErrSproutIDInvalid:
		w.WriteHeader(http.StatusBadRequest)
		jw, _ := json.Marshal(types.Inline{Success: false, Error: err})
		w.Write(jw)
		return
	case types.ErrSproutIDNotFound:
		w.WriteHeader(http.StatusNotFound)
		jw, _ := json.Marshal(types.Inline{Success: false, Error: err})
		w.Write(jw)
		return
	case nil:
		w.WriteHeader(http.StatusOK)
		jw, _ := json.Marshal(types.Inline{Success: true, Error: err})
		w.Write(jw)
		return
	default:
		w.WriteHeader(http.StatusServiceUnavailable)
		jw, _ := json.Marshal(types.Inline{Success: false, Error: err})
		w.Write(jw)
		return
	}
}

func DeleteNKey(w http.ResponseWriter, r *http.Request) {
	var km types.KeyManager
	// Auth first as user
	// 401 for unauthorized
	err := json.NewDecoder(r.Body).Decode(&km)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = pki.DeleteNKey(km.SproutID)
	switch err {
	case types.ErrSproutIDInvalid:
		w.WriteHeader(http.StatusBadRequest)
		jw, _ := json.Marshal(types.Inline{Success: false, Error: err})
		w.Write(jw)
		return
	case types.ErrSproutIDNotFound:
		w.WriteHeader(http.StatusNotFound)
		jw, _ := json.Marshal(types.Inline{Success: false, Error: err})
		w.Write(jw)
		return
	case nil:
		w.WriteHeader(http.StatusOK)
		jw, _ := json.Marshal(types.Inline{Success: true, Error: err})
		w.Write(jw)
		return
	default:
		w.WriteHeader(http.StatusServiceUnavailable)
		jw, _ := json.Marshal(types.Inline{Success: false, Error: err})
		w.Write(jw)
		return
	}
}
