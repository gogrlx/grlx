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
	name, ok := f.params["name"].(string)
	if !ok {
		return types.Result{
			Succeeded: false, Failed: true,
		}, content, types.ErrMissingName
	}
	name = filepath.Clean(name)
	if name == "" {
		return types.Result{
			Succeeded: false, Failed: true,
		}, content, types.ErrMissingName
	}
	if name == "/" {
		return types.Result{
			Succeeded: false, Failed: true,
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
				return types.Result{
					Succeeded: false, Failed: true,
				}, content, types.ErrMissingHash
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
					return types.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: []fmt.Stringer{
							types.SimpleNote("sources and source_hashes must be the same length"),
						},
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
						return types.Result{
							Succeeded: false, Failed: true,
							Changed: false, Notes: []fmt.Stringer{
								types.SimpleNote(fmt.Sprintf("missing source_hash for source %s", srcStr)),
							},
						}, content, types.ErrMissingHash
					}
				}
				file, err = f.Parse(fmt.Sprintf("%s-source-%d", f.id, i), "cached", map[string]interface{}{
					"source":      srcStr,
					"skip_verify": skip, "name": cachedName,
				})
				if err != nil {
					return types.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: []fmt.Stringer{
							types.SimpleNote(fmt.Sprintf("failed to cache source %s", srcStr)),
						},
					}, content, err
				}
			} else {
				return types.Result{
					Succeeded: false, Failed: true,
					Changed: false, Notes: []fmt.Stringer{
						types.SimpleNote(fmt.Sprintf("invalid source %v", src)),
					},
				}, content, types.ErrMissingSource
			}
			cacheRes, err := file.Apply(ctx)
			if err != nil || !cacheRes.Succeeded {
				return types.Result{
					Succeeded: false, Failed: true,
					Changed: false, Notes: []fmt.Stringer{
						types.SimpleNote(fmt.Sprintf("failed to cache source %s", src)),
					},
				}, content, errors.Join(err, types.ErrCacheFailure)
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
				return types.Result{
					Succeeded: false, Failed: true,
				}, content, types.ErrMissingHash
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
	file, err := os.Open(name)
	if err != nil {
		return types.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: []fmt.Stringer{
				types.SimpleNote(fmt.Sprintf("failed to open %s", name)),
			},
		}, content, err
	}
	// TODO look into effects of sorting vs not sorting this slice
	sort.Strings(content)
	contents := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		contents = append(contents, scanner.Text())
	}
	file.Close()
	sort.Strings(contents)
	isSubset, missing := stringSliceIsSubset(content, contents)
	if isSubset {
		return types.Result{
			Succeeded: true, Failed: false,
		}, []string{}, nil
	}
	return types.Result{
		Succeeded: false, Failed: true,
		Changed: false, Notes: []fmt.Stringer{
			types.SimpleNote(fmt.Sprintf("file %s does not contain all specified content", name)),
		},
	}, missing, types.ErrMissingContent
}
