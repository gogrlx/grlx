package file

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/ingredients"
)

func (f File) managed(ctx context.Context, test bool) (cook.Result, error) {
	// Params: "name", "source", "source_hash", "user", "group", "mode", "attrs",
	// "template", "makedirs", "dir_mode", "replace", "backup", "show_changes",
	// "create", "follow_symlinks", "skip_verify"

	var notes []fmt.Stringer

	name, ok := f.params["name"].(string)
	if !ok || name == "" {
		return cook.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: []fmt.Stringer{},
		}, ingredients.ErrMissingName
	}
	name = filepath.Clean(name)
	if name == "/" {
		return cook.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: []fmt.Stringer{},
		}, ErrModifyRoot
	}

	source, _ := f.params["source"].(string)
	sourceHash, _ := f.params["source_hash"].(string)
	skipVerify, _ := f.params["skip_verify"].(bool)
	makedirs, _ := f.params["makedirs"].(bool)
	create := true // default: create if missing
	if c, ok := f.params["create"].(bool); ok {
		create = c
	}
	backup, _ := f.params["backup"].(string)
	_ = backup

	// Validate source hash requirement
	if source != "" && sourceHash == "" && !skipVerify {
		return cook.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: []fmt.Stringer{},
		}, ErrMissingHash
	}

	// Ensure parent directory exists
	dir := filepath.Dir(name)
	if _, statErr := os.Stat(dir); os.IsNotExist(statErr) {
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
		} else {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return cook.Result{
					Succeeded: false, Failed: true,
					Changed: false, Notes: notes,
				}, err
			}
			notes = append(notes, cook.Snprintf("created directory `%s`", dir))
		}
	}

	// Check if file exists
	_, statErr := os.Stat(name)
	fileExists := statErr == nil

	if !fileExists && !create {
		return cook.Result{
			Succeeded: true, Failed: false,
			Changed: false, Notes: []fmt.Stringer{
				cook.Snprintf("file `%s` does not exist and create is false", name),
			},
		}, nil
	}

	// Cache the source file if provided
	if source == "" {
		return cook.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: []fmt.Stringer{},
		}, ErrMissingSource
	}

	cachedName := fmt.Sprintf("%s-source", f.id)
	cacheParams := map[string]interface{}{
		"source":      source,
		"skip_verify": skipVerify,
		"name":        cachedName,
	}
	if sourceHash != "" {
		cacheParams["hash"] = sourceHash
	}

	cacheFile, err := f.Parse(cachedName, "cached", cacheParams)
	if err != nil {
		notes = append(notes, cook.Snprintf("failed to parse cache for source `%s`", source))
		return cook.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: notes,
		}, err
	}

	if test {
		cacheRes, err := cacheFile.Test(ctx)
		notes = append(notes, cacheRes.Notes...)
		if err != nil || !cacheRes.Succeeded {
			return cook.Result{
				Succeeded: false, Failed: true,
				Changed: false, Notes: notes,
			}, errors.Join(err, ErrCacheFailure)
		}
		changed := !fileExists
		if !changed {
			notes = append(notes, cook.Snprintf("file `%s` would be updated from source", name))
			changed = true
		}
		return cook.Result{
			Succeeded: true, Failed: false,
			Changed: changed, Notes: notes,
		}, nil
	}

	// Apply: download/cache the source
	cacheRes, err := cacheFile.Apply(ctx)
	notes = append(notes, cacheRes.Notes...)
	if err != nil || !cacheRes.Succeeded {
		return cook.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: notes,
		}, errors.Join(err, ErrCacheFailure)
	}

	// Get the cached file path
	sourceDest, err := cacheFile.(*File).dest()
	if err != nil {
		notes = append(notes, cook.Snprintf("failed to get cached source destination: %v", err))
		return cook.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: notes,
		}, err
	}

	// Copy cached source to destination
	srcFile, err := os.Open(sourceDest)
	if err != nil {
		return cook.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: notes,
		}, err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(name)
	if err != nil {
		return cook.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: notes,
		}, err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return cook.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: notes,
		}, err
	}

	notes = append(notes, cook.Snprintf("file `%s` managed from source `%s`", name, source))
	return cook.Result{
		Succeeded: true, Failed: false,
		Changed: true, Notes: notes,
	}, nil
}
