package cook

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gogrlx/grlx/types"
)

func TestCook(t *testing.T) {
	t.Run("apache", func(t *testing.T) {
		err := SendCookEvent("", "apache", "")
		if err != nil {
			t.Error(err)
		}
		// fmt.Println(jid)
	})
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
		id:       "apache dot grlx",
		recipe:   "apache.grlx",
		filepath: "",
		err:      os.ErrNotExist,
	}, {
		id:       "apache dot apache dot grlx",
		recipe:   "apache.apache.grlx",
		filepath: filepath.Join(getBasePath(), "apache/apache.grlx"),
		err:      nil,
	}, {
		id:       "apache slash path",
		recipe:   "apache/apache",
		filepath: filepath.Join(getBasePath(), "apache/apache.grlx"),
		err:      nil,
	}, {
		id:       "apache dot path",
		recipe:   "apache.apache",
		filepath: filepath.Join(getBasePath(), "apache/apache.grlx"),
		err:      nil,
	}, {
		id:       "dev",
		recipe:   "dev",
		filepath: filepath.Join(getBasePath(), "dev.grlx"),
		err:      nil,
	}, {
		id:       "apache init",
		recipe:   "apache",
		filepath: filepath.Join(getBasePath(), "apache/init.grlx"),
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

// func TestParseRecipeFile(t *testing.T) {
// 	testCases := []struct {
// 		id          string
// 		recipe      types.RecipeName
// 		recipeSteps []types.RecipeCooker
// 	}{}
//
// 	for _, tc := range testCases {
// 		t.Run(tc.id, func(t *testing.T) {
// 			steps := ParseRecipeFile(tc.recipe)
// 			_ = steps
// 		})
// 	}
// }
