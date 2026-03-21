package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gogrlx/grlx/v2/internal/rbac"
)

func TestListCohortsEmpty(t *testing.T) {
	cohortRegistry = rbac.NewRegistry()
	defer func() { cohortRegistry = nil }()

	req := httptest.NewRequest(http.MethodGet, "/cohorts", nil)
	w := httptest.NewRecorder()
	ListCohorts(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp CohortListResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Cohorts) != 0 {
		t.Fatalf("expected 0 cohorts, got %d", len(resp.Cohorts))
	}
}

func TestListCohortsNilRegistry(t *testing.T) {
	cohortRegistry = nil

	req := httptest.NewRequest(http.MethodGet, "/cohorts", nil)
	w := httptest.NewRecorder()
	ListCohorts(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestListCohortsWithEntries(t *testing.T) {
	registry := rbac.NewRegistry()
	registry.Register(&rbac.Cohort{
		Name:    "webservers",
		Type:    rbac.CohortTypeStatic,
		Members: []string{"sprout-1", "sprout-2"},
	})
	registry.Register(&rbac.Cohort{
		Name: "linux",
		Type: rbac.CohortTypeDynamic,
		Match: &rbac.DynamicMatch{
			PropName:  "os",
			PropValue: "linux",
		},
	})
	cohortRegistry = registry
	defer func() { cohortRegistry = nil }()

	req := httptest.NewRequest(http.MethodGet, "/cohorts", nil)
	w := httptest.NewRecorder()
	ListCohorts(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp CohortListResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Cohorts) != 2 {
		t.Fatalf("expected 2 cohorts, got %d", len(resp.Cohorts))
	}
}

func TestResolveCohortNotFound(t *testing.T) {
	cohortRegistry = rbac.NewRegistry()
	defer func() { cohortRegistry = nil }()

	body, _ := json.Marshal(CohortResolveRequest{Name: "nonexistent"})
	req := httptest.NewRequest(http.MethodPost, "/cohorts/resolve", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	ResolveCohort(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestResolveCohortNilRegistry(t *testing.T) {
	cohortRegistry = nil

	body, _ := json.Marshal(CohortResolveRequest{Name: "test"})
	req := httptest.NewRequest(http.MethodPost, "/cohorts/resolve", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	ResolveCohort(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestResolveCohortEmptyName(t *testing.T) {
	cohortRegistry = rbac.NewRegistry()
	defer func() { cohortRegistry = nil }()

	body, _ := json.Marshal(CohortResolveRequest{Name: ""})
	req := httptest.NewRequest(http.MethodPost, "/cohorts/resolve", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	ResolveCohort(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestResolveCohortInvalidBody(t *testing.T) {
	cohortRegistry = rbac.NewRegistry()
	defer func() { cohortRegistry = nil }()

	req := httptest.NewRequest(http.MethodPost, "/cohorts/resolve", bytes.NewBufferString("not json"))
	w := httptest.NewRecorder()
	ResolveCohort(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSetCohortRegistry(t *testing.T) {
	r := rbac.NewRegistry()
	SetCohortRegistry(r)
	defer func() { cohortRegistry = nil }()

	if cohortRegistry != r {
		t.Error("expected cohortRegistry to be set")
	}
}

func TestRefreshCohorts_NilRegistry(t *testing.T) {
	cohortRegistry = nil

	req := httptest.NewRequest(http.MethodPost, "/cohorts/refresh", nil)
	w := httptest.NewRecorder()
	RefreshCohorts(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestRefreshCohorts_All(t *testing.T) {
	tmpDir := setupTestPKI(t)
	addTestSprout(t, tmpDir, "accepted", "web-1", "UKEY1")
	addTestSprout(t, tmpDir, "accepted", "web-2", "UKEY2")

	registry := rbac.NewRegistry()
	registry.Register(&rbac.Cohort{
		Name:    "webservers",
		Type:    rbac.CohortTypeStatic,
		Members: []string{"web-1", "web-2"},
	})
	cohortRegistry = registry
	defer func() { cohortRegistry = nil }()

	req := httptest.NewRequest(http.MethodPost, "/cohorts/refresh", nil)
	w := httptest.NewRecorder()
	RefreshCohorts(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp CohortRefreshResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Refreshed) != 1 {
		t.Fatalf("expected 1 refresh result, got %d", len(resp.Refreshed))
	}
}

func TestRefreshCohorts_ByName(t *testing.T) {
	tmpDir := setupTestPKI(t)
	addTestSprout(t, tmpDir, "accepted", "db-1", "UKEY3")

	registry := rbac.NewRegistry()
	registry.Register(&rbac.Cohort{
		Name:    "databases",
		Type:    rbac.CohortTypeStatic,
		Members: []string{"db-1"},
	})
	cohortRegistry = registry
	defer func() { cohortRegistry = nil }()

	body, _ := json.Marshal(CohortRefreshRequest{Name: "databases"})
	req := httptest.NewRequest(http.MethodPost, "/cohorts/refresh", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	RefreshCohorts(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRefreshCohorts_NotFound(t *testing.T) {
	cohortRegistry = rbac.NewRegistry()
	defer func() { cohortRegistry = nil }()

	body, _ := json.Marshal(CohortRefreshRequest{Name: "nonexistent"})
	req := httptest.NewRequest(http.MethodPost, "/cohorts/refresh", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	RefreshCohorts(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestResolveCohortStatic(t *testing.T) {
	registry := rbac.NewRegistry()
	registry.Register(&rbac.Cohort{
		Name:    "webservers",
		Type:    rbac.CohortTypeStatic,
		Members: []string{"sprout-1", "sprout-2"},
	})
	cohortRegistry = registry
	defer func() { cohortRegistry = nil }()

	body, _ := json.Marshal(CohortResolveRequest{Name: "webservers"})
	req := httptest.NewRequest(http.MethodPost, "/cohorts/resolve", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	ResolveCohort(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp CohortResolveResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "webservers" {
		t.Fatalf("expected name 'webservers', got %q", resp.Name)
	}
	if len(resp.Sprouts) != 2 {
		t.Fatalf("expected 2 sprouts, got %d", len(resp.Sprouts))
	}

	sproutSet := make(map[string]bool)
	for _, s := range resp.Sprouts {
		sproutSet[s] = true
	}
	if !sproutSet["sprout-1"] || !sproutSet["sprout-2"] {
		t.Fatalf("expected sprout-1 and sprout-2, got %v", resp.Sprouts)
	}
}
