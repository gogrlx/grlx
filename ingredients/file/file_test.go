package file

import (
	"context"
	"os"
	"testing"

	"github.com/gogrlx/grlx/v2/types"
)

func TestRecipeStepUsage(t *testing.T) {
	var x types.RecipeCooker
	x, err := (File{}).Parse("testFile", "append", map[string]any{
		"name": "testFile",
	})
	if err != nil {
		t.Error(err)
		return
	}
	res, err := x.Apply(context.Background())
	if err != nil {
		t.Error(err)
		return
	}
	if !res.Succeeded {
		t.Errorf("error running %v", x)
	}
	t.Cleanup(func() {
		// remove file
		os.Remove("testFile")
	})
}

func TestDest(t *testing.T) {
	tests := []struct {
		name   string
		params map[string]interface{}
		out    string
		error  error
	}{
		{
			name: "TestMissingName",
			params: map[string]interface{}{
				"name": "",
			},
			out:   "",
			error: types.ErrMissingName,
		},
		{
			name: "TestSkipVerify",
			params: map[string]interface{}{
				"name":        "testFile",
				"skip_verify": true,
			},
			out:   "skip_testFile",
			error: nil,
		},
		{
			name: "TestMissingHash",
			params: map[string]interface{}{
				"name": "testFile",
			},
			out:   "",
			error: types.ErrMissingHash,
		},
		{
			name: "TestMissingHash",
			params: map[string]interface{}{
				"name": "testFile",
				"hash": "testHash",
			},
			out:   "testHash",
			error: nil,
		},
	}
	for _, test := range tests {
		file := File{
			id:     "",
			method: "",
			params: test.params,
		}
		t.Run(test.name, func(t *testing.T) {
			out, err := file.dest()
			if err != test.error {
				t.Errorf("expected error %v, got %v", test.error, err)
			}
			if out != test.out {
				t.Errorf("expected %s, got %s", test.out, out)
			}
		})
	}
}
