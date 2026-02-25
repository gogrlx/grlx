package file

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gogrlx/grlx/v2/internal/cook"
)

func (f File) absent(ctx context.Context, test bool) (cook.Result, error) {
	var notes []fmt.Stringer
	err := f.validate()
	if err != nil {
		return cook.Result{
			Succeeded: false, Failed: true, Notes: notes,
		}, err
	}
	name := f.params["name"].(string)
	name = filepath.Clean(name)
	if name == "/" {
		return cook.Result{
			Succeeded: false, Failed: true, Notes: notes,
		}, ErrDeleteRoot
	}
	_, err = os.Stat(name)
	if errors.Is(err, os.ErrNotExist) {
		notes = append(notes, cook.Snprintf("%v is already absent", name))
		return cook.Result{
			Succeeded: true, Failed: false,
			Changed: false, Notes: notes,
		}, nil
	}
	if err != nil {
		return cook.Result{
			Succeeded: false, Failed: true, Notes: notes,
		}, err
	}
	if test {
		notes = append(notes, cook.Snprintf("%v would be deleted", name))
		return cook.Result{
			Succeeded: true, Failed: false,
			Changed: true, Notes: notes,
		}, nil
	}
	err = os.Remove(name)
	if err != nil {
		return cook.Result{
			Succeeded: false, Failed: true, Notes: notes,
		}, err
	}
	notes = append(notes, cook.Snprintf("%s has been deleted", name))
	return cook.Result{
		Succeeded: true, Failed: false,
		Changed: true, Notes: notes,
	}, nil
}
