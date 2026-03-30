package natsapi

import (
	"context"
	"testing"
	"time"

	"github.com/gogrlx/grlx/v2/internal/rbac"
)

func TestStartCohortRefresher_DisabledOnZeroInterval(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Should not panic or block with zero interval.
	done := StartCohortRefresher(ctx, 0)
	<-done
	done = StartCohortRefresher(ctx, -1*time.Second)
	<-done
}

func TestStartCohortRefresher_StopsOnCancel(t *testing.T) {
	old := cohortRegistry
	cohortRegistry = rbac.NewRegistry()

	ctx, cancel := context.WithCancel(context.Background())
	done := StartCohortRefresher(ctx, 50*time.Millisecond)

	// Let it tick a couple of times.
	time.Sleep(150 * time.Millisecond)

	// Cancel and wait for the goroutine to fully exit before restoring.
	cancel()
	<-done
	cohortRegistry = old
}

func TestRefreshAllCohorts_NilRegistry(t *testing.T) {
	old := cohortRegistry
	defer func() { cohortRegistry = old }()
	cohortRegistry = nil

	// Should not panic.
	refreshAllCohorts()
}

func TestRefreshAllCohorts_EmptyRegistry(t *testing.T) {
	old := cohortRegistry
	defer func() { cohortRegistry = old }()
	cohortRegistry = rbac.NewRegistry()

	// Should not panic with an empty registry.
	refreshAllCohorts()
}

func TestRefreshAllCohorts_WithStaticCohort(t *testing.T) {
	old := cohortRegistry
	defer func() { cohortRegistry = old }()

	reg := rbac.NewRegistry()
	err := reg.Register(&rbac.Cohort{
		Name:    "test-static",
		Type:    rbac.CohortTypeStatic,
		Members: []string{"sprout-1", "sprout-2"},
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	cohortRegistry = reg

	// Should refresh without error (PKI will return empty sprout list in test).
	refreshAllCohorts()

	// Verify cache was populated.
	cached, ok := reg.GetCachedMembership("test-static")
	if !ok {
		t.Fatal("expected cached membership after refresh")
	}
	if len(cached.Members) != 2 {
		t.Fatalf("expected 2 cached members, got %d", len(cached.Members))
	}
	if cached.LastRefreshed.IsZero() {
		t.Fatal("expected non-zero LastRefreshed timestamp")
	}
}
