package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gogrlx/grlx/v2/internal/ingredients"
	"github.com/gogrlx/grlx/v2/types"
)

var ErrUserMethodUndefined = fmt.Errorf("user method undefined")

type User struct {
	id     string
	method string
	params map[string]interface{}
}

func (u User) Parse(id, method string, params map[string]interface{}) (types.RecipeCooker, error) {
	if params == nil {
		params = map[string]interface{}{}
	}
	return User{
		id: id, method: method,
		params: params,
	}, nil
}

func (u User) validate() error {
	set, err := u.PropertiesForMethod(u.method)
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
				name, ok := u.params[v.Key].(string)
				if !ok {
					return types.ErrMissingName
				}
				if name == "" {
					return types.ErrMissingName
				}

			} else {
				if _, ok := u.params[v.Key]; !ok {
					return fmt.Errorf("missing required property %s", v.Key)
				}
			}
		}
	}
	return nil
}

func (u User) Test(ctx context.Context) (types.Result, error) {
	switch u.method {
	case "present":
		return u.present(ctx, true)
	case "exists":
		return u.exists(ctx, true)
	case "absent":
		return u.absent(ctx, true)
	default:
		return types.Result{Succeeded: false, Failed: true, Changed: false, Notes: nil},
			errors.Join(ErrUserMethodUndefined, fmt.Errorf("method %s undefined", u.method))
	}
}

func (u User) Apply(ctx context.Context) (types.Result, error) {
	switch u.method {
	case "present":
		return u.present(ctx, false)
	case "exists":
		return u.exists(ctx, false)
	case "absent":
		return u.absent(ctx, false)
	default:
		return types.Result{Succeeded: false, Failed: true, Changed: false, Notes: nil},
			errors.Join(ErrUserMethodUndefined, fmt.Errorf("method %s undefined", u.method))

	}
}

func (u User) PropertiesForMethod(method string) (map[string]string, error) {
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
			ingredients.MethodProps{Key: "uid", Type: "string", IsReq: false},
			ingredients.MethodProps{Key: "gid", Type: "string", IsReq: false},
			ingredients.MethodProps{Key: "groups", Type: "[]string", IsReq: false},
			ingredients.MethodProps{Key: "shell", Type: "string", IsReq: false},
			ingredients.MethodProps{Key: "home", Type: "string", IsReq: false},
		}.ToMap(), nil
	default:
		return nil, fmt.Errorf("method %s undefined", method)
	}
}

func (u User) Methods() (string, []string) {
	return "user", []string{"absent", "exists", "present"}
}

func (u User) Properties() (map[string]interface{}, error) {
	m := map[string]interface{}{}
	b, err := json.Marshal(u.params)
	if err != nil {
		return m, err
	}
	err = json.Unmarshal(b, &m)
	return m, err
}

func init() {
	ingredients.RegisterAllMethods(User{})
}
