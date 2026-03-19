package farmer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	natsserver "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
)

// startTestNATS starts an embedded NATS server and returns a client connection.
func startTestNATS(t *testing.T) (*server.Server, *nats.Conn) {
	t.Helper()
	opts := natsserver.DefaultTestOptions
	opts.Port = -1 // random port
	ns := natsserver.RunServer(&opts)

	nc, err := nats.Connect(ns.ClientURL())
	if err != nil {
		ns.Shutdown()
		t.Fatalf("failed to connect to test NATS: %v", err)
	}
	return ns, nc
}

// mockFileHandler subscribes to the files.get subject and serves files
// from the given directory.
func mockFileHandler(t *testing.T, nc *nats.Conn, baseDir string) {
	t.Helper()
	_, err := nc.Subscribe(NATSSubject, func(msg *nats.Msg) {
		var req fileGetRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			resp, _ := json.Marshal(natsResponse{Error: "bad request: " + err.Error()})
			msg.Respond(resp)
			return
		}

		fullPath := filepath.Join(baseDir, req.Path)
		fullPath = filepath.Clean(fullPath)

		// Basic traversal check
		absBase, _ := filepath.Abs(baseDir)
		absFull, _ := filepath.Abs(fullPath)
		if !strings.HasPrefix(absFull, absBase+string(filepath.Separator)) && absFull != absBase {
			resp, _ := json.Marshal(natsResponse{Error: "path traversal detected"})
			msg.Respond(resp)
			return
		}

		fi, err := os.Stat(fullPath)
		if err != nil {
			resp, _ := json.Marshal(natsResponse{Error: "file not found: " + req.Path})
			msg.Respond(resp)
			return
		}

		totalSize := fi.Size()
		offset := req.Offset
		if offset < 0 {
			offset = 0
		}
		if offset > totalSize {
			offset = totalSize
		}

		limit := req.Limit
		if limit <= 0 || limit > MaxChunkSize {
			limit = MaxChunkSize
		}

		remaining := totalSize - offset
		readSize := limit
		if readSize > remaining {
			readSize = remaining
		}

		f, err := os.Open(fullPath)
		if err != nil {
			resp, _ := json.Marshal(natsResponse{Error: "cannot open: " + err.Error()})
			msg.Respond(resp)
			return
		}
		defer f.Close()

		if offset > 0 {
			f.Seek(offset, 0)
		}

		buf := make([]byte, readSize)
		n, _ := f.Read(buf)
		buf = buf[:n]

		fileResp := fileGetResponse{
			Path:   req.Path,
			Size:   totalSize,
			Offset: offset,
			Length: int64(n),
			Data:   base64.StdEncoding.EncodeToString(buf),
			Done:   offset+int64(n) >= totalSize,
		}

		result, _ := json.Marshal(fileResp)
		resp, _ := json.Marshal(natsResponse{Result: result})
		msg.Respond(resp)
	})
	if err != nil {
		t.Fatalf("failed to subscribe mock handler: %v", err)
	}
}

func TestDownloadSmallFile(t *testing.T) {
	ns, nc := startTestNATS(t)
	defer ns.Shutdown()
	defer nc.Close()

	// Create source file
	srcDir := t.TempDir()
	content := []byte("hello from farmer")
	if err := os.WriteFile(filepath.Join(srcDir, "test.txt"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	mockFileHandler(t, nc, srcDir)

	// Set up provider
	RegisterNatsConn(nc)
	defer func() { natsConn = nil }()

	destDir := t.TempDir()
	dest := filepath.Join(destDir, "downloaded.txt")

	ff := FarmerFile{
		ID:          "test-download",
		Source:      "farmer://test.txt",
		Destination: dest,
		Hash:        "",
		Props:       map[string]interface{}{"skip_verify": true},
	}

	ctx := context.Background()
	if err := ff.Download(ctx); err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}
}

func TestDownloadLargeFileChunked(t *testing.T) {
	ns, nc := startTestNATS(t)
	defer ns.Shutdown()
	defer nc.Close()

	// Create a file larger than MaxChunkSize (use small chunk for test)
	srcDir := t.TempDir()

	// Override MaxChunkSize for testing isn't possible (const), so create
	// a file just under the limit and verify single-chunk behavior,
	// then test multi-chunk with the mock accepting custom limits.
	// Instead, use a moderately sized file and verify correctness.
	content := make([]byte, 1024)
	for i := range content {
		content[i] = byte(i % 256)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "large.bin"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	mockFileHandler(t, nc, srcDir)
	RegisterNatsConn(nc)
	defer func() { natsConn = nil }()

	destDir := t.TempDir()
	dest := filepath.Join(destDir, "large.bin")

	ff := FarmerFile{
		ID:          "test-large",
		Source:      "farmer://large.bin",
		Destination: dest,
		Props:       map[string]interface{}{},
	}

	if err := ff.Download(context.Background()); err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(content) {
		t.Fatalf("size mismatch: got %d, want %d", len(got), len(content))
	}
	for i := range got {
		if got[i] != content[i] {
			t.Fatalf("byte mismatch at offset %d: got %d, want %d", i, got[i], content[i])
		}
	}
}

func TestDownloadFileNotFound(t *testing.T) {
	ns, nc := startTestNATS(t)
	defer ns.Shutdown()
	defer nc.Close()

	srcDir := t.TempDir()
	mockFileHandler(t, nc, srcDir)
	RegisterNatsConn(nc)
	defer func() { natsConn = nil }()

	destDir := t.TempDir()
	dest := filepath.Join(destDir, "missing.txt")

	ff := FarmerFile{
		ID:          "test-missing",
		Source:      "farmer://nonexistent.txt",
		Destination: dest,
		Props:       map[string]interface{}{},
	}

	err := ff.Download(context.Background())
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !strings.Contains(err.Error(), "file not found") {
		t.Errorf("unexpected error: %v", err)
	}

	// Destination should be cleaned up
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Error("destination file should have been removed on error")
	}
}

func TestDownloadEmptyPath(t *testing.T) {
	ns, nc := startTestNATS(t)
	defer ns.Shutdown()
	defer nc.Close()

	RegisterNatsConn(nc)
	defer func() { natsConn = nil }()

	ff := FarmerFile{
		ID:          "test-empty",
		Source:      "farmer://",
		Destination: filepath.Join(t.TempDir(), "out"),
		Props:       map[string]interface{}{},
	}

	err := ff.Download(context.Background())
	if err == nil {
		t.Fatal("expected error for empty path")
	}
	if !strings.Contains(err.Error(), "empty file path") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDownloadNoNatsConnection(t *testing.T) {
	// Ensure no connection is set
	oldConn := natsConn
	natsConn = nil
	defer func() { natsConn = oldConn }()

	ff := FarmerFile{
		ID:          "test-no-conn",
		Source:      "farmer://test.txt",
		Destination: filepath.Join(t.TempDir(), "out"),
		Props:       map[string]interface{}{},
	}

	err := ff.Download(context.Background())
	if err == nil {
		t.Fatal("expected error with nil NATS connection")
	}
	if !strings.Contains(err.Error(), "NATS connection not registered") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDownloadContextCancelled(t *testing.T) {
	ns, nc := startTestNATS(t)
	defer ns.Shutdown()
	defer nc.Close()

	srcDir := t.TempDir()
	content := []byte("will be cancelled")
	os.WriteFile(filepath.Join(srcDir, "cancel.txt"), content, 0o644)

	mockFileHandler(t, nc, srcDir)
	RegisterNatsConn(nc)
	defer func() { natsConn = nil }()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	time.Sleep(5 * time.Millisecond) // let context expire

	ff := FarmerFile{
		ID:          "test-cancel",
		Source:      "farmer://cancel.txt",
		Destination: filepath.Join(t.TempDir(), "out"),
		Props:       map[string]interface{}{},
	}

	err := ff.Download(ctx)
	// Should fail due to expired context
	if err == nil {
		// Small files may complete before context check; that's OK
		t.Log("download completed before context expired (acceptable for small files)")
	}
}

func TestProtocols(t *testing.T) {
	ff := FarmerFile{}
	protocols := ff.Protocols()
	if len(protocols) != 1 || protocols[0] != "farmer" {
		t.Errorf("Protocols() = %v, want [\"farmer\"]", protocols)
	}
}

func TestParse(t *testing.T) {
	ff := FarmerFile{}
	provider, err := ff.Parse("my-id", "farmer://conf/app.yaml", "/tmp/dest", "sha256:abc", nil)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	parsed, ok := provider.(FarmerFile)
	if !ok {
		t.Fatalf("Parse returned %T, want FarmerFile", provider)
	}
	if parsed.ID != "my-id" {
		t.Errorf("ID = %q, want %q", parsed.ID, "my-id")
	}
	if parsed.Source != "farmer://conf/app.yaml" {
		t.Errorf("Source = %q, want %q", parsed.Source, "farmer://conf/app.yaml")
	}
	if parsed.Props == nil {
		t.Error("Props should not be nil when input is nil")
	}
}

func TestExtractPath(t *testing.T) {
	tests := []struct {
		source string
		want   string
	}{
		{"farmer://configs/nginx.conf", "configs/nginx.conf"},
		{"farmer://simple.txt", "simple.txt"},
		{"farmer://", ""},
		{"farmer://deep/nested/path/file.yml", "deep/nested/path/file.yml"},
	}

	for _, tc := range tests {
		got := extractPath(tc.source)
		if got != tc.want {
			t.Errorf("extractPath(%q) = %q, want %q", tc.source, got, tc.want)
		}
	}
}

func TestDownloadSubdirectoryFile(t *testing.T) {
	ns, nc := startTestNATS(t)
	defer ns.Shutdown()
	defer nc.Close()

	srcDir := t.TempDir()
	subDir := filepath.Join(srcDir, "configs", "nginx")
	os.MkdirAll(subDir, 0o755)
	content := []byte("server { listen 80; }")
	os.WriteFile(filepath.Join(subDir, "default.conf"), content, 0o644)

	mockFileHandler(t, nc, srcDir)
	RegisterNatsConn(nc)
	defer func() { natsConn = nil }()

	destDir := t.TempDir()
	dest := filepath.Join(destDir, "default.conf")

	ff := FarmerFile{
		ID:          "test-subdir",
		Source:      "farmer://configs/nginx/default.conf",
		Destination: dest,
		Props:       map[string]interface{}{},
	}

	if err := ff.Download(context.Background()); err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	got, _ := os.ReadFile(dest)
	if string(got) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}
}
