package file

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/gogrlx/grlx/types"
)

func TestCached(t *testing.T) {
	tests := []struct {
		name     string
		params   map[string]interface{}
		expected types.Result
		error    error
		test     bool
	}{
		{
			name:   "TestCachedMissingSource",
			params: map[string]interface{}{},
			expected: types.Result{
				Succeeded: false,
				Failed:    true,
				Notes:     []fmt.Stringer{},
			},
			error: types.ErrMissingSource,
		},
		{
			name: "TestCachedMissingHash",
			params: map[string]interface{}{
				"source": "test",
			},
			expected: types.Result{
				Succeeded: false,
				Failed:    true,
				Notes:     []fmt.Stringer{},
			},
			error: types.ErrMissingHash,
		},
		{
			name: "TestCachedUnkwnownProtocol",
			params: map[string]interface{}{
				"name":        "testName",
				"source":      "/test",
				"skip_verify": true,
			},
			expected: types.Result{
				Succeeded: false,
				Failed:    true,
				Notes:     []fmt.Stringer{},
			},
			error: errors.Join(ErrUnknownProtocol, errors.New("unknown protocol: file")),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := File{
				id:     "",
				method: "",
				params: test.params,
			}
			result, err := f.cached(context.TODO(), test.test)
			if err != nil || test.error != nil {
				if (err == nil && test.error != nil) || (err != nil && test.error == nil) {
					t.Errorf("expected error %v, got %v", test.error, err)
				} else if err.Error() != test.error.Error() {
					t.Errorf("expected error %v, got %v", test.error, err)
				}
			}
			compareResults(t, result, test.expected)
		})
	}
}

// return types.Result{
// 	Succeeded: false, Failed: true, Notes: notes,
// }, types.ErrMissingSource
