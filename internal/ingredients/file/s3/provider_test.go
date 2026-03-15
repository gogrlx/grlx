package s3

import (
	"context"
	"testing"

	"github.com/gogrlx/grlx/v2/internal/ingredients/file"
)

// Compile-time interface check.
var _ file.FileProvider = S3File{}

func TestProtocols(t *testing.T) {
	sf := S3File{}
	protos := sf.Protocols()
	if len(protos) != 1 || protos[0] != "file" {
		t.Errorf("expected [\"file\"], got %v", protos)
	}
}

func TestParse(t *testing.T) {
	sf := S3File{}
	cases := []struct {
		name  string
		id    string
		src   string
		dst   string
		hash  string
		props map[string]interface{}
	}{
		{
			name:  "basic parse",
			id:    "s3step1",
			src:   "s3://bucket/key",
			dst:   "/tmp/dst",
			hash:  "abc123",
			props: map[string]interface{}{"region": "us-east-1"},
		},
		{
			name:  "nil props initialized",
			id:    "s3step2",
			src:   "s3://bucket/key2",
			dst:   "/tmp/dst2",
			hash:  "def456",
			props: nil,
		},
		{
			name:  "empty props",
			id:    "s3step3",
			src:   "s3://bucket/key3",
			dst:   "/tmp/dst3",
			hash:  "",
			props: map[string]interface{}{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fp, err := sf.Parse(tc.id, tc.src, tc.dst, tc.hash, tc.props)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			parsed, ok := fp.(S3File)
			if !ok {
				t.Fatal("expected S3File type")
			}
			if parsed.ID != tc.id {
				t.Errorf("ID: want %q, got %q", tc.id, parsed.ID)
			}
			if parsed.Source != tc.src {
				t.Errorf("Source: want %q, got %q", tc.src, parsed.Source)
			}
			if parsed.Destination != tc.dst {
				t.Errorf("Destination: want %q, got %q", tc.dst, parsed.Destination)
			}
			if parsed.Hash != tc.hash {
				t.Errorf("Hash: want %q, got %q", tc.hash, parsed.Hash)
			}
			if parsed.Props == nil {
				t.Error("Props should not be nil (nil input should be initialized)")
			}
		})
	}
}

func TestProperties(t *testing.T) {
	props := map[string]interface{}{
		"bucket":   "my-bucket",
		"region":   "us-west-2",
		"endpoint": "https://s3.example.com",
	}
	sf := S3File{Props: props}
	got, err := sf.Properties()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["bucket"] != "my-bucket" {
		t.Errorf("bucket: want my-bucket, got %v", got["bucket"])
	}
	if got["region"] != "us-west-2" {
		t.Errorf("region: want us-west-2, got %v", got["region"])
	}
	if got["endpoint"] != "https://s3.example.com" {
		t.Errorf("endpoint: want https://s3.example.com, got %v", got["endpoint"])
	}
}

func TestDownloadStub(t *testing.T) {
	// S3 Download is currently a stub that returns nil.
	// This test documents the current behavior and ensures it doesn't panic.
	sf := S3File{
		ID:          "test-s3-download",
		Source:      "s3://bucket/key",
		Destination: "/tmp/dst",
		Hash:        "abc123",
		Props:       map[string]interface{}{},
	}

	ctx := context.Background()
	err := sf.Download(ctx)
	if err != nil {
		t.Fatalf("Download stub should return nil, got: %v", err)
	}
}

func TestVerifyStub(t *testing.T) {
	// S3 Verify is currently a stub that returns (false, nil).
	// This test documents the current behavior.
	sf := S3File{
		ID:          "test-s3-verify",
		Source:      "s3://bucket/key",
		Destination: "/tmp/dst",
		Hash:        "abc123",
		Props:       map[string]interface{}{},
	}

	ctx := context.Background()
	ok, err := sf.Verify(ctx)
	if err != nil {
		t.Fatalf("Verify stub should return nil error, got: %v", err)
	}
	if ok {
		t.Error("Verify stub should return false (not yet implemented)")
	}
}

func TestDownloadWithCancelledContext(t *testing.T) {
	sf := S3File{
		ID:          "test-cancel",
		Source:      "s3://bucket/key",
		Destination: "/tmp/dst",
		Hash:        "abc123",
		Props:       map[string]interface{}{},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Current stub ignores context — this documents that behavior.
	err := sf.Download(ctx)
	if err != nil {
		t.Fatalf("Download stub should return nil even with cancelled context, got: %v", err)
	}
}

func TestVerifyWithCancelledContext(t *testing.T) {
	sf := S3File{
		ID:          "test-cancel-verify",
		Source:      "s3://bucket/key",
		Destination: "/tmp/dst",
		Hash:        "abc123",
		Props:       map[string]interface{}{},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Current stub ignores context — this documents that behavior.
	ok, err := sf.Verify(ctx)
	if err != nil {
		t.Fatalf("Verify stub should return nil error even with cancelled context, got: %v", err)
	}
	if ok {
		t.Error("expected false from Verify stub")
	}
}

func TestParsePreservesAllFields(t *testing.T) {
	sf := S3File{}
	props := map[string]interface{}{
		"bucket":   "test-bucket",
		"region":   "eu-west-1",
		"endpoint": "https://minio.local:9000",
		"hashType": "sha256",
	}

	fp, err := sf.Parse("full-test", "s3://test-bucket/path/to/file.tar.gz", "/var/cache/grlx/file.tar.gz", "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", props)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	parsed := fp.(S3File)
	gotProps, err := parsed.Properties()
	if err != nil {
		t.Fatalf("Properties error: %v", err)
	}

	for key, want := range props {
		got, exists := gotProps[key]
		if !exists {
			t.Errorf("property %q missing after Parse", key)
			continue
		}
		if got != want {
			t.Errorf("property %q: want %v, got %v", key, want, got)
		}
	}
}
