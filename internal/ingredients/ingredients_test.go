package ingredients

import (
	"testing"
)

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
