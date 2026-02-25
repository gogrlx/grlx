package hashers

import (
	"context"
	"errors"
	"fmt"
	"os"
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
	hash, matches, err := hf(f, cf.Hash)
	if err != nil {
		return false, errors.Join(err, ErrHashMismatch, fmt.Errorf("recipe step %s: hash for %s failed: expected %s but found %s", cf.ID, cf.Destination, cf.Hash, hash))
	}
	return matches, err
}
