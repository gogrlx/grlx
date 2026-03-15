package rbac

import (
	"errors"
	"fmt"
	"testing"

	"github.com/gogrlx/grlx/v2/internal/props"
)

func TestCohortValidate(t *testing.T) {
	tests := []struct {
		name    string
		cohort  Cohort
		wantErr bool
	}{
		{
			name:    "valid static cohort",
			cohort:  Cohort{Name: "web", Type: CohortTypeStatic, Members: []string{"s1", "s2"}},
			wantErr: false,
		},
		{
			name:    "static cohort with no members is valid",
			cohort:  Cohort{Name: "empty", Type: CohortTypeStatic},
			wantErr: false,
		},
		{
			name:    "missing name",
			cohort:  Cohort{Type: CohortTypeStatic},
			wantErr: true,
		},
		{
			name:    "unknown type",
			cohort:  Cohort{Name: "bad", Type: "bogus"},
			wantErr: true,
		},
		{
			name:    "dynamic cohort missing match",
			cohort:  Cohort{Name: "d", Type: CohortTypeDynamic},
			wantErr: true,
		},
		{
			name: "dynamic cohort missing prop_name",
			cohort: Cohort{Name: "d", Type: CohortTypeDynamic, Match: &DynamicMatch{
				PropValue: "linux",
			}},
			wantErr: true,
		},
		{
			name: "valid dynamic cohort",
			cohort: Cohort{Name: "d", Type: CohortTypeDynamic, Match: &DynamicMatch{
				PropName: "os", PropValue: "linux",
			}},
			wantErr: false,
		},
		{
			name:    "compound cohort missing expr",
			cohort:  Cohort{Name: "c", Type: CohortTypeCompound},
			wantErr: true,
		},
		{
			name: "compound cohort with one operand",
			cohort: Cohort{Name: "c", Type: CohortTypeCompound, Compound: &CompoundExpr{
				Operator: OperatorAND, Operands: []string{"one"},
			}},
			wantErr: true,
		},
		{
			name: "compound cohort with bad operator",
			cohort: Cohort{Name: "c", Type: CohortTypeCompound, Compound: &CompoundExpr{
				Operator: "XOR", Operands: []string{"a", "b"},
			}},
			wantErr: true,
		},
		{
			name: "valid compound cohort",
			cohort: Cohort{Name: "c", Type: CohortTypeCompound, Compound: &CompoundExpr{
				Operator: OperatorOR, Operands: []string{"a", "b"},
			}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cohort.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRegistryRegisterAndGet(t *testing.T) {
	reg := NewRegistry()

	c := &Cohort{Name: "web", Type: CohortTypeStatic, Members: []string{"s1"}}
	if err := reg.Register(c); err != nil {
		t.Fatalf("Register() unexpected error: %v", err)
	}

	got, err := reg.Get("web")
	if err != nil {
		t.Fatalf("Get() unexpected error: %v", err)
	}
	if got.Name != "web" {
		t.Errorf("Get() name = %q, want %q", got.Name, "web")
	}

	_, err = reg.Get("nonexistent")
	if err == nil {
		t.Error("Get() expected error for nonexistent cohort")
	}
}

func TestRegistryList(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(&Cohort{Name: "a", Type: CohortTypeStatic})
	_ = reg.Register(&Cohort{Name: "b", Type: CohortTypeStatic})

	names := reg.List()
	if len(names) != 2 {
		t.Fatalf("List() returned %d names, want 2", len(names))
	}
	nameSet := map[string]bool{}
	for _, n := range names {
		nameSet[n] = true
	}
	if !nameSet["a"] || !nameSet["b"] {
		t.Errorf("List() = %v, want [a, b]", names)
	}
}

func TestResolveStatic(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(&Cohort{
		Name: "web", Type: CohortTypeStatic,
		Members: []string{"sprout-1", "sprout-2", "sprout-3"},
	})

	result, err := reg.Resolve("web", nil)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("Resolve() returned %d, want 3", len(result))
	}
	for _, id := range []string{"sprout-1", "sprout-2", "sprout-3"} {
		if !result[id] {
			t.Errorf("Resolve() missing %q", id)
		}
	}
}

func TestResolveDynamic(t *testing.T) {
	// Set up props for test sprouts.
	setProp := props.SetPropFunc("sprout-linux-1")
	_ = setProp("os", "linux")
	setProp = props.SetPropFunc("sprout-linux-2")
	_ = setProp("os", "linux")
	setProp = props.SetPropFunc("sprout-win-1")
	_ = setProp("os", "windows")

	reg := NewRegistry()
	_ = reg.Register(&Cohort{
		Name: "linux-hosts", Type: CohortTypeDynamic,
		Match: &DynamicMatch{PropName: "os", PropValue: "linux"},
	})

	allSprouts := []string{"sprout-linux-1", "sprout-linux-2", "sprout-win-1"}
	result, err := reg.Resolve("linux-hosts", allSprouts)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("Resolve() returned %d, want 2", len(result))
	}
	if !result["sprout-linux-1"] || !result["sprout-linux-2"] {
		t.Errorf("Resolve() = %v, want sprout-linux-1 and sprout-linux-2", result)
	}
}

func TestResolveDynamicWildcard(t *testing.T) {
	setProp := props.SetPropFunc("sprout-a")
	_ = setProp("role", "web-frontend")
	setProp = props.SetPropFunc("sprout-b")
	_ = setProp("role", "web-backend")
	setProp = props.SetPropFunc("sprout-c")
	_ = setProp("role", "db")

	reg := NewRegistry()
	_ = reg.Register(&Cohort{
		Name: "web-any", Type: CohortTypeDynamic,
		Match: &DynamicMatch{PropName: "role", PropValue: "web-*"},
	})

	allSprouts := []string{"sprout-a", "sprout-b", "sprout-c"}
	result, err := reg.Resolve("web-any", allSprouts)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("Resolve() returned %d, want 2", len(result))
	}
	if !result["sprout-a"] || !result["sprout-b"] {
		t.Errorf("Resolve() = %v, want sprout-a and sprout-b", result)
	}
}

func TestResolveCompoundAND(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(&Cohort{
		Name: "group-a", Type: CohortTypeStatic,
		Members: []string{"s1", "s2", "s3"},
	})
	_ = reg.Register(&Cohort{
		Name: "group-b", Type: CohortTypeStatic,
		Members: []string{"s2", "s3", "s4"},
	})
	_ = reg.Register(&Cohort{
		Name: "both", Type: CohortTypeCompound,
		Compound: &CompoundExpr{
			Operator: OperatorAND,
			Operands: []string{"group-a", "group-b"},
		},
	})

	result, err := reg.Resolve("both", nil)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("Resolve() returned %d, want 2 (s2, s3)", len(result))
	}
	if !result["s2"] || !result["s3"] {
		t.Errorf("Resolve() = %v, want s2 and s3", result)
	}
}

func TestResolveCompoundOR(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(&Cohort{
		Name: "group-a", Type: CohortTypeStatic,
		Members: []string{"s1", "s2"},
	})
	_ = reg.Register(&Cohort{
		Name: "group-b", Type: CohortTypeStatic,
		Members: []string{"s3", "s4"},
	})
	_ = reg.Register(&Cohort{
		Name: "either", Type: CohortTypeCompound,
		Compound: &CompoundExpr{
			Operator: OperatorOR,
			Operands: []string{"group-a", "group-b"},
		},
	})

	result, err := reg.Resolve("either", nil)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if len(result) != 4 {
		t.Fatalf("Resolve() returned %d, want 4", len(result))
	}
}

func TestResolveCompoundEXCEPT(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(&Cohort{
		Name: "all", Type: CohortTypeStatic,
		Members: []string{"s1", "s2", "s3", "s4"},
	})
	_ = reg.Register(&Cohort{
		Name: "excluded", Type: CohortTypeStatic,
		Members: []string{"s2", "s4"},
	})
	_ = reg.Register(&Cohort{
		Name: "remaining", Type: CohortTypeCompound,
		Compound: &CompoundExpr{
			Operator: OperatorEXCEPT,
			Operands: []string{"all", "excluded"},
		},
	})

	result, err := reg.Resolve("remaining", nil)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("Resolve() returned %d, want 2 (s1, s3)", len(result))
	}
	if !result["s1"] || !result["s3"] {
		t.Errorf("Resolve() = %v, want s1 and s3", result)
	}
}

func TestResolveCompoundNested(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(&Cohort{
		Name: "a", Type: CohortTypeStatic,
		Members: []string{"s1", "s2", "s3"},
	})
	_ = reg.Register(&Cohort{
		Name: "b", Type: CohortTypeStatic,
		Members: []string{"s2", "s3", "s4"},
	})
	_ = reg.Register(&Cohort{
		Name: "c", Type: CohortTypeStatic,
		Members: []string{"s3", "s4", "s5"},
	})
	// a AND b = {s2, s3}
	_ = reg.Register(&Cohort{
		Name: "ab", Type: CohortTypeCompound,
		Compound: &CompoundExpr{Operator: OperatorAND, Operands: []string{"a", "b"}},
	})
	// (a AND b) OR c = {s2, s3, s4, s5}
	_ = reg.Register(&Cohort{
		Name: "ab-or-c", Type: CohortTypeCompound,
		Compound: &CompoundExpr{Operator: OperatorOR, Operands: []string{"ab", "c"}},
	})

	result, err := reg.Resolve("ab-or-c", nil)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if len(result) != 4 {
		t.Fatalf("Resolve() returned %d, want 4 (s2,s3,s4,s5)", len(result))
	}
}

func TestResolveCircularReference(t *testing.T) {
	reg := NewRegistry()
	// Create a circular reference: x references y, y references x.
	_ = reg.Register(&Cohort{
		Name: "x", Type: CohortTypeCompound,
		Compound: &CompoundExpr{Operator: OperatorAND, Operands: []string{"y", "y"}},
	})
	_ = reg.Register(&Cohort{
		Name: "y", Type: CohortTypeCompound,
		Compound: &CompoundExpr{Operator: OperatorAND, Operands: []string{"x", "x"}},
	})

	_, err := reg.Resolve("x", nil)
	if err == nil {
		t.Fatal("Resolve() expected circular reference error")
	}
}

func TestResolveNotFound(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Resolve("nope", nil)
	if err == nil {
		t.Fatal("Resolve() expected not found error")
	}
}

func TestMatchesPropValue(t *testing.T) {
	tests := []struct {
		actual   string
		expected string
		want     bool
	}{
		{"linux", "linux", true},
		{"linux", "windows", false},
		{"linux", "*", true},
		{"", "*", false},
		{"web-frontend", "web-*", true},
		{"web-backend", "web-*", true},
		{"db", "web-*", false},
		{"my-web", "*-web", true},
		{"my-db", "*-web", false},
		{"hello-web-world", "*web*", true},
		{"hello-world", "*web*", false},
	}

	for _, tt := range tests {
		t.Run(tt.actual+"_"+tt.expected, func(t *testing.T) {
			got := matchesPropValue(tt.actual, tt.expected)
			if got != tt.want {
				t.Errorf("matchesPropValue(%q, %q) = %v, want %v", tt.actual, tt.expected, got, tt.want)
			}
		})
	}
}

func TestRefreshSingleCohort(t *testing.T) {
	// Set up props for test sprouts.
	setProp := props.SetPropFunc("refresh-s1")
	_ = setProp("env", "prod")
	setProp = props.SetPropFunc("refresh-s2")
	_ = setProp("env", "prod")
	setProp = props.SetPropFunc("refresh-s3")
	_ = setProp("env", "staging")

	reg := NewRegistry()
	_ = reg.Register(&Cohort{
		Name: "prod-hosts", Type: CohortTypeDynamic,
		Match: &DynamicMatch{PropName: "env", PropValue: "prod"},
	})

	allSprouts := []string{"refresh-s1", "refresh-s2", "refresh-s3"}
	result, err := reg.Refresh("prod-hosts", allSprouts)
	if err != nil {
		t.Fatalf("Refresh() error: %v", err)
	}
	if result.Name != "prod-hosts" {
		t.Errorf("Refresh() name = %q, want %q", result.Name, "prod-hosts")
	}
	if len(result.Members) != 2 {
		t.Fatalf("Refresh() returned %d members, want 2", len(result.Members))
	}
	if result.LastRefreshed.IsZero() {
		t.Error("Refresh() LastRefreshed is zero")
	}

	// Verify cache was populated.
	cached, ok := reg.GetCachedMembership("prod-hosts")
	if !ok {
		t.Fatal("GetCachedMembership() returned false after refresh")
	}
	if len(cached.Members) != 2 {
		t.Errorf("cached members = %d, want 2", len(cached.Members))
	}
}

func TestRefreshAllCohorts(t *testing.T) {
	setProp := props.SetPropFunc("rall-s1")
	_ = setProp("os", "linux")
	setProp = props.SetPropFunc("rall-s2")
	_ = setProp("os", "windows")

	reg := NewRegistry()
	_ = reg.Register(&Cohort{
		Name: "static-group", Type: CohortTypeStatic,
		Members: []string{"rall-s1", "rall-s2"},
	})
	_ = reg.Register(&Cohort{
		Name: "linux-only", Type: CohortTypeDynamic,
		Match: &DynamicMatch{PropName: "os", PropValue: "linux"},
	})

	allSprouts := []string{"rall-s1", "rall-s2"}
	results, err := reg.RefreshAll(allSprouts)
	if err != nil {
		t.Fatalf("RefreshAll() error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("RefreshAll() returned %d results, want 2", len(results))
	}

	// Check both cohorts were cached.
	for _, name := range []string{"static-group", "linux-only"} {
		_, ok := reg.GetCachedMembership(name)
		if !ok {
			t.Errorf("GetCachedMembership(%q) returned false after RefreshAll", name)
		}
	}
}

func TestRefreshNotFound(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Refresh("nonexistent", nil)
	if err == nil {
		t.Fatal("Refresh() expected error for nonexistent cohort")
	}
}

func TestRefreshInvalidatesCacheOnRegister(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(&Cohort{
		Name: "group", Type: CohortTypeStatic,
		Members: []string{"s1"},
	})

	// Refresh to populate cache.
	_, err := reg.Refresh("group", nil)
	if err != nil {
		t.Fatalf("Refresh() error: %v", err)
	}

	_, ok := reg.GetCachedMembership("group")
	if !ok {
		t.Fatal("cache should exist after refresh")
	}

	// Re-register with different members — cache should be invalidated.
	_ = reg.Register(&Cohort{
		Name: "group", Type: CohortTypeStatic,
		Members: []string{"s1", "s2"},
	})

	_, ok = reg.GetCachedMembership("group")
	if ok {
		t.Error("cache should be invalidated after re-register")
	}
}

func TestRegisterSelfReference(t *testing.T) {
	reg := NewRegistry()
	err := reg.Register(&Cohort{
		Name: "self", Type: CohortTypeCompound,
		Compound: &CompoundExpr{Operator: OperatorAND, Operands: []string{"self", "other"}},
	})
	if err == nil {
		t.Fatal("Register() expected self-reference error")
	}
	if !errors.Is(err, ErrSelfReference) {
		t.Errorf("Register() error = %v, want ErrSelfReference", err)
	}
}

func TestValidateReferences(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(&Cohort{Name: "a", Type: CohortTypeStatic, Members: []string{"s1"}})
	_ = reg.Register(&Cohort{
		Name: "combo", Type: CohortTypeCompound,
		Compound: &CompoundExpr{Operator: OperatorAND, Operands: []string{"a", "missing"}},
	})

	err := reg.ValidateReferences()
	if err == nil {
		t.Fatal("ValidateReferences() expected error for missing operand")
	}
	if !errors.Is(err, ErrCohortNotFound) {
		t.Errorf("ValidateReferences() error = %v, want ErrCohortNotFound", err)
	}
}

func TestValidateReferencesAllPresent(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(&Cohort{Name: "a", Type: CohortTypeStatic, Members: []string{"s1"}})
	_ = reg.Register(&Cohort{Name: "b", Type: CohortTypeStatic, Members: []string{"s2"}})
	_ = reg.Register(&Cohort{
		Name: "combo", Type: CohortTypeCompound,
		Compound: &CompoundExpr{Operator: OperatorOR, Operands: []string{"a", "b"}},
	})

	err := reg.ValidateReferences()
	if err != nil {
		t.Fatalf("ValidateReferences() unexpected error: %v", err)
	}
}

func TestValidateReferencesCircular(t *testing.T) {
	reg := NewRegistry()
	// Manually set up circular references (bypassing Register's self-ref check).
	reg.cohorts["x"] = &Cohort{
		Name: "x", Type: CohortTypeCompound,
		Compound: &CompoundExpr{Operator: OperatorAND, Operands: []string{"y", "y"}},
	}
	reg.cohorts["y"] = &Cohort{
		Name: "y", Type: CohortTypeCompound,
		Compound: &CompoundExpr{Operator: OperatorAND, Operands: []string{"x", "x"}},
	}

	err := reg.ValidateReferences()
	if err == nil {
		t.Fatal("ValidateReferences() expected circular reference error")
	}
}

func TestResolveMaxDepthExceeded(t *testing.T) {
	reg := NewRegistry()
	// Build a chain deeper than MaxNestingDepth.
	_ = reg.Register(&Cohort{Name: "leaf", Type: CohortTypeStatic, Members: []string{"s1"}})
	prev := "leaf"
	// Need a second leaf for the 2-operand requirement.
	_ = reg.Register(&Cohort{Name: "leaf2", Type: CohortTypeStatic, Members: []string{"s2"}})
	for i := range MaxNestingDepth + 2 {
		name := fmt.Sprintf("level-%d", i)
		reg.cohorts[name] = &Cohort{
			Name: name, Type: CohortTypeCompound,
			Compound: &CompoundExpr{Operator: OperatorOR, Operands: []string{prev, "leaf2"}},
		}
		prev = name
	}

	_, err := reg.Resolve(prev, nil)
	if err == nil {
		t.Fatal("Resolve() expected max depth exceeded error")
	}
	if !errors.Is(err, ErrMaxDepthExceeded) {
		t.Errorf("Resolve() error = %v, want ErrMaxDepthExceeded", err)
	}
}

func TestResolveDeeplyNestedOK(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(&Cohort{Name: "leaf", Type: CohortTypeStatic, Members: []string{"s1"}})
	_ = reg.Register(&Cohort{Name: "leaf2", Type: CohortTypeStatic, Members: []string{"s2"}})
	prev := "leaf"
	// Build a chain at exactly MaxNestingDepth - should still work.
	for i := range MaxNestingDepth - 1 {
		name := fmt.Sprintf("deep-%d", i)
		reg.cohorts[name] = &Cohort{
			Name: name, Type: CohortTypeCompound,
			Compound: &CompoundExpr{Operator: OperatorOR, Operands: []string{prev, "leaf2"}},
		}
		prev = name
	}

	result, err := reg.Resolve(prev, nil)
	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}
	if !result["s1"] || !result["s2"] {
		t.Errorf("Resolve() = %v, want s1 and s2", result)
	}
}

func TestResolveSharedOperandNotCircular(t *testing.T) {
	// Two compound cohorts sharing a common operand should not trigger circular ref.
	reg := NewRegistry()
	_ = reg.Register(&Cohort{Name: "shared", Type: CohortTypeStatic, Members: []string{"s1"}})
	_ = reg.Register(&Cohort{Name: "a", Type: CohortTypeStatic, Members: []string{"s2"}})
	_ = reg.Register(&Cohort{Name: "b", Type: CohortTypeStatic, Members: []string{"s3"}})
	_ = reg.Register(&Cohort{
		Name: "combo-a", Type: CohortTypeCompound,
		Compound: &CompoundExpr{Operator: OperatorOR, Operands: []string{"shared", "a"}},
	})
	_ = reg.Register(&Cohort{
		Name: "combo-b", Type: CohortTypeCompound,
		Compound: &CompoundExpr{Operator: OperatorOR, Operands: []string{"shared", "b"}},
	})
	_ = reg.Register(&Cohort{
		Name: "top", Type: CohortTypeCompound,
		Compound: &CompoundExpr{Operator: OperatorOR, Operands: []string{"combo-a", "combo-b"}},
	})

	result, err := reg.Resolve("top", nil)
	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("Resolve() returned %d, want 3 (s1, s2, s3)", len(result))
	}
}

func TestResolveMultiOperandAND(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(&Cohort{Name: "a", Type: CohortTypeStatic, Members: []string{"s1", "s2", "s3"}})
	_ = reg.Register(&Cohort{Name: "b", Type: CohortTypeStatic, Members: []string{"s1", "s2", "s4"}})
	_ = reg.Register(&Cohort{Name: "c", Type: CohortTypeStatic, Members: []string{"s1", "s3", "s4"}})
	_ = reg.Register(&Cohort{
		Name: "all-three", Type: CohortTypeCompound,
		Compound: &CompoundExpr{Operator: OperatorAND, Operands: []string{"a", "b", "c"}},
	})

	result, err := reg.Resolve("all-three", nil)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if len(result) != 1 || !result["s1"] {
		t.Errorf("Resolve() = %v, want only s1", result)
	}
}
