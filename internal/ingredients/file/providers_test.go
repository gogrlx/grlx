package file

import (
	"context"
	"os"
	"testing"
)

// testLocalFile is a minimal file provider for tests.
// Can't import local package (circular dependency), so we inline it.
type testLocalFile struct {
	ID          string
	Source      string
	Destination string
	Hash        string
	Props       map[string]interface{}
}

func (lf testLocalFile) Download(ctx context.Context) error {
	// For tests, just create the destination file
	f, err := os.Create(lf.Destination)
	if err != nil {
		return err
	}
	return f.Close()
}

func (lf testLocalFile) Properties() (map[string]interface{}, error) {
	return lf.Props, nil
}

func (lf testLocalFile) Parse(id, source, destination, hash string, properties map[string]interface{}) (FileProvider, error) {
	if properties == nil {
		properties = make(map[string]interface{})
	}
	return testLocalFile{ID: id, Source: source, Destination: destination, Hash: hash, Props: properties}, nil
}

func (lf testLocalFile) Protocols() []string {
	return []string{"file"}
}

func (lf testLocalFile) Verify(ctx context.Context) (bool, error) {
	_, err := os.Stat(lf.Destination)
	if err != nil {
		return false, ErrFileNotFound
	}
	return true, nil
}

// testHTTPFile is a minimal http provider for tests.
type testHTTPFile struct {
	ID          string
	Source      string
	Destination string
	Hash        string
	Props       map[string]interface{}
}

func (hf testHTTPFile) Download(ctx context.Context) error {
	f, err := os.Create(hf.Destination)
	if err != nil {
		return err
	}
	return f.Close()
}

func (hf testHTTPFile) Properties() (map[string]interface{}, error) {
	return hf.Props, nil
}

func (hf testHTTPFile) Parse(id, source, destination, hash string, properties map[string]interface{}) (FileProvider, error) {
	if properties == nil {
		properties = make(map[string]interface{})
	}
	return testHTTPFile{ID: id, Source: source, Destination: destination, Hash: hash, Props: properties}, nil
}

func (hf testHTTPFile) Protocols() []string {
	return []string{"http", "https"}
}

func (hf testHTTPFile) Verify(ctx context.Context) (bool, error) {
	_, err := os.Stat(hf.Destination)
	if err != nil {
		return false, ErrFileNotFound
	}
	return true, nil
}

func TestMain(m *testing.M) {
	provMap["file"] = testLocalFile{}
	provMap["http"] = testHTTPFile{}
	provMap["https"] = testHTTPFile{}
	provMap["grlx"] = testHTTPFile{}
	os.Exit(m.Run())
}

// tempFile is a file:// URL to a temporary file for tests that need a valid local source.
var tempFile string

func init() {
	f, err := os.CreateTemp("", "grlx-test-source-*")
	if err != nil {
		panic(err)
	}
	f.WriteString("test content for caching")
	f.Close()
	tempFile = "file://" + f.Name()
}
