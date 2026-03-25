package file

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/ingredients"
)

func (f File) contains(ctx context.Context, test bool) (cook.Result, bytes.Buffer, error) {
	content := bytes.Buffer{}
	var notes []fmt.Stringer

	name, ok := f.params["name"].(string)
	if !ok {
		return cook.Result{
			Succeeded: false, Failed: true, Notes: notes,
		}, content, ingredients.ErrMissingName
	}
	name = filepath.Clean(name)
	if name == "" {
		return cook.Result{
			Succeeded: false, Failed: true, Notes: notes,
		}, content, ingredients.ErrMissingName
	}
	if name == "/" {
		return cook.Result{
			Succeeded: false, Failed: true, Notes: notes,
		}, content, ErrModifyRoot
	}

	// Collect text content.
	if text, ok := f.params["text"].(string); ok && text != "" {
		content.WriteString(text)
	} else if texti, ok := f.params["text"].([]interface{}); ok {
		for _, v := range texti {
			content.WriteString(fmt.Sprintf("%v", v))
		}
	}

	skipVerify, _ := f.params["skip_verify"].(bool)

	// Collect content from single source.
	sourceNotes, err := f.gatherSourceBuf(ctx, &content, name, skipVerify)
	notes = append(notes, sourceNotes...)
	if err != nil {
		return cook.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: notes,
		}, content, err
	}

	// Collect content from multiple sources.
	sourcesNotes, err := f.gatherSourcesBuf(ctx, &content, skipVerify, test)
	notes = append(notes, sourcesNotes...)
	if err != nil {
		return cook.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: notes,
		}, content, err
	}

	// Read current file contents.
	file, err := os.Open(name)
	if err != nil {
		notes = append(notes, cook.Snprintf("failed to open %s", name))
		return cook.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: notes,
		}, content, err
	}
	currentContents := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		currentContents = append(currentContents, scanner.Text())
	}
	file.Close()
	sort.Strings(currentContents)

	shouldContents := []string{}
	scanner = bufio.NewScanner(&content)
	for scanner.Scan() {
		shouldContents = append(shouldContents, scanner.Text())
	}
	sort.Strings(shouldContents)

	isSubset, _ := stringSliceIsSubset(shouldContents, currentContents)
	if isSubset {
		return cook.Result{
			Succeeded: true, Failed: false, Notes: notes,
		}, bytes.Buffer{}, nil
	}
	notes = append(notes, cook.Snprintf("file %s does not contain all specified content", name))
	return cook.Result{
		Succeeded: false, Failed: true,
		Changed: false, Notes: notes,
	}, content, ErrMissingContent
}

// gatherSourceBuf collects content from a single "source"/"source_hash" param pair into buf.
func (f File) gatherSourceBuf(ctx context.Context, buf *bytes.Buffer, name string, skipVerify bool) ([]fmt.Stringer, error) {
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

// gatherSourcesBuf collects content from multiple "sources"/"source_hashes" param pairs into buf.
func (f File) gatherSourcesBuf(ctx context.Context, buf *bytes.Buffer, skipVerify bool, test bool) ([]fmt.Stringer, error) {
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

		if test {
			notes = append(notes, cook.Snprintf("copy %s", cached.Name()))
		}
	}

	return notes, nil
}
