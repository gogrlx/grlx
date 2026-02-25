package file

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/ingredients"
)

func (f File) exists(ctx context.Context, test bool) (cook.Result, error) {
	var notes []fmt.Stringer
	name, ok := f.params["name"].(string)
	if !ok {
		return cook.Result{
			Succeeded: false, Failed: true,
		}, ingredients.ErrMissingName
	}
	name = filepath.Clean(name)
	if name == "" {
		return cook.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: nil,
		}, ingredients.ErrMissingName
	}
	_, err := os.Stat(name)
	if errors.Is(err, os.ErrNotExist) {
		notes = append(notes, cook.Snprintf("file `%s` does not exist", name))
		return cook.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: notes,
		}, nil
	}
	if err != nil {
		return cook.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: notes,
		}, err
	}
	notes = append(notes, cook.Snprintf("file `%s` exists", name))
	return cook.Result{
		Succeeded: true,
		Failed:    false,
		Changed:   false,
		Notes:     notes,
	}, err
}
