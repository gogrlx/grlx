package rbac

import (
	"testing"
	"time"
)

func TestMembershipResultsDoNotMutateCache(t *testing.T) {
	for _, operation := range []string{"refresh", "refresh-all", "cached"} {
		t.Run(operation, func(t *testing.T) {
			registry := NewRegistry()
			if err := registry.Register(&Cohort{Name: "hosts", Type: CohortTypeStatic, Members: []string{"sprout-a", "sprout-b"}}); err != nil {
				t.Fatal(err)
			}
			refreshed, err := registry.Refresh("hosts", nil)
			if err != nil {
				t.Fatal(err)
			}
			timestamp := refreshed.LastRefreshed
			switch operation {
			case "refresh":
				refreshed.Members[0] = "changed"
			case "refresh-all":
				results, err := registry.RefreshAll(nil)
				if err != nil {
					t.Fatal(err)
				}
				timestamp = results[0].LastRefreshed
				results[0].Members[0] = "changed"
			case "cached":
				cached, ok := registry.GetCachedMembership("hosts")
				if !ok {
					t.Fatal("missing cached membership")
				}
				cached.Members[0] = "changed"
				cached.LastRefreshed = time.Time{}
			}
			cached, ok := registry.GetCachedMembership("hosts")
			if !ok {
				t.Fatal("missing cached membership")
			}
			if len(cached.Members) != 2 || cached.Members[0] != "sprout-a" || cached.Members[1] != "sprout-b" {
				t.Errorf("caller mutated cached members: %v", cached.Members)
			}
			if !cached.LastRefreshed.Equal(timestamp) {
				t.Errorf("caller mutated cache timestamp: %v", cached.LastRefreshed)
			}
		})
	}
}

func TestMissingCachedMembership(t *testing.T) {
	cached, ok := NewRegistry().GetCachedMembership("missing")
	if ok || cached != nil {
		t.Fatalf("GetCachedMembership(missing) = %v, %v; want nil, false", cached, ok)
	}
}
