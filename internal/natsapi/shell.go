package natsapi

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/gogrlx/grlx/v2/internal/pki"
	"github.com/gogrlx/grlx/v2/internal/shell"
)

func handleShellStart(params json.RawMessage) (any, error) {
	var req shell.CLIStartRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	if req.SproutID == "" {
		return nil, fmt.Errorf("sprout_id is required")
	}

	// Validate the sprout exists and is accepted.
	if !pki.IsValidSproutID(req.SproutID) || strings.Contains(req.SproutID, "_") {
		return nil, fmt.Errorf("invalid sprout ID: %s", req.SproutID)
	}
	registered, _ := pki.NKeyExists(req.SproutID, "")
	if !registered {
		return nil, fmt.Errorf("unknown sprout: %s", req.SproutID)
	}

	// Generate a unique session ID.
	sessionID := uuid.New().String()

	// Forward the start request to the sprout.
	startReq := shell.StartRequest{
		SessionID: sessionID,
		Cols:      req.Cols,
		Rows:      req.Rows,
		Shell:     req.Shell,
	}
	data, _ := json.Marshal(startReq)
	topic := "grlx.sprouts." + req.SproutID + ".shell.start"

	msg, err := natsConn.Request(topic, data, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("sprout did not respond: %w", err)
	}

	// Check if sprout returned an error.
	var errResp struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(msg.Data, &errResp) == nil && errResp.Error != "" {
		return nil, fmt.Errorf("sprout error: %s", errResp.Error)
	}

	// Return session subjects to the CLI.
	var resp shell.StartResponse
	if err := json.Unmarshal(msg.Data, &resp); err != nil {
		return nil, fmt.Errorf("invalid sprout response: %w", err)
	}

	return resp, nil
}
