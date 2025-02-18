package file

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gogrlx/grlx/v2/types"
)

func (f File) absent(ctx context.Context, test bool) (types.Result, error) {
	var notes []fmt.Stringer
	err := f.validate()
	if err != nil {
		return types.Result{
			Succeeded: false, Failed: true, Notes: notes,
		}, err
	}
	name := f.params["name"].(string)
	name = filepath.Clean(name)
	if name == "/" {
		return types.Result{
			Succeeded: false, Failed: true, Notes: notes,
		}, types.ErrDeleteRoot
	}
	_, err = os.Stat(name)
	if errors.Is(err, os.ErrNotExist) {
		notes = append(notes, types.Snprintf("%v is already absent", name))
		return types.Result{
			Succeeded: true, Failed: false,
			Changed: false, Notes: notes,
		}, nil
	}
	if err != nil {
		return types.Result{
			Succeeded: false, Failed: true, Notes: notes,
		}, err
	}
	if test {
		notes = append(notes, types.Snprintf("%v would be deleted", name))
		return types.Result{
			Succeeded: true, Failed: false,
			Changed: true, Notes: notes,
		}, nil
	}
	err = os.Remove(name)
	if err != nil {
		return types.Result{
			Succeeded: false, Failed: true, Notes: notes,
		}, err
	}
	notes = append(notes, types.Snprintf("%s has been deleted", name))
	return types.Result{
		Succeeded: true, Failed: false,
		Changed: true, Notes: notes,
	}, nil
}
