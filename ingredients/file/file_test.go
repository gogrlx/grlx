package file

import (
	"context"
	"testing"

	"github.com/gogrlx/grlx/types"
)

func TestRecipeStepUsage(t *testing.T) {
	var x types.RecipeStep = New("testFile", "append", map[string]any{})
	res, err := x.Apply(context.Background())
	if err != nil {
		t.Error(err)
		return
	}
	if !res.Succeeded {
		t.Errorf("error running %v", x)
	}
}
