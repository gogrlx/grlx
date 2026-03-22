package pkg

import (
	"context"
	"fmt"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/ingredients"
	"github.com/gogrlx/snack"
	"github.com/gogrlx/snack/detect"
)

// Pkg implements cook.RecipeCooker for package management operations.
type Pkg struct {
	id         string
	method     string
	name       string
	properties map[string]interface{}
}

func init() {
	ingredients.RegisterAllMethods(Pkg{})
}

func (p Pkg) Methods() (string, []string) {
	return "pkg", []string{
		"cleaned",
		"group_installed",
		"held",
		"installed",
		"key_managed",
		"latest",
		"purged",
		"removed",
		"repo_managed",
		"unheld",
		"upgraded",
		"uptodate",
	}
}

func (p Pkg) Parse(id, method string, properties map[string]interface{}) (cook.RecipeCooker, error) {
	nameI, ok := properties["name"]
	if !ok {
		return nil, ingredients.ErrMissingName
	}
	name, ok := nameI.(string)
	if !ok || name == "" {
		return nil, ingredients.ErrMissingName
	}
	_, methods := p.Methods()
	for _, m := range methods {
		if m == method {
			return Pkg{
				id:         id,
				name:       name,
				method:     method,
				properties: properties,
			}, nil
		}
	}
	return nil, ingredients.ErrInvalidMethod
}

func (p Pkg) Properties() (map[string]interface{}, error) {
	return p.properties, nil
}

func (p Pkg) PropertiesForMethod(method string) (map[string]string, error) {
	switch method {
	case "cleaned":
		return map[string]string{
			"name":       "string,req",
			"autoremove": "bool,opt",
		}, nil
	case "installed":
		return map[string]string{
			"name":      "string,req",
			"version":   "string,opt",
			"fromrepo":  "string,opt",
			"pkgs":      "[]string,opt",
			"refresh":   "bool,opt",
			"reinstall": "bool,opt",
		}, nil
	case "latest":
		return map[string]string{
			"name":     "string,req",
			"fromrepo": "string,opt",
			"pkgs":     "[]string,opt",
			"refresh":  "bool,opt",
		}, nil
	case "removed":
		return map[string]string{
			"name": "string,req",
			"pkgs": "[]string,opt",
		}, nil
	case "purged":
		return map[string]string{
			"name": "string,req",
			"pkgs": "[]string,opt",
		}, nil
	case "uptodate":
		return map[string]string{
			"name":    "string,req",
			"refresh": "bool,opt",
		}, nil
	case "held":
		return map[string]string{
			"name": "string,req",
			"pkgs": "[]string,opt",
		}, nil
	case "unheld":
		return map[string]string{
			"name": "string,req",
			"pkgs": "[]string,opt",
		}, nil
	case "group_installed":
		return map[string]string{
			"name": "string,req",
		}, nil
	case "repo_managed":
		return map[string]string{
			"name":   "string,req",
			"url":    "string,opt",
			"absent": "bool,opt",
		}, nil
	case "key_managed":
		return map[string]string{
			"name":   "string,req",
			"absent": "bool,opt",
		}, nil
	case "upgraded":
		return map[string]string{
			"name":     "string,req",
			"fromrepo": "string,opt",
			"pkgs":     "[]string,opt",
			"refresh":  "bool,opt",
		}, nil
	default:
		return nil, ingredients.ErrInvalidMethod
	}
}

func (p Pkg) Apply(ctx context.Context) (cook.Result, error) {
	switch p.method {
	case "cleaned":
		return p.cleaned(ctx, false)
	case "installed":
		return p.installed(ctx, false)
	case "latest":
		return p.latest(ctx, false)
	case "removed":
		return p.removed(ctx, false)
	case "purged":
		return p.purged(ctx, false)
	case "uptodate":
		return p.uptodate(ctx, false)
	case "held":
		return p.held(ctx, false)
	case "unheld":
		return p.unheld(ctx, false)
	case "group_installed":
		return p.groupInstalled(ctx, false)
	case "repo_managed":
		return p.repoManaged(ctx, false)
	case "key_managed":
		return p.keyManaged(ctx, false)
	case "upgraded":
		return p.upgraded(ctx, false)
	default:
		return cook.Result{}, ingredients.ErrInvalidMethod
	}
}

func (p Pkg) Test(ctx context.Context) (cook.Result, error) {
	switch p.method {
	case "cleaned":
		return p.cleaned(ctx, true)
	case "installed":
		return p.installed(ctx, true)
	case "latest":
		return p.latest(ctx, true)
	case "removed":
		return p.removed(ctx, true)
	case "purged":
		return p.purged(ctx, true)
	case "uptodate":
		return p.uptodate(ctx, true)
	case "held":
		return p.held(ctx, true)
	case "unheld":
		return p.unheld(ctx, true)
	case "group_installed":
		return p.groupInstalled(ctx, true)
	case "repo_managed":
		return p.repoManaged(ctx, true)
	case "key_managed":
		return p.keyManaged(ctx, true)
	case "upgraded":
		return p.upgraded(ctx, true)
	default:
		return cook.Result{}, ingredients.ErrInvalidMethod
	}
}

// getManager returns the system's default package manager.
// It is a variable so tests can substitute a mock implementation.
var getManager = func() (snack.Manager, error) {
	return detect.Default()
}

// parseTargets extracts package targets from properties.
// If "pkgs" is set, it parses that list (supporting name:version maps).
// Otherwise, it uses "name" (and optional "version"/"fromrepo").
func (p Pkg) parseTargets() []snack.Target {
	if pkgsI, ok := p.properties["pkgs"]; ok {
		return parsePkgsList(pkgsI)
	}
	target := snack.Target{Name: p.name}
	if ver, ok := p.properties["version"].(string); ok {
		target.Version = ver
	}
	if repo, ok := p.properties["fromrepo"].(string); ok {
		target.FromRepo = repo
	}
	return []snack.Target{target}
}

// parseTargetNames extracts just the package names from targets.
func (p Pkg) parseTargetNames() []string {
	targets := p.parseTargets()
	return snack.TargetNames(targets)
}

// parsePkgsList parses the "pkgs" property into a slice of snack.Target.
// Supports both plain strings and map entries like {"redis": ">=7.0"}.
func parsePkgsList(pkgsI interface{}) []snack.Target {
	pkgsList, ok := pkgsI.([]interface{})
	if !ok {
		return nil
	}
	var targets []snack.Target
	for _, item := range pkgsList {
		switch val := item.(type) {
		case string:
			targets = append(targets, snack.Target{Name: val})
		case map[string]interface{}:
			for name, verI := range val {
				target := snack.Target{Name: name}
				if ver, ok := verI.(string); ok {
					target.Version = ver
				}
				targets = append(targets, target)
			}
		case map[interface{}]interface{}:
			for nameI, verI := range val {
				name, ok := nameI.(string)
				if !ok {
					continue
				}
				target := snack.Target{Name: name}
				if ver, ok := verI.(string); ok {
					target.Version = ver
				}
				targets = append(targets, target)
			}
		}
	}
	return targets
}

// getBoolProp extracts a boolean property with a default value.
func getBoolProp(props map[string]interface{}, key string, defaultVal bool) bool {
	val, ok := props[key]
	if !ok {
		return defaultVal
	}
	bval, ok := val.(bool)
	if !ok {
		return defaultVal
	}
	return bval
}

// buildOptions builds snack.Option slice from properties.
func (p Pkg) buildOptions() []snack.Option {
	var opts []snack.Option
	opts = append(opts, snack.WithAssumeYes())
	if getBoolProp(p.properties, "refresh", false) {
		opts = append(opts, snack.WithRefresh())
	}
	if repo, ok := p.properties["fromrepo"].(string); ok && repo != "" {
		opts = append(opts, snack.WithFromRepo(repo))
	}
	if getBoolProp(p.properties, "reinstall", false) {
		opts = append(opts, snack.WithReinstall())
	}
	return opts
}

// failResult returns a standard failure result.
func failResult(err error) (cook.Result, error) {
	return cook.Result{Succeeded: false, Failed: true, Changed: false}, err
}

// note creates a SimpleNote from a format string.
func note(format string, args ...interface{}) fmt.Stringer {
	return cook.SimpleNote(fmt.Sprintf(format, args...))
}
