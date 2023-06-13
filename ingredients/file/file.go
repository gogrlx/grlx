package file

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

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
	return New(id, method, params)
}

func New(id, method string, params map[string]interface{}) (File, error) {
	return File{id: id, method: method, params: params}, nil
}

func (f File) Test(ctx context.Context) (types.Result, error) {
	switch f.method {
	case "absent":
		return f.absent(ctx, true)
	case "append":
		fallthrough
	case "contains":
		fallthrough
	case "content":
		fallthrough
	case "managed":
		fallthrough
	case "present":
		fallthrough
	case "symlink":
		fallthrough
	default:
		// TODO define error type
		return types.Result{Succeeded: false, Failed: true, Changed: false, Changes: nil}, fmt.Errorf("method %s undefined", f.method)

	}
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
		fallthrough
	case "contains":
		fallthrough
	case "content":
		fallthrough
	case "managed":
		fallthrough
	case "present":
		fallthrough
	case "symlink":
		fallthrough
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
		fallthrough
	case "contains":
		fallthrough
	case "content":
		fallthrough
	case "managed":
		fallthrough
	case "present":
		fallthrough
	case "symlink":
		fallthrough
	default:
		// TODO define error type
		return nil, fmt.Errorf("method %s undefined", f.method)

	}
}

func (f File) Methods() (string, []string) {
	return "file", []string{
		"absent",
		"append",
		"contains",
		"content",
		"managed",
		"present",
		"symlink",
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
	fmt.Println("file initialized")
	ingredients.RegisterAllMethods(File{})
}
