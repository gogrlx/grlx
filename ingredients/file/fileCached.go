package file

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/gogrlx/grlx/config"
	"github.com/gogrlx/grlx/types"
)

func (f File) cached(ctx context.Context, test bool) (types.Result, error) {
	source, ok := f.params["source"].(string)
	if !ok || source == "" {
		// TODO join with an error type for missing params
		return types.Result{Succeeded: false, Failed: true}, fmt.Errorf("source not defined")
	}
	// TODO allow for skip_verify here
	hash, ok := f.params["hash"].(string)
	if !ok || hash == "" {
		return types.Result{Succeeded: false, Failed: true}, fmt.Errorf("hash not defined")
	}
	// TODO determine filename scheme for skip_verify downloads
	cacheDest := filepath.Join(config.CacheDir, hash)
	fp, err := NewFileProvider(f.id, source, cacheDest, hash, f.params)
	if err != nil {
		return types.Result{Succeeded: false, Failed: true}, err
	}
	// TODO allow for skip_verify here
	valid, errVal := fp.Verify(ctx)
	if errVal != nil && !errors.Is(errVal, types.ErrFileNotFound) {
		return types.Result{Succeeded: false, Failed: true}, errVal
	}
	if !valid {
		if test {
			return types.Result{
				Succeeded: true, Failed: false,
				// TODO: make changes a proper stringer
				Changed: true, Notes: []fmt.Stringer{types.SimpleNote(fmt.Sprintf("%v", fp))},
			}, nil
		} else {
			err = fp.Download(ctx)
			if err != nil {
				return types.Result{Succeeded: false, Failed: true}, err
			}
			return types.Result{Succeeded: true, Failed: false, Changed: true, Notes: []fmt.Stringer{types.SimpleNote(fmt.Sprintf("%v", fp))}}, nil
		}
	}
	return types.Result{Succeeded: true, Failed: false, Changed: false}, nil
}
