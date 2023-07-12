package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gogrlx/grlx/types"
)

// symlink creates a symlink at the given path
//
// the expected outcome is that name is a symlink pointing to target
// if force is true, then name will be replaced if it already exists
// if backupname is set, then name will be backed up to backupname if it already exists
// and force is true, and name is not a symlink
// if makedirs is true, then any missing directories in the path will be created
// if user is set, then the symlink will be owned by that user
// if group is set, then the symlink will be owned by that group
// if mode is set, then the symlink will be set to that mode
// if test is true, then the symlink will not be created, but the result will indicate
// what would have happened
func (f File) symlink(ctx context.Context, test bool) (types.Result, error) {
	// parameters to implement:
	// "name": "string", "target": "string", "force": "bool", "backupname": "string",
	// "makedirs": "bool", "user": "string", "group": "string", "mode": "string",
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
	target, ok := f.params["target"].(string)
	if !ok {
		return types.Result{
			Succeeded: false, Failed: true,
		}, types.ErrMissingTarget
	}
	target = filepath.Clean(target)
	if target == "" {
		return types.Result{
			Succeeded: false, Failed: true,
		}, types.ErrMissingTarget
	}

	nameStat, err := os.Stat(name)
	if os.IsNotExist(err) {
		if test {
			return types.Result{
				Succeeded: true, Failed: false,
				Changed: true, Notes: []fmt.Stringer{
					types.SimpleNote(fmt.Sprintf("symlink would be created %s -> %s", name, target)),
				},
			}, nil
		}
		// check if it's not already a symlink
		if nameStat.Mode()&os.ModeSymlink == 0 {
		}
	} else if err != nil {
		return types.Result{
			Succeeded: false, Failed: true,
		}, err
	}

	return f.undef()
}
