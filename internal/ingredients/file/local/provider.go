package local

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"

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

const downloadTempPattern = ".grlx-local-download-*"

// Compile-time interface check.
var _ file.FileProvider = LocalFile{}

func (lf LocalFile) Download(ctx context.Context) error {
	ok, err := lf.Verify(ctx)
	if err != nil && !errors.Is(err, file.ErrFileNotFound) {
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
	destinationDir := filepath.Dir(lf.Destination)
	stagedFile, err := os.CreateTemp(destinationDir, downloadTempPattern)
	if err != nil {
		return err
	}
	stagedPath := stagedFile.Name()
	cleanupStaged := true
	defer func() {
		if cleanupStaged {
			os.Remove(stagedPath)
		}
	}()

	_, copyErr := io.Copy(stagedFile, f)
	closeErr := stagedFile.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	mode := os.FileMode(0o644)
	if info, statErr := os.Stat(lf.Destination); statErr == nil {
		mode = info.Mode().Perm()
	}
	if err := os.Chmod(stagedPath, mode); err != nil {
		return err
	}
	if err := os.Rename(stagedPath, lf.Destination); err != nil {
		return err
	}
	cleanupStaged = false
	_, err = lf.Verify(ctx)
	return err
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
