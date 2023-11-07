package group

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gogrlx/grlx/ingredients"
	"github.com/gogrlx/grlx/types"
)

var ErrGroupMethodUndefined = fmt.Errorf("group method undefined")

type Group struct {
	id     string
	method string
	params map[string]interface{}
}

func (g Group) Parse(id, method string, params map[string]interface{}) (types.RecipeCooker, error) {
	if params == nil {
		params = map[string]interface{}{}
	}
	return Group{
		id: id, method: method,
		params: params,
	}, nil
}

func (g Group) validate() error {
	set, err := g.PropertiesForMethod(g.method)
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
				name, ok := g.params[v.Key].(string)
				if !ok {
					return types.ErrMissingName
				}
				if name == "" {
					return types.ErrMissingName
				}

			} else {
				if _, ok := g.params[v.Key]; !ok {
					return fmt.Errorf("missing required property %s", v.Key)
				}
			}
		}
	}
	return nil
}

func (g Group) Test(ctx context.Context) (types.Result, error) {
	switch g.method {
	case "present":
		return g.present(ctx, true)
	case "exists":
		return g.exists(ctx, true)
	case "absent":
		return g.absent(ctx, true)
	default:
		return types.Result{Succeeded: false, Failed: true, Changed: false, Notes: nil},
			errors.Join(ErrGroupMethodUndefined, fmt.Errorf("method %s undefined", g.method))
	}
}

func (g Group) Apply(ctx context.Context) (types.Result, error) {
	switch g.method {
	case "present":
		return g.present(ctx, false)
	case "exists":
		return g.exists(ctx, false)
	case "absent":
		return g.absent(ctx, false)
	default:
		return types.Result{Succeeded: false, Failed: true, Changed: false, Notes: nil},
			errors.Join(ErrGroupMethodUndefined, fmt.Errorf("method %s undefined", g.method))

	}
}

func (g Group) PropertiesForMethod(method string) (map[string]string, error) {
	switch method {
	case "absent":
		return ingredients.MethodPropsSet{
			ingredients.MethodProps{Key: "name", Type: "string", IsReq: true},
		}.ToMap(), nil
	case "exists":
		return ingredients.MethodPropsSet{
			ingredients.MethodProps{Key: "name", Type: "string", IsReq: true},
		}.ToMap(), nil
	case "present":
		return ingredients.MethodPropsSet{
			ingredients.MethodProps{Key: "name", Type: "string", IsReq: true},
			ingredients.MethodProps{Key: "gid", Type: "string", IsReq: false},
		}.ToMap(), nil
	default:
		return nil, fmt.Errorf("method %s undefined", method)
	}
}

func (u Group) Methods() (string, []string) {
	return "group", []string{"absent", "exists", "present"}
}

func (g Group) Properties() (map[string]interface{}, error) {
	m := map[string]interface{}{}
	b, err := json.Marshal(g.params)
	if err != nil {
		return m, err
	}
	err = json.Unmarshal(b, &m)
	return m, err
}

func init() {
	ingredients.RegisterAllMethods(Group{})
}
