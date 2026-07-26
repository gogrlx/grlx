package cook

import (
	"errors"
	"testing"
)

func step(id StepID, requires ...StepID) Step {
	s := Step{ID: id}
	if len(requires) > 0 {
		s.Requisites = RequisiteSet{{Condition: Require, StepIDs: requires}}
	}
	return s
}

func ids(steps []Step) []StepID {
	out := make([]StepID, len(steps))
	for i, s := range steps {
		out[i] = s.ID
	}
	return out
}

func contains(steps []Step, id StepID) bool {
	for _, s := range steps {
		if s.ID == id {
			return true
		}
	}
	return false
}

func TestPruneToTargetTransitiveClosure(t *testing.T) {
	// c -> b -> a ; d is unrelated.
	steps := []Step{
		step("a"),
		step("b", "a"),
		step("c", "b"),
		step("d"),
	}
	pruned, err := PruneToTarget(steps, "c")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []StepID{"a", "b", "c"} {
		if !contains(pruned, want) {
			t.Errorf("expected pruned set to contain %q, got %v", want, ids(pruned))
		}
	}
	if contains(pruned, "d") {
		t.Errorf("expected unrelated step d to be excluded, got %v", ids(pruned))
	}
}

func TestPruneToTargetPreservesOrder(t *testing.T) {
	steps := []Step{
		step("a"),
		step("b", "a"),
		step("c", "b"),
	}
	pruned, err := PruneToTarget(steps, "c")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := ids(pruned)
	want := []StepID{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order not preserved: expected %v, got %v", want, got)
		}
	}
}

func TestPruneToTargetLeafStep(t *testing.T) {
	steps := []Step{
		step("a"),
		step("b", "a"),
	}
	pruned, err := PruneToTarget(steps, "a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pruned) != 1 || pruned[0].ID != "a" {
		t.Errorf("expected only [a], got %v", ids(pruned))
	}
}

func TestPruneToTargetMultipleRequisites(t *testing.T) {
	// c requires both a and b.
	steps := []Step{
		step("a"),
		step("b"),
		{ID: "c", Requisites: RequisiteSet{{Condition: Require, StepIDs: []StepID{"a", "b"}}}},
		step("d"),
	}
	pruned, err := PruneToTarget(steps, "c")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []StepID{"a", "b", "c"} {
		if !contains(pruned, want) {
			t.Errorf("expected %q in pruned set, got %v", want, ids(pruned))
		}
	}
	if contains(pruned, "d") {
		t.Errorf("expected d excluded, got %v", ids(pruned))
	}
}

func TestPruneToTargetUnknownTarget(t *testing.T) {
	steps := []Step{step("a")}
	_, err := PruneToTarget(steps, "missing")
	if !errors.Is(err, ErrTargetStepNotFound) {
		t.Errorf("expected ErrTargetStepNotFound for unknown target, got %v", err)
	}
}

func TestPruneToTargetDanglingRequisite(t *testing.T) {
	// b requires a step that isn't present.
	steps := []Step{step("b", "ghost")}
	_, err := PruneToTarget(steps, "b")
	if !errors.Is(err, ErrDanglingRequisite) {
		t.Errorf("expected ErrDanglingRequisite for dangling requisite, got %v", err)
	}
}
