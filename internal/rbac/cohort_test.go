package rbac

import (
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
