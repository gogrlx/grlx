package client

import (
	"testing"
	"time"
)

func TestGetCohort_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	want := CohortDetail{
		Name:     "web-servers",
		Type:     "static",
		Members:  []string{"web-01", "web-02"},
		Resolved: []string{"web-01", "web-02"},
		Count:    2,
	}
	mockHandler(t, NatsConn, "grlx.api.cohorts.get", want)

	got, err := GetCohort("web-servers")
	if err != nil {
		t.Fatalf("GetCohort: %v", err)
	}
	if got.Name != "web-servers" {
		t.Fatalf("expected name web-servers, got %q", got.Name)
	}
	if got.Count != 2 {
		t.Fatalf("expected count 2, got %d", got.Count)
	}
	if len(got.Resolved) != 2 {
		t.Fatalf("expected 2 resolved sprouts, got %d", len(got.Resolved))
	}
	if got.Type != "static" {
		t.Fatalf("expected type static, got %q", got.Type)
	}
}

func TestGetCohort_CompoundType(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	want := CohortDetail{
		Name: "all-servers",
		Type: "compound",
		Compound: &struct {
			Operator string   `json:"operator"`
			Operands []string `json:"operands"`
		}{
			Operator: "AND",
			Operands: []string{"web-servers", "us-east"},
		},
		Resolved: []string{"web-01"},
		Count:    1,
	}
	mockHandler(t, NatsConn, "grlx.api.cohorts.get", want)

	got, err := GetCohort("all-servers")
	if err != nil {
		t.Fatalf("GetCohort: %v", err)
	}
	if got.Compound == nil {
		t.Fatal("expected compound to be set")
	}
	if got.Compound.Operator != "AND" {
		t.Fatalf("expected AND operator, got %q", got.Compound.Operator)
	}
}

func TestGetCohort_Error(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockErrorHandler(t, NatsConn, "grlx.api.cohorts.get", "cohort not found")

	_, err := GetCohort("missing")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveCohort_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	want := CohortResolveResponse{
		Name:    "web-servers",
		Sprouts: []string{"web-01", "web-02", "web-03"},
	}
	mockHandler(t, NatsConn, "grlx.api.cohorts.resolve", want)

	sprouts, err := ResolveCohort("web-servers")
	if err != nil {
		t.Fatalf("ResolveCohort: %v", err)
	}
	if len(sprouts) != 3 {
		t.Fatalf("expected 3 sprouts, got %d", len(sprouts))
	}
	if sprouts[0] != "web-01" {
		t.Fatalf("expected first sprout web-01, got %q", sprouts[0])
	}
}

func TestResolveCohort_Empty(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	want := CohortResolveResponse{
		Name:    "empty-cohort",
		Sprouts: []string{},
	}
	mockHandler(t, NatsConn, "grlx.api.cohorts.resolve", want)

	_, err := ResolveCohort("empty-cohort")
	if err == nil {
		t.Fatal("expected error for zero sprouts")
	}
}

func TestResolveCohort_NATSError(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockErrorHandler(t, NatsConn, "grlx.api.cohorts.resolve", "access denied")

	_, err := ResolveCohort("restricted")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRefreshCohort_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	want := CohortRefreshResponse{
		Refreshed: []CohortRefreshResult{
			{
				Name:          "web-servers",
				Members:       []string{"web-01", "web-02"},
				LastRefreshed: time.Now().UTC(),
			},
		},
	}
	mockHandler(t, NatsConn, "grlx.api.cohorts.refresh", want)

	got, err := RefreshCohort("web-servers")
	if err != nil {
		t.Fatalf("RefreshCohort: %v", err)
	}
	if len(got.Refreshed) != 1 {
		t.Fatalf("expected 1 refreshed cohort, got %d", len(got.Refreshed))
	}
	if got.Refreshed[0].Name != "web-servers" {
		t.Fatalf("expected name web-servers, got %q", got.Refreshed[0].Name)
	}
}

func TestRefreshAllCohorts_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	want := CohortRefreshResponse{
		Refreshed: []CohortRefreshResult{
			{Name: "web-servers", Members: []string{"web-01"}, LastRefreshed: time.Now().UTC()},
			{Name: "db-servers", Members: []string{"db-01", "db-02"}, LastRefreshed: time.Now().UTC()},
		},
	}
	mockHandler(t, NatsConn, "grlx.api.cohorts.refresh", want)

	got, err := RefreshAllCohorts()
	if err != nil {
		t.Fatalf("RefreshAllCohorts: %v", err)
	}
	if len(got.Refreshed) != 2 {
		t.Fatalf("expected 2 refreshed cohorts, got %d", len(got.Refreshed))
	}
}

func TestRefreshCohort_Error(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockErrorHandler(t, NatsConn, "grlx.api.cohorts.refresh", "refresh failed")

	_, err := RefreshCohort("bad")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateCohorts_Valid(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	want := CohortValidateResponse{
		Valid:   true,
		Cohorts: 3,
	}
	mockHandler(t, NatsConn, "grlx.api.cohorts.validate", want)

	got, err := ValidateCohorts()
	if err != nil {
		t.Fatalf("ValidateCohorts: %v", err)
	}
	if !got.Valid {
		t.Error("expected valid")
	}
	if got.Cohorts != 3 {
		t.Errorf("expected 3 cohorts, got %d", got.Cohorts)
	}
}

func TestValidateCohorts_Invalid(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	want := CohortValidateResponse{
		Valid:   false,
		Errors:  []string{"cohort not found: \"bad\" references unknown operand \"ghost\""},
		Cohorts: 2,
	}
	mockHandler(t, NatsConn, "grlx.api.cohorts.validate", want)

	got, err := ValidateCohorts()
	if err != nil {
		t.Fatalf("ValidateCohorts: %v", err)
	}
	if got.Valid {
		t.Error("expected invalid")
	}
	if len(got.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(got.Errors))
	}
}

func TestValidateCohorts_NATSError(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockErrorHandler(t, NatsConn, "grlx.api.cohorts.validate", "connection failed")

	_, err := ValidateCohorts()
	if err == nil {
		t.Fatal("expected error")
	}
}
