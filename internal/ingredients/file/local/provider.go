package local

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/gogrlx/grlx/v2/internal/ingredients/file/hashers"
	"github.com/gogrlx/grlx/v2/internal/types"
)

type LocalFile struct {
	ID          string
	Source      string
	Destination string
	Hash        string
	Props       map[string]interface{}
}

func (lf LocalFile) Download(ctx context.Context) error {
	ok, err := lf.Verify(ctx)
	// if verification failed because the file doesn't exist,
	// that's ok. Otherwise, return the error.
	if !errors.Is(err, types.ErrFileNotFound) {
		return err
	}
	// if the file exists and the hash matches, we're done.
	if ok {
		return nil
	}
	// otherwise, "download" the file.
	f, err := os.Open(lf.Source)
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

func (lf LocalFile) Properties() (map[string]interface{}, error) {
	return lf.Props, nil
}

func (lf LocalFile) Parse(id, source, destination, hash string, properties map[string]interface{}) (types.FileProvider, error) {
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
}
