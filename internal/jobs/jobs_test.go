package jobs

import (
	"context"
	"testing"

	"github.com/gogrlx/grlx/v2/internal/cook"
)

func TestPropMapToPropSet_Valid(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]string
		count int
	}{
		{
			name:  "string required",
			input: map[string]string{"name": "string,req"},
			count: 1,
		},
		{
			name:  "string optional",
			input: map[string]string{"name": "string,opt"},
			count: 1,
		},
		{
			name:  "bool no qualifier",
			input: map[string]string{"flag": "bool"},
			count: 1,
		},
		{
			name:  "slice string required",
			input: map[string]string{"tags": "[]string,req"},
			count: 1,
		},
		{
			name: "multiple keys",
			input: map[string]string{
				"name": "string,req",
				"age":  "string,opt",
				"flag": "bool",
			},
			count: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			propset, err := PropMapToPropSet(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(propset) != tt.count {
				t.Errorf("expected %d props, got %d", tt.count, len(propset))
			}
		})
	}
}

func TestPropMapToPropSet_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]string
	}{
		{
			name:  "empty value",
			input: map[string]string{"name": ""},
		},
		{
			name:  "too many commas",
			input: map[string]string{"name": "string,req,extra"},
		},
		{
			name:  "invalid qualifier",
			input: map[string]string{"name": "string,badqualifier"},
		},
		{
			name:  "invalid type",
			input: map[string]string{"name": "int"},
		},
		{
			name:  "invalid type with qualifier",
			input: map[string]string{"name": "float,req"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := PropMapToPropSet(tt.input)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestPropMapToPropSet_Required(t *testing.T) {
	propset, err := PropMapToPropSet(map[string]string{"field": "string,req"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(propset) != 1 {
		t.Fatalf("expected 1 prop, got %d", len(propset))
	}
	if !propset[0].IsReq {
		t.Error("expected IsReq=true for 'req' qualifier")
	}
	if propset[0].Key != "field" {
		t.Errorf("expected key 'field', got %q", propset[0].Key)
	}
	if propset[0].Type != "string" {
		t.Errorf("expected type 'string', got %q", propset[0].Type)
	}
}

func TestPropMapToPropSet_Optional(t *testing.T) {
	propset, err := PropMapToPropSet(map[string]string{"field": "bool,opt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if propset[0].IsReq {
		t.Error("expected IsReq=false for 'opt' qualifier")
	}
}

func TestPropMapToPropSet_NoQualifier(t *testing.T) {
	propset, err := PropMapToPropSet(map[string]string{"field": "string"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if propset[0].IsReq {
		t.Error("expected IsReq=false when no qualifier given")
	}
}

func TestPropMapToPropSet_EmptyMap(t *testing.T) {
	propset, err := PropMapToPropSet(map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(propset) != 0 {
		t.Errorf("expected 0 props, got %d", len(propset))
	}
}

func TestMethodPropsSet_ToMap(t *testing.T) {
	propset := MethodPropsSet{
		{Key: "name", Type: "string", IsReq: true},
		{Key: "debug", Type: "bool", IsReq: false},
		{Key: "tags", Type: "[]string", IsReq: true},
	}

	m := propset.ToMap()
	if m["name"] != "string,req" {
		t.Errorf("expected 'string,req', got %q", m["name"])
	}
	if m["debug"] != "bool,opt" {
		t.Errorf("expected 'bool,opt', got %q", m["debug"])
	}
	if m["tags"] != "[]string,req" {
		t.Errorf("expected '[]string,req', got %q", m["tags"])
	}
}

func TestMethodPropsSet_ToMap_Empty(t *testing.T) {
	propset := MethodPropsSet{}
	m := propset.ToMap()
	if len(m) != 0 {
		t.Errorf("expected empty map, got %d entries", len(m))
	}
}

func TestMethodPropsSet_Roundtrip(t *testing.T) {
	original := map[string]string{
		"source": "string,req",
		"dest":   "string,opt",
		"force":  "bool,opt",
	}

	propset, err := PropMapToPropSet(original)
	if err != nil {
		t.Fatalf("PropMapToPropSet: %v", err)
	}

	roundtripped := propset.ToMap()
	for k, v := range original {
		if roundtripped[k] != v {
			t.Errorf("roundtrip mismatch for key %q: expected %q, got %q", k, v, roundtripped[k])
		}
	}
}

// mockCooker implements cook.RecipeCooker for testing registration.
type mockCooker struct {
	name    string
	methods []string
}

func (m *mockCooker) Apply(context.Context) (cook.Result, error) { return cook.Result{}, nil }
func (m *mockCooker) Test(context.Context) (cook.Result, error)  { return cook.Result{}, nil }
func (m *mockCooker) Properties() (map[string]interface{}, error) {
	return nil, nil
}
func (m *mockCooker) Parse(id, method string, properties map[string]interface{}) (cook.RecipeCooker, error) {
	return &mockCooker{name: m.name, methods: m.methods}, nil
}
func (m *mockCooker) Methods() (string, []string) {
	return m.name, m.methods
}
func (m *mockCooker) PropertiesForMethod(method string) (map[string]string, error) {
	return nil, nil
}

func TestRegisterAllMethods(t *testing.T) {
	// Reset ingMap for clean test.
	ingTex.Lock()
	ingMap = make(map[cook.Ingredient]map[string]cook.RecipeCooker)
	ingTex.Unlock()

	cooker := &mockCooker{name: "test.ingredient", methods: []string{"apply", "remove"}}
	RegisterAllMethods(cooker)

	ingTex.Lock()
	defer ingTex.Unlock()

	ing, ok := ingMap[cook.Ingredient("test.ingredient")]
	if !ok {
		t.Fatal("expected ingredient to be registered")
	}
	if _, ok := ing["apply"]; !ok {
		t.Error("expected 'apply' method to be registered")
	}
	if _, ok := ing["remove"]; !ok {
		t.Error("expected 'remove' method to be registered")
	}
}

func TestRegisterAllMethods_MultipleRegistrations(t *testing.T) {
	ingTex.Lock()
	ingMap = make(map[cook.Ingredient]map[string]cook.RecipeCooker)
	ingTex.Unlock()

	cooker1 := &mockCooker{name: "file", methods: []string{"managed"}}
	cooker2 := &mockCooker{name: "file", methods: []string{"absent"}}
	RegisterAllMethods(cooker1)
	RegisterAllMethods(cooker2)

	ingTex.Lock()
	defer ingTex.Unlock()

	ing := ingMap[cook.Ingredient("file")]
	if len(ing) != 2 {
		t.Errorf("expected 2 methods, got %d", len(ing))
	}
}

func TestNewRecipeCooker_Success(t *testing.T) {
	ingTex.Lock()
	ingMap = make(map[cook.Ingredient]map[string]cook.RecipeCooker)
	ingTex.Unlock()

	cooker := &mockCooker{name: "pkg", methods: []string{"installed", "removed"}}
	RegisterAllMethods(cooker)

	result, err := NewRecipeCooker("step-1", "pkg", "installed", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil cooker")
	}
}

func TestNewRecipeCooker_UnknownIngredient(t *testing.T) {
	ingTex.Lock()
	ingMap = make(map[cook.Ingredient]map[string]cook.RecipeCooker)
	ingTex.Unlock()

	_, err := NewRecipeCooker("step-1", "nonexistent", "method", nil)
	if err != ErrUnknownIngredient {
		t.Errorf("expected ErrUnknownIngredient, got %v", err)
	}
}

func TestNewRecipeCooker_UnknownMethod(t *testing.T) {
	ingTex.Lock()
	ingMap = make(map[cook.Ingredient]map[string]cook.RecipeCooker)
	ingTex.Unlock()

	cooker := &mockCooker{name: "service", methods: []string{"running"}}
	RegisterAllMethods(cooker)

	_, err := NewRecipeCooker("step-1", "service", "nonexistent", nil)
	if err != ErrUnknownMethod {
		t.Errorf("expected ErrUnknownMethod, got %v", err)
	}
}
