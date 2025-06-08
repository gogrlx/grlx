package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gogrlx/grlx/v2/ingredients"
	"github.com/gogrlx/grlx/v2/types"
)

var ErrCmdMethodUndefined = fmt.Errorf("cmd method undefined")

type Cmd struct {
	id     string
	method string
	params map[string]interface{}
}

// TODO parse out the map here
func (c Cmd) Parse(id, method string, params map[string]interface{}) (types.RecipeCooker, error) {
	if params == nil {
		params = map[string]interface{}{}
	}
	return Cmd{
		id: id, method: method,
		params: params,
	}, nil
}

func (c Cmd) validate() error {
	set, err := c.PropertiesForMethod(c.method)
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
				name, ok := c.params[v.Key].(string)
				if !ok {
					return types.ErrMissingName
				}
				if name == "" {
					return types.ErrMissingName
				}

			} else {
				if _, ok := c.params[v.Key]; !ok {
					return fmt.Errorf("missing required property %s", v.Key)
				}
			}
		}
	}
	return nil
}

func (c Cmd) Test(ctx context.Context) (types.Result, error) {
	switch c.method {
	case "run":
		return c.run(ctx, true)
	default:
		return types.Result{Succeeded: false, Failed: true, Changed: false, Notes: nil},
			errors.Join(ErrCmdMethodUndefined, fmt.Errorf("method %s undefined", c.method))

	}
}

func (c Cmd) Apply(ctx context.Context) (types.Result, error) {
	switch c.method {
	case "run":
		return c.run(ctx, false)
	default:
		return types.Result{Succeeded: false, Failed: true, Changed: false, Notes: nil},
			errors.Join(ErrCmdMethodUndefined, fmt.Errorf("method %s undefined", c.method))

	}
}

// TODO create map for method: type
func (c Cmd) PropertiesForMethod(method string) (map[string]string, error) {
	switch method {
	case "run":
		return ingredients.MethodPropsSet{
			ingredients.MethodProps{Key: "name", Type: "string", IsReq: true},
			ingredients.MethodProps{Key: "args", Type: "string", IsReq: false},
			ingredients.MethodProps{Key: "env", Type: "[]string", IsReq: false},
			ingredients.MethodProps{Key: "cwd", Type: "string", IsReq: false},
			ingredients.MethodProps{Key: "runas", Type: "string", IsReq: false},
			ingredients.MethodProps{Key: "path", Type: "string", IsReq: false},
			ingredients.MethodProps{Key: "timeout", Type: "string", IsReq: false},
		}.ToMap(), nil
	default:
		return nil, fmt.Errorf("method %s undefined", method)
	}
}

func (c Cmd) Methods() (string, []string) {
	return "cmd", []string{"run"}
}

func (c Cmd) Properties() (map[string]interface{}, error) {
	m := map[string]interface{}{}
	b, err := json.Marshal(c.params)
	if err != nil {
		return m, err
	}
	err = json.Unmarshal(b, &m)
	return m, err
}

func init() {
	ingredients.RegisterAllMethods(Cmd{})
}
