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
	return types.Result{Succeeded: false, Failed: true, Changed: false, Changes: nil}, fmt.Errorf("method %s undefined", f.method)
}

func (f File) Test(ctx context.Context) (types.Result, error) {
	switch f.method {
	case "absent":
		return f.absent(ctx, true)
	case "append":
		return f.undef()
	case "directory":
		return f.undef()
	case "missing":
		return f.undef()
	case "prepend":
		return f.undef()
	case "touch":
		return f.undef()
	case "cached":
		return f.cached(ctx, true)
	case "contains":
		return f.undef()
	case "content":
		return f.undef()
	case "managed":
		return f.undef()
	case "present":
		return f.undef()
	case "symlink":
		return f.undef()
	default:
		// TODO define error type
		return types.Result{Succeeded: false, Failed: true, Changed: false, Changes: nil}, fmt.Errorf("method %s undefined", f.method)

	}
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
				Changed: true, Changes: fp,
			}, nil
		} else {
			err = fp.Download(ctx)
			if err != nil {
				return types.Result{Succeeded: false, Failed: true}, err
			}
			return types.Result{Succeeded: true, Failed: false, Changed: true, Changes: fp}, nil
		}
	}
	return types.Result{Succeeded: true, Failed: false, Changed: false}, nil
}

func (f File) absent(ctx context.Context, test bool) (types.Result, error) {
	name, ok := f.params["name"].(string)
	if !ok {
		// TODO join with an error type for missing params
		return types.Result{Succeeded: false, Failed: true}, fmt.Errorf("name not defined")
	}
	name = filepath.Clean(name)
	if name == "" {
		return types.Result{Succeeded: false, Failed: true}, fmt.Errorf("name not defined")
	}
	if name == "/" {
		return types.Result{Succeeded: false, Failed: true}, fmt.Errorf("refusing to delete root")
	}
	_, err := os.Stat(name)
	if errors.Is(err, os.ErrNotExist) {
		return types.Result{Succeeded: true, Failed: false, Changed: false, Changes: nil}, nil
	}
	if err != nil {
		return types.Result{Succeeded: false, Failed: true}, err
	}
	if test {
		return types.Result{Succeeded: true, Failed: false, Changed: true, Changes: nil}, nil
	}
	err = os.Remove(name)
	if err != nil {
		return types.Result{Succeeded: false, Failed: true}, err
	}
	return types.Result{Succeeded: true, Failed: false, Changed: true, Changes: struct{ Removed []string }{Removed: []string{name}}}, nil
}

func (f File) Apply(ctx context.Context) (types.Result, error) {
	switch f.method {
	case "absent":
		return f.absent(ctx, false)
	case "append":
		return f.undef()
	case "directory":
		return f.undef()
	case "missing":
		return f.undef()
	case "prepend":
		return f.undef()
	case "touch":
		return f.undef()
	case "cached":
		return f.cached(ctx, false)
	case "contains":
		return f.undef()
	case "content":
		return f.undef()
	case "managed":
		return f.undef()
	case "present":
		return f.undef()
	case "symlink":
		return f.undef()
	default:
		// TODO define error type
		return types.Result{Succeeded: false, Failed: true, Changed: false, Changes: nil}, fmt.Errorf("method %s undefined", f.method)

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
	case "directory":
		return map[string]string{
			"name": "string", "user": "string", "group": "string", "recurse": "bool",
			"max_depth": "int", "dir_mode": "string", "file_mode": "string", "makedirs": "bool",
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
	case "present":
		return map[string]string{
			"name": "string", "target": "string", "force": "bool", "backupname": "string",
			"makedirs": "bool", "user": "string", "group": "string", "mode": "string",
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
		"present",
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
