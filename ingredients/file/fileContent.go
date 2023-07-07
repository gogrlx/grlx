package file

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gogrlx/grlx/types"
)

func (f File) content(ctx context.Context, test bool) (types.Result, error) {
	// TODO
	// "text": "[]string",
	// "makedirs": "bool", "source": "string",
	// "source_hash": "string", "template": "bool",
	// "sources": "[]string", "source_hashes": "[]string",
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
	notedChanges := []fmt.Stringer{}
	var ok bool
	{
		name, ok = f.params["name"].(string)
		if !ok {
			return types.Result{Succeeded: false, Failed: true}, types.ErrMissingName
		}
		name = filepath.Clean(name)
		if name == "" {
			return types.Result{Succeeded: false, Failed: true}, types.ErrMissingName
		}
		if name == "/" {
			return types.Result{Succeeded: false, Failed: true}, types.ErrModifyRoot
		}
	}
	{
		makedirs, _ = f.params["makedirs"].(bool)
		dir := filepath.Dir(name)
		_, statErr := os.Stat(dir)
		if os.IsNotExist(statErr) && makedirs {
			err := os.MkdirAll(dir, 0o755)
			if err != nil {
				return types.Result{
					Succeeded: false, Failed: true,
					Changed: false, Notes: nil,
				}, err
			}
			notedChanges = append(notedChanges, types.SimpleNote(fmt.Sprintf("created directory %s", dir)))
		} else if statErr != nil {
			return types.Result{
				Succeeded: false, Failed: true,
				Changed: false, Notes: nil,
			}, statErr
		}
	}
	{
		skipVerify, _ = f.params["skip_verify"].(bool)
		if skipVerify {
			_, statErr := os.Stat(name)
			if statErr == nil {
				return types.Result{
					Succeeded: false, Failed: true,
					Changed: len(notedChanges) != 0, Notes: notedChanges,
				}, nil
			} else if !os.IsNotExist(statErr) {
				return types.Result{
					Succeeded: false, Failed: true,
					Changed: len(notedChanges) != 0, Notes: notedChanges,
				}, statErr
			}
		}
	}
	{
		source, _ = f.params["source"].(string)
		sourceHash, _ = f.params["source_hash"].(string)
		if source != "" && sourceHash == "" && !skipVerify {
			return types.Result{Succeeded: false, Failed: true}, types.ErrMissingHash
		} else if source != "" {
			cachedName := fmt.Sprintf("%s-source", f.id)
			file, err := f.Parse(cachedName, "cached", map[string]interface{}{
				"source": source, "hash": sourceHash,
				"skip_verify": skipVerify, "name": cachedName,
			})
			if err != nil {
				return types.Result{
					Succeeded: false, Failed: true,
					Changed: false, Notes: []fmt.Stringer{
						types.SimpleNote(fmt.Sprintf("failed to cache source %s", source)),
					},
				}, err
			}
			cacheRes, err := file.Apply(ctx)
			if err != nil || !cacheRes.Succeeded {
				return types.Result{
					Succeeded: false, Failed: true,
					Changed: false, Notes: []fmt.Stringer{
						types.SimpleNote(fmt.Sprintf("failed to cache source %s", source)),
					},
				}, errors.Join(err, types.ErrCacheFailure)
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
		if srces, ok = f.params["sources"].([]interface{}); ok && len(srces) > 0 {
			if srcHashes, ok = f.params["source_hashes"].([]interface{}); ok {
				foundSource = true
				if skipVerify {
				} else if len(srces) != len(srcHashes) {
					notedChanges = append(notedChanges, types.SimpleNote("sources and source_hashes must be the same length"))
					return types.Result{
						Succeeded: false, Failed: true,
						Changed: len(notedChanges) != 1, Notes: notedChanges,
					}, types.ErrMissingHash
				}
			}
		}
		for i, src := range srces {
			var file types.RecipeCooker
			var err error
			if srcStr, ok := src.(string); ok && srcStr != "" {
				cachedName := fmt.Sprintf("%s-source-%d", f.id, i)
				if !skipVerify {
					if srcHash, ok := srcHashes[i].(string); ok && srcHash != "" {
						cachedName = srcHash
					} else {
						return types.Result{
							Succeeded: false, Failed: true,
							Changed: false, Notes: []fmt.Stringer{
								types.SimpleNote(fmt.Sprintf("missing source_hash for source %s", srcStr)),
							},
						}, types.ErrMissingHash
					}
				}
				file, err = f.Parse(cachedName, "cached", map[string]interface{}{
					"source":      srcStr,
					"skip_verify": skipVerify, "name": cachedName,
				})
				if err != nil {
					return types.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: []fmt.Stringer{
							types.SimpleNote(fmt.Sprintf("failed to cache source %s", srcStr)),
						},
					}, err
				}
			} else {
				return types.Result{
					Succeeded: false, Failed: true,
					Changed: false, Notes: []fmt.Stringer{
						types.SimpleNote(fmt.Sprintf("invalid source %v", src)),
					},
				}, types.ErrMissingSource
			}
			cacheRes, err := file.Apply(ctx)
			if err != nil || !cacheRes.Succeeded {
				return types.Result{
					Succeeded: false, Failed: true,
					Changed: false, Notes: []fmt.Stringer{
						types.SimpleNote(fmt.Sprintf("failed to cache source %s", src)),
					},
				}, errors.Join(err, types.ErrCacheFailure)
			}
			sourceDest, err := file.(*File).dest()
			if err != nil {
				f, err := os.Open(sourceDest)
				if err != nil {
					return types.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: []fmt.Stringer{
							types.SimpleNote(fmt.Sprintf("failed to open cached source %s", sourceDest)),
						},
					}, content, err
				}
				defer f.Close()
				io.Copy(&content, f)
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
					return types.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: []fmt.Stringer{
							types.SimpleNote(fmt.Sprintf("failed to cache source %s", srcFile)),
						},
					}, content, err
				}
				cacheRes, err := srcFile.Apply(ctx)
				if err != nil || !cacheRes.Succeeded {
					return types.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: []fmt.Stringer{
							types.SimpleNote(fmt.Sprintf("failed to cache source %s", srcFile)),
						},
					}, content, errors.Join(err, types.ErrCacheFailure)
				}
				sourceDest, err = srcFile.(*File).dest()
			} else if skipVerify, ok := f.params["skip_verify"].(bool); ok && skipVerify {
				srcFile, err := f.Parse(f.id+"-source", "cached", map[string]interface{}{
					"source":      src,
					"skip_verify": skipVerify, "name": name + "-source",
				})
				if err != nil {
					return types.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: []fmt.Stringer{
							types.SimpleNote(fmt.Sprintf("failed to cache source %s", srcFile)),
						},
					}, content, err
				}
				cacheRes, err := srcFile.Apply(ctx)
				if err != nil || !cacheRes.Succeeded {
					return types.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: []fmt.Stringer{
							types.SimpleNote(fmt.Sprintf("failed to cache source %s", srcFile)),
						},
					}, content, errors.Join(err, types.ErrCacheFailure)
				}
				sourceDest, err = srcFile.(*File).dest()
			} else {
				return types.Result{Succeeded: false, Failed: true}, content, types.ErrMissingHash
			}
		}
		f, err := os.Open(sourceDest)
		if err != nil {
			return types.Result{
				Succeeded: false, Failed: true,
				Changed: false, Notes: []fmt.Stringer{
					types.SimpleNote(fmt.Sprintf("failed to open cached source %s", sourceDest)),
				},
			}, content, err
		}
		defer f.Close()
		io.Copy(&content, f)
	}

	return f.undef()
}
