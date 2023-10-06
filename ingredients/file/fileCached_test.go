package file

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/gogrlx/grlx/types"
)

func TestCached(t *testing.T) {
	tempDir := t.TempDir()
	tempFile := tempDir + "/testFile"
	_, err := os.Create(tempFile)
	if err != nil {
		t.Error(err)
	}

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
	f := File{
		id:     "test",
		method: "cached",
		params: map[string]interface{}{
			"name":        dest,
			"source":      host,
			"skip_verify": true,
		},
	}
	if err != nil {
		t.Fatalf("failed to register local file provider: %v", err)
	}
	_, err = f.cached(context.Background(), false)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}
