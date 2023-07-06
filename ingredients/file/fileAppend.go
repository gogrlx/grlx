package file

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gogrlx/grlx/types"
)

func (f File) append(ctx context.Context, test bool) (types.Result, error) {
	// TODO
	// "name": "string", "text": "[]string", "makedirs": "bool",
	// "source": "string", "source_hash": "string",
	// "template": "bool", "sources": "[]string",
	// "source_hashes": "[]string",
	name, ok := f.params["name"].(string)
	if !ok {
		return types.Result{Succeeded: false, Failed: true}, types.ErrMissingName
	}
	name = filepath.Clean(name)
	if name == "" {
		return types.Result{Succeeded: false, Failed: true}, types.ErrMissingName
	}
	if name == "/" {
		return types.Result{Succeeded: false, Failed: true}, types.ErrModifyRoot
	}
	res, missing, err := f.contains(ctx, test)
	if err == nil {
		return types.Result{
			Succeeded: res.Succeeded, Failed: res.Failed,
			Changed: res.Changed, Notes: res.Notes,
		}, err
	}
	if os.IsNotExist(err) {
		f, err := os.Create(name)
		if err != nil {
			return types.Result{Succeeded: false, Failed: true}, err
		}
		defer f.Close()
		_, writeErr := missing.WriteTo(f)
		if writeErr != nil {
			return types.Result{Succeeded: false, Failed: true}, err
		}
		return types.Result{
			Succeeded: true, Failed: false,
			Changed: true, Notes: []fmt.Stringer{
				types.SimpleNote(fmt.Sprintf("appended %v", name)),
			},
		}, nil
	}
	if errors.Is(err, types.ErrMissingContent) {
		f, err := os.OpenFile(name, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return types.Result{Succeeded: false, Failed: true}, err
		}
		defer f.Close()
		for _, line := range missing {
			_, err := f.WriteString(line)
			if err != nil {
				return types.Result{Succeeded: false, Failed: true}, err
			}
		}
		return types.Result{
			Succeeded: true, Failed: false,
			Changed: true, Notes: []fmt.Stringer{
				types.SimpleNote(fmt.Sprintf("appended %v", name)),
			},
		}, nil
	} else {
		return types.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: res.Notes,
		}, err
	}

	return f.undef()
}
