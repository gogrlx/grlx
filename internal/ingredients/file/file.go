package file

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/ingredients"
)

var ErrFileMethodUndefined = errors.New("file method undefined")

type File struct {
	id     string
	method string
	params map[string]interface{}
}

// TODO error check, set id, properly parse
func (f File) Parse(id, method string, params map[string]interface{}) (cook.RecipeCooker, error) {
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
					return ingredients.ErrMissingName
				}
				if name == "" {
					return ingredients.ErrMissingName
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
func (f File) undef() (cook.Result, error) {
	return cook.Result{
		Succeeded: false, Failed: true,
		Changed: false, Notes: nil,
	}, errors.Join(ErrFileMethodUndefined, fmt.Errorf("method %s undefined", f.method))
}

func (f File) Test(ctx context.Context) (cook.Result, error) {
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
		return "", ingredients.ErrMissingName
	}
	basename := filepath.Base(name)
	if sv, okSkip := f.params["skip_verify"].(bool); okSkip && sv {
		return filepath.Join(config.CacheDir, "skip_"+basename), nil
	}
	hash, ok := f.params["hash"].(string)
	if !ok || hash == "" {
		return "", ErrMissingHash
	}
	return filepath.Join(config.CacheDir, hash), nil
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

func (f File) Apply(ctx context.Context) (cook.Result, error) {
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
		return cook.Result{
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
			ingredients.MethodProps{Key: "name", Type: "string", IsReq: true, Description: "the name/path of the file to delete"},
		}.ToMap(), nil
	case "append":
		return ingredients.MethodPropsSet{
			ingredients.MethodProps{Key: "name", Type: "string", IsReq: true, Description: "the name/path of the file to append to"},
			ingredients.MethodProps{Key: "makedirs", Type: "bool", IsReq: false, Description: "create parent directories if they do not exist"},
			ingredients.MethodProps{Key: "source", Type: "string", IsReq: false, Description: "append lines from a file sourced from this path/URL"},
			ingredients.MethodProps{Key: "source_hash", Type: "string", IsReq: false, Description: "hash to verify the file specified by source"},
			ingredients.MethodProps{Key: "source_hashes", Type: "[]string", IsReq: false, Description: "corresponding hashes for sources"},
			ingredients.MethodProps{Key: "sources", Type: "[]string", IsReq: false, Description: "source, but in list format"},
			ingredients.MethodProps{Key: "template", Type: "bool", IsReq: false, Description: "whether to render the file as a template before appending (experimental)"},
			ingredients.MethodProps{Key: "text", Type: "[]string", IsReq: false, Description: "the text to append to the file"},
		}.ToMap(), nil
	case "cached":
		return ingredients.MethodPropsSet{
			ingredients.MethodProps{Key: "name", Type: "string", IsReq: true},
			ingredients.MethodProps{Key: "hash", Type: "string", IsReq: false},
			ingredients.MethodProps{Key: "skip_verify", Type: "bool", IsReq: false},
			ingredients.MethodProps{Key: "source", Type: "string", IsReq: true},
		}.ToMap(), nil
	case "contains":
		return ingredients.MethodPropsSet{
			ingredients.MethodProps{Key: "name", Type: "string", IsReq: true},
			ingredients.MethodProps{Key: "source", Type: "string", IsReq: true},
			ingredients.MethodProps{Key: "source_hash", Type: "string", IsReq: false},
			ingredients.MethodProps{Key: "source_hashes", Type: "[]string", IsReq: false},
			ingredients.MethodProps{Key: "sources", Type: "[]string", IsReq: false},
			ingredients.MethodProps{Key: "template", Type: "bool", IsReq: false},
			ingredients.MethodProps{Key: "text", Type: "[]string", IsReq: false},
		}.ToMap(), nil
	case "content":
		return ingredients.MethodPropsSet{
			ingredients.MethodProps{Key: "name", Type: "string", IsReq: true},
			ingredients.MethodProps{Key: "text", Type: "[]string", IsReq: false},
			ingredients.MethodProps{Key: "makedirs", Type: "bool", IsReq: false},
			ingredients.MethodProps{Key: "source", Type: "string", IsReq: false},
			ingredients.MethodProps{Key: "source_hash", Type: "string", IsReq: false},
			ingredients.MethodProps{Key: "template", Type: "bool", IsReq: false},
			ingredients.MethodProps{Key: "sources", Type: "[]string", IsReq: false},
			ingredients.MethodProps{Key: "source_hashes", Type: "[]string", IsReq: false},
		}.ToMap(), nil
	case "directory":
		return ingredients.MethodPropsSet{
			ingredients.MethodProps{Key: "name", Type: "string", IsReq: true},
			ingredients.MethodProps{Key: "user", Type: "string", IsReq: false},
			ingredients.MethodProps{Key: "group", Type: "string", IsReq: false},
			ingredients.MethodProps{Key: "recurse", Type: "bool", IsReq: false},
			ingredients.MethodProps{Key: "dir_mode", Type: "string", IsReq: false},
			ingredients.MethodProps{Key: "file_mode", Type: "string", IsReq: false},
			ingredients.MethodProps{Key: "makedirs", Type: "bool", IsReq: false},
		}.ToMap(), nil
	case "managed":
		return ingredients.MethodPropsSet{
			ingredients.MethodProps{Key: "name", Type: "string", IsReq: true},
			ingredients.MethodProps{Key: "source", Type: "string", IsReq: true},
			ingredients.MethodProps{Key: "source_hash", Type: "string", IsReq: false},
			ingredients.MethodProps{Key: "user", Type: "string", IsReq: false},
			ingredients.MethodProps{Key: "group", Type: "string", IsReq: false},
			ingredients.MethodProps{Key: "mode", Type: "string", IsReq: false},
			ingredients.MethodProps{Key: "template", Type: "bool", IsReq: false},
			ingredients.MethodProps{Key: "makedirs", Type: "bool", IsReq: false},
			ingredients.MethodProps{Key: "dir_mode", Type: "string", IsReq: false},
			ingredients.MethodProps{Key: "sources", Type: "[]string", IsReq: true},
			ingredients.MethodProps{Key: "source_hashes", Type: "[]string", IsReq: false},
		}.ToMap(), nil
	case "missing":
		return ingredients.MethodPropsSet{
			ingredients.MethodProps{Key: "name", Type: "string", IsReq: true},
		}.ToMap(), nil
	case "prepend":
		return ingredients.MethodPropsSet{
			ingredients.MethodProps{Key: "name", Type: "string", IsReq: true},
			ingredients.MethodProps{Key: "text", Type: "[]string", IsReq: false},
			ingredients.MethodProps{Key: "makedirs", Type: "bool", IsReq: false},
			ingredients.MethodProps{Key: "source", Type: "string", IsReq: false},
			ingredients.MethodProps{Key: "source_hash", Type: "string", IsReq: false},
			ingredients.MethodProps{Key: "template", Type: "bool", IsReq: false},
			ingredients.MethodProps{Key: "sources", Type: "[]string", IsReq: false},
			ingredients.MethodProps{Key: "source_hashes", Type: "[]string", IsReq: false},
		}.ToMap(), nil
	case "exists":
		return ingredients.MethodPropsSet{
			ingredients.MethodProps{Key: "name", Type: "string", IsReq: true},
		}.ToMap(), nil
	case "symlink":
		return ingredients.MethodPropsSet{
			ingredients.MethodProps{Key: "name", Type: "string", IsReq: true},
			ingredients.MethodProps{Key: "target", Type: "string", IsReq: true},
			ingredients.MethodProps{Key: "makedirs", Type: "bool", IsReq: false},
			ingredients.MethodProps{Key: "user", Type: "string", IsReq: false},
			ingredients.MethodProps{Key: "group", Type: "string", IsReq: false},
			ingredients.MethodProps{Key: "mode", Type: "string", IsReq: false},
		}.ToMap(), nil
	case "touch":
		return ingredients.MethodPropsSet{
			ingredients.MethodProps{Key: "name", Type: "string", IsReq: true},
			ingredients.MethodProps{Key: "atime", Type: "string", IsReq: false},
			ingredients.MethodProps{Key: "mtime", Type: "string", IsReq: false},
			ingredients.MethodProps{Key: "makedirs", Type: "bool", IsReq: false},
		}.ToMap(), nil
	default:
		return nil, errors.Join(ErrFileMethodUndefined, fmt.Errorf("method %s undefined", f.method))
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
