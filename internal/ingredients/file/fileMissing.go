package file

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gogrlx/grlx/v2/internal/types"
)

func (f File) missing(ctx context.Context, test bool) (types.Result, error) {
	var notes []fmt.Stringer
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
			Changed: false, Notes: nil,
		}, types.ErrMissingName
	}
	_, err := os.Stat(name)
	if errors.Is(err, os.ErrNotExist) {
		notes = append(notes, types.Snprintf("file `%s` is missing", name))
		return types.Result{
			Succeeded: true, Failed: false,
			Changed: false, Notes: notes,
		}, nil
	}
	if err != nil {
		return types.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: notes,
		}, err
	}

	notes = append(notes, types.Snprintf("file `%s` is not missing", name))
	return types.Result{
		Succeeded: false, Failed: true,
		Changed: false, Notes: notes,
	}, err
}
