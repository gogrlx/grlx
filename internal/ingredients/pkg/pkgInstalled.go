package pkg

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogrlx/grlx/v2/internal/cook"
)

func (p Pkg) installed(ctx context.Context, test bool) (cook.Result, error) {
	mgr, err := getManager()
	if err != nil {
		return failResult(err)
	}
	targets := p.parseTargets()
	var toInstall []string
	var alreadyInstalled []string
	for _, target := range targets {
		installed, err := mgr.IsInstalled(ctx, target.Name)
		if err != nil {
			return failResult(err)
		}
		if installed {
			if target.Version != "" {
				currentVer, err := mgr.Version(ctx, target.Name)
				if err != nil {
					return failResult(err)
				}
				if currentVer != target.Version {
					toInstall = append(toInstall, fmt.Sprintf("%s (version %s -> %s)", target.Name, currentVer, target.Version))
					continue
				}
			}
			if !getBoolProp(p.properties, "reinstall", false) {
				alreadyInstalled = append(alreadyInstalled, target.Name)
				continue
			}
		}
		toInstall = append(toInstall, target.Name)
	}
	if len(toInstall) == 0 {
		return cook.Result{
			Succeeded: true,
			Changed:   false,
			Notes:     []fmt.Stringer{note("all packages are already installed: %s", strings.Join(alreadyInstalled, ", "))},
		}, nil
	}
	if test {
		return cook.Result{
			Succeeded: true,
			Changed:   true,
			Notes:     []fmt.Stringer{note("would install: %s", strings.Join(toInstall, ", "))},
		}, nil
	}
	opts := p.buildOptions()
	err = mgr.Install(ctx, targets, opts...)
	if err != nil {
		return failResult(err)
	}
	return cook.Result{
		Succeeded: true,
		Changed:   true,
		Notes:     []fmt.Stringer{note("installed: %s", strings.Join(toInstall, ", "))},
	}, nil
}
