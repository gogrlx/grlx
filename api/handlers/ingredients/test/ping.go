package test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	//. "github.com/gogrlx/grlx/v2/config"
	"github.com/gogrlx/grlx/v2/ingredients/test"
	"github.com/gogrlx/grlx/v2/pki"
	. "github.com/gogrlx/grlx/v2/types"
	log "github.com/taigrr/log-socket/log"
)

// TODO: add callback event for when new key is PUT to the server
func HTestPing(w http.ResponseWriter, r *http.Request) {
	var targetAction TargetedAction
	// grab the body of the req
	err := json.NewDecoder(r.Body).Decode(&targetAction)
	if err != nil {
		log.Trace("An invalid ping request was made.")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	jw, _ := json.Marshal(targetAction.Action)
	var ping PingPong
	json.NewDecoder(bytes.NewBuffer(jw)).Decode(&ping)

	// verify our sprout id is valid
	for _, target := range targetAction.Target {
		if !pki.IsValidSproutID(target.SproutID) || strings.Contains(target.SproutID, "_") {
			log.Trace("An invalid Sprout ID was submitted. Ignoring.")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		registered, _ := pki.NKeyExists(target.SproutID, "")
		if !registered {
			var results TargetedResults
			results.Results = nil
			log.Trace("An unknown Sprout was pinged. Ignoring.")
			jw, _ := json.Marshal(results)
			w.WriteHeader(http.StatusNotFound)
			w.Write(jw)
			return
		}
	}

	// check if the id exists in any of the folders
	// if it does, append a counter to the end, and check again
	// if we hit 100 sprouts with the same id, kick back a StatusBadRequest
	var results TargetedResults
	results.Results = make(map[string]interface{})
	var wg sync.WaitGroup
	var m sync.Mutex
	for _, target := range targetAction.Target {
		wg.Add(1)

		go func(target KeyManager) {
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
	jr, err := json.Marshal(results)
	if err != nil {
		log.Error(err)
	}
	w.WriteHeader(http.StatusOK)
	w.Write(jr)
	return
}
