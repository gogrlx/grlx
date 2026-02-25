package file

import (
	"context"
	"path/filepath"

	"github.com/gogrlx/grlx/v2/internal/types"
)

func (f File) managed(ctx context.Context, test bool) (types.Result, error) {
	// TODO
	// "name": "string", "source": "string", "source_hash": "string", "user": "string",
	// "group": "string", "mode": "string", "attrs": "string", "template": "bool",
	// "makedirs": "bool", "dir_mode": "string", "replace": "bool", "backup": "string", "show_changes": "bool",
	// "create":          "bool",
	// "follow_symlinks": "bool", "skip_verify": "bool",

	return f.undef()
	name, ok := f.params["name"].(string)
	if !ok {
		return types.Result{
			Succeeded: false, Failed: true,
		}, types.ErrMissingName
	}
	name = filepath.Clean(name)
	if name == "" {
		return types.Result{
			Succeeded: false, Failed: true,
		}, types.ErrMissingName
	}
	if name == "/" {
		return types.Result{
			Succeeded: false, Failed: true,
		}, types.ErrModifyRoot
	}
	return f.undef()
}
