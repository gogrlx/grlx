package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"

	"github.com/gogrlx/grlx/v2/internal/pki"
	"github.com/gogrlx/grlx/v2/internal/rbac"
)

// cohortRegistry is the farmer's loaded cohort registry.
// It is set during farmer startup via SetCohortRegistry.
var cohortRegistry *rbac.Registry

// SetCohortRegistry assigns the global cohort registry used by API handlers.
func SetCohortRegistry(r *rbac.Registry) {
	cohortRegistry = r
}

// CohortListResponse is the response for GET /cohorts.
type CohortListResponse struct {
	Cohorts []CohortSummary `json:"cohorts"`
}

// CohortSummary provides basic info about a named cohort.
type CohortSummary struct {
	Name string          `json:"name"`
	Type rbac.CohortType `json:"type"`
}

// CohortResolveRequest is the request body for POST /cohorts/resolve.
type CohortResolveRequest struct {
	Name string `json:"name"`
}

// CohortResolveResponse is the response for POST /cohorts/resolve.
type CohortResolveResponse struct {
	Name    string   `json:"name"`
	Sprouts []string `json:"sprouts"`
}

// ListCohorts returns all configured cohort names and types.
func ListCohorts(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if cohortRegistry == nil {
		json.NewEncoder(w).Encode(CohortListResponse{Cohorts: []CohortSummary{}})
		return
	}

	names := cohortRegistry.List()
	summaries := make([]CohortSummary, 0, len(names))
	for _, name := range names {
		c, err := cohortRegistry.Get(name)
		if err != nil {
			continue
		}
		summaries = append(summaries, CohortSummary{
			Name: name,
			Type: c.Type,
		})
	}

	json.NewEncoder(w).Encode(CohortListResponse{Cohorts: summaries})
}

// ResolveCohort resolves a cohort name to its member sprout IDs.
func ResolveCohort(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if cohortRegistry == nil {
		http.Error(w, `{"error":"no cohort registry configured"}`, http.StatusServiceUnavailable)
		return
	}

	var req CohortResolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, `{"error":"cohort name is required"}`, http.StatusBadRequest)
		return
	}

	// Get all accepted sprout IDs for resolution
	allKeys := pki.ListNKeysByType()
	allSproutIDs := make([]string, 0, len(allKeys.Accepted.Sprouts))
	for _, km := range allKeys.Accepted.Sprouts {
		allSproutIDs = append(allSproutIDs, km.SproutID)
	}

	members, err := cohortRegistry.Resolve(req.Name, allSproutIDs)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, rbac.ErrCohortNotFound) {
			status = http.StatusNotFound
		}
		errResp := map[string]string{"error": err.Error()}
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(errResp)
		return
	}

	sprouts := make([]string, 0, len(members))
	for id := range members {
		sprouts = append(sprouts, id)
	}

	json.NewEncoder(w).Encode(CohortResolveResponse{
		Name:    req.Name,
		Sprouts: sprouts,
	})
}

// CohortRefreshRequest is the request body for POST /cohorts/refresh.
type CohortRefreshRequest struct {
	Name string `json:"name,omitempty"`
}

// CohortRefreshResponse is the response for POST /cohorts/refresh.
type CohortRefreshResponse struct {
	Refreshed []rbac.RefreshResult `json:"refreshed"`
}

// RefreshCohorts re-evaluates cohort membership against current sprouts.
// If a name is provided in the body, only that cohort is refreshed.
// Otherwise all cohorts are refreshed.
func RefreshCohorts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if cohortRegistry == nil {
		http.Error(w, `{"error":"no cohort registry configured"}`, http.StatusServiceUnavailable)
		return
	}

	var req CohortRefreshRequest
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req)
	}

	allKeys := pki.ListNKeysByType()
	allSproutIDs := make([]string, 0, len(allKeys.Accepted.Sprouts))
	for _, km := range allKeys.Accepted.Sprouts {
		allSproutIDs = append(allSproutIDs, km.SproutID)
	}
	sort.Strings(allSproutIDs)

	if req.Name != "" {
		result, err := cohortRegistry.Refresh(req.Name, allSproutIDs)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, rbac.ErrCohortNotFound) {
				status = http.StatusNotFound
			}
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(CohortRefreshResponse{
			Refreshed: []rbac.RefreshResult{*result},
		})
		return
	}

	results, err := cohortRegistry.RefreshAll(allSproutIDs)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(CohortRefreshResponse{Refreshed: results})
}
