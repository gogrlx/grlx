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
	// TODO
	// "name": "string", "text": "[]string", "makedirs": "bool",
	// "source": "string", "source_hash": "string",
	// "template": "bool", "sources": "[]string",
	// "source_hashes": "[]string",
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
	res, missing, err := f.contains(ctx, test)
	notes = append(notes, res.Notes...)
	if err == nil {
		return cook.Result{
			Succeeded: res.Succeeded, Failed: res.Failed,
			Changed: res.Changed, Notes: notes,
		}, err
	}
	if os.IsNotExist(err) {
		f, err := os.Create(name)
		if err != nil {
			return cook.Result{
				Succeeded: false, Failed: true, Notes: notes,
			}, err
		}
		defer f.Close()
		_, writeErr := missing.WriteTo(f)
		if writeErr != nil {
			return cook.Result{
				Succeeded: false, Failed: true, Notes: notes,
			}, err
		}
		notes = append(notes, cook.Snprintf("appended %v", name))
		return cook.Result{
			Succeeded: true, Failed: false,
			Changed: true, Notes: notes,
		}, nil
	}
	if errors.Is(err, ErrMissingContent) {
		f, err := os.OpenFile(name, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return cook.Result{
				Succeeded: false, Failed: true, Notes: notes,
			}, fmt.Errorf("failed to open %s for appending: %w", name, err)
		}
		defer f.Close()
		scanner := bufio.NewScanner(&missing)
		line := ""
		for scanner.Scan() {
			line = scanner.Text()
			_, err := f.WriteString(line)
			if err != nil {
				return cook.Result{
					Succeeded: false, Failed: true, Notes: notes,
				}, err
			}
		}
		notes = append(notes, cook.Snprintf("appended %v", name))
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
