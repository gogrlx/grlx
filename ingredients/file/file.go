package file

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gogrlx/grlx/config"
	"github.com/gogrlx/grlx/ingredients"
	"github.com/gogrlx/grlx/types"
)

var ErrFileMethodUndefined = errors.New("file method undefined")

type File struct {
	id     string
	method string
	params map[string]interface{}
}

// TODO error check, set id, properly parse
func (f File) Parse(id, method string, params map[string]interface{}) (types.RecipeCooker, error) {
	if params == nil {
		params = make(map[string]interface{})
	}
	return File{
		id: id, method: method,
		params: params,
	}, nil
}

func (f File) validate() error {
	set, err := f.PropertiesForMethod(f.method)
	if err != nil {
		return err
	}
	propSet, err := ingredients.PropMapToPropSet(set)
	if err != nil {
		return err
	}
	for _, v := range propSet {
		if v.IsReq {
			if v.Key == "name" {
				name, ok := f.params[v.Key].(string)
				if !ok {
					return types.ErrMissingName
				}
				if name == "" {
					return types.ErrMissingName
				}

			} else {
				// TODO: this might need to be changed to be more deterministic
				if _, ok := f.params[v.Key]; !ok {
					return fmt.Errorf("missing required property %s", v.Key)
				}
			}
		}
	}
	return nil
}

// this is a helper func to replace fallthroughs so I can keep the
// cases sorted alphabetically. It's not exported and won't stick around.
// TODO remove undef func
func (f File) undef() (types.Result, error) {
	return types.Result{
		Succeeded: false, Failed: true,
		Changed: false, Notes: nil,
	}, errors.Join(ErrFileMethodUndefined, fmt.Errorf("method %s undefined", f.method))
}

func (f File) Test(ctx context.Context) (types.Result, error) {
	// Technically, we should be able to do the name check here, but
	// I'm not sure if that's a good idea or not.
	// For now, the name check is done in the method functions.
	// it is more concise to do it here, but it also opens up the
	// possibility of an error in the logic later on.
	switch f.method {
	case "absent":
		return f.absent(ctx, true)
	case "append":
		return f.append(ctx, true)
	case "directory":
		return f.directory(ctx, true)
	case "exists":
		return f.exists(ctx, true)
	case "missing":
		return f.missing(ctx, true)
	case "prepend":
		return f.prepend(ctx, true)
	case "touch":
		return f.touch(ctx, true)
	case "cached":
		return f.cached(ctx, true)
	case "contains":
		res, _, err := f.contains(ctx, true)
		return res, err
	case "content":
		return f.content(ctx, true)
	case "managed":
		return f.managed(ctx, true)
	case "symlink":
		return f.symlink(ctx, true)
	default:
		// TODO define error type
		return f.undef()
	}
}

func (f File) dest() (string, error) {
	name, ok := f.params["name"].(string)
	if !ok || name == "" {
		return "", types.ErrMissingName
	}
	basename := filepath.Base(name)
	if sv, okSkip := f.params["skip_verify"].(bool); okSkip && sv {
		return filepath.Join(config.CacheDir, "skip_"+basename), nil
	}
	hash, ok := f.params["hash"].(string)
	if !ok || hash == "" {
		return "", types.ErrMissingHash
	}
	return filepath.Join(config.CacheDir, hash), nil
}

func (f File) cacheSources(ctx context.Context, test bool) (types.Result, error) {
	name, ok := f.params["name"].(string)
	if !ok {
		return types.Result{
			Succeeded: false, Failed: true,
		}, types.ErrMissingName
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
					}, err
				}
				cacheRes, err := srcFile.Apply(ctx)
				if err != nil || !cacheRes.Succeeded {
					return types.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: []fmt.Stringer{
							types.SimpleNote(fmt.Sprintf("failed to cache source %s", srcFile)),
						},
					}, errors.Join(err, types.ErrCacheFailure)
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
					}, err
				}
				cacheRes, err := srcFile.Apply(ctx)
				if err != nil || !cacheRes.Succeeded {
					return types.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: []fmt.Stringer{
							types.SimpleNote(fmt.Sprintf("failed to cache source %s", srcFile)),
						},
					}, errors.Join(err, types.ErrCacheFailure)
				}
				sourceDest, err = srcFile.(*File).dest()
			} else {
				return types.Result{
					Succeeded: false, Failed: true,
				}, types.ErrMissingHash
			}
		}
		_, err := os.Stat(sourceDest)
		if err != nil {
			return types.Result{
				Succeeded: false, Failed: true,
				Changed: false, Notes: []fmt.Stringer{
					types.SimpleNote(fmt.Sprintf("failed to open cached source %s", sourceDest)),
				},
			}, err
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
					return types.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: []fmt.Stringer{
							types.SimpleNote("sources and source_hashes must be the same length"),
						},
					}, types.ErrMissingHash
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
						}, types.ErrMissingHash
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
				_, err := os.Stat(sourceDest)
				if err != nil {
					return types.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: []fmt.Stringer{
							types.SimpleNote(fmt.Sprintf("failed to open cached source %s", sourceDest)),
						},
					}, err
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
					return types.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: []fmt.Stringer{
							types.SimpleNote(fmt.Sprintf("failed to cache source %s", srcFile)),
						},
					}, err
				}
				cacheRes, err := srcFile.Apply(ctx)
				if err != nil || !cacheRes.Succeeded {
					return types.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: []fmt.Stringer{
							types.SimpleNote(fmt.Sprintf("failed to cache source %s", srcFile)),
						},
					}, errors.Join(err, types.ErrCacheFailure)
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
					}, err
				}
				cacheRes, err := srcFile.Apply(ctx)
				if err != nil || !cacheRes.Succeeded {
					return types.Result{
						Succeeded: false, Failed: true,
						Changed: false, Notes: []fmt.Stringer{
							types.SimpleNote(fmt.Sprintf("failed to cache source %s", srcFile)),
						},
					}, errors.Join(err, types.ErrCacheFailure)
				}
				sourceDest, err = srcFile.(*File).dest()
			} else {
				return types.Result{
					Succeeded: false, Failed: true,
				}, types.ErrMissingHash
			}
		}
		_, err := os.Stat(sourceDest)
		if err != nil {
			return types.Result{
				Succeeded: false, Failed: true,
				Changed: false, Notes: []fmt.Stringer{
					types.SimpleNote(fmt.Sprintf("failed to open cached source %s", sourceDest)),
				},
			}, err
		}
	}
	return types.Result{
		Succeeded: true, Failed: false,
	}, nil
}

func stringSliceIsSubset(a, b []string) (bool, []string) {
	missing := []string{}
	for len(a) > 0 {
		switch {
		case len(b) == 0:
			missing = append(missing, a...)
			return len(missing) == 0, missing
		case a[0] == b[0]:
			a = a[1:]
			b = b[1:]
		case a[0] < b[0]:
			missing = append(missing, a[0])
			if len(a) == 1 {
				return len(missing) == 0, missing
			}
			a = a[1:]
			b = b[1:]
		case a[0] > b[0]:
			b = b[1:]
		}
	}
	return len(missing) == 0, missing
}

func (f File) Apply(ctx context.Context) (types.Result, error) {
	switch f.method {
	case "absent":
		return f.absent(ctx, false)
	case "append":
		return f.append(ctx, false)
	case "directory":
		return f.directory(ctx, false)
	case "exists":
		return f.exists(ctx, false)
	case "missing":
		return f.missing(ctx, false)
	case "prepend":
		return f.prepend(ctx, false)
	case "touch":
		return f.touch(ctx, false)
	case "cached":
		return f.cached(ctx, false)
	case "contains":
		res, _, err := f.contains(ctx, false)
		return res, err
	case "content":
		return f.content(ctx, false)
	case "managed":
		return f.managed(ctx, false)
	case "symlink":
		return f.symlink(ctx, false)
	default:
		// TODO define error type
		return types.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: nil,
		}, fmt.Errorf("method %s undefined", f.method)

	}
}

func (f File) PropertiesForMethod(method string) (map[string]string, error) {
	switch f.method {
	// TODO use ingredients.MethodPropsSet for remaining methods
	case "absent":
		return ingredients.MethodPropsSet{
			ingredients.MethodProps{Key: "name", Type: "string", IsReq: true},
		}.ToMap(), nil
	case "append":
		return map[string]string{
			"name": "string", "text": "[]string", "makedirs": "bool",
			"source": "string", "source_hash": "string",
			"template": "bool", "sources": "[]string",
			"source_hashes": "[]string",
		}, nil
	case "cached":
		return ingredients.MethodPropsSet{
			ingredients.MethodProps{Key: "name", Type: "string", IsReq: true},
			ingredients.MethodProps{Key: "source", Type: "string", IsReq: true},
			ingredients.MethodProps{Key: "hash", Type: "string", IsReq: false},
			ingredients.MethodProps{Key: "skip_verify", Type: "bool", IsReq: false},
		}.ToMap(), nil
	case "contains":
		return map[string]string{
			"name": "string", "text": "[]string",
			"source":      "string",
			"source_hash": "string", "template": "bool",
			"sources": "[]string", "source_hashes": "[]string",
		}, nil
	case "content":
		return map[string]string{
			"name": "string", "text": "[]string",
			"makedirs": "bool", "source": "string",
			"source_hash": "string", "template": "bool",
			"sources": "[]string", "source_hashes": "[]string",
		}, nil
	case "directory":
		return map[string]string{
			"name": "string", "user": "string", "group": "string", "recurse": "bool",
			"dir_mode": "string", "file_mode": "string", "makedirs": "bool",
			"clean": "bool", "follow_symlinks": "bool", "force": "bool", "backupname": "string", "allow_symlink": "bool",
		}, nil
	case "managed":
		return map[string]string{
			"name": "string", "source": "string", "source_hash": "string", "user": "string",
			"group": "string", "mode": "string", "attrs": "string", "template": "bool",
			"makedirs": "bool", "dir_mode": "string", "replace": "bool", "backup": "string", "show_changes": "bool",
			"create":          "bool",
			"follow_symlinks": "bool", "skip_verify": "bool",
		}, nil
	case "missing":
		return map[string]string{"name": "string"}, nil
	case "prepend":
		return map[string]string{
			"name": "string", "text": "[]string", "makedirs": "bool",
			"source": "string", "source_hash": "string",
			"template": "bool", "sources": "[]string",
			"source_hashes": "[]string",
		}, nil
	case "exists":
		return map[string]string{
			"name": "string",
		}, nil
	case "symlink":
		return map[string]string{
			"name": "string", "target": "string", "force": "bool", "backupname": "string",
			"makedirs": "bool", "user": "string", "group": "string", "mode": "string",
		}, nil
	case "touch":
		return map[string]string{
			"name": "string", "atime": "string",
			"mtime": "string", "makedirs": "bool",
		}, nil
	default:
		// TODO define error type
		return nil, errors.Join(ErrDuplicateProtocol, fmt.Errorf("method %s undefined", f.method))

	}
}

func (f File) Methods() (string, []string) {
	return "file", []string{
		"absent",
		"append",
		"cached",
		"contains",
		"content",
		"directory",
		"managed",
		"missing",
		"prepend",
		"exists",
		"symlink",
		"touch",
	}
}

func (f File) Properties() (map[string]interface{}, error) {
	m := map[string]interface{}{}
	b, err := json.Marshal(f.params)
	if err != nil {
		return m, err
	}
	err = json.Unmarshal(b, &m)
	return m, err
}

func init() {
	ingredients.RegisterAllMethods(File{})
}
