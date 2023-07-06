package file

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gogrlx/grlx/types"
)

func (f File) absent(ctx context.Context, test bool) (types.Result, error) {
	name, ok := f.params["name"].(string)
	if !ok {
		return types.Result{Succeeded: false, Failed: true}, types.ErrMissingName
	}
	name = filepath.Clean(name)
	if name == "" {
		return types.Result{Succeeded: false, Failed: true}, types.ErrMissingName
	}
	if name == "/" {
		return types.Result{Succeeded: false, Failed: true}, types.ErrDeleteRoot
	}
	_, err := os.Stat(name)
	if errors.Is(err, os.ErrNotExist) {
		return types.Result{
			Succeeded: true, Failed: false,
			Changed: false, Notes: nil,
		}, nil
	}
	if err != nil {
		return types.Result{Succeeded: false, Failed: true}, err
	}
	if test {
		return types.Result{
			Succeeded: true, Failed: false,
			Changed: true, Notes: nil,
		}, nil
	}
	err = os.Remove(name)
	if err != nil {
		return types.Result{Succeeded: false, Failed: true}, err
	}
	return types.Result{
		Succeeded: true, Failed: false,
		Changed: true, Notes: []fmt.Stringer{
			types.SimpleNote(fmt.Sprintf("removed %v", name)),
		},
	}, nil
}
