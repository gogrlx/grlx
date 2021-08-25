package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	//. "github.com/gogrlx/grlx/config"
	"github.com/gogrlx/grlx/ingredients/cmd"
	"github.com/gogrlx/grlx/pki"
	. "github.com/gogrlx/grlx/types"
	log "github.com/taigrr/log-socket/log"
)

// TODO: add callback event for when new key is PUT to the server
func HCmdRun(w http.ResponseWriter, r *http.Request) {
	var targetAction TargetedAction
	// grab the body of the req
	err := json.NewDecoder(r.Body).Decode(&targetAction)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Trace("An invalid ping request was made.")
		return
	}
	command, ok := targetAction.Action.(CmdRun)
	if !ok {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Trace("An invalid ping request was made.")
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
			var results TargetedResults
			results.Results = nil
			log.Trace("An unknown Sprout was pinged. Ignoring.")
			jw, _ := json.Marshal(results)
			w.WriteHeader(http.StatusNotFound)
			w.Write(jw)
			return
		}
	}

	var results TargetedResults
	var wg sync.WaitGroup
	var m sync.Mutex
	for _, target := range targetAction.Target {
		wg.Add(1)

		go func(target KeyManager) {
			defer wg.Done()
			result, err := cmd.FRun(target, command)
			if err != nil {
				log.Tracef("Error running command on the Sprout: %v", err)
			}
			m.Lock()
			results.Results[target] = result
			m.Unlock()
		}(target)
	}
	wg.Wait()
	jw, _ := json.Marshal(results)
	w.WriteHeader(http.StatusOK)
	w.Write(jw)
	return

}
