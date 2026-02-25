package cook

import (
	"errors"
)

var (
	ErrNoRecipe        = errors.New("no recipe")
	ErrInvalidFormat   = errors.New("invalid recipe format")
	ErrDuplicateKey    = errors.New("duplicate key in joined maps")
	ErrPathIsDirectory = errors.New("path provided is a directory")
)
