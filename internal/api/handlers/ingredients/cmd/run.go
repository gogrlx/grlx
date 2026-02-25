package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	//. "github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/ingredients/cmd"
	"github.com/gogrlx/grlx/v2/internal/pki"
	"github.com/gogrlx/grlx/v2/internal/types"
	log "github.com/taigrr/log-socket/log"
)

// TODO: add callback event for when new key is PUT to the server
func HCmdRun(w http.ResponseWriter, r *http.Request) {
	var targetAction types.TargetedAction
	// grab the body of the req
	err := json.NewDecoder(r.Body).Decode(&targetAction)
	if err != nil {
		log.Trace("An invalid request was made.")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	jw, _ := json.Marshal(targetAction.Action)
	var command types.CmdRun
	err = json.NewDecoder(bytes.NewBuffer(jw)).Decode(&command)
	if err != nil {
		log.Trace("An invalid request was made.")
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
			var results types.TargetedResults
			results.Results = nil
			log.Trace("An unknown Sprout was pinged. Ignoring.")
			jw, _ := json.Marshal(results)
			w.WriteHeader(http.StatusNotFound)
			w.Write(jw)
			return
		}
	}

	var results types.TargetedResults
	var wg sync.WaitGroup
	var m sync.Mutex
	results.Results = make(map[string]interface{})
	for _, target := range targetAction.Target {
		wg.Add(1)

		go func(target types.KeyManager) {
			defer wg.Done()
			result, err := cmd.FRun(target, command)
			if err != nil {
				log.Tracef("Error running command on the Sprout: %v", err)
			}
			result.Error = err
			m.Lock()
			results.Results[target.SproutID] = result
			m.Unlock()
		}(target)
	}
	wg.Wait()
	jr, _ := json.Marshal(results)
	w.WriteHeader(http.StatusOK)
	w.Write(jr)
}
