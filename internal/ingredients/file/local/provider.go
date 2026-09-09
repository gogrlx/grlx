package local

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/gogrlx/grlx/v2/internal/ingredients/file"
	"github.com/gogrlx/grlx/v2/internal/ingredients/file/hashers"
)

type LocalFile struct {
	ID          string
	Source      string
	Destination string
	Hash        string
	Props       map[string]interface{}
}

// Compile-time interface check.
var _ file.FileProvider = LocalFile{}

func (lf LocalFile) Download(ctx context.Context) error {
	ok, err := lf.Verify(ctx)
	// if verification failed because the file doesn't exist,
	// that's ok. Otherwise, return the error.
	if !errors.Is(err, file.ErrFileNotFound) {
		return err
	}
	// if the file exists and the hash matches, we're done.
	if ok {
		return nil
	}
	// otherwise, "download" the file.
	source, err := localSourcePath(lf.Source)
	if err != nil {
		return err
	}
	f, err := os.Open(source)
	if err != nil {
		return err
	}
	defer f.Close()
	dest, err := os.Create(lf.Destination)
	if err != nil {
		return err
	}
	_, err = io.Copy(dest, f)
	dest.Close()
	if err != nil {
		return err
	}
	_, err = lf.Verify(ctx)
	return err
}

func localSourcePath(source string) (string, error) {
	if !strings.HasPrefix(source, "file://") {
		return source, nil
	}
	sourceURL, err := url.Parse(source)
	if err != nil {
		return "", err
	}
	if sourceURL.Host != "" && sourceURL.Host != "localhost" {
		return "", fmt.Errorf("unsupported file URL host %q", sourceURL.Host)
	}
	return sourceURL.Path, nil
}

func (lf LocalFile) Properties() (map[string]interface{}, error) {
	return lf.Props, nil
}

func (lf LocalFile) Parse(id, source, destination, hash string, properties map[string]interface{}) (file.FileProvider, error) {
	if properties == nil {
		properties = make(map[string]interface{})
	}
	return LocalFile{ID: id, Source: source, Destination: destination, Hash: hash, Props: properties}, nil
}

func (lf LocalFile) Protocols() []string {
	return []string{"file"}
}

func (lf LocalFile) Verify(ctx context.Context) (bool, error) {
	hashType := ""
	if lf.Props["hashType"] == nil {
		hashType = hashers.GuessHashType(lf.Hash)
	} else if ht, ok := lf.Props["hashType"].(string); !ok {
		hashType = hashers.GuessHashType(lf.Hash)
	} else {
		hashType = ht
	}
	cf := hashers.CacheFile{
		ID:          lf.ID,
		Destination: lf.Destination,
		Hash:        lf.Hash,
		HashType:    hashType,
	}
	return cf.Verify(ctx)
}

func init() {
	file.RegisterProvider(LocalFile{})
}
