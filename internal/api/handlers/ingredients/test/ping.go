package test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/ingredients/test"
	log "github.com/gogrlx/grlx/v2/internal/log"
	"github.com/gogrlx/grlx/v2/internal/pki"
)

// TODO: add callback event for when new key is PUT to the server
func HTestPing(w http.ResponseWriter, r *http.Request) {
	var targetAction apitypes.TargetedAction
	// grab the body of the req
	err := json.NewDecoder(r.Body).Decode(&targetAction)
	if err != nil {
		log.Trace("An invalid ping request was made.")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	jw, err := json.Marshal(targetAction.Action)
	if err != nil {
		log.Trace("An invalid ping request was made.")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var ping apitypes.PingPong
	err = json.NewDecoder(bytes.NewBuffer(jw)).Decode(&ping)
	if err != nil {
		log.Trace("An invalid ping request was made.")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// verify our sprout id is valid
	for _, target := range targetAction.Target {
		if !pki.IsValidSproutID(target.SproutID) || strings.Contains(target.SproutID, "_") {
			log.Trace("An invalid Sprout ID was submitted. Ignoring.")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		registered, _ := pki.NKeyExists(target.SproutID, "")
		if !registered {
			var results apitypes.TargetedResults
			results.Results = nil
			log.Trace("An unknown Sprout was pinged. Ignoring.")
			writeJSONResponse(w, http.StatusNotFound, results)
			return
		}
	}

	// check if the id exists in any of the folders
	// if it does, append a counter to the end, and check again
	// if we hit 100 sprouts with the same id, kick back a StatusBadRequest
	var results apitypes.TargetedResults
	results.Results = make(map[string]interface{})
	var wg sync.WaitGroup
	var m sync.Mutex
	for _, target := range targetAction.Target {
		wg.Add(1)

		go func(target pki.KeyManager) {
			defer wg.Done()
			pong, err := test.FPing(target, ping)
			if err != nil {
				log.Tracef("Error pinging the Sprout: %v", err)
			}
			m.Lock()
			results.Results[target.SproutID] = pong
			m.Unlock()
		}(target)
	}
	wg.Wait()
	writeJSONResponse(w, http.StatusOK, results)
}

func writeJSONResponse(w http.ResponseWriter, status int, value interface{}) {
	response, err := json.Marshal(value)
	if err != nil {
		log.Error(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(response); err != nil {
		log.Error(err)
	}
}
