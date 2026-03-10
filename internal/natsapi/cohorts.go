package natsapi

import (
	"encoding/json"
	"fmt"

	"github.com/gogrlx/grlx/v2/internal/pki"
	"github.com/gogrlx/grlx/v2/internal/rbac"
)

var cohortRegistry *rbac.Registry

// SetCohortRegistry assigns the cohort registry for NATS API handlers.
func SetCohortRegistry(r *rbac.Registry) {
	cohortRegistry = r
}

// CohortSummary provides basic info about a named cohort.
type CohortSummary struct {
	Name string          `json:"name"`
	Type rbac.CohortType `json:"type"`
}

// CohortResolveParams is the request for resolving a cohort.
type CohortResolveParams struct {
	Name string `json:"name"`
}

func handleCohortsList(_ json.RawMessage) (any, error) {
	if cohortRegistry == nil {
		return map[string][]CohortSummary{"cohorts": {}}, nil
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

	return map[string][]CohortSummary{"cohorts": summaries}, nil
}

func handleCohortsResolve(params json.RawMessage) (any, error) {
	if cohortRegistry == nil {
		return nil, fmt.Errorf("no cohort registry configured")
	}

	var p CohortResolveParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("cohort name is required")
	}

	allKeys := pki.ListNKeysByType()
	allSproutIDs := make([]string, 0, len(allKeys.Accepted.Sprouts))
	for _, km := range allKeys.Accepted.Sprouts {
		allSproutIDs = append(allSproutIDs, km.SproutID)
	}

	members, err := cohortRegistry.Resolve(p.Name, allSproutIDs)
	if err != nil {
		return nil, err
	}

	sprouts := make([]string, 0, len(members))
	for id := range members {
		sprouts = append(sprouts, id)
	}

	return map[string]any{
		"name":    p.Name,
		"sprouts": sprouts,
	}, nil
}
