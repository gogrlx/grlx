package shell

import (
	"testing"
	"time"
)

func TestTrackerAddRemove(t *testing.T) {
	tracker := NewTracker()

	info := &SessionInfo{
		SessionID: "sess-001",
		SproutID:  "sprout-a",
		Pubkey:    "UABC123",
		RoleName:  "admin",
		StartedAt: time.Now().UTC(),
	}

	tracker.Add(info)

	if tracker.Active() != 1 {
		t.Fatalf("expected 1 active session, got %d", tracker.Active())
	}

	got := tracker.Get("sess-001")
	if got == nil {
		t.Fatal("expected to find session sess-001")
	}
	if got.SproutID != "sprout-a" {
		t.Fatalf("expected sprout-a, got %s", got.SproutID)
	}

	removed := tracker.Remove("sess-001")
	if removed == nil {
		t.Fatal("expected to remove session sess-001")
	}
	if removed.SessionID != "sess-001" {
		t.Fatalf("expected sess-001, got %s", removed.SessionID)
	}

	if tracker.Active() != 0 {
		t.Fatalf("expected 0 active sessions after remove, got %d", tracker.Active())
	}
}

func TestTrackerRemoveNonexistent(t *testing.T) {
	tracker := NewTracker()

	removed := tracker.Remove("nonexistent")
	if removed != nil {
		t.Fatal("expected nil for nonexistent session")
	}
}

func TestTrackerGetNonexistent(t *testing.T) {
	tracker := NewTracker()

	got := tracker.Get("nonexistent")
	if got != nil {
		t.Fatal("expected nil for nonexistent session")
	}
}

func TestTrackerList(t *testing.T) {
	tracker := NewTracker()

	for i := 0; i < 3; i++ {
		tracker.Add(&SessionInfo{
			SessionID: "sess-" + string(rune('a'+i)),
			SproutID:  "sprout-" + string(rune('a'+i)),
			StartedAt: time.Now().UTC(),
		})
	}

	list := tracker.List()
	if len(list) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(list))
	}
}

func TestTrackerMultipleSessions(t *testing.T) {
	tracker := NewTracker()

	tracker.Add(&SessionInfo{SessionID: "s1", SproutID: "sprout-a", StartedAt: time.Now().UTC()})
	tracker.Add(&SessionInfo{SessionID: "s2", SproutID: "sprout-b", StartedAt: time.Now().UTC()})
	tracker.Add(&SessionInfo{SessionID: "s3", SproutID: "sprout-a", StartedAt: time.Now().UTC()})

	if tracker.Active() != 3 {
		t.Fatalf("expected 3 active, got %d", tracker.Active())
	}

	// Remove middle one.
	tracker.Remove("s2")
	if tracker.Active() != 2 {
		t.Fatalf("expected 2 active after removing s2, got %d", tracker.Active())
	}

	// Remaining should be s1 and s3.
	if tracker.Get("s1") == nil {
		t.Fatal("s1 should still exist")
	}
	if tracker.Get("s3") == nil {
		t.Fatal("s3 should still exist")
	}
	if tracker.Get("s2") != nil {
		t.Fatal("s2 should be gone")
	}
}

func TestTrackerDoubleRemove(t *testing.T) {
	tracker := NewTracker()
	tracker.Add(&SessionInfo{SessionID: "s1", StartedAt: time.Now().UTC()})

	first := tracker.Remove("s1")
	if first == nil {
		t.Fatal("first remove should return info")
	}

	second := tracker.Remove("s1")
	if second != nil {
		t.Fatal("second remove should return nil")
	}
}
