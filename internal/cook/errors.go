package cook

import (
	"errors"
)

var (
	ErrNoRecipe              = errors.New("no recipe")
	ErrInvalidFormat         = errors.New("invalid recipe format")
	ErrDuplicateKey          = errors.New("duplicate key in joined maps")
	ErrRecipePathIsDirectory = errors.New("recipe path resolved to a directory instead of a .grlx file")
)
