package ingredients

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/gogrlx/grlx/v2/internal/cook"
)

// mockCooker is a minimal RecipeCooker implementation for testing.
type mockCooker struct {
	name    string
	methods []string
}

func (m *mockCooker) Apply(_ context.Context) (cook.Result, error) {
	return cook.Result{}, nil
}

func (m *mockCooker) Test(_ context.Context) (cook.Result, error) {
	return cook.Result{}, nil
}

func (m *mockCooker) Properties() (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockCooker) Parse(id, method string, properties map[string]interface{}) (cook.RecipeCooker, error) {
	return &mockCooker{name: m.name, methods: m.methods}, nil
}

func (m *mockCooker) Methods() (string, []string) {
	return m.name, m.methods
}

func (m *mockCooker) PropertiesForMethod(_ string) (map[string]string, error) {
	return nil, nil
}

func TestPropMapToPropSet(t *testing.T) {
	tests := []struct {
		name    string
		input   map[string]string
		wantErr bool
		wantLen int
	}{
		{
			name:    "valid required string",
			input:   map[string]string{"name": "string,req"},
			wantErr: false,
			wantLen: 1,
		},
		{
			name:    "valid optional string",
			input:   map[string]string{"name": "string,opt"},
			wantErr: false,
			wantLen: 1,
		},
		{
			name:    "valid bool type",
			input:   map[string]string{"flag": "bool,opt"},
			wantErr: false,
			wantLen: 1,
		},
		{
			name:    "valid slice type",
			input:   map[string]string{"items": "[]string,opt"},
			wantErr: false,
			wantLen: 1,
		},
		{
			name:    "empty value",
			input:   map[string]string{"name": ""},
			wantErr: true,
		},
		{
			name:    "invalid type",
			input:   map[string]string{"name": "int,req"},
			wantErr: true,
		},
		{
			name:    "invalid req/opt flag",
			input:   map[string]string{"name": "string,maybe"},
			wantErr: true,
		},
		{
			name:    "too many commas",
			input:   map[string]string{"name": "string,req,extra"},
			wantErr: true,
		},
		{
			name:    "multiple fields",
			input:   map[string]string{"name": "string,req", "age": "string,opt"},
			wantErr: false,
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PropMapToPropSet(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("PropMapToPropSet() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) != tt.wantLen {
				t.Errorf("PropMapToPropSet() returned %d items, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestMethodPropsSetToMapRoundTrip(t *testing.T) {
	original := MethodPropsSet{
		{Key: "name", Type: "string", IsReq: true},
		{Key: "shell", Type: "string", IsReq: false},
	}

	m := original.ToMap()
	if m["name"] != "string,req" {
		t.Errorf("expected 'string,req', got %q", m["name"])
	}
	if m["shell"] != "string,opt" {
		t.Errorf("expected 'string,opt', got %q", m["shell"])
	}

	roundTripped, err := PropMapToPropSet(m)
	if err != nil {
		t.Fatal(err)
	}
	if len(roundTripped) != len(original) {
		t.Errorf("round trip lost items: got %d, want %d", len(roundTripped), len(original))
	}
}

func TestPropMapToPropSetTypeOnly(t *testing.T) {
	// When no req/opt flag is given, IsReq should default to false.
	input := map[string]string{"name": "string"}
	got, err := PropMapToPropSet(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 prop, got %d", len(got))
	}
	if got[0].IsReq {
		t.Error("expected IsReq=false when no flag is given")
	}
	if got[0].Type != "string" {
		t.Errorf("expected type 'string', got %q", got[0].Type)
	}
}

func TestPropMapToPropSetAllTypes(t *testing.T) {
	input := map[string]string{
		"s":  "string,req",
		"ss": "[]string,opt",
		"b":  "bool,req",
	}
	got, err := PropMapToPropSet(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("expected 3 props, got %d", len(got))
	}
}

func TestToMapEmpty(t *testing.T) {
	var empty MethodPropsSet
	m := empty.ToMap()
	if len(m) != 0 {
		t.Errorf("expected empty map, got %v", m)
	}
}

// resetIngMap clears the global ingredient map between tests.
func resetIngMap() {
	ingTex.Lock()
	defer ingTex.Unlock()
	ingMap = make(IngredientMap)
}

func TestRegisterAllMethods(t *testing.T) {
	resetIngMap()
	defer resetIngMap()

	mc := &mockCooker{name: "file", methods: []string{"managed", "absent", "exists"}}
	RegisterAllMethods(mc)

	ingTex.Lock()
	defer ingTex.Unlock()

	fileMap, ok := ingMap[cook.Ingredient("file")]
	if !ok {
		t.Fatal("expected 'file' ingredient to be registered")
	}
	for _, method := range []string{"managed", "absent", "exists"} {
		if _, ok := fileMap[method]; !ok {
			t.Errorf("expected method %q to be registered for 'file'", method)
		}
	}
}

func TestRegisterAllMethodsMultipleIngredients(t *testing.T) {
	resetIngMap()
	defer resetIngMap()

	fileCooker := &mockCooker{name: "file", methods: []string{"managed"}}
	svcCooker := &mockCooker{name: "service", methods: []string{"running", "stopped"}}

	RegisterAllMethods(fileCooker)
	RegisterAllMethods(svcCooker)

	ingTex.Lock()
	defer ingTex.Unlock()

	if _, ok := ingMap[cook.Ingredient("file")]; !ok {
		t.Error("expected 'file' ingredient")
	}
	if _, ok := ingMap[cook.Ingredient("service")]; !ok {
		t.Error("expected 'service' ingredient")
	}
	if len(ingMap[cook.Ingredient("service")]) != 2 {
		t.Errorf("expected 2 methods for 'service', got %d", len(ingMap[cook.Ingredient("service")]))
	}
}

func TestRegisterAllMethodsIdempotent(t *testing.T) {
	resetIngMap()
	defer resetIngMap()

	mc := &mockCooker{name: "cmd", methods: []string{"run"}}
	RegisterAllMethods(mc)
	RegisterAllMethods(mc) // register again — should not panic or duplicate

	ingTex.Lock()
	defer ingTex.Unlock()

	cmdMap := ingMap[cook.Ingredient("cmd")]
	if len(cmdMap) != 1 {
		t.Errorf("expected 1 method, got %d", len(cmdMap))
	}
}

func TestNewRecipeCookerSuccess(t *testing.T) {
	resetIngMap()
	defer resetIngMap()

	mc := &mockCooker{name: "file", methods: []string{"managed", "absent"}}
	RegisterAllMethods(mc)

	cooker, err := NewRecipeCooker("step1", "file", "managed", map[string]interface{}{"name": "/tmp/test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cooker == nil {
		t.Fatal("expected non-nil cooker")
	}
}

func TestNewRecipeCookerUnknownIngredient(t *testing.T) {
	resetIngMap()
	defer resetIngMap()

	_, err := NewRecipeCooker("step1", "nonexistent", "managed", nil)
	if err == nil {
		t.Fatal("expected error for unknown ingredient")
	}
	if err != ErrUnknownIngredient {
		t.Errorf("expected ErrUnknownIngredient, got %v", err)
	}
}

func TestNewRecipeCookerUnknownMethod(t *testing.T) {
	resetIngMap()
	defer resetIngMap()

	mc := &mockCooker{name: "file", methods: []string{"managed"}}
	RegisterAllMethods(mc)

	_, err := NewRecipeCooker("step1", "file", "nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for unknown method")
	}
	if err != ErrUnknownMethod {
		t.Errorf("expected ErrUnknownMethod, got %v", err)
	}
}

func TestIngredientMapString(t *testing.T) {
	resetIngMap()
	defer resetIngMap()

	mc := &mockCooker{name: "test", methods: []string{"run"}}
	RegisterAllMethods(mc)

	ingTex.Lock()
	s := ingMap.String()
	ingTex.Unlock()

	if !strings.Contains(s, "IngredientMap: ") {
		t.Errorf("expected prefix 'IngredientMap: ', got %q", s)
	}
	if !strings.Contains(s, "test") {
		t.Errorf("expected 'test' ingredient in string output, got %q", s)
	}
}

func TestIngredientMapStringEmpty(t *testing.T) {
	m := make(IngredientMap)
	s := m.String()
	if s != "IngredientMap: " {
		t.Errorf("expected 'IngredientMap: ' for empty map, got %q", s)
	}
}

func TestSentinelErrors(t *testing.T) {
	// Verify sentinel errors are distinct and have expected messages.
	errors := []struct {
		err  error
		name string
	}{
		{ErrUnknownIngredient, "unknown ingredient"},
		{ErrUnknownMethod, "unknown method"},
		{ErrNotImplemented, "this feature is not yet implemented"},
		{ErrInvalidMethod, "invalid method"},
		{ErrMissingName, "recipe is missing a name"},
	}
	for _, e := range errors {
		t.Run(e.name, func(t *testing.T) {
			if e.err.Error() != e.name {
				t.Errorf("expected %q, got %q", e.name, e.err.Error())
			}
		})
	}
}

func TestNewRecipeCookerMultipleMethods(t *testing.T) {
	resetIngMap()
	defer resetIngMap()

	mc := &mockCooker{name: "pkg", methods: []string{"installed", "removed", "latest"}}
	RegisterAllMethods(mc)

	for _, method := range []string{"installed", "removed", "latest"} {
		t.Run(method, func(t *testing.T) {
			cooker, err := NewRecipeCooker(cook.StepID(fmt.Sprintf("step-%s", method)), "pkg", method, nil)
			if err != nil {
				t.Fatalf("unexpected error for method %q: %v", method, err)
			}
			if cooker == nil {
				t.Fatalf("expected non-nil cooker for method %q", method)
			}
		})
	}
}
