package cmd

import (
	"sort"
	"testing"

	"github.com/gogrlx/grlx/v2/internal/api/client"
)

func sampleSprouts() []client.SproutInfo {
	return []client.SproutInfo{
		{ID: "alpha", KeyState: "accepted", Connected: true},
		{ID: "bravo", KeyState: "accepted", Connected: false},
		{ID: "charlie", KeyState: "unaccepted", Connected: true},
		{ID: "delta", KeyState: "denied", Connected: false},
	}
}

func acceptedOnline(sprouts []client.SproutInfo) (accepted, online int) {
	for _, s := range sprouts {
		if s.KeyState == "accepted" {
			accepted++
			if s.Connected {
				online++
			}
		}
	}
	return accepted, online
}

// TestFilterSproutsDoesNotCorruptSummary reproduces the aliasing bug: filtering
// with sprouts[:0] followed by a sort permuted the original slice, corrupting
// the accepted/online summary computed over it.
func TestFilterSproutsDoesNotCorruptSummary(t *testing.T) {
	cases := []struct {
		name        string
		cohort      map[string]bool
		stateFilter string
		onlineOnly  bool
	}{
		{name: "no filter"},
		{name: "state accepted", stateFilter: "accepted"},
		{name: "state unaccepted", stateFilter: "unaccepted"},
		{name: "online only", onlineOnly: true},
		{name: "state+online", stateFilter: "accepted", onlineOnly: true},
		{name: "cohort", cohort: map[string]bool{"alpha": true, "delta": true}},
		{name: "cohort+online", cohort: map[string]bool{"alpha": true, "delta": true}, onlineOnly: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sprouts := sampleSprouts()
			filtered := filterSprouts(sprouts, tc.cohort, tc.stateFilter, tc.onlineOnly)

			// Sorting the filtered result must not disturb the original.
			sort.Slice(filtered, func(i, j int) bool {
				if filtered[i].Connected != filtered[j].Connected {
					return filtered[i].Connected
				}
				return filtered[i].ID < filtered[j].ID
			})

			accepted, online := acceptedOnline(sprouts)
			if accepted != 2 || online != 1 {
				t.Fatalf("summary corrupted: got %d accepted, %d online; want 2 accepted, 1 online", accepted, online)
			}
		})
	}
}

func TestFilterSprouts(t *testing.T) {
	sprouts := sampleSprouts()

	if got := filterSprouts(sprouts, nil, "accepted", false); len(got) != 2 {
		t.Errorf("state filter: got %d, want 2", len(got))
	}
	if got := filterSprouts(sprouts, nil, "", true); len(got) != 2 {
		t.Errorf("online filter: got %d, want 2", len(got))
	}
	if got := filterSprouts(sprouts, nil, "accepted", true); len(got) != 1 {
		t.Errorf("state+online filter: got %d, want 1", len(got))
	}
	cohort := map[string]bool{"alpha": true, "charlie": true}
	if got := filterSprouts(sprouts, cohort, "", false); len(got) != 2 {
		t.Errorf("cohort filter: got %d, want 2", len(got))
	}
	if got := filterSprouts(sprouts, cohort, "accepted", false); len(got) != 1 {
		t.Errorf("cohort+state filter: got %d, want 1", len(got))
	}
}
