package file

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/djherbis/atime"

	"github.com/gogrlx/grlx/types"
)

func (f File) touch(ctx context.Context, test bool) (types.Result, error) {
	// TODO
	name, ok := f.params["name"].(string)
	if !ok {
		return types.Result{
			Succeeded: false, Failed: true,
		}, types.ErrMissingName
	}

	aTime := time.Now()
	mTime := time.Now()
	{
		// parse atime
		atimeStr, ok := f.params["atime"].(string)
		if ok && atimeStr != "" {
			at, err := time.Parse(time.RFC3339, atimeStr)
			if err != nil {
				return types.Result{
					Succeeded: false, Failed: true,
					Changed: false, Notes: []fmt.Stringer{
						types.SimpleNote(""),
					},
				}, err
			}
			aTime = at
		}
	}
	{
		// parse mtime
		mtimeStr, ok := f.params["mtime"].(string)
		if ok && mtimeStr != "" {
			at, err := time.Parse(time.RFC3339, mtimeStr)
			if err != nil {
				return types.Result{
					Succeeded: false, Failed: true,
					Changed: false, Notes: []fmt.Stringer{
						types.SimpleNote(""),
					},
				}, err
			}
			aTime = at
		}
	}
	mkdirs := false
	{
		mkd, ok := f.params["makedirs"].(bool)
		if ok && mkd {
			mkdirs = true
		}
	}

	name = filepath.Clean(name)
	if name == "" {
		return types.Result{
			Succeeded: false, Failed: true,
		}, types.ErrMissingName
	}
	if name == "/" {
		return types.Result{
			Succeeded: false, Failed: true,
		}, types.ErrModifyRoot
	}
	stt, err := os.Stat(name)
	if errors.Is(err, os.ErrNotExist) {
		needsMkdirs := false
		fileDir := filepath.Dir(name)
		_, dirErr := os.Stat(fileDir)
		if errors.Is(dirErr, os.ErrNotExist) {
			needsMkdirs = true
		}
		if !mkdirs && needsMkdirs {
			return types.Result{
				Succeeded: false, Failed: true,
				Changed: false, Notes: []fmt.Stringer{
					types.SimpleNote(fmt.Sprintf("filepath `%s` is missing and `makedirs` is false", fileDir)),
				},
			}, types.ErrPathNotFound
		}
		if needsMkdirs {
			if test {
				return types.Result{
					Succeeded: true, Failed: false,
					Changed: true, Notes: []fmt.Stringer{
						types.SimpleNote(fmt.Sprintf("file `%s` to be created with provided timestamps", name)),
					},
				}, nil
			}
			dirErr = os.MkdirAll(fileDir, 0o755)
			if dirErr != nil {
				return types.Result{
					Succeeded: false, Failed: true,
					Changed: false, Notes: []fmt.Stringer{
						types.SimpleNote(fmt.Sprintf("failed to create parent directory %s", fileDir)),
					},
				}, dirErr
			}
		}
		f, errCreate := os.Create(name)
		if errCreate != nil {
			return types.Result{
				Succeeded: false, Failed: true,
				Changed: false, Notes: []fmt.Stringer{
					types.SimpleNote(fmt.Sprintf("failed to create file %s", name)),
				},
			}, errCreate
		}
		f.Close()
		stt, _ = os.Stat(name)
	}
	omt := stt.ModTime()
	oat, err := atime.Stat(name)
	if err != nil {
		oat = time.Now()
	}
	_ = omt
	_ = oat
	if test {
		// TODO add notes for each changed timestamp if changed
		return types.Result{
			Succeeded: true, Failed: false,
			Changed: true, Notes: nil,
		}, nil
	}

	err = os.Chtimes(name, aTime, mTime)
	if err != nil {
		return types.Result{
			Succeeded: false, Failed: true,
		}, err
	}
	return types.Result{
		Succeeded: true, Failed: false,
		Changed: true, Notes: nil,
	}, nil
}
