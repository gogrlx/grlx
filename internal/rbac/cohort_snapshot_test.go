package rbac

import (
	"reflect"
	"testing"
)

func TestRegistryDefinitionOwnership(t *testing.T) {
	for _, access := range []string{"register", "get"} {
		t.Run(access, func(t *testing.T) {
			registry := NewRegistry()
			definitions := []*Cohort{
				{Name: "static", Type: CohortTypeStatic, Members: []string{"sprout"}},
				{Name: "dynamic", Type: CohortTypeDynamic, Match: &DynamicMatch{PropName: "os", PropValue: "linux"}},
				{Name: "compound", Type: CohortTypeCompound, Compound: &CompoundExpr{Operator: OperatorOR, Operands: []string{"static", "dynamic"}}},
			}
			for _, definition := range definitions {
				if err := registry.Register(definition); err != nil {
					t.Fatal(err)
				}
			}
			for _, definition := range definitions {
				expected := Cohort{Name: definition.Name, Type: definition.Type}
				expected.Members = append([]string(nil), definition.Members...)
				if definition.Match != nil {
					match := *definition.Match
					expected.Match = &match
				}
				if definition.Compound != nil {
					compound := *definition.Compound
					compound.Operands = append([]string(nil), compound.Operands...)
					expected.Compound = &compound
				}
				mutable := definition
				if access == "get" {
					var err error
					mutable, err = registry.Get(definition.Name)
					if err != nil {
						t.Fatal(err)
					}
				}
				mutable.Name = "changed"
				mutable.Type = "invalid"
				if len(mutable.Members) > 0 {
					mutable.Members[0] = "changed"
				}
				if mutable.Match != nil {
					mutable.Match.PropName = "changed"
					mutable.Match.PropValue = "changed"
				}
				if mutable.Compound != nil {
					mutable.Compound.Operator = OperatorAND
					mutable.Compound.Operands[0] = "changed"
				}
				actual, err := registry.Get(expected.Name)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(*actual, expected) {
					t.Errorf("registry definition changed through %s: got %#v, want %#v", access, actual, expected)
				}
			}
			members, err := registry.Resolve("compound", nil)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(members, map[string]bool{"sprout": true}) {
				t.Errorf("unexpected resolved members: %v", members)
			}
		})
	}
}
