package file

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/ingredients"
)

func (f File) append(ctx context.Context, test bool) (cook.Result, error) {
	var notes []fmt.Stringer
	name, ok := f.params["name"].(string)
	if !ok {
		return cook.Result{
			Succeeded: false, Failed: true, Notes: notes,
		}, ingredients.ErrMissingName
	}
	name = filepath.Clean(name)
	if name == "/" {
		return cook.Result{
			Succeeded: false, Failed: true, Notes: notes,
		}, ErrModifyRoot
	}

	makedirs, _ := f.params["makedirs"].(bool)

	// Check if file already contains the desired content.
	res, missing, err := f.contains(ctx, test)
	notes = append(notes, res.Notes...)
	if err == nil {
		// File already contains all desired content.
		return cook.Result{
			Succeeded: res.Succeeded, Failed: res.Failed,
			Changed: res.Changed, Notes: notes,
		}, err
	}

	if os.IsNotExist(err) {
		// Ensure parent directory exists.
		dir := filepath.Dir(name)
		if _, dirErr := os.Stat(dir); os.IsNotExist(dirErr) {
			if !makedirs {
				return cook.Result{
					Succeeded: false, Failed: true,
					Changed: false, Notes: []fmt.Stringer{
						cook.Snprintf("parent directory `%s` does not exist and makedirs is false", dir),
					},
				}, ErrPathNotFound
			}
			if test {
				notes = append(notes, cook.Snprintf("directory `%s` would be created", dir))
				notes = append(notes, cook.Snprintf("file `%s` would be created with appended content", name))
				return cook.Result{
					Succeeded: true, Failed: false,
					Changed: true, Notes: notes,
				}, nil
			}
			if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
				return cook.Result{
					Succeeded: false, Failed: true,
					Changed: false, Notes: notes,
				}, fmt.Errorf("failed to create parent directory %s: %w", dir, mkErr)
			}
			notes = append(notes, cook.Snprintf("created directory `%s`", dir))
		}

		if test {
			notes = append(notes, cook.Snprintf("file `%s` would be created with appended content", name))
			return cook.Result{
				Succeeded: true, Failed: false,
				Changed: true, Notes: notes,
			}, nil
		}

		newFile, createErr := os.Create(name)
		if createErr != nil {
			return cook.Result{
				Succeeded: false, Failed: true, Notes: notes,
			}, fmt.Errorf("failed to create %s: %w", name, createErr)
		}
		defer newFile.Close()
		_, writeErr := missing.WriteTo(newFile)
		if writeErr != nil {
			return cook.Result{
				Succeeded: false, Failed: true, Notes: notes,
			}, fmt.Errorf("failed to write to %s: %w", name, writeErr)
		}
		notes = append(notes, cook.Snprintf("created and appended to %s", name))
		return cook.Result{
			Succeeded: true, Failed: false,
			Changed: true, Notes: notes,
		}, nil
	}

	if errors.Is(err, ErrMissingContent) {
		if test {
			notes = append(notes, cook.Snprintf("content would be appended to `%s`", name))
			return cook.Result{
				Succeeded: true, Failed: false,
				Changed: true, Notes: notes,
			}, nil
		}

		appendFile, openErr := os.OpenFile(name, os.O_APPEND|os.O_WRONLY, 0o644)
		if openErr != nil {
			return cook.Result{
				Succeeded: false, Failed: true, Notes: notes,
			}, fmt.Errorf("failed to open %s for appending: %w", name, openErr)
		}
		defer appendFile.Close()
		scanner := bufio.NewScanner(&missing)
		for scanner.Scan() {
			line := scanner.Text()
			_, writeErr := appendFile.WriteString(line + "\n")
			if writeErr != nil {
				return cook.Result{
					Succeeded: false, Failed: true, Notes: notes,
				}, writeErr
			}
		}
		notes = append(notes, cook.Snprintf("appended to %s", name))
		return cook.Result{
			Succeeded: true, Failed: false,
			Changed: true, Notes: notes,
		}, nil
	}

	return cook.Result{
		Succeeded: false, Failed: true,
		Changed: false, Notes: notes,
	}, err
}
