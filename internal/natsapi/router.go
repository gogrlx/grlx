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

	"github.com/gogrlx/grlx/v2/internal/audit"
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
	MethodVersion: handleVersion,

	// PKI management
	MethodPKIList:     handlePKIList,
	MethodPKIAccept:   handlePKIAccept,
	MethodPKIReject:   handlePKIReject,
	MethodPKIDeny:     handlePKIDeny,
	MethodPKIUnaccept: handlePKIUnaccept,
	MethodPKIDelete:   handlePKIDelete,

	// Sprouts
	MethodSproutsList: handleSproutsList,
	MethodSproutsGet:  handleSproutsGet,

	// Test
	MethodTestPing: handleTestPing,

	// Cmd
	MethodCmdRun: handleCmdRun,

	// Cook
	MethodCook: handleCook,

	// Jobs
	MethodJobsList:      handleJobsList,
	MethodJobsGet:       handleJobsGet,
	MethodJobsCancel:    handleJobsCancel,
	MethodJobsForSprout: handleJobsListForSprout,

	// Props
	MethodPropsGetAll: handlePropsGetAll,
	MethodPropsGet:    handlePropsGet,
	MethodPropsSet:    handlePropsSet,
	MethodPropsDelete: handlePropsDelete,

	// Cohorts
	MethodCohortsList:    handleCohortsList,
	MethodCohortsGet:     handleCohortsGet,
	MethodCohortsResolve: handleCohortsResolve,
	MethodCohortsRefresh: handleCohortsRefresh,

	// Auth
	MethodAuthWhoAmI:     handleAuthWhoAmI,
	MethodAuthListUsers:  handleAuthListUsers,
	MethodAuthAddUser:    handleAuthAddUser,
	MethodAuthRemoveUser: handleAuthRemoveUser,
	MethodAuthExplain:    handleAuthExplain,

	// Shell (interactive SSH-like sessions)
	MethodShellStart: handleShellStart,

	// Recipes
	MethodRecipesList: handleRecipesList,
	MethodRecipesGet:  handleRecipesGet,

	// Audit
	MethodAuditDates: handleAuditList,
	MethodAuditQuery: handleAuditQuery,
}

// Subscribe registers all NATS API handlers on the given connection.
// It subscribes to "grlx.api.>" and dispatches based on subject suffix.
// Each handler is wrapped with RBAC enforcement middleware that checks
// the caller's token before dispatching.
func Subscribe(nc *nats.Conn) error {
	SetNatsConn(nc)

	for method, h := range routes {
		subject := Subject(method)
		handler := authMiddleware(method, h) // wrap with RBAC enforcement
		action := method                     // capture for audit
		_, err := nc.Subscribe(subject, func(msg *nats.Msg) {
			result, err := handler(msg.Data)

			// Audit log: record actions based on configured audit level.
			if audit.ShouldLog(action) {
				if auditErr := audit.LogAction(action, msg.Data, result, err); auditErr != nil {
					log.Errorf("natsapi: audit log failed for %s: %v", action, auditErr)
				}
			}

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
