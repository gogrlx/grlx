package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gogrlx/grlx/v2/cook"
	"github.com/gogrlx/grlx/v2/pki"
	"github.com/gogrlx/grlx/v2/types"
	nats "github.com/nats-io/nats.go"
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
	sub, err := conn.SubscribeSync(fmt.Sprintf("grlx.farmer.cook.trigger.%s", jid))
	if err != nil {
		log.Errorf("error subscribing to NATS: %v", err)
		return
	}
	go func(jid string, sub *nats.Subscription) {
		defer sub.Unsubscribe()
		msg, err := sub.NextMsg(time.Second * 15)
		if errors.Is(err, nats.ErrTimeout) {
			log.Errorf("timeout waiting for message from NATS on JID `%s`, job execution cancelled", jid)
			return
		} else if err != nil {
			log.Errorf("error receiving message from NATS: %v", err)
			return
		}
		msg.Ack()
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
	}(jid, sub)

	command.JID = jid
	w.WriteHeader(http.StatusOK)
	jr, _ := json.Marshal(command)
	w.Write(jr)
}
