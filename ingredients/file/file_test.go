package file

import (
	"context"
	"os"
	"testing"

	"github.com/gogrlx/grlx/types"
)

func TestRecipeStepUsage(t *testing.T) {
	var x types.RecipeCooker
	x, err := (File{}).Parse("testFile", "append", map[string]any{
		"name": "testFile",
	})
	if err != nil {
		t.Error(err)
		return
	}
	res, err := x.Apply(context.Background())
	if err != nil {
		t.Error(err)
		return
	}
	if !res.Succeeded {
		t.Errorf("error running %v", x)
	}
	t.Cleanup(func() {
		// remove file
		os.Remove("testFile")
	})
}
