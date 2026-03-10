package natsapi

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/ingredients/test"
	"github.com/gogrlx/grlx/v2/internal/pki"
)

func handleTestPing(params json.RawMessage) (any, error) {
	var ta apitypes.TargetedAction
	if err := json.Unmarshal(params, &ta); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	var ping apitypes.PingPong
	actionBytes, _ := json.Marshal(ta.Action)
	json.Unmarshal(actionBytes, &ping)

	for _, target := range ta.Target {
		if !pki.IsValidSproutID(target.SproutID) || strings.Contains(target.SproutID, "_") {
			return nil, fmt.Errorf("invalid sprout ID: %s", target.SproutID)
		}
		registered, _ := pki.NKeyExists(target.SproutID, "")
		if !registered {
			return nil, fmt.Errorf("unknown sprout: %s", target.SproutID)
		}
	}

	results := apitypes.TargetedResults{
		Results: make(map[string]interface{}),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, target := range ta.Target {
		wg.Add(1)
		go func(t pki.KeyManager) {
			defer wg.Done()
			pong, err := test.FPing(t, ping)
			mu.Lock()
			if err != nil {
				results.Results[t.SproutID] = map[string]string{"error": err.Error()}
			} else {
				results.Results[t.SproutID] = pong
			}
			mu.Unlock()
		}(target)
	}
	wg.Wait()

	return results, nil
}
