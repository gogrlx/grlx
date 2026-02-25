package service

import (
	"context"
	"fmt"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/ingredients"
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

func (s Service) Apply(ctx context.Context) (cook.Result, error) {
	sp, err := NewServiceProvider(s.id, s.method, s.properties)
	if err != nil {
		return cook.Result{}, err
	}
	switch s.method {
	case "masked":
		var isMasked bool
		if isMasked, err = sp.IsMasked(ctx); err != nil {
			return cook.Result{Succeeded: false, Failed: true, Changed: false, Notes: nil}, err
		} else if !isMasked {
			err = sp.Mask(ctx)
			if err != nil {
				return cook.Result{Succeeded: false, Failed: true, Changed: false, Notes: nil}, err
			}
			return cook.Result{Succeeded: true, Failed: false, Changed: true, Notes: []fmt.Stringer{cook.SimpleNote(fmt.Sprintf("%s has been masked", s.name))}}, err
		}
		return cook.Result{Succeeded: true, Failed: false, Changed: false, Notes: []fmt.Stringer{cook.SimpleNote(fmt.Sprintf("%s is already masked.", s.name))}}, err
	case "unmasked":
		var isMasked bool
		if isMasked, err = sp.IsMasked(ctx); err != nil {
			return cook.Result{Succeeded: false, Failed: true, Changed: false, Notes: nil}, err
		} else if isMasked {
			err = sp.Unmask(ctx)
			if err != nil {
				return cook.Result{Succeeded: false, Failed: true, Changed: false, Notes: nil}, err
			}
			return cook.Result{Succeeded: true, Failed: false, Changed: true, Notes: []fmt.Stringer{cook.SimpleNote(fmt.Sprintf("%s has been unmasked", s.name))}}, err
		}
		return cook.Result{Succeeded: true, Failed: false, Changed: false, Notes: []fmt.Stringer{cook.SimpleNote(fmt.Sprintf("%s is already unmasked.", s.name))}}, err
	case "running":
		var isRunning bool
		if isRunning, err = sp.IsRunning(ctx); err != nil {
			return cook.Result{Succeeded: false, Failed: true, Changed: false, Notes: nil}, err
		} else if !isRunning {
			err = sp.Start(ctx)
			if err != nil {
				return cook.Result{Succeeded: false, Failed: true, Changed: false, Notes: nil}, err
			}
			return cook.Result{Succeeded: true, Failed: false, Changed: true, Notes: []fmt.Stringer{cook.SimpleNote(fmt.Sprintf("%s has been started", s.name))}}, err
		}
		return cook.Result{Succeeded: true, Failed: false, Changed: false, Notes: []fmt.Stringer{cook.SimpleNote(fmt.Sprintf("%s is already running.", s.name))}}, err
	case "stopped":
		var isRunning bool
		if isRunning, err = sp.IsRunning(ctx); err != nil {
			return cook.Result{Succeeded: false, Failed: true, Changed: false, Notes: nil}, err
		} else if isRunning {
			err = sp.Stop(ctx)
			if err != nil {
				return cook.Result{Succeeded: false, Failed: true, Changed: false, Notes: nil}, err
			}
			return cook.Result{Succeeded: true, Failed: false, Changed: true, Notes: []fmt.Stringer{cook.SimpleNote(fmt.Sprintf("%s has been stopped", s.name))}}, err
		}
		return cook.Result{Succeeded: true, Failed: false, Changed: false, Notes: []fmt.Stringer{cook.SimpleNote(fmt.Sprintf("%s is already stopped.", s.name))}}, err
	case "enabled":
		var isEnabled bool
		if isEnabled, err = sp.IsEnabled(ctx); err != nil {
			return cook.Result{Succeeded: false, Failed: true, Changed: false, Notes: nil}, err
		} else if !isEnabled {
			err = sp.Enable(ctx)
			if err != nil {
				return cook.Result{Succeeded: false, Failed: true, Changed: false, Notes: nil}, err
			}
			return cook.Result{Succeeded: true, Failed: false, Changed: true, Notes: []fmt.Stringer{cook.SimpleNote(fmt.Sprintf("%s has been enabled", s.name))}}, err
		}
		return cook.Result{Succeeded: true, Failed: false, Changed: false, Notes: []fmt.Stringer{cook.SimpleNote(fmt.Sprintf("%s is already enabled.", s.name))}}, err
	case "disabled":
		var isEnabled bool
		if isEnabled, err = sp.IsEnabled(ctx); err != nil {
			return cook.Result{Succeeded: false, Failed: true, Changed: false, Notes: nil}, err
		} else if isEnabled {
			err = sp.Disable(ctx)
			if err != nil {
				return cook.Result{Succeeded: false, Failed: true, Changed: false, Notes: nil}, err
			}
			return cook.Result{Succeeded: true, Failed: false, Changed: true, Notes: []fmt.Stringer{cook.SimpleNote(fmt.Sprintf("%s has been disabled", s.name))}}, err
		}
		return cook.Result{Succeeded: true, Failed: false, Changed: false, Notes: []fmt.Stringer{cook.SimpleNote(fmt.Sprintf("%s is already disabled.", s.name))}}, err
	case "restarted":
		err = sp.Restart(ctx)
		if err != nil {
			return cook.Result{Succeeded: false, Failed: true, Changed: false, Notes: nil}, err
		}
		return cook.Result{Succeeded: true, Failed: false, Changed: true, Notes: []fmt.Stringer{cook.SimpleNote(fmt.Sprintf("%s has been restarted", s.name))}}, err
	default:
		return cook.Result{}, ingredients.ErrInvalidMethod
	}
}

func (s Service) Test(ctx context.Context) (cook.Result, error) {
	sp, err := NewServiceProvider(s.id, s.method, s.properties)
	if err != nil {
		return cook.Result{}, err
	}
	switch s.method {
	case "masked":
		isMasked, err := sp.IsMasked(ctx)
		if err != nil {
			return cook.Result{Succeeded: false, Failed: true}, err
		}
		if !isMasked {
			return cook.Result{Succeeded: true, Changed: true, Notes: []fmt.Stringer{cook.SimpleNote(fmt.Sprintf("%s would be masked", s.name))}}, nil
		}
		return cook.Result{Succeeded: true, Notes: []fmt.Stringer{cook.SimpleNote(fmt.Sprintf("%s is already masked", s.name))}}, nil
	case "unmasked":
		isMasked, err := sp.IsMasked(ctx)
		if err != nil {
			return cook.Result{Succeeded: false, Failed: true}, err
		}
		if isMasked {
			return cook.Result{Succeeded: true, Changed: true, Notes: []fmt.Stringer{cook.SimpleNote(fmt.Sprintf("%s would be unmasked", s.name))}}, nil
		}
		return cook.Result{Succeeded: true, Notes: []fmt.Stringer{cook.SimpleNote(fmt.Sprintf("%s is already unmasked", s.name))}}, nil
	case "running":
		isRunning, err := sp.IsRunning(ctx)
		if err != nil {
			return cook.Result{Succeeded: false, Failed: true}, err
		}
		if !isRunning {
			return cook.Result{Succeeded: true, Changed: true, Notes: []fmt.Stringer{cook.SimpleNote(fmt.Sprintf("%s would be started", s.name))}}, nil
		}
		return cook.Result{Succeeded: true, Notes: []fmt.Stringer{cook.SimpleNote(fmt.Sprintf("%s is already running", s.name))}}, nil
	case "stopped":
		isRunning, err := sp.IsRunning(ctx)
		if err != nil {
			return cook.Result{Succeeded: false, Failed: true}, err
		}
		if isRunning {
			return cook.Result{Succeeded: true, Changed: true, Notes: []fmt.Stringer{cook.SimpleNote(fmt.Sprintf("%s would be stopped", s.name))}}, nil
		}
		return cook.Result{Succeeded: true, Notes: []fmt.Stringer{cook.SimpleNote(fmt.Sprintf("%s is already stopped", s.name))}}, nil
	case "enabled":
		isEnabled, err := sp.IsEnabled(ctx)
		if err != nil {
			return cook.Result{Succeeded: false, Failed: true}, err
		}
		if !isEnabled {
			return cook.Result{Succeeded: true, Changed: true, Notes: []fmt.Stringer{cook.SimpleNote(fmt.Sprintf("%s would be enabled", s.name))}}, nil
		}
		return cook.Result{Succeeded: true, Notes: []fmt.Stringer{cook.SimpleNote(fmt.Sprintf("%s is already enabled", s.name))}}, nil
	case "disabled":
		isEnabled, err := sp.IsEnabled(ctx)
		if err != nil {
			return cook.Result{Succeeded: false, Failed: true}, err
		}
		if isEnabled {
			return cook.Result{Succeeded: true, Changed: true, Notes: []fmt.Stringer{cook.SimpleNote(fmt.Sprintf("%s would be disabled", s.name))}}, nil
		}
		return cook.Result{Succeeded: true, Notes: []fmt.Stringer{cook.SimpleNote(fmt.Sprintf("%s is already disabled", s.name))}}, nil
	case "restarted":
		return cook.Result{Succeeded: true, Changed: true, Notes: []fmt.Stringer{cook.SimpleNote(fmt.Sprintf("%s would be restarted", s.name))}}, nil
	default:
		return cook.Result{}, ingredients.ErrInvalidMethod
	}
}

func (s Service) Properties() (map[string]interface{}, error) {
	return s.properties, nil
}

func (s Service) Parse(id, method string, properties map[string]interface{}) (cook.RecipeCooker, error) {
	nameI, ok := properties["name"]
	if !ok {
		return nil, ingredients.ErrMissingName
	}
	name := ""
	switch nameI := nameI.(type) {
	case string:
		name = nameI
	default:
		return nil, ingredients.ErrMissingName
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
	return nil, ingredients.ErrInvalidMethod
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
