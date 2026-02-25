package file

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/gogrlx/grlx/v2/internal/types"
)

func (f File) prepend(ctx context.Context, test bool) (types.Result, error) {
	// TODO
	// "name": "string", "text": "[]string", "makedirs": "bool",
	// "source": "string", "source_hash": "string",
	// "template": "bool", "sources": "[]string",
	notes := []fmt.Stringer{}

	name, ok := f.params["name"].(string)
	if !ok {
		return types.Result{
			Succeeded: false, Failed: true, Notes: notes,
		}, types.ErrMissingName
	}
	name = filepath.Clean(name)
	if name == "" {
		return types.Result{
			Succeeded: false, Failed: true, Notes: notes,
		}, types.ErrMissingName
	}
	if name == "/" {
		return types.Result{
			Succeeded: false, Failed: true, Notes: notes,
		}, types.ErrModifyRoot
	}

	return f.undef()
}
