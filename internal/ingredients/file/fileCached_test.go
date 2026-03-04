package file

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/gogrlx/grlx/v2/internal/config"
	_ "github.com/gogrlx/grlx/v2/internal/ingredients/file/hashers"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/ingredients"
)

func TestCached(t *testing.T) {
	tests := []struct {
		name     string
		params   map[string]interface{}
		expected func(cacheDir string) cook.Result
		error    error
		test     bool
	}{
		{
			name:   "TestCachedMissingSource",
			params: map[string]interface{}{},
			expected: func(_ string) cook.Result {
				return cook.Result{
					Succeeded: false,
					Failed:    true,
					Notes:     []fmt.Stringer{},
				}
			},
			error: ErrMissingSource,
		},
		{
			name: "TestCachedMissingHash",
			params: map[string]interface{}{
				"source": "test",
			},
			expected: func(_ string) cook.Result {
				return cook.Result{
					Succeeded: false,
					Failed:    true,
					Notes:     []fmt.Stringer{},
				}
			},
			error: ErrMissingHash,
		},
		{
			name: "TestSuccesfulCached",
			params: map[string]interface{}{
				"name":        "testName",
				"source":      tempFile,
				"skip_verify": true,
			},
			expected: func(cacheDir string) cook.Result {
				return cook.Result{
					Succeeded: true,
					Failed:    false,
					Notes:     []fmt.Stringer{cook.Snprintf("%s has been cached", filepath.Join(cacheDir, "skip_testName"))},
				}
			},
			error: nil,
		},
		{
			name: "TestSuccesfulCachedTest",
			params: map[string]interface{}{
				"name":        "testName",
				"source":      tempFile,
				"skip_verify": true,
			},
			expected: func(cacheDir string) cook.Result {
				return cook.Result{
					Succeeded: true,
					Failed:    false,
					Notes:     []fmt.Stringer{cook.Snprintf("%s would be cached", filepath.Join(cacheDir, "skip_testName"))},
				}
			},
			error: nil,
			test:  true,
		},
		{
			name: "TestMissingName",
			params: map[string]interface{}{
				"name":        "",
				"source":      tempFile,
				"skip_verify": true,
			},
			expected: func(_ string) cook.Result {
				return cook.Result{
					Succeeded: false,
					Failed:    true,
					Notes:     []fmt.Stringer{},
				}
			},
			error: ingredients.ErrMissingName,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			td := t.TempDir()
			config.CacheDir = td
			defer func() { config.CacheDir = "" }()

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
			compareResults(t, result, test.expected(td))
		})
	}
}

type testServer struct{}

func (h *testServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("testData"))
}

// Caching works well when the file is being downloaded
func TestCachedSkipVerify(t *testing.T) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}

	go func() {
		v := &testServer{}
		http.Serve(listener, v)
	}()
	port := listener.Addr().(*net.TCPAddr).Port
	td := t.TempDir()
	host := fmt.Sprintf("http://localhost:%d/test", port)
	dest := filepath.Join(td, "skip_dst")
	skipped := filepath.Join(td, "skip_skip_dst")
	config.CacheDir = td
	defer func() {
		config.CacheDir = ""
	}()
	expected := cook.Result{
		Succeeded: true,
		Failed:    false,
		Notes:     []fmt.Stringer{cook.Snprintf("%s already exists and skipVerify is true", skipped)},
	}
	f := File{
		id:     "test",
		method: "cached",
		params: map[string]interface{}{
			"name":        dest,
			"source":      host,
			"skip_verify": true,
		},
	}
	_, err = f.cached(context.Background(), false)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	res, _ := f.cached(context.Background(), false)
	compareResults(t, res, expected)
}
