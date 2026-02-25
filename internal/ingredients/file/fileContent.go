package file

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/ingredients"
)

func (f File) content(ctx context.Context, test bool) (cook.Result, error) {
	// TODO
	// "text": "[]string",
	// "makedirs": "bool", "source": "string",
	// "source_hash": "string", "template": "bool",
	// "sources": "[]string", "source_hashes": "[]string",
	var notes []fmt.Stringer
	name := ""
	makedirs := false
	source := ""
	sourceHash := ""
	text := []string{}
	template := false
	sources := []string{}
	sourceHashes := []string{}
	skipVerify := false
	foundSource := false
	_, _, _, _ = template, sources, sourceHashes, foundSource
	var ok bool
	err := f.validate()
	if err != nil {
		return cook.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: notes,
		}, err
	}
	{
		name, ok = f.params["name"].(string)
		if !ok {
			return cook.Result{
				Succeeded: false, Failed: true,
			}, ingredients.ErrMissingName
		}
		name = filepath.Clean(name)
		if name == "" {
			return cook.Result{
				Succeeded: false, Failed: true,
			}, ingredients.ErrMissingName
		}
		if name == "/" {
			return cook.Result{
				Succeeded: false, Failed: true,
			}, ErrModifyRoot
		}
	}
	{
		makedirs, _ = f.params["makedirs"].(bool)
		dir := filepath.Dir(name)
		_, statErr := os.Stat(dir)
		if os.IsNotExist(statErr) && makedirs {
			err := os.MkdirAll(dir, 0o755)
			if err != nil {
				return cook.Result{
					Succeeded: false, Failed: true,
					Changed: false, Notes: notes,
				}, err
			}
			notes = append(notes, cook.Snprintf("created directory %s", dir))
		} else if statErr != nil {
			return cook.Result{
				Succeeded: false, Failed: true,
				Changed: false, Notes: notes,
			}, statErr
		}
	}
	{
		skipVerify, _ = f.params["skip_verify"].(bool)
		if skipVerify {
			_, statErr := os.Stat(name)
			if statErr == nil {
				return cook.Result{
					Succeeded: false, Failed: true,
					Changed: len(notes) != 0, Notes: notes,
				}, nil
			} else if !os.IsNotExist(statErr) {
				return cook.Result{
					Succeeded: false, Failed: true,
					Changed: len(notes) != 0, Notes: notes,
				}, statErr
			}
		}
	}
	{
		source, _ = f.params["source"].(string)
		sourceHash, _ = f.params["source_hash"].(string)
		if source != "" && sourceHash == "" && !skipVerify {
			return cook.Result{
				Succeeded: false, Failed: true, Notes: notes,
			}, ErrMissingHash
		} else if source != "" {
			cachedName := fmt.Sprintf("%s-source", f.id)
			file, err := f.Parse(cachedName, "cached", map[string]interface{}{
				"source": source, "hash": sourceHash,
				"skip_verify": skipVerify, "name": cachedName,
			})
			if err != nil {
				notes = append(notes, cook.Snprintf("failed to cache source %s", source))
				return cook.Result{
					Succeeded: false, Failed: true,
					Changed: false, Notes: notes,
				}, err
			}
			cacheRes, err := file.Apply(ctx)
			// Append the cache apply to the notes and append the rest
			notes = append(notes, cacheRes.Notes...)
			if err != nil || !cacheRes.Succeeded {
				return cook.Result{
					Succeeded: false, Failed: true,
					Changed: false, Notes: notes,
				}, errors.Join(err, ErrCacheFailure)
			}
			foundSource = true
		}
	}
	{
		if texts, ok := f.params["text"].(string); ok && texts != "" {
			text = []string{texts}
		} else if texti, ok := f.params["text"].([]interface{}); ok {
			for _, v := range texti {
				// need to make sure it's a string and not yaml parsing as an int
				text = append(text, fmt.Sprintf("%v", v))
			}
		}
	}
	{
		var srces []interface{}
		var srcHashes []interface{}
		var ok bool
		fmt.Printf("sources: %v\n", f.params["sources"])
		if srces, ok = f.params["sources"].([]interface{}); ok && len(srces) > 0 {
			if srcHashes, ok = f.params["source_hashes"].([]interface{}); ok {
				foundSource = true
				if skipVerify {
					// TODO
				} else if len(srces) != len(srcHashes) {
					notes = append(notes, cook.SimpleNote("sources and source_hashes must be the same length"))
					return cook.Result{
						Succeeded: false, Failed: true,
						Changed: len(notes) != 1, Notes: notes,
					}, ErrMissingHash
				}
			}
		}
		for i, src := range srces {
			var file cook.RecipeCooker
			var err error
			if srcStr, ok := src.(string); ok && srcStr != "" {
				cachedName := fmt.Sprintf("%s-source-%d", f.id, i)
				if !skipVerify {
					if srcHash, ok := srcHashes[i].(string); ok && srcHash != "" {
						cachedName = srcHash
					} else {
						notes = append(notes, cook.Snprintf("missing source_hash for source %s", srcStr))
						return cook.Result{
							Succeeded: false, Failed: true,
							Changed: false, Notes: notes,
						}, ErrMissingHash
					}
				}
				file, err = f.Parse(cachedName, "cached", map[string]interface{}{
					"source":      srcStr,
					"skip_verify": skipVerify, "name": cachedName,
				})
				if err != nil {
					notes = append(notes, cook.Snprintf("failed to cache source %s", srcStr))
					return cook.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: notes,
					}, err
				}
			} else {
				notes = append(notes, cook.Snprintf("invalid source %v", src))
				return cook.Result{
					Succeeded: false, Failed: true,
					Changed: false, Notes: notes,
				}, ErrMissingSource
			}
			cacheRes, err := file.Apply(ctx)
			// Append the cache apply to the notes and append the rest
			notes = append(notes, cacheRes.Notes...)
			if err != nil || !cacheRes.Succeeded {
				notes = append(notes, cook.Snprintf("failed to cache source %s", src))
				return cook.Result{
					Succeeded: false, Failed: true,
					Changed: false, Notes: notes,
				}, errors.Join(err, ErrCacheFailure)
			}
			sourceDest, err := file.(*File).dest()
			if err != nil {
				f, err := os.Open(sourceDest)
				if err != nil {
					notes = append(notes, cook.Snprintf("failed to open cached source %s", sourceDest))
					return cook.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: notes,
					}, err
				}
				defer f.Close()
				//			io.Copy(&content, f)
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
					notes = append(notes, cook.Snprintf("failed to cache source %s", src))
					return cook.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: notes,
					}, err
				}
				cacheRes, err := srcFile.Apply(ctx)
				// Append the cache apply to the notes and append the rest
				notes = append(notes, cacheRes.Notes...)
				if err != nil || !cacheRes.Succeeded {
					notes = append(notes, cook.Snprintf("failed to cache source %s", src))
					return cook.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: notes,
					}, errors.Join(err, ErrCacheFailure)
				}
				sourceDest, err = srcFile.(*File).dest()
				if err != nil {
					notes = append(notes, cook.Snprintf("failed to get cached source destination: %v", err))
					return cook.Result{Succeeded: false, Failed: true, Changed: false, Notes: notes}, err
				}
			} else if skipVerify, ok := f.params["skip_verify"].(bool); ok && skipVerify {
				srcFile, err := f.Parse(f.id+"-source", "cached", map[string]interface{}{
					"source":      src,
					"skip_verify": skipVerify, "name": name + "-source",
				})
				if err != nil {
					notes = append(notes, cook.Snprintf("failed to cache source %s", srcFile))
					return cook.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: notes,
					}, err
				}
				cacheRes, err := srcFile.Apply(ctx)
				// Append the cache apply to the notes and append the rest
				notes = append(notes, cacheRes.Notes...)
				if err != nil || !cacheRes.Succeeded {
					notes = append(notes, cook.Snprintf("failed to cache source %s", srcFile))
					return cook.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: notes,
					}, errors.Join(err, ErrCacheFailure)
				}
				sourceDest, err = srcFile.(*File).dest()
				if err != nil {
					notes = append(notes, cook.Snprintf("failed to get cached source destination: %v", err))
					return cook.Result{Succeeded: false, Failed: true, Changed: false, Notes: notes}, err
				}
			} else {
				return cook.Result{
					Succeeded: false, Failed: true, Notes: notes,
				}, ErrMissingHash
			}
		}
		f, err := os.Open(sourceDest)
		if err != nil {
			notes = append(notes, cook.Snprintf("failed to open cached source %s", sourceDest))
			return cook.Result{
				Succeeded: false, Failed: true,
				Changed: false, Notes: notes,
			}, err
		}
		defer f.Close()
		//	io.Copy(&content, f)
	}

	// TODO: text and sources processing is incomplete
	_ = text
	return f.undef()
}
