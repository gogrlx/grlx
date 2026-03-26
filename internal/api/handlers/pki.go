package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	log "github.com/gogrlx/grlx/v2/internal/log"
	"github.com/nats-io/nkeys"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/pki"
)

// PutNKey handles NKey registration from sprouts during PKI bootstrap.
// Sprouts that have not yet been enrolled call this endpoint over HTTPS
// to submit their public NKey. The farmer stores the key in the
// appropriate PKI directory (unaccepted for new sprouts, rejected for
// sprouts presenting a different key than the one already on file).
//
// This is one of the few remaining HTTP endpoints — it cannot move to
// NATS because sprouts need to register before they have NATS credentials.
func PutNKey(w http.ResponseWriter, r *http.Request) {
	var submission pki.KeySubmission
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
		jw, _ := json.Marshal(apitypes.Inline{Success: true})
		w.WriteHeader(http.StatusOK)
		w.Write(jw)
		return
	}
	if !registered {
		log.Trace("A previously unknown NKey was submitted. Saving to Unaccepted.")
		pki.UnacceptNKey(submission.SproutID, submission.NKey)
		jw, _ := json.Marshal(apitypes.Inline{Success: true})
		w.WriteHeader(http.StatusOK)
		w.Write(jw)
		return
	}
	// otherwise, it's registered and doesn't match
	for trailingIndex := 1; trailingIndex < 100; trailingIndex++ {
		registered, matches := pki.NKeyExists(submission.SproutID+"_"+strconv.Itoa(trailingIndex), submission.NKey)
		if registered && matches {
			log.Trace("A previously known NKey was submitted. Ignoring.")
			jw, _ := json.Marshal(apitypes.Inline{Success: true})
			w.WriteHeader(http.StatusOK)
			w.Write(jw)
			return
		}
		if !registered {
			log.Trace("A previously accepted ID is presenting a new NKey. Saving to Rejected.")
			pki.RejectNKey(submission.SproutID+"_"+strconv.Itoa(trailingIndex), submission.NKey)
			jw, _ := json.Marshal(apitypes.Inline{Success: true})
			w.WriteHeader(http.StatusOK)
			w.Write(jw)
			return
		}
	}
	// if there are more than 100 nkeys with the same id,
	// you're probably under attack
	log.Error("There are over 100 keys submitted with the same ID. Ignoring submission.")
	jw, _ := json.Marshal(apitypes.Inline{Success: false})
	w.WriteHeader(http.StatusServiceUnavailable)
	w.Write(jw)
}

// GetCertificate serves the farmer's root CA certificate.
// Sprouts fetch this during bootstrap to establish TLS trust.
func GetCertificate(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, config.RootCA)
}
