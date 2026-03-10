package natsapi

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/ingredients/cmd"
	"github.com/gogrlx/grlx/v2/internal/pki"
)

func handleCmdRun(params json.RawMessage) (any, error) {
	var ta apitypes.TargetedAction
	if err := json.Unmarshal(params, &ta); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	var command apitypes.CmdRun
	actionBytes, _ := json.Marshal(ta.Action)
	if err := json.Unmarshal(actionBytes, &command); err != nil {
		return nil, fmt.Errorf("invalid command: %w", err)
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

	results := apitypes.TargetedResults{
		Results: make(map[string]interface{}),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, target := range ta.Target {
		wg.Add(1)
		go func(t pki.KeyManager) {
			defer wg.Done()
			result, err := cmd.FRun(t, command)
			if err != nil {
				result.Error = err
			}
			mu.Lock()
			results.Results[t.SproutID] = result
			mu.Unlock()
		}(target)
	}
	wg.Wait()

	return results, nil
}
