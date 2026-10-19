package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nats-io/nats.go"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/pki"
)

func mockPingTargets(t *testing.T, conn *nats.Conn, ids ...string) {
	t.Helper()

	keys := pki.KeysByType{
		Accepted: pki.KeySet{Sprouts: make([]pki.KeyManager, len(ids))},
	}
	for i, id := range ids {
		keys.Accepted.Sprouts[i] = pki.KeyManager{SproutID: id}
	}

	if _, err := conn.Subscribe("grlx.api.pki.list", func(msg *nats.Msg) {
		natsRespond(msg, keys)
	}); err != nil {
		t.Fatalf("subscribe pki.list: %v", err)
	}
	conn.Flush()
}

func mockPingResults(t *testing.T, conn *nats.Conn, results apitypes.TargetedResults) {
	t.Helper()

	if _, err := conn.Subscribe("grlx.api.test.ping", func(msg *nats.Msg) {
		var request apitypes.TargetedAction
		if err := json.Unmarshal(msg.Data, &request); err != nil {
			t.Errorf("decode ping request: %v", err)
		}
		natsRespond(msg, results)
	}); err != nil {
		t.Fatalf("subscribe test.ping: %v", err)
	}
	conn.Flush()
}

func TestTestPingCommand_TextOutput(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	mockPingTargets(t, conn, "web-01", "db-01")
	mockPingResults(t, conn, apitypes.TargetedResults{
		Results: map[string]interface{}{
			"web-01": map[string]interface{}{"pong": true},
			"db-01":  map[string]interface{}{"pong": false},
		},
	})

	oldMode, oldTarget, oldAll, oldCohort := outputMode, sproutTarget, targetAll, cohortTarget
	defer func() {
		outputMode = oldMode
		sproutTarget = oldTarget
		targetAll = oldAll
		cohortTarget = oldCohort
	}()
	outputMode = "text"
	sproutTarget = "web-01,db-01"
	targetAll = false
	cohortTarget = ""

	out := captureStdout(t, func() {
		testCmdPing.Run(testCmdPing, nil)
	})

	if !strings.Contains(out, `web-01: "pong!"`) {
		t.Fatalf("expected web-01 pong output, got %q", out)
	}
	if !strings.Contains(out, "db-01 is offline!") {
		t.Fatalf("expected db-01 offline output, got %q", out)
	}
}

func TestTestPingCommand_JSONOutput(t *testing.T) {
	conn, cleanup := setupTestNATS(t)
	defer cleanup()

	mockPingTargets(t, conn, "web-01")
	mockPingResults(t, conn, apitypes.TargetedResults{
		Results: map[string]interface{}{
			"web-01": map[string]interface{}{"pong": true},
		},
	})

	oldMode, oldTarget, oldAll, oldCohort := outputMode, sproutTarget, targetAll, cohortTarget
	defer func() {
		outputMode = oldMode
		sproutTarget = oldTarget
		targetAll = oldAll
		cohortTarget = oldCohort
	}()
	outputMode = "json"
	sproutTarget = "web-01"
	targetAll = false
	cohortTarget = ""

	out := captureStdout(t, func() {
		testCmdPing.Run(testCmdPing, nil)
	})

	var got apitypes.TargetedResults
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &got); err != nil {
		t.Fatalf("decode json output: %v\n%s", err, out)
	}
	if _, ok := got.Results["web-01"]; !ok {
		t.Fatalf("expected web-01 result in %#v", got.Results)
	}
}
