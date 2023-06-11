package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	//. "github.com/gogrlx/grlx/config"

	"github.com/gogrlx/grlx/cook"
	"github.com/gogrlx/grlx/pki"
	"github.com/gogrlx/grlx/types"
	log "github.com/taigrr/log-socket/log"
)

func Cook(w http.ResponseWriter, r *http.Request) {
	// TODO consider using middleware to validate the targets instead of doing it here
	var targetAction types.TargetedAction
	// grab the body of the req
	err := json.NewDecoder(r.Body).Decode(&targetAction)
	if err != nil {
		log.Trace("An invalid request was made.")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	jw, _ := json.Marshal(targetAction.Action)
	var command types.CmdCook
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

	jid := cook.GenerateJobID()

	var wg sync.WaitGroup
	var m sync.Mutex
	errs := make(map[string]error)
	wg.Add(len(targetAction.Target))

	for _, target := range targetAction.Target {
		go func(target types.KeyManager) {
			defer wg.Done()
			err := cook.SendCookEvent(target.SproutID, command.Recipe, jid)
			if err != nil {
				m.Lock()
				errs[target.SproutID] = err
				m.Unlock()
			}
		}(target)
	}
	wg.Wait()
	for sproutID, err := range errs {
		log.Errorf("an error occurred while cooking recipe for %s: %v", sproutID, err)
	}
	command.JID = jid
	command.Errors = errs
	if len(errs) > 0 {
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	jr, _ := json.Marshal(command)
	w.Write(jr)
}
