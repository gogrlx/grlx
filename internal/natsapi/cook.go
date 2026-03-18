package natsapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/auth"
	"github.com/gogrlx/grlx/v2/internal/cook"
	log "github.com/gogrlx/grlx/v2/internal/log"
	"github.com/gogrlx/grlx/v2/internal/pki"
	nats "github.com/nats-io/nats.go"
)

func handleCook(params json.RawMessage) (any, error) {
	var ta apitypes.TargetedAction
	if err := json.Unmarshal(params, &ta); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	var command apitypes.CmdCook
	actionBytes, _ := json.Marshal(ta.Action)
	if err := json.Unmarshal(actionBytes, &command); err != nil {
		return nil, fmt.Errorf("invalid cook command: %w", err)
	}

	for _, target := range ta.Target {
		if !pki.IsValidSproutID(target.SproutID) || strings.Contains(target.SproutID, "_") {
			return nil, fmt.Errorf("invalid sprout ID: %s", target.SproutID)
		}
		registered, _ := pki.NKeyExists(target.SproutID, "")
		if !registered {
			return nil, fmt.Errorf("unknown sprout: %s", target.SproutID)
		}
	}

	if natsConn == nil {
		return nil, fmt.Errorf("NATS connection not available")
	}

	// Resolve the invoker's identity for job attribution.
	var invokerPubkey string
	var tp tokenParams
	if len(params) > 0 {
		json.Unmarshal(params, &tp)
	}
	if tp.Token != "" {
		if pk, _, resolveErr := auth.WhoAmI(tp.Token); resolveErr == nil {
			invokerPubkey = pk
		}
	}

	jid := cook.GenerateJobID()
	sub, err := natsConn.SubscribeSync(fmt.Sprintf("grlx.farmer.cook.trigger.%s", jid))
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe for cook trigger: %w", err)
	}

	go func(jid string, sub *nats.Subscription) {
		defer sub.Unsubscribe()
		msg, err := sub.NextMsg(time.Second * 15)
		if errors.Is(err, nats.ErrTimeout) {
			log.Errorf("timeout waiting for cook trigger on JID %s", jid)
			return
		} else if err != nil {
			log.Errorf("error receiving cook trigger: %v", err)
			return
		}

		sproutIDs := make([]string, len(ta.Target))
		for i, t := range ta.Target {
			sproutIDs[i] = t.SproutID
		}
		replyData, _ := json.Marshal(sproutIDs)
		if err := msg.Respond(replyData); err != nil {
			log.Errorf("error replying to cook trigger: %v", err)
		}

		var wg sync.WaitGroup
		var mu sync.Mutex
		errs := make(map[string]error)
		wg.Add(len(ta.Target))

		for _, target := range ta.Target {
			go func(t pki.KeyManager) {
				defer wg.Done()
				err := cook.SendCookEvent(t.SproutID, command.Recipe, jid, command.Test, cook.WithInvoker(invokerPubkey))
				if err != nil {
					mu.Lock()
					errs[t.SproutID] = err
					mu.Unlock()
				}
			}(target)
		}
		wg.Wait()
		for sproutID, err := range errs {
			log.Errorf("error cooking recipe for %s: %v", sproutID, err)
		}
	}(jid, sub)

	command.JID = jid
	return command, nil
}
