package cook

import (
	"errors"
)

var (
	ErrNoRecipe      = errors.New("no recipe")
	ErrInvalidFormat = errors.New("invalid recipe format")
)
