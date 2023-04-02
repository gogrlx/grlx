package cook

import (
	"errors"
	"os"
	"testing"

	"github.com/gogrlx/grlx/types"
)

func TestCook(t *testing.T) {
}

func TestResolveRecipeFilePath(t *testing.T) {
	testCases := []struct {
		id       string
		recipe   types.RecipeName
		filepath string
		err      error
	}{{
		id:       "file doesn't exist",
		recipe:   "",
		filepath: "",
		err:      os.ErrNotExist,
	}, {
		id:       "",
		recipe:   "",
		filepath: "",
		err:      nil,
	}}
	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			filepath, err := ResolveRecipeFilePath(getBasePath(), tc.recipe)
			if filepath != tc.filepath {
				t.Errorf("expected %s but got %s", tc.filepath, filepath)
			}
			if !errors.Is(err, tc.err) {
				t.Errorf("expected error %v but got %v", tc.err, err)
			}
		})
	}
}

func TestParseRecipeFile(t *testing.T) {
	testCases := []struct {
		id          string
		recipe      types.RecipeName
		recipeSteps []types.RecipeStep
	}{}

	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			steps := ParseRecipeFile(tc.recipe)
			_ = steps
		})
	}
}

func TestGetIncludes(t *testing.T) {
	testCases := []struct {
		id     string
		recipe map[string]interface{}
		err    error
	}{}

	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			recipeNames, err := getIncludes(tc.recipe)
			if !errors.Is(tc.err, err) {
				t.Errorf("Expected err %v but got %v", tc.err, err)
			}
			_ = recipeNames
		})
	}
}
