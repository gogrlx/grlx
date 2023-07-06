package file

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gogrlx/grlx/types"
)

func (f File) exists(ctx context.Context, test bool) (types.Result, error) {
	name, ok := f.params["name"].(string)
	if !ok {
		return types.Result{Succeeded: false, Failed: true}, types.ErrMissingName
	}
	name = filepath.Clean(name)
	if name == "" {
		return types.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: nil,
		}, types.ErrMissingName
	}
	_, err := os.Stat(name)
	if errors.Is(err, os.ErrNotExist) {
		return types.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: []fmt.Stringer{
				types.SimpleNote(fmt.Sprintf("file or directory `%s` does not exist", name)),
			},
		}, nil
	}
	if err != nil {
		return types.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: []fmt.Stringer{
				types.SimpleNote(fmt.Sprintf("error checking if file or directory `%s` exists: %s", name, err.Error())),
			},
		}, err
	}
	return types.Result{
		Succeeded: true,
		Failed:    false,
		Changed:   false,
		Notes: []fmt.Stringer{
			types.SimpleNote(fmt.Sprintf("file %s exists", name)),
		},
	}, err
}
