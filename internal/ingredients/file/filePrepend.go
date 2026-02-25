package file

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/ingredients"
)

func (f File) prepend(ctx context.Context, test bool) (cook.Result, error) {
	// TODO
	// "name": "string", "text": "[]string", "makedirs": "bool",
	// "source": "string", "source_hash": "string",
	// "template": "bool", "sources": "[]string",
	notes := []fmt.Stringer{}

	name, ok := f.params["name"].(string)
	if !ok {
		return cook.Result{
			Succeeded: false, Failed: true, Notes: notes,
		}, ingredients.ErrMissingName
	}
	name = filepath.Clean(name)
	if name == "" {
		return cook.Result{
			Succeeded: false, Failed: true, Notes: notes,
		}, ingredients.ErrMissingName
	}
	if name == "/" {
		return cook.Result{
			Succeeded: false, Failed: true, Notes: notes,
		}, ErrModifyRoot
	}

	return f.undef()
}
