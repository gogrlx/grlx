package hashers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

var (
	ErrFileNotFound = errors.New("file not found")
	ErrHashMismatch = errors.New("file hash mismatch")
)

type CacheFile struct {
	ID          string
	Destination string
	Hash        string
	HashType    string
}

func (cf CacheFile) Verify(ctx context.Context) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	f, err := os.Open(cf.Destination)
	if err != nil {
		if os.IsNotExist(err) {
			return false, errors.Join(err, ErrFileNotFound)
		}
		return false, err
	}
	defer f.Close()
	if cf.HashType == "" {
		cf.HashType = GuessHashType(cf.Hash)
	}
	hf, err := GetHashFunc(cf.HashType)
	if err != nil {
		return false, err
	}
	hashValue := cf.Hash
	if strings.Contains(hashValue, ":") {
		hashValue = strings.SplitN(hashValue, ":", 2)[1]
	}
	hash, matches, err := hf(contextReadCloser{ReadCloser: f, ctx: ctx}, hashValue)
	if err != nil {
		return false, errors.Join(err, ErrHashMismatch, fmt.Errorf("recipe step %s: hash for %s failed: expected %s but found %s", cf.ID, cf.Destination, cf.Hash, hash))
	}
	return matches, err
}

// contextReadCloser checks cancellation between reads while preserving the
// registered hasher's ability to close its input.
type contextReadCloser struct {
	io.ReadCloser
	ctx context.Context
}

func (reader contextReadCloser) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.ReadCloser.Read(buffer)
}
