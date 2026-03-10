// Package natsapi provides a NATS-based API for the farmer.
// Authenticated users connect to the NATS bus and send requests to
// grlx.api.<method> subjects. The farmer subscribes to these subjects
// and dispatches to the appropriate handler.
//
// Request/response follows a simple JSON-RPC-like pattern:
//
//	Request:  JSON params (or empty)
//	Response: {"result": ...} or {"error": "..."}
package natsapi

import (
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"

	log "github.com/gogrlx/grlx/v2/internal/log"
)

// handler is a function that processes a NATS API request.
// It receives the raw JSON params and returns a result or error.
type handler func(params json.RawMessage) (any, error)

// response is the envelope returned to the caller.
type response struct {
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

// routes maps subject suffixes (after "grlx.api.") to handlers.
var routes = map[string]handler{
	// Version
	"version": handleVersion,

	// PKI management
	"pki.list":     handlePKIList,
	"pki.accept":   handlePKIAccept,
	"pki.reject":   handlePKIReject,
	"pki.deny":     handlePKIDeny,
	"pki.unaccept": handlePKIUnaccept,
	"pki.delete":   handlePKIDelete,

	// Sprouts
	"sprouts.list": handleSproutsList,
	"sprouts.get":  handleSproutsGet,

	// Test
	"test.ping": handleTestPing,

	// Cmd
	"cmd.run": handleCmdRun,

	// Cook
	"cook": handleCook,

	// Jobs
	"jobs.list":      handleJobsList,
	"jobs.get":       handleJobsGet,
	"jobs.cancel":    handleJobsCancel,
	"jobs.forsprout": handleJobsListForSprout,

	// Props
	"props.getall": handlePropsGetAll,
	"props.get":    handlePropsGet,
	"props.set":    handlePropsSet,
	"props.delete": handlePropsDelete,

	// Cohorts
	"cohorts.list":    handleCohortsList,
	"cohorts.resolve": handleCohortsResolve,

	// Auth
	"auth.whoami": handleAuthWhoAmI,
	"auth.users":  handleAuthListUsers,
}

// Subscribe registers all NATS API handlers on the given connection.
// It subscribes to "grlx.api.>" and dispatches based on subject suffix.
func Subscribe(nc *nats.Conn) error {
	SetNatsConn(nc)

	for method, h := range routes {
		subject := "grlx.api." + method
		handler := h // capture for closure
		_, err := nc.Subscribe(subject, func(msg *nats.Msg) {
			result, err := handler(msg.Data)
			var resp response
			if err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = result
			}
			data, marshalErr := json.Marshal(resp)
			if marshalErr != nil {
				data = []byte(fmt.Sprintf(`{"error":"marshal error: %s"}`, marshalErr.Error()))
			}
			if msg.Reply != "" {
				if pubErr := msg.Respond(data); pubErr != nil {
					log.Errorf("natsapi: failed to respond to %s: %v", msg.Subject, pubErr)
				}
			}
		})
		if err != nil {
			return fmt.Errorf("natsapi: failed to subscribe to %s: %w", subject, err)
		}
		log.Tracef("natsapi: registered handler for %s", subject)
	}

	return nil
}
