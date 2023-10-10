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

	"github.com/gogrlx/grlx/types"
)

func (f File) contains(ctx context.Context, test bool) (types.Result, bytes.Buffer, error) {
	// TODO
	// "template": "bool",

	content := bytes.Buffer{}
	notes := []fmt.Stringer{}
	name, ok := f.params["name"].(string)
	if !ok {
		return types.Result{
			Succeeded: false, Failed: true, Notes: notes,
		}, content, types.ErrMissingName
	}
	name = filepath.Clean(name)
	if name == "" {
		return types.Result{
			Succeeded: false, Failed: true, Notes: notes,
		}, content, types.ErrMissingName
	}
	if name == "/" {
		return types.Result{
			Succeeded: false, Failed: true, Notes: notes,
		}, content, types.ErrModifyRoot
	}
	{
		if text, ok := f.params["text"].(string); ok && text != "" {
			content.WriteString(text)
		} else if texti, ok := f.params["text"].([]interface{}); ok {
			for _, v := range texti {
				// need to make sure it's a string and not yaml parsing as an int
				content.WriteString(fmt.Sprintf("%v", v))
			}
		}
	}
	{
		sourceDest := ""
		if src, ok := f.params["source"].(string); ok && src != "" {
			if srcHash, ok := f.params["source_hash"].(string); ok && srcHash != "" {
				srcFile, err := f.Parse(f.id+"-source", "cached", map[string]interface{}{
					"source": src, "hash": srcHash,
					"name": name + "-source",
				})
				if err != nil {
					notes = append(notes, types.Snprintf("failed to cache source %s", srcFile))
					return types.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: notes,
					}, content, err
				}
				cacheRes, err := srcFile.Apply(ctx)
				notes = append(notes, cacheRes.Notes...)
				if err != nil || !cacheRes.Succeeded {
					notes = append(notes, types.Snprintf("failed to cache source %s", srcFile))
					return types.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: notes,
					}, content, errors.Join(err, types.ErrCacheFailure)
				}
				sourceDest, err = srcFile.(*File).dest()
			} else if skipVerify, ok := f.params["skip_verify"].(bool); ok && skipVerify {
				srcFile, err := f.Parse(f.id+"-source", "cached", map[string]interface{}{
					"source":      src,
					"skip_verify": skipVerify, "name": name + "-source",
				})
				if err != nil {
					notes = append(notes, types.Snprintf("failed to cache source %s", srcFile))
					return types.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: notes,
					}, content, err
				}
				cacheRes, err := srcFile.Apply(ctx)
				if err != nil || !cacheRes.Succeeded {
					notes = append(notes, types.Snprintf("failed to cache source %s", srcFile))
					return types.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: notes,
					}, content, errors.Join(err, types.ErrCacheFailure)
				}
				sourceDest, err = srcFile.(*File).dest()
			} else {
				return types.Result{
					Succeeded: false, Failed: true, Notes: notes,
				}, content, types.ErrMissingHash
			}
			f, err := os.Open(sourceDest)
			if err != nil {
				notes = append(notes, types.Snprintf("failed to open cached source %s", sourceDest))
				return types.Result{
					Succeeded: false, Failed: true,
					Changed: false, Notes: notes,
				}, content, err
			}
			defer f.Close()
			io.Copy(&content, f)
		}
	}
	{
		var srces []interface{}
		var srcHashes []interface{}
		var ok bool
		skip := false
		if srces, ok = f.params["sources"].([]interface{}); ok && len(srces) > 0 {
			if srcHashes, ok = f.params["source_hashes"].([]interface{}); ok {
				if skipVerify, ok := f.params["skip_verify"].(bool); ok && skipVerify {
					skip = true
				} else if len(srces) != len(srcHashes) {
					notes = append(notes, types.SimpleNote("sources and source_hashes must be the same length"))
					return types.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: notes,
					}, content, types.ErrMissingHash
				}
			}
		}
		for i, src := range srces {
			var file types.RecipeCooker
			var err error
			if srcStr, ok := src.(string); ok && srcStr != "" {
				cachedName := fmt.Sprintf("%s-source-%d", f.id, i)
				if !skip {
					if srcHash, ok := srcHashes[i].(string); ok && srcHash != "" {
						cachedName = srcHash
					} else {
						notes = append(notes, types.Snprintf("missing source_hash for source %s", srcStr))
						return types.Result{
							Succeeded: false, Failed: true,
							Changed: false, Notes: notes,
						}, content, types.ErrMissingHash
					}
				}
				file, err = f.Parse(fmt.Sprintf("%s-source-%d", f.id, i), "cached", map[string]interface{}{
					"source":      srcStr,
					"skip_verify": skip, "name": cachedName,
				})
				if err != nil {
					notes = append(notes, types.Snprintf("failed to cache source %s", srcStr))
					return types.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: notes,
					}, content, err
				}
			} else {
				notes = append(notes, types.Snprintf("invalid source %v", src))
				return types.Result{
					Succeeded: false, Failed: true,
					Changed: false, Notes: notes,
				}, content, types.ErrMissingSource
			}
			cacheRes, err := file.Apply(ctx)
			notes = append(notes, cacheRes.Notes...)
			if err != nil || !cacheRes.Succeeded {
				notes = append(notes, types.Snprintf("failed to cache source %s", src))
				return types.Result{
					Succeeded: false, Failed: true,
					Changed: false, Notes: notes,
				}, content, errors.Join(err, types.ErrCacheFailure)
			}
			sourceDest, err := file.(*File).dest()
			if err != nil {
				f, err := os.Open(sourceDest)
				if err != nil {
					notes = append(notes, types.Snprintf("failed to open cached source %s", sourceDest))
					return types.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: notes,
					}, content, err
				}
				defer f.Close()
				io.Copy(&content, f)
				if test {
					notes = append(notes, types.Snprintf("copy %s", f.Name()))
				}
			}

		}
		sourceDest := ""
		if src, ok := f.params["source"].(string); ok && src != "" {
			if srcHash, ok := f.params["source_hash"].(string); ok && srcHash != "" {
				srcFile, err := f.Parse(f.id+"-source", "cached", map[string]interface{}{
					"source": src, "hash": srcHash,
					"name": name + "-source",
				})
				if err != nil {
					notes = append(notes, types.Snprintf("failed to cache source %s", srcFile))
					return types.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: notes,
					}, content, err
				}
				cacheRes, err := srcFile.Apply(ctx)
				notes = append(notes, cacheRes.Notes...)
				if err != nil || !cacheRes.Succeeded {
					notes = append(notes, types.Snprintf("failed to cache source %s", srcFile))
					return types.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: notes,
					}, content, errors.Join(err, types.ErrCacheFailure)
				}
				sourceDest, err = srcFile.(*File).dest()
			} else if skipVerify, ok := f.params["skip_verify"].(bool); ok && skipVerify {
				srcFile, err := f.Parse(f.id+"-source", "cached", map[string]interface{}{
					"source":      src,
					"skip_verify": skipVerify, "name": name + "-source",
				})
				if err != nil {
					notes = append(notes, types.Snprintf("failed to cache source %s", srcFile))
					return types.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: notes,
					}, content, err
				}
				cacheRes, err := srcFile.Apply(ctx)
				notes = append(notes, cacheRes.Notes...)
				if err != nil || !cacheRes.Succeeded {
					notes = append(notes, types.Snprintf("failed to cache source %s", srcFile))
					return types.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: notes,
					}, content, errors.Join(err, types.ErrCacheFailure)
				}
				sourceDest, err = srcFile.(*File).dest()
			} else {
				return types.Result{
					Succeeded: false, Failed: true,
				}, content, types.ErrMissingHash
			}
			f, err := os.Open(sourceDest)
			if err != nil {
				notes = append(notes, types.Snprintf("failed to open cached source %s", sourceDest))
				return types.Result{
					Succeeded: false, Failed: true,
					Changed: false, Notes: notes,
				}, content, err
			}
			defer f.Close()
			io.Copy(&content, f)
		}
	}
	file, err := os.Open(name)
	if err != nil {
		notes = append(notes, types.Snprintf("failed to open %s", name))
		return types.Result{
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
		return types.Result{
			Succeeded: true, Failed: false, Notes: notes,
		}, bytes.Buffer{}, nil
	}
	notes = append(notes, types.Snprintf("file %s does not contain all specified content", name))
	return types.Result{
		Succeeded: false, Failed: true,
		Changed: false, Notes: notes,
	}, content, types.ErrMissingContent
}
