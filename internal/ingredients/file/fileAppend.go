package file

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gogrlx/grlx/v2/types"
)

func (f File) append(ctx context.Context, test bool) (types.Result, error) {
	// TODO
	// "name": "string", "text": "[]string", "makedirs": "bool",
	// "source": "string", "source_hash": "string",
	// "template": "bool", "sources": "[]string",
	// "source_hashes": "[]string",
	var notes []fmt.Stringer
	name, ok := f.params["name"].(string)
	if !ok {
		return types.Result{
			Succeeded: false, Failed: true, Notes: notes,
		}, types.ErrMissingName
	}
	name = filepath.Clean(name)
	if name == "/" {
		return types.Result{
			Succeeded: false, Failed: true, Notes: notes,
		}, types.ErrModifyRoot
	}
	res, missing, err := f.contains(ctx, test)
	notes = append(notes, res.Notes...)
	if err == nil {
		return types.Result{
			Succeeded: res.Succeeded, Failed: res.Failed,
			Changed: res.Changed, Notes: notes,
		}, err
	}
	if os.IsNotExist(err) {
		f, err := os.Create(name)
		if err != nil {
			return types.Result{
				Succeeded: false, Failed: true, Notes: notes,
			}, err
		}
		defer f.Close()
		_, writeErr := missing.WriteTo(f)
		if writeErr != nil {
			return types.Result{
				Succeeded: false, Failed: true, Notes: notes,
			}, err
		}
		notes = append(notes, types.Snprintf("appended %v", name))
		return types.Result{
			Succeeded: true, Failed: false,
			Changed: true, Notes: notes,
		}, nil
	}
	if errors.Is(err, types.ErrMissingContent) {
		f, err := os.OpenFile(name, os.O_APPEND|os.O_WRONLY, 0o644)
		// TODO: Bug consider muxing errors to make this more descriptive of the issue that occurred
		if err != nil {
			return types.Result{
				Succeeded: false, Failed: true, Notes: notes,
			}, err
		}
		defer f.Close()
		scanner := bufio.NewScanner(&missing)
		line := ""
		for scanner.Scan() {
			line = scanner.Text()
			_, err := f.WriteString(line)
			if err != nil {
				return types.Result{
					Succeeded: false, Failed: true, Notes: notes,
				}, err
			}
		}
		notes = append(notes, types.Snprintf("appended %v", name))
		return types.Result{
			Succeeded: true, Failed: false,
			Changed: true, Notes: notes,
		}, nil
	}
	return types.Result{
		Succeeded: false, Failed: true,
		Changed: false, Notes: notes,
	}, err
}
