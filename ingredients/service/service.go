package service

import (
	"context"
	"fmt"

	"github.com/gogrlx/grlx/ingredients"
	"github.com/gogrlx/grlx/types"
)

type Service struct {
	id         string
	name       string
	method     string
	properties map[string]interface{}
}

func init() {
	ingredients.RegisterAllMethods(Service{})
}

func (s Service) Apply(ctx context.Context) (types.Result, error) {
	sp, err := NewServiceProvider(s.id, s.method, s.properties)
	if err != nil {
		return types.Result{}, err
	}
	switch s.method {
	case "masked":
		var isMasked bool
		if isMasked, err = sp.IsMasked(ctx); err != nil {
			return types.Result{Succeeded: false, Failed: true, Changed: false, Changes: nil}, err
		} else if !isMasked {
			err = sp.Mask(ctx)
			if err != nil {
				return types.Result{Succeeded: false, Failed: true, Changed: false, Changes: nil}, err
			}
			return types.Result{Succeeded: true, Failed: false, Changed: true, Changes: fmt.Sprintf("%s has been masked", s.name)}, err

		}
		return types.Result{Succeeded: true, Failed: false, Changed: false, Changes: fmt.Sprintf("%s is already masked.", s.name)}, err
	case "unmasked":
		var isMasked bool
		if isMasked, err = sp.IsMasked(ctx); err != nil {
			return types.Result{Succeeded: false, Failed: true, Changed: false, Changes: nil}, err
		} else if isMasked {
			err = sp.Unmask(ctx)
			if err != nil {
				return types.Result{Succeeded: false, Failed: true, Changed: false, Changes: nil}, err
			}
			return types.Result{Succeeded: true, Failed: false, Changed: true, Changes: fmt.Sprintf("%s has been unmasked", s.name)}, err
		}
		return types.Result{Succeeded: true, Failed: false, Changed: false, Changes: fmt.Sprintf("%s is already unmasked.", s.name)}, err
	case "running":
		var isRunning bool
		if isRunning, err = sp.IsRunning(ctx); err != nil {
			return types.Result{Succeeded: false, Failed: true, Changed: false, Changes: nil}, err
		} else if !isRunning {
			err = sp.Start(ctx)
			if err != nil {
				return types.Result{Succeeded: false, Failed: true, Changed: false, Changes: nil}, err
			}
			return types.Result{Succeeded: true, Failed: false, Changed: true, Changes: fmt.Sprintf("%s has been started", s.name)}, err
		}
		return types.Result{Succeeded: true, Failed: false, Changed: false, Changes: fmt.Sprintf("%s is already running.", s.name)}, err
	case "stopped":
		var isRunning bool
		if isRunning, err = sp.IsRunning(ctx); err != nil {
			return types.Result{Succeeded: false, Failed: true, Changed: false, Changes: nil}, err
		} else if isRunning {
			err = sp.Stop(ctx)
			if err != nil {
				return types.Result{Succeeded: false, Failed: true, Changed: false, Changes: nil}, err
			}
			return types.Result{Succeeded: true, Failed: false, Changed: true, Changes: fmt.Sprintf("%s has been stopped", s.name)}, err
		}
		return types.Result{Succeeded: true, Failed: false, Changed: false, Changes: fmt.Sprintf("%s is already stopped.", s.name)}, err
	case "enabled":
		var isEnabled bool
		if isEnabled, err = sp.IsEnabled(ctx); err != nil {
			return types.Result{Succeeded: false, Failed: true, Changed: false, Changes: nil}, err
		} else if !isEnabled {
			err = sp.Enable(ctx)
			if err != nil {
				return types.Result{Succeeded: false, Failed: true, Changed: false, Changes: nil}, err
			}
			return types.Result{Succeeded: true, Failed: false, Changed: true, Changes: fmt.Sprintf("%s has been enabled", s.name)}, err
		}
		return types.Result{Succeeded: true, Failed: false, Changed: false, Changes: fmt.Sprintf("%s is already enabled.", s.name)}, err
	case "disabled":
		var isEnabled bool
		if isEnabled, err = sp.IsEnabled(ctx); err != nil {
			return types.Result{Succeeded: false, Failed: true, Changed: false, Changes: nil}, err
		} else if isEnabled {
			err = sp.Disable(ctx)
			if err != nil {
				return types.Result{Succeeded: false, Failed: true, Changed: false, Changes: nil}, err
			}
			return types.Result{Succeeded: true, Failed: false, Changed: true, Changes: fmt.Sprintf("%s has been disabled", s.name)}, err
		}
		return types.Result{Succeeded: true, Failed: false, Changed: false, Changes: fmt.Sprintf("%s is already disabled.", s.name)}, err
	case "restarted":
		err = sp.Restart(ctx)
		if err != nil {
			return types.Result{Succeeded: false, Failed: true, Changed: false, Changes: nil}, err
		}
		return types.Result{Succeeded: true, Failed: false, Changed: true, Changes: fmt.Sprintf("%s has been restarted", s.name)}, err
	default:
		return types.Result{}, types.ErrInvalidMethod
	}
}

func (s Service) Test(context.Context) (types.Result, error) {
	// TODO implement test applies
	switch s.method {
	case "masked":
	case "unmasked":
	case "running":
	case "stopped":
	case "enabled":
	case "disabled":
	case "restarted":
	}
	return types.Result{}, nil
}

func (s Service) Properties() (map[string]interface{}, error) {
	return s.properties, nil
}

func (s Service) Parse(id, method string, properties map[string]interface{}) (types.RecipeCooker, error) {
	nameI, ok := properties["name"]
	if !ok {
		return nil, types.ErrMissingName
	}
	name := ""
	switch nameI := nameI.(type) {
	case string:
		name = nameI
	default:
		return nil, types.ErrMissingName
	}
	_, methods := s.Methods()
	for _, smethod := range methods {
		if smethod == method {
			return Service{
				id:         id,
				name:       name,
				method:     method,
				properties: properties,
			}, nil
		}
	}
	return nil, types.ErrInvalidMethod
}

func (s Service) Methods() (string, []string) {
	return "service", []string{
		"disabled",
		"enabled",
		"masked",
		"restarted",
		"running",
		"stopped",
		"unmasked",
	}
}

func (s Service) PropertiesForMethod(method string) (map[string]string, error) {
	return nil, nil
}
