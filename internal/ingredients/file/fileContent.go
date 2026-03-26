package file

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/ingredients"
)

func (f File) content(ctx context.Context, test bool) (cook.Result, error) {
	// Params: "name", "text", "makedirs", "source", "source_hash",
	// "template", "sources", "source_hashes", "skip_verify"
	var notes []fmt.Stringer

	name, ok := f.params["name"].(string)
	if !ok || name == "" {
		return cook.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: notes,
		}, ingredients.ErrMissingName
	}
	name = filepath.Clean(name)
	if name == "/" {
		return cook.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: notes,
		}, ErrModifyRoot
	}

	makedirs, _ := f.params["makedirs"].(bool)
	skipVerify, _ := f.params["skip_verify"].(bool)

	// Ensure parent directory exists.
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

	// Gather desired content from text, source, and sources params.
	var desired bytes.Buffer

	// Collect text content.
	if text, ok := f.params["text"].(string); ok && text != "" {
		desired.WriteString(text)
		if text[len(text)-1] != '\n' {
			desired.WriteByte('\n')
		}
	} else if texti, ok := f.params["text"].([]interface{}); ok {
		for _, v := range texti {
			desired.WriteString(fmt.Sprintf("%v\n", v))
		}
	}

	// Collect content from single source.
	sourceNotes, err := f.gatherSource(ctx, &desired, name, skipVerify)
	notes = append(notes, sourceNotes...)
	if err != nil {
		return cook.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: notes,
		}, err
	}

	// Collect content from multiple sources.
	sourcesNotes, err := f.gatherSources(ctx, &desired, skipVerify)
	notes = append(notes, sourcesNotes...)
	if err != nil {
		return cook.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: notes,
		}, err
	}

	desiredBytes := desired.Bytes()

	// Check if file already has the desired content (idempotency).
	existing, readErr := os.ReadFile(name)
	if readErr == nil {
		if bytes.Equal(existing, desiredBytes) {
			return cook.Result{
				Succeeded: true, Failed: false,
				Changed: false, Notes: notes,
			}, nil
		}
	} else if !os.IsNotExist(readErr) {
		return cook.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: notes,
		}, fmt.Errorf("failed to read existing file %s: %w", name, readErr)
	}

	// In test mode, report what would change.
	if test {
		if os.IsNotExist(readErr) {
			notes = append(notes, cook.Snprintf("file `%s` would be created with specified content", name))
		} else {
			notes = append(notes, cook.Snprintf("file `%s` content would be updated", name))
		}
		return cook.Result{
			Succeeded: true, Failed: false,
			Changed: true, Notes: notes,
		}, nil
	}

	// Write the content to the file.
	if writeErr := os.WriteFile(name, desiredBytes, 0o644); writeErr != nil {
		return cook.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: notes,
		}, fmt.Errorf("failed to write content to %s: %w", name, writeErr)
	}

	if os.IsNotExist(readErr) {
		notes = append(notes, cook.Snprintf("created `%s` with specified content", name))
	} else {
		notes = append(notes, cook.Snprintf("updated content of `%s`", name))
	}

	return cook.Result{
		Succeeded: true, Failed: false,
		Changed: true, Notes: notes,
	}, nil
}

// gatherSource collects content from a single "source"/"source_hash" param pair.
func (f File) gatherSource(ctx context.Context, buf *bytes.Buffer, name string, skipVerify bool) ([]fmt.Stringer, error) {
	var notes []fmt.Stringer

	source, _ := f.params["source"].(string)
	if source == "" {
		return notes, nil
	}

	sourceHash, _ := f.params["source_hash"].(string)
	if sourceHash == "" && !skipVerify {
		return notes, ErrMissingHash
	}

	cacheParams := map[string]interface{}{
		"source":      source,
		"skip_verify": skipVerify,
		"name":        name + "-source",
	}
	if sourceHash != "" {
		cacheParams["hash"] = sourceHash
	}

	srcFile, err := f.Parse(f.id+"-source", "cached", cacheParams)
	if err != nil {
		notes = append(notes, cook.Snprintf("failed to parse source %s", source))
		return notes, err
	}

	cacheRes, err := srcFile.Apply(ctx)
	notes = append(notes, cacheRes.Notes...)
	if err != nil || !cacheRes.Succeeded {
		notes = append(notes, cook.Snprintf("failed to cache source %s", source))
		return notes, errors.Join(err, ErrCacheFailure)
	}

	srcF, ok := srcFile.(File)
	if !ok {
		notes = append(notes, cook.Snprintf("unexpected type for cached source"))
		return notes, fmt.Errorf("cached source is not a File")
	}
	sourceDest, err := srcF.dest()
	if err != nil {
		notes = append(notes, cook.Snprintf("failed to get cached source destination: %v", err))
		return notes, err
	}

	cached, err := os.Open(sourceDest)
	if err != nil {
		notes = append(notes, cook.Snprintf("failed to open cached source %s", sourceDest))
		return notes, err
	}
	defer cached.Close()

	if _, cpErr := io.Copy(buf, cached); cpErr != nil {
		notes = append(notes, cook.Snprintf("failed to read source: %v", cpErr))
		return notes, cpErr
	}

	return notes, nil
}

// gatherSources collects content from multiple "sources"/"source_hashes" param pairs.
func (f File) gatherSources(ctx context.Context, buf *bytes.Buffer, skipVerify bool) ([]fmt.Stringer, error) {
	var notes []fmt.Stringer

	srces, ok := f.params["sources"].([]interface{})
	if !ok || len(srces) == 0 {
		return notes, nil
	}

	var srcHashes []interface{}
	if !skipVerify {
		srcHashes, ok = f.params["source_hashes"].([]interface{})
		if !ok || len(srcHashes) != len(srces) {
			notes = append(notes, cook.SimpleNote("sources and source_hashes must be the same length"))
			return notes, ErrMissingHash
		}
	}

	for i, src := range srces {
		srcStr, ok := src.(string)
		if !ok || srcStr == "" {
			notes = append(notes, cook.Snprintf("invalid source at index %d", i))
			return notes, ErrMissingSource
		}

		cacheParams := map[string]interface{}{
			"source":      srcStr,
			"skip_verify": skipVerify,
			"name":        fmt.Sprintf("%s-source-%d", f.id, i),
		}
		if !skipVerify {
			hash, ok := srcHashes[i].(string)
			if !ok || hash == "" {
				notes = append(notes, cook.Snprintf("missing source_hash for source %s", srcStr))
				return notes, ErrMissingHash
			}
			cacheParams["hash"] = hash
		}

		file, err := f.Parse(fmt.Sprintf("%s-source-%d", f.id, i), "cached", cacheParams)
		if err != nil {
			notes = append(notes, cook.Snprintf("failed to parse source %s", srcStr))
			return notes, err
		}

		cacheRes, err := file.Apply(ctx)
		notes = append(notes, cacheRes.Notes...)
		if err != nil || !cacheRes.Succeeded {
			notes = append(notes, cook.Snprintf("failed to cache source %s", srcStr))
			return notes, errors.Join(err, ErrCacheFailure)
		}

		srcF, ok := file.(File)
		if !ok {
			notes = append(notes, cook.Snprintf("unexpected type for cached source"))
			return notes, fmt.Errorf("cached source is not a File")
		}
		sourceDest, err := srcF.dest()
		if err != nil {
			notes = append(notes, cook.Snprintf("failed to get destination for cached source: %v", err))
			return notes, err
		}

		cached, err := os.Open(sourceDest)
		if err != nil {
			notes = append(notes, cook.Snprintf("failed to open cached source %s", sourceDest))
			return notes, err
		}
		defer cached.Close()

		if _, cpErr := io.Copy(buf, cached); cpErr != nil {
			notes = append(notes, cook.Snprintf("failed to read source: %v", cpErr))
			return notes, cpErr
		}
	}

	return notes, nil
}
