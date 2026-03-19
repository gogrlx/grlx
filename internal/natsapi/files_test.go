package natsapi

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gogrlx/grlx/v2/internal/config"
)

func setupTestRecipeDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	config.RecipeDir = dir
	return dir
}

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHandleFilesGet_BasicFile(t *testing.T) {
	dir := setupTestRecipeDir(t)
	writeTestFile(t, dir, "test.txt", "hello world")

	params, _ := json.Marshal(FileGetRequest{Path: "test.txt"})
	result, err := handleFilesGet(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, ok := result.(FileGetResponse)
	if !ok {
		t.Fatalf("unexpected result type: %T", result)
	}

	if resp.Path != "test.txt" {
		t.Errorf("path = %q, want %q", resp.Path, "test.txt")
	}
	if resp.Size != 11 {
		t.Errorf("size = %d, want 11", resp.Size)
	}
	if resp.Offset != 0 {
		t.Errorf("offset = %d, want 0", resp.Offset)
	}
	if resp.Length != 11 {
		t.Errorf("length = %d, want 11", resp.Length)
	}
	if !resp.Done {
		t.Error("expected done=true for small file")
	}

	decoded, err := base64.StdEncoding.DecodeString(resp.Data)
	if err != nil {
		t.Fatalf("failed to decode base64: %v", err)
	}
	if string(decoded) != "hello world" {
		t.Errorf("data = %q, want %q", string(decoded), "hello world")
	}
}

func TestHandleFilesGet_NestedPath(t *testing.T) {
	dir := setupTestRecipeDir(t)
	writeTestFile(t, dir, "subdir/nested.txt", "nested content")

	params, _ := json.Marshal(FileGetRequest{Path: "subdir/nested.txt"})
	result, err := handleFilesGet(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := result.(FileGetResponse)
	decoded, _ := base64.StdEncoding.DecodeString(resp.Data)
	if string(decoded) != "nested content" {
		t.Errorf("data = %q, want %q", string(decoded), "nested content")
	}
	if !resp.Done {
		t.Error("expected done=true")
	}
}

func TestHandleFilesGet_Chunking(t *testing.T) {
	dir := setupTestRecipeDir(t)

	// Create a file larger than one chunk would allow with a small limit.
	content := make([]byte, 100)
	for i := range content {
		content[i] = byte('A' + (i % 26))
	}
	writeTestFile(t, dir, "large.bin", string(content))

	// Request first 30 bytes.
	params, _ := json.Marshal(FileGetRequest{Path: "large.bin", Offset: 0, Limit: 30})
	result, err := handleFilesGet(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := result.(FileGetResponse)
	if resp.Size != 100 {
		t.Errorf("size = %d, want 100", resp.Size)
	}
	if resp.Offset != 0 {
		t.Errorf("offset = %d, want 0", resp.Offset)
	}
	if resp.Length != 30 {
		t.Errorf("length = %d, want 30", resp.Length)
	}
	if resp.Done {
		t.Error("expected done=false for partial read")
	}

	decoded, _ := base64.StdEncoding.DecodeString(resp.Data)
	if string(decoded) != string(content[:30]) {
		t.Errorf("chunk 1 data mismatch")
	}

	// Request next chunk from offset 30.
	params, _ = json.Marshal(FileGetRequest{Path: "large.bin", Offset: 30, Limit: 30})
	result, err = handleFilesGet(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp = result.(FileGetResponse)
	if resp.Offset != 30 {
		t.Errorf("offset = %d, want 30", resp.Offset)
	}
	if resp.Length != 30 {
		t.Errorf("length = %d, want 30", resp.Length)
	}
	if resp.Done {
		t.Error("expected done=false for partial read")
	}

	decoded, _ = base64.StdEncoding.DecodeString(resp.Data)
	if string(decoded) != string(content[30:60]) {
		t.Errorf("chunk 2 data mismatch")
	}

	// Request final chunk (remaining 40 bytes from offset 60).
	params, _ = json.Marshal(FileGetRequest{Path: "large.bin", Offset: 60, Limit: 100})
	result, err = handleFilesGet(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp = result.(FileGetResponse)
	if resp.Offset != 60 {
		t.Errorf("offset = %d, want 60", resp.Offset)
	}
	if resp.Length != 40 {
		t.Errorf("length = %d, want 40", resp.Length)
	}
	if !resp.Done {
		t.Error("expected done=true for final chunk")
	}
}

func TestHandleFilesGet_OffsetBeyondSize(t *testing.T) {
	dir := setupTestRecipeDir(t)
	writeTestFile(t, dir, "small.txt", "hi")

	params, _ := json.Marshal(FileGetRequest{Path: "small.txt", Offset: 999})
	result, err := handleFilesGet(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := result.(FileGetResponse)
	if resp.Length != 0 {
		t.Errorf("length = %d, want 0 for offset beyond file", resp.Length)
	}
	if !resp.Done {
		t.Error("expected done=true")
	}
}

func TestHandleFilesGet_PathTraversal(t *testing.T) {
	setupTestRecipeDir(t)

	cases := []string{
		"../../../etc/passwd",
		"..%2f..%2fetc/passwd",
		"subdir/../../etc/passwd",
	}

	for _, path := range cases {
		params, _ := json.Marshal(FileGetRequest{Path: path})
		_, err := handleFilesGet(params)
		if err == nil {
			t.Errorf("expected error for path %q, got nil", path)
		}
	}
}

func TestHandleFilesGet_FileNotFound(t *testing.T) {
	setupTestRecipeDir(t)

	params, _ := json.Marshal(FileGetRequest{Path: "nonexistent.txt"})
	_, err := handleFilesGet(params)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestHandleFilesGet_EmptyPath(t *testing.T) {
	setupTestRecipeDir(t)

	params, _ := json.Marshal(FileGetRequest{Path: ""})
	_, err := handleFilesGet(params)
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestHandleFilesGet_Directory(t *testing.T) {
	dir := setupTestRecipeDir(t)
	os.MkdirAll(filepath.Join(dir, "adir"), 0o755)

	params, _ := json.Marshal(FileGetRequest{Path: "adir"})
	_, err := handleFilesGet(params)
	if err == nil {
		t.Fatal("expected error when path is a directory")
	}
}

func TestHandleFilesGet_BinaryContent(t *testing.T) {
	dir := setupTestRecipeDir(t)

	// Write binary content with null bytes and high bytes.
	binary := []byte{0x00, 0x01, 0xFF, 0xFE, 0x80, 0x7F}
	if err := os.WriteFile(filepath.Join(dir, "binary.bin"), binary, 0o644); err != nil {
		t.Fatal(err)
	}

	params, _ := json.Marshal(FileGetRequest{Path: "binary.bin"})
	result, err := handleFilesGet(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := result.(FileGetResponse)
	decoded, err := base64.StdEncoding.DecodeString(resp.Data)
	if err != nil {
		t.Fatalf("failed to decode base64: %v", err)
	}
	if len(decoded) != len(binary) {
		t.Fatalf("decoded length = %d, want %d", len(decoded), len(binary))
	}
	for i, b := range decoded {
		if b != binary[i] {
			t.Errorf("byte %d: got %#x, want %#x", i, b, binary[i])
		}
	}
}

func TestHandleFilesGet_DefaultLimit(t *testing.T) {
	dir := setupTestRecipeDir(t)
	writeTestFile(t, dir, "default.txt", "test content")

	// No limit specified — should default to MaxFileChunkSize.
	params, _ := json.Marshal(FileGetRequest{Path: "default.txt"})
	result, err := handleFilesGet(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := result.(FileGetResponse)
	if resp.Length != 12 {
		t.Errorf("length = %d, want 12", resp.Length)
	}
}

func TestHandleFilesGet_NegativeOffset(t *testing.T) {
	dir := setupTestRecipeDir(t)
	writeTestFile(t, dir, "neg.txt", "content")

	params, _ := json.Marshal(FileGetRequest{Path: "neg.txt", Offset: -5})
	result, err := handleFilesGet(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := result.(FileGetResponse)
	if resp.Offset != 0 {
		t.Errorf("offset = %d, want 0 (negative should clamp to 0)", resp.Offset)
	}
}

func TestHandleFilesGet_EmptyFile(t *testing.T) {
	dir := setupTestRecipeDir(t)
	writeTestFile(t, dir, "empty.txt", "")

	params, _ := json.Marshal(FileGetRequest{Path: "empty.txt"})
	result, err := handleFilesGet(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := result.(FileGetResponse)
	if resp.Size != 0 {
		t.Errorf("size = %d, want 0", resp.Size)
	}
	if resp.Length != 0 {
		t.Errorf("length = %d, want 0", resp.Length)
	}
	if !resp.Done {
		t.Error("expected done=true for empty file")
	}
}
