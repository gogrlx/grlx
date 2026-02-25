package file

import (
	"context"
	"errors"
)

type FileProvider interface {
	Download(context.Context) error
	Properties() (map[string]interface{}, error)
	Parse(id, source, destination, hash string, properties map[string]interface{}) (FileProvider, error)
	Protocols() []string
	Verify(context.Context) (bool, error)
}

var (
	ErrMissingSource  = errors.New("recipe is missing a source")
	ErrMissingHash    = errors.New("file is missing a hash")
	ErrCacheFailure   = errors.New("file caching failed")
	ErrMissingContent = errors.New("file is missing content")
	ErrFileNotFound   = errors.New("file not found")
	ErrHashMismatch   = errors.New("file hash mismatch")
	ErrDeleteRoot     = errors.New("cannot delete root directory")
	ErrModifyRoot     = errors.New("cannot modify root directory")
	ErrMissingTarget  = errors.New("target is missing")
	ErrPathNotFound   = errors.New("path not found")
)
