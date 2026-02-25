package http

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"testing"
)

type testServer struct{}

func (h *testServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("testData"))
}

func TestDownload(t *testing.T) {
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
	type tCase struct {
		name     string
		src      string
		dst      string
		hash     string
		hashType string
		err      error
		ctx      context.Context
	}
	cases := []tCase{{
		name:     "test",
		src:      fmt.Sprintf("http://localhost:%d/test", port),
		dst:      filepath.Join(td, "dst"),
		hash:     "3a760fae784d30a1b50e304e97a17355",
		err:      nil,
		ctx:      context.Background(),
		hashType: "md5",
	}}
	for _, tc := range cases {
		func(tc tCase) {
			t.Run(tc.name, func(t *testing.T) {
				props := make(map[string]interface{})
				props["hashType"] = tc.hashType
				hf, err := (HTTPFile{}).Parse(tc.name, tc.src, tc.dst, tc.hash, props)
				if !errors.Is(err, tc.err) {
					t.Errorf("want error %v, got %v", tc.err, err)
				}
				err = hf.Download(tc.ctx)
				if !errors.Is(err, tc.err) {
					t.Errorf("want error %v, got %v", tc.err, err)
				}
			})
		}(tc)
	}
}
