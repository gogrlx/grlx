package file

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/gogrlx/grlx/types"
)

func (f File) cached(ctx context.Context, test bool) (types.Result, error) {
	var notes []fmt.Stringer
	source, ok := f.params["source"].(string)
	if !ok || source == "" {
		// TODO join with an error type for missing params
		return types.Result{
			Succeeded: false, Failed: true, Notes: notes,
		}, types.ErrMissingSource
	}
	skipVerify, _ := f.params["skip_verify"].(bool)
	hash, ok := f.params["hash"].(string)
	if (!ok || hash == "") && !skipVerify {
		return types.Result{
			Succeeded: false, Failed: true, Notes: notes,
		}, types.ErrMissingHash
	}
	cacheDest, err := f.dest()
	if err != nil {
		return types.Result{
			Succeeded: false, Failed: true,
		}, err
	}
	fp, err := NewFileProvider(f.id, source, cacheDest, hash, f.params)
	if err != nil {
		return types.Result{
			Succeeded: false, Failed: true,
		}, err
	}
	if skipVerify {
		_, statErr := os.Stat(cacheDest)
		if statErr == nil {
			notes = append(notes, types.Snprintf("%s already exists and skipVerify is true", cacheDest))
			return types.Result{
				Succeeded: true, Failed: false,
				Changed: false, Notes: notes,
			}, nil
		} else {
			if test {
				notes = append(notes, types.Snprintf("%s would be cached", cacheDest))
				return types.Result{
					Succeeded: true, Failed: false,
					Changed: true, Notes: notes,
				}, nil
			}
			err = fp.Download(ctx)
			if err != nil {
				return types.Result{
					Succeeded: false, Failed: true,
				}, err
			}
			notes = append(notes, types.Snprintf("%s has been cached", cacheDest))
			return types.Result{
				Succeeded: true, Failed: false,
				Changed: true, Notes: notes,
			}, nil

		}
	}
	valid, errVal := fp.Verify(ctx)
	if errVal != nil && !errors.Is(errVal, types.ErrFileNotFound) {
		return types.Result{
			Succeeded: false, Failed: true,
		}, errVal
	}
	if !valid {
		if test {
			notes = append(notes, types.Snprintf("%s would be cached", cacheDest))
			return types.Result{
				Succeeded: true, Failed: false,
				Changed: true, Notes: notes,
			}, nil
		}
		err = fp.Download(ctx)
		if err != nil {
			return types.Result{
				Succeeded: false, Failed: true,
			}, err
		}
		notes = append(notes, types.Snprintf("%s has been cached", cacheDest))
		return types.Result{
			Succeeded: true, Failed: false,
			Changed: true, Notes: notes,
		}, nil

	}
	return types.Result{
		Succeeded: true, Failed: false,
		Changed: false, Notes: notes,
	}, nil
}
