package file

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gogrlx/grlx/config"
	"github.com/gogrlx/grlx/ingredients"
	"github.com/gogrlx/grlx/types"
)

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
	return File{id: id, method: method, params: params}, nil
}

// this is a helper func to replace fallthroughs so I can keep the
// cases sorted alphabetically. It's not exported and won't stick around.
// TODO remove undef func
func (f File) undef() (types.Result, error) {
	return types.Result{Succeeded: false, Failed: true, Changed: false, Notes: nil}, fmt.Errorf("method %s undefined", f.method)
}

func (f File) Test(ctx context.Context) (types.Result, error) {
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
		return f.contains(ctx, true)
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

func (f File) absent(ctx context.Context, test bool) (types.Result, error) {
	name, ok := f.params["name"].(string)
	if !ok {
		return types.Result{Succeeded: false, Failed: true}, types.ErrMissingName
	}
	name = filepath.Clean(name)
	if name == "" {
		return types.Result{Succeeded: false, Failed: true}, types.ErrMissingName
	}
	if name == "/" {
		return types.Result{Succeeded: false, Failed: true}, types.ErrDeleteRoot
	}
	_, err := os.Stat(name)
	if errors.Is(err, os.ErrNotExist) {
		return types.Result{
			Succeeded: true, Failed: false,
			Changed: false, Notes: nil,
		}, nil
	}
	if err != nil {
		return types.Result{Succeeded: false, Failed: true}, err
	}
	if test {
		return types.Result{
			Succeeded: true, Failed: false,
			Changed: true, Notes: nil,
		}, nil
	}
	err = os.Remove(name)
	if err != nil {
		return types.Result{Succeeded: false, Failed: true}, err
	}
	return types.Result{
		Succeeded: true, Failed: false,
		Changed: true, Notes: []fmt.Stringer{
			types.SimpleNote(fmt.Sprintf("removed %v", name)),
		},
	}, nil
}

func (f File) append(ctx context.Context, test bool) (types.Result, error) {
	// TODO
	// "name": "string", "text": "[]string", "makedirs": "bool",
	// "source": "string", "source_hash": "string",
	// "template": "bool", "sources": "[]string",
	// "source_hashes": "[]string", "ignore_whitespace": "bool",

	return f.undef()
}

func (f File) cached(ctx context.Context, test bool) (types.Result, error) {
	source, ok := f.params["source"].(string)
	if !ok || source == "" {
		// TODO join with an error type for missing params
		return types.Result{Succeeded: false, Failed: true}, fmt.Errorf("source not defined")
	}
	// TODO allow for skip_verify here
	hash, ok := f.params["hash"].(string)
	if !ok || hash == "" {
		return types.Result{Succeeded: false, Failed: true}, fmt.Errorf("hash not defined")
	}
	// TODO determine filename scheme for skip_verify downloads
	cacheDest := filepath.Join(config.CacheDir, hash)
	fp, err := NewFileProvider(f.id, source, cacheDest, hash, f.params)
	if err != nil {
		return types.Result{Succeeded: false, Failed: true}, err
	}
	// TODO allow for skip_verify here
	valid, errVal := fp.Verify(ctx)
	if errVal != nil && !errors.Is(errVal, types.ErrFileNotFound) {
		return types.Result{Succeeded: false, Failed: true}, errVal
	}
	if !valid {
		if test {
			return types.Result{
				Succeeded: true, Failed: false,
				// TODO: make changes a proper stringer
				Changed: true, Notes: []fmt.Stringer{types.SimpleNote(fmt.Sprintf("%v", fp))},
			}, nil
		} else {
			err = fp.Download(ctx)
			if err != nil {
				return types.Result{Succeeded: false, Failed: true}, err
			}
			return types.Result{Succeeded: true, Failed: false, Changed: true, Notes: []fmt.Stringer{types.SimpleNote(fmt.Sprintf("%v", fp))}}, nil
		}
	}
	return types.Result{Succeeded: true, Failed: false, Changed: false}, nil
}

func (f File) directory(ctx context.Context, test bool) (types.Result, error) {
	changes := []fmt.Stringer{}
	type dir struct {
		user           string
		group          string
		recurse        bool
		maxDepth       int
		dirMode        string
		fileMode       string
		makeDirs       bool
		clean          bool
		followSymlinks bool
		force          bool
		backupName     string
		allowSymlink   bool
	}
	name, ok := f.params["name"].(string)
	if !ok {
		return types.Result{Succeeded: false, Failed: true, Notes: changes, Changed: changes != nil}, types.ErrMissingName
	}
	name = filepath.Clean(name)
	if name == "" {
		return types.Result{Succeeded: false, Failed: true}, types.ErrMissingName
	}
	if name == "/" {
		return types.Result{Succeeded: false, Failed: true}, fmt.Errorf("refusing to delete root")
	}
	d := dir{}
	// create the directory if it doesn't exist
	{
		// create the dir if "makeDirs" is true or not defined
		if val, ok := f.params["makedirs"].(bool); ok && val || !ok {
			d.makeDirs = true
			errCreate := os.MkdirAll(name, 0o755)
			if errCreate != nil {
				return types.Result{Succeeded: false, Failed: true}, errCreate
			}

		}
	}
	// chown the directory to the named user
	// TODO add test apply path here
	{
		if val, ok := f.params["user"].(string); ok {
			d.user = val
			userL, lookupErr := user.Lookup(d.user)
			if lookupErr != nil {
				return types.Result{Succeeded: false, Failed: true}, lookupErr
			}
			uid, parseErr := strconv.ParseUint(userL.Uid, 10, 32)
			if parseErr != nil {
				return types.Result{Succeeded: false, Failed: true}, parseErr
			}

			chownErr := os.Chown(name, int(uid), -1)
			if chownErr != nil {
				return types.Result{Succeeded: false, Failed: true}, chownErr
			}
			if val, ok := f.params["recurse"].(bool); ok && val {
				walkErr := filepath.WalkDir(name, func(path string, d fs.DirEntry, err error) error {
					return os.Chown(path, int(uid), -1)
				})
				if walkErr != nil {
					return types.Result{Succeeded: false, Failed: true}, walkErr
				}
			}
		}
	}
	// chown the directory to the named group
	// TODO add test apply path here
	{
		if val, ok := f.params["group"].(string); ok {
			d.group = val
			group, lookupErr := user.LookupGroup(d.group)
			if lookupErr != nil {
				return types.Result{Succeeded: false, Failed: true}, lookupErr
			}
			gid, parseErr := strconv.ParseUint(group.Gid, 10, 32)
			if parseErr != nil {
				return types.Result{Succeeded: false, Failed: true}, parseErr
			}
			chownErr := os.Chown(name, -1, int(gid))
			if chownErr != nil {
				return types.Result{Succeeded: false, Failed: true}, chownErr
			}
			if val, ok := f.params["recurse"].(bool); ok && val {
				walkErr := filepath.WalkDir(name, func(path string, d fs.DirEntry, err error) error {
					return os.Chown(path, -1, int(gid))
				})
				if walkErr != nil {
					return types.Result{Succeeded: false, Failed: true}, walkErr
				}
			}
		}
	}
	// chmod the directory to the named dirmode if it is defined
	// TODO add test apply path here
	{
		if val, ok := f.params["dir_mode"].(string); ok {
			d.dirMode = val
			modeVal, _ := strconv.ParseUint(d.dirMode, 8, 32)
			// "dir_mode": "string", "file_mode": "string"
			//"clean": "bool", "follow_symlinks": "bool", "force": "bool", "backupname": "string", "allow_symlink": "bool",
			err := os.Chmod(name, os.FileMode(modeVal))
			if err != nil {
				return types.Result{Succeeded: false, Failed: true}, err
			}
		}
	}
	// chmod the directory to the named dirmode if it is defined
	// TODO add test apply path here
	{
		if val, ok := f.params["file_mode"].(string); ok {
			d.fileMode = val
			modeVal, _ := strconv.ParseUint(d.fileMode, 8, 32)
			// "makedirs": "bool",
			//"clean": "bool", "follow_symlinks": "bool", "force": "bool", "backupname": "string", "allow_symlink": "bool",
			err := os.Chmod(name, os.FileMode(modeVal))
			if err != nil {
				return types.Result{Succeeded: false, Failed: true}, err
			}
		}
	} // recurse the file_mode if it is defined
	// TODO add test apply path here
	{
		if val, ok := f.params["group"].(string); ok {
			d.group = val
			group, lookupErr := user.LookupGroup(d.group)
			if lookupErr != nil {
				return types.Result{Succeeded: false, Failed: true}, lookupErr
			}
			gid, parseErr := strconv.ParseUint(group.Gid, 10, 32)
			if parseErr != nil {
				return types.Result{Succeeded: false, Failed: true}, parseErr
			}
			chownErr := os.Chown(name, -1, int(gid))
			if chownErr != nil {
				return types.Result{Succeeded: false, Failed: true}, chownErr
			}
			if val, ok := f.params["recurse"].(bool); ok && val {
				walkErr := filepath.WalkDir(name, func(path string, d fs.DirEntry, err error) error {
					return os.Chown(path, -1, int(gid))
				})
				if walkErr != nil {
					return types.Result{Succeeded: false, Failed: true}, walkErr
				}
			}
		}
	}

	return f.undef()
}

func (f File) exists(ctx context.Context, test bool) (types.Result, error) {
	name, ok := f.params["name"].(string)
	if !ok {
		return types.Result{Succeeded: false, Failed: true}, types.ErrMissingName
	}
	name = filepath.Clean(name)
	if name == "" {
		return types.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: nil,
		}, types.ErrMissingName
	}
	_, err := os.Stat(name)
	if errors.Is(err, os.ErrNotExist) {
		return types.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: []fmt.Stringer{
				types.SimpleNote(fmt.Sprintf("file or directory `%s` does not exist", name)),
			},
		}, nil
	}
	if err != nil {
		return types.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: []fmt.Stringer{
				types.SimpleNote(fmt.Sprintf("error checking if file or directory `%s` exists: %s", name, err.Error())),
			},
		}, err
	}
	return types.Result{
		Succeeded: true,
		Failed:    false,
		Changed:   false,
		Notes: []fmt.Stringer{
			types.SimpleNote(fmt.Sprintf("file %s exists", name)),
		},
	}, err
}

func (f File) missing(ctx context.Context, test bool) (types.Result, error) {
	name, ok := f.params["name"].(string)
	if !ok {
		return types.Result{Succeeded: false, Failed: true}, types.ErrMissingName
	}
	name = filepath.Clean(name)
	if name == "" {
		return types.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: nil,
		}, types.ErrMissingName
	}
	_, err := os.Stat(name)
	if errors.Is(err, os.ErrNotExist) {
		return types.Result{
			Succeeded: true, Failed: false,
			Changed: false, Notes: []fmt.Stringer{
				types.SimpleNote(fmt.Sprintf("file `%s` is missing", name)),
			},
		}, nil
	}
	if err != nil {
		return types.Result{
			Succeeded: false, Failed: true,
			Changed: false, Notes: []fmt.Stringer{
				types.SimpleNote(fmt.Sprintf("error checking file `%s` is missing: %s", name, err.Error())),
			},
		}, err
	}
	return types.Result{
		Succeeded: false,
		Failed:    true,
		Changed:   false,
		Notes: []fmt.Stringer{
			types.SimpleNote(fmt.Sprintf("file `%s` is not missing", name)),
		},
	}, err
}

func (f File) prepend(ctx context.Context, test bool) (types.Result, error) {
	// TODO
	// "name": "string", "text": "[]string", "makedirs": "bool",
	// "source": "string", "source_hash": "string",
	// "template": "bool", "sources": "[]string",
	// "source_hashes": "[]string", "ignore_whitespace": "bool",
	return f.undef()
}

func (f File) touch(ctx context.Context, test bool) (types.Result, error) {
	// TODO
	return f.undef()
	name, ok := f.params["name"].(string)
	if !ok {
		return types.Result{Succeeded: false, Failed: true}, types.ErrMissingName
	}

	// "name": "string", "atime": "string",
	// "mtime": "string", "makedirs": "bool",
	atime := time.Now()
	mtime := time.Now()
	{
		// parse atime
		atimeStr, ok := f.params["atime"].(string)
		if ok && atimeStr != "" {
			at, err := time.Parse(time.RFC3339, atimeStr)
			if err != nil {
				return types.Result{Succeeded: false, Failed: true, Changed: false, Notes: []fmt.Stringer{types.SimpleNote("")}}, err
			}
			atime = at
		}
	}
	{
		// parse mtime
		mtimeStr, ok := f.params["mtime"].(string)
		if ok && mtimeStr != "" {
			at, err := time.Parse(time.RFC3339, mtimeStr)
			if err != nil {
				return types.Result{Succeeded: false, Failed: true, Changed: false, Notes: []fmt.Stringer{types.SimpleNote("")}}, err
			}
			atime = at
		}
	}
	name = filepath.Clean(name)
	if name == "" {
		return types.Result{Succeeded: false, Failed: true}, types.ErrMissingName
	}
	if name == "/" {
		return types.Result{Succeeded: false, Failed: true}, types.ErrModifyRoot
	}
	_, err := os.Stat(name)
	if errors.Is(err, os.ErrNotExist) {
		// TODO pull in makedirs param
		if test {
			return types.Result{
				Succeeded: true, Failed: false,
				Changed: true, Notes: []fmt.Stringer{
					types.SimpleNote(fmt.Sprintf("file `%s` to be created with provided timestamps", name)),
				},
			}, nil
		}
	}
	if err != nil {
		return types.Result{Succeeded: false, Failed: true}, err
	}
	if test {
		return types.Result{
			Succeeded: true, Failed: false,
			Changed: true, Notes: nil,
		}, nil
	}

	err = os.Chtimes(name, atime, mtime)
	if err != nil {
		return types.Result{Succeeded: false, Failed: true}, err
	}
	return types.Result{
		Succeeded: true, Failed: false,
		Changed: true, Notes: nil,
	}, nil
}

func (f File) contains(ctx context.Context, test bool) (types.Result, error) {
	// TODO
	// "name": "string", "text": "[]string",
	// "makedirs": "bool", "source": "string",
	// "source_hash": "string", "template": "bool",
	// "sources": "[]string", "source_hashes": "[]string",

	return f.undef()
}

func (f File) content(ctx context.Context, test bool) (types.Result, error) {
	// TODO
	// "name": "string", "text": "[]string",
	// "makedirs": "bool", "source": "string",
	// "source_hash": "string", "template": "bool",
	// "sources": "[]string", "source_hashes": "[]string",

	return f.undef()
}

func (f File) managed(ctx context.Context, test bool) (types.Result, error) {
	// TODO
	// "name": "string", "source": "string", "source_hash": "string", "user": "string",
	// "group": "string", "mode": "string", "attrs": "string", "template": "bool",
	// "makedirs": "bool", "dir_mode": "string", "replace": "bool", "backup": "string", "show_changes": "bool",
	// "create":          "bool",
	// "follow_symlinks": "bool", "skip_verify": "bool",

	return f.undef()
}

func (f File) symlink(ctx context.Context, test bool) (types.Result, error) {
	// "name": "string", "target": "string", "force": "bool", "backupname": "string",
	// "makedirs": "bool", "user": "string", "group": "string", "mode": "string",
	return f.undef()
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
		return f.contains(ctx, false)
	case "content":
		return f.content(ctx, false)
	case "managed":
		return f.managed(ctx, false)
	case "symlink":
		return f.symlink(ctx, false)
	default:
		// TODO define error type
		return types.Result{Succeeded: false, Failed: true, Changed: false, Notes: nil}, fmt.Errorf("method %s undefined", f.method)

	}
}

func (f File) PropertiesForMethod(method string) (map[string]string, error) {
	switch f.method {
	case "absent":
		return map[string]string{"name": "string"}, nil
	case "append":
		return map[string]string{
			"name": "string", "text": "[]string", "makedirs": "bool",
			"source": "string", "source_hash": "string",
			"template": "bool", "sources": "[]string",
			"source_hashes": "[]string", "ignore_whitespace": "bool",
		}, nil
	case "cached":
		return map[string]string{
			"source": "string", "source_hash": "string",
		}, nil
	case "contains":
		return map[string]string{
			"name": "string", "text": "[]string",
			"makedirs": "bool", "source": "string",
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
			"source_hashes": "[]string", "ignore_whitespace": "bool",
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
		return nil, fmt.Errorf("method %s undefined", f.method)

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
