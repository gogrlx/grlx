// Package natsapi — subjects.go defines NATS subject constants and
// typed request/reply message types for all farmer API endpoints.
//
// All API subjects follow the pattern: grlx.api.<domain>.<action>
// Sprout-facing subjects use: grlx.sprouts.<sproutID>.<domain>.<action>
package natsapi

import (
	"github.com/gogrlx/grlx/v2/internal/audit"
	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/jobs"
	"github.com/gogrlx/grlx/v2/internal/pki"
	"github.com/gogrlx/grlx/v2/internal/shell"
)

// ──────────────────────────────────────────────
// Subject prefix
// ──────────────────────────────────────────────

// SubjectPrefix is the root prefix for all farmer API subjects.
const SubjectPrefix = "grlx.api."

// SproutSubjectPrefix is the root prefix for sprout-facing subjects.
const SproutSubjectPrefix = "grlx.sprouts."

// ──────────────────────────────────────────────
// API method constants (suffix after SubjectPrefix)
// ──────────────────────────────────────────────

const (
	// Version
	MethodVersion = "version"

	// PKI management
	MethodPKIList     = "pki.list"
	MethodPKIAccept   = "pki.accept"
	MethodPKIReject   = "pki.reject"
	MethodPKIDeny     = "pki.deny"
	MethodPKIUnaccept = "pki.unaccept"
	MethodPKIDelete   = "pki.delete"

	// Sprouts
	MethodSproutsList = "sprouts.list"
	MethodSproutsGet  = "sprouts.get"

	// Test
	MethodTestPing = "test.ping"

	// Cmd
	MethodCmdRun = "cmd.run"

	// Cook
	MethodCook = "cook"

	// Jobs
	MethodJobsList      = "jobs.list"
	MethodJobsGet       = "jobs.get"
	MethodJobsDelete    = "jobs.delete"
	MethodJobsCancel    = "jobs.cancel"
	MethodJobsForSprout = "jobs.forsprout"

	// Props
	MethodPropsGetAll = "props.getall"
	MethodPropsGet    = "props.get"
	MethodPropsSet    = "props.set"
	MethodPropsDelete = "props.delete"

	// Cohorts
	MethodCohortsList     = "cohorts.list"
	MethodCohortsGet      = "cohorts.get"
	MethodCohortsResolve  = "cohorts.resolve"
	MethodCohortsRefresh  = "cohorts.refresh"
	MethodCohortsValidate = "cohorts.validate"

	// Auth
	MethodAuthWhoAmI     = "auth.whoami"
	MethodAuthListUsers  = "auth.users"
	MethodAuthAddUser    = "auth.users.add"
	MethodAuthRemoveUser = "auth.users.remove"
	MethodAuthExplain    = "auth.explain"

	// Shell
	MethodShellStart = "shell.start"

	// Recipes
	MethodRecipesList = "recipes.list"
	MethodRecipesGet  = "recipes.get"

	// Audit
	MethodAuditDates = "audit.dates"
	MethodAuditQuery = "audit.query"
)

// Subject returns the full NATS subject for a given API method.
func Subject(method string) string {
	return SubjectPrefix + method
}

// SproutSubject builds a sprout-facing subject:
// grlx.sprouts.<sproutID>.<suffix>
func SproutSubject(sproutID, suffix string) string {
	return SproutSubjectPrefix + sproutID + "." + suffix
}

// ──────────────────────────────────────────────
// Sprout-facing subject suffixes
// ──────────────────────────────────────────────

const (
	// SproutTestPing is the suffix for ping probes to a sprout.
	SproutTestPing = "test.ping"

	// SproutCancel is the suffix for job cancel messages to a sprout.
	SproutCancel = "cancel"

	// SproutShellStart is the suffix for starting a shell session on a sprout.
	SproutShellStart = "shell.start"

	// SproutCookTrigger is the prefix for cook trigger responses.
	// Full subject: grlx.farmer.cook.trigger.<jid>
	SproutCookTriggerPrefix = "grlx.farmer.cook.trigger."
)

// ──────────────────────────────────────────────
// Request types
// ──────────────────────────────────────────────

// PKIRequest identifies a sprout for PKI operations (accept/reject/deny/unaccept/delete).
type PKIRequest = pki.KeyManager

// SproutsGetRequest identifies a sprout to retrieve.
type SproutsGetRequest = pki.KeyManager

// JobsListRequest holds optional parameters for listing jobs.
type JobsListRequest = JobsListParams

// JobsGetRequest identifies a job by JID.
type JobsGetRequest = JobsGetParams

// JobsForSproutRequest identifies a sprout for job listing.
type JobsForSproutRequest = JobsForSproutParams

// PropsRequest holds sprout and property identifiers for props operations.
type PropsRequest = PropsParams

// CohortGetRequest identifies a cohort by name.
type CohortGetRequest = CohortGetParams

// CohortResolveRequest identifies a cohort to resolve.
type CohortResolveRequest = CohortResolveParams

// CohortRefreshRequest optionally identifies a cohort to refresh (empty = all).
type CohortRefreshRequest = CohortRefreshParams

// AuthTokenRequest holds a token for auth operations.
type AuthTokenRequest = AuthParams

// ShellStartRequest is the request to start an interactive shell session.
type ShellStartRequest = shell.CLIStartRequest

// RecipesGetRequest identifies a recipe by name.
type RecipesGetRequest struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// AuditQueryRequest holds query parameters for audit log searches.
type AuditQueryRequest = audit.QueryParams

// ──────────────────────────────────────────────
// Response types
// ──────────────────────────────────────────────

// VersionResponse is the response for the version endpoint.
type VersionResponse = config.Version

// PKIListResponse is the list of all NKeys grouped by state.
type PKIListResponse = pki.KeysByType

// SproutsListResponse wraps the sprout list.
type SproutsListResponse struct {
	Sprouts []SproutInfo `json:"sprouts"`
}

// JobsListResponse is a list of job summaries.
type JobsListResponse = []jobs.JobSummary

// JobsGetResponse is a single job detail.
type JobsGetResponse = jobs.JobSummary

// JobsDeleteResponse confirms a job was deleted from the farmer-side store.
type JobsDeleteResponse struct {
	JID     string `json:"jid"`
	Message string `json:"message"`
}

// JobsCancelResponse confirms a cancel request was sent.
type JobsCancelResponse struct {
	JID     string `json:"jid"`
	Sprout  string `json:"sprout"`
	Message string `json:"message"`
}

// PropsGetAllResponse is a map of all properties for a sprout.
type PropsGetAllResponse = map[string]interface{}

// PropsGetResponse is a single property value.
type PropsGetResponse struct {
	SproutID string `json:"sprout_id"`
	Name     string `json:"name"`
	Value    string `json:"value"`
}

// PropsSuccessResponse indicates a successful set/delete operation.
type PropsSuccessResponse struct {
	Success bool `json:"success"`
}

// CohortsListResponse wraps the cohort summary list.
type CohortsListResponse struct {
	Cohorts []CohortSummary `json:"cohorts"`
}

// CohortsGetResponse is the full detail of a single cohort.
type CohortsGetResponse = CohortDetail

// CohortsResolveResponse lists the resolved sprout members of a cohort.
type CohortsResolveResponse struct {
	Name    string   `json:"name"`
	Sprouts []string `json:"sprouts"`
}

// CohortsRefreshResponse wraps the results of a refresh operation.
type CohortsRefreshResponse = CohortRefreshResponse

// CohortsValidateResponse describes whether all cohort references are valid.
type CohortsValidateResponse = CohortValidateResponse

// ShellStartResponse contains session subjects for the CLI to use.
type ShellStartResponse = shell.StartResponse

// RecipesListResponse wraps the recipe list.
type RecipesListResponse struct {
	Recipes []RecipeInfo `json:"recipes"`
}

// RecipesGetResponse is the full content of a recipe.
type RecipesGetResponse = RecipeContent

// AuditDatesResponse is a list of dates with audit entries.
type AuditDatesResponse = []string

// AuditQueryResponse is the result of an audit log query.
type AuditQueryResponse = audit.QueryResult

// ──────────────────────────────────────────────
// Generic response envelope
// ──────────────────────────────────────────────

// Response is the standard envelope for all NATS API responses.
// Handlers return either a Result or an Error, never both.
type Response = response

// AllMethods returns all registered API method constants.
// Useful for documentation generation and client code generation.
func AllMethods() []string {
	return []string{
		MethodVersion,
		MethodPKIList, MethodPKIAccept, MethodPKIReject,
		MethodPKIDeny, MethodPKIUnaccept, MethodPKIDelete,
		MethodSproutsList, MethodSproutsGet,
		MethodTestPing,
		MethodCmdRun,
		MethodCook,
		MethodJobsList, MethodJobsGet, MethodJobsDelete, MethodJobsCancel, MethodJobsForSprout,
		MethodPropsGetAll, MethodPropsGet, MethodPropsSet, MethodPropsDelete,
		MethodCohortsList, MethodCohortsGet, MethodCohortsResolve, MethodCohortsRefresh, MethodCohortsValidate,
		MethodAuthWhoAmI, MethodAuthListUsers, MethodAuthAddUser, MethodAuthRemoveUser, MethodAuthExplain,
		MethodShellStart,
		MethodRecipesList, MethodRecipesGet,
		MethodAuditDates, MethodAuditQuery,
	}
}
