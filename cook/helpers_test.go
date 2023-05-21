package cook

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gogrlx/grlx/types"
)

func TestExtractIncludes(t *testing.T) {
	testCases := []struct {
		id       string
		sprout   string
		basepath string
		recipe   types.RecipeName
	}{{
		id:       "dev",
		sprout:   "testSprout",
		basepath: getBasePath(),
		recipe:   "dev",
	}}
	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			f, _ := os.ReadFile(filepath.Join(tc.basepath, string(tc.recipe)+".grlx"))
			r, err := extractIncludes(tc.sprout, tc.basepath, string(tc.recipe), f)
			fmt.Printf("extractedIncludes: %v, %v", r, err)
		})
	}
}

func TestCollectAllIncludes(t *testing.T) {
	testCases := []struct {
		id     string
		recipe types.RecipeName
		sprout string
	}{{
		id:     "dev",
		recipe: "dev",
		sprout: "testSprout",
	}}
	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			recipes, err := collectAllIncludes(tc.sprout, getBasePath(), tc.recipe)
			fmt.Printf("%v, %v", recipes, err)
		})
	}
}

func TestRelativeRecipeToAbsolute(t *testing.T) {
	testCases := []struct {
		id              string
		recipe          types.RecipeName
		filepath        string
		err             error
		relatedFilepath string
	}{{
		id:              "file doesn't exist",
		recipe:          "",
		filepath:        "",
		err:             os.ErrNotExist,
		relatedFilepath: "",
	}, {
		id:              "valid missing recipe",
		recipe:          ".missing",
		filepath:        "missing",
		err:             nil,
		relatedFilepath: filepath.Join(getBasePath(), "dev.grlx"),
	}}
	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			filepath, err := relativeRecipeToAbsolute(getBasePath(), tc.relatedFilepath, tc.recipe)
			if string(filepath) != tc.filepath {
				t.Errorf("expected %s but got %s", tc.filepath, filepath)
			}
			if !errors.Is(err, tc.err) {
				t.Errorf("expected error %v but got %v", tc.err, err)
			}
		})
	}
}
