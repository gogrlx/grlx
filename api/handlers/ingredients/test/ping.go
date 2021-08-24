package test

import (
	"encoding/json"
	"net/http"
	"strings"

	//. "github.com/gogrlx/grlx/config"
	"github.com/gogrlx/grlx/ingredients/test"
	"github.com/gogrlx/grlx/pki"
	. "github.com/gogrlx/grlx/types"
	log "github.com/taigrr/log-socket/log"
)

// TODO: add callback event for when new key is PUT to the server
func TestPing(w http.ResponseWriter, r *http.Request) {
	var target KeyManager
	// grab the body of the req
	err := json.NewDecoder(r.Body).Decode(&target)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Trace("An invalid ping request was made.")
		return
	}

	// verify our sprout id is valid
	if !pki.IsValidSproutID(target.SproutID) || strings.Contains(target.SproutID, "_") {
		log.Trace("An invalid Sprout ID was submitted. Ignoring.")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// check if the id exists in any of the folders
	// if it does, append a counter to the end, and check again
	// if we hit 100 sprouts with the same id, kick back a StatusBadRequest

	registered, _ := pki.NKeyExists(target.SproutID, "")
	if !registered {
		log.Trace("An unknown Sprout was pinged. Ignoring.")
		jw, _ := json.Marshal(Inline200{Success: false})
		w.WriteHeader(http.StatusNotFound)
		w.Write(jw)
		return
	}
	success, err := test.FPing(target.SproutID)
	if err != nil {
		log.Tracef("Error pinging the Sprout: %v", err)
	}
	jw, _ := json.Marshal(Inline200{Success: success})
	w.WriteHeader(http.StatusOK)
	w.Write(jw)
	return

}
