package ingredients

import "errors"

var (
	ErrNotImplemented = errors.New("this feature is not yet implemented")
	ErrInvalidMethod  = errors.New("invalid method")
	ErrMissingName    = errors.New("recipe is missing a name")
)
