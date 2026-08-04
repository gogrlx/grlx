package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/ingredients"
)

var ErrCmdMethodUndefined = errors.New("cmd method undefined")

const cmdRunMethod = "run"

var cmdMethodProps = map[string]ingredients.MethodPropsSet{
	cmdRunMethod: {
		ingredients.MethodProps{Key: "name", Type: "string", IsReq: true},
		ingredients.MethodProps{Key: "args", Type: "string", IsReq: false},
		ingredients.MethodProps{Key: "env", Type: "[]string", IsReq: false},
		ingredients.MethodProps{Key: "cwd", Type: "string", IsReq: false},
		ingredients.MethodProps{Key: "runas", Type: "string", IsReq: false},
		ingredients.MethodProps{Key: "path", Type: "string", IsReq: false},
		ingredients.MethodProps{Key: "timeout", Type: "string", IsReq: false},
	},
}

// Compile-time interface check.
var _ cook.RecipeCooker = Cmd{}

type Cmd struct {
	id     string
	method string
	params map[string]interface{}
}

func (c Cmd) Parse(id, method string, params map[string]interface{}) (cook.RecipeCooker, error) {
	if params == nil {
		params = map[string]interface{}{}
	}
	parsed := Cmd{
		id: id, method: method,
		params: params,
	}
	if err := parsed.validate(); err != nil {
		return nil, err
	}
	return parsed, nil
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
					return ingredients.ErrMissingName
				}
				if name == "" {
					return ingredients.ErrMissingName
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

func (c Cmd) Test(ctx context.Context) (cook.Result, error) {
	switch c.method {
	case cmdRunMethod:
		return c.run(ctx, true)
	default:
		return cook.Result{Succeeded: false, Failed: true, Changed: false, Notes: nil},
			errors.Join(ErrCmdMethodUndefined, fmt.Errorf("method %s undefined", c.method))

	}
}

func (c Cmd) Apply(ctx context.Context) (cook.Result, error) {
	switch c.method {
	case cmdRunMethod:
		return c.run(ctx, false)
	default:
		return cook.Result{Succeeded: false, Failed: true, Changed: false, Notes: nil},
			errors.Join(ErrCmdMethodUndefined, fmt.Errorf("method %s undefined", c.method))

	}
}

func (c Cmd) PropertiesForMethod(method string) (map[string]string, error) {
	props, ok := cmdMethodProps[method]
	if !ok {
		return nil, fmt.Errorf("method %s undefined", method)
	}
	return props.ToMap(), nil
}

func (c Cmd) Methods() (string, []string) {
	return "cmd", []string{cmdRunMethod}
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
