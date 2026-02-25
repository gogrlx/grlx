package file

import (
	"context"

	"github.com/gogrlx/grlx/v2/internal/cook"
)

func (f File) managed(ctx context.Context, test bool) (cook.Result, error) {
	// TODO
	// "name": "string", "source": "string", "source_hash": "string", "user": "string",
	// "group": "string", "mode": "string", "attrs": "string", "template": "bool",
	// "makedirs": "bool", "dir_mode": "string", "replace": "bool", "backup": "string", "show_changes": "bool",
	// "create":          "bool",
	// "follow_symlinks": "bool", "skip_verify": "bool",

	return f.undef()
}
