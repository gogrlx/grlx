package pkg

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/snack"
)

func (p Pkg) latest(ctx context.Context, test bool) (cook.Result, error) {
	mgr, err := getManager()
	if err != nil {
		return failResult(err)
	}
	targets := p.parseTargets()
	var toInstall []string
	var toUpgrade []string
	var upToDate []string
	for _, target := range targets {
		installed, err := mgr.IsInstalled(ctx, target.Name)
		if err != nil {
			return failResult(err)
		}
		if !installed {
			toInstall = append(toInstall, target.Name)
			continue
		}
		// Check if upgrade is available
		if vq, ok := mgr.(snack.VersionQuerier); ok {
			upgradeAvail, err := vq.UpgradeAvailable(ctx, target.Name)
			if err != nil {
				return failResult(err)
			}
			if upgradeAvail {
				toUpgrade = append(toUpgrade, target.Name)
				continue
			}
		}
		upToDate = append(upToDate, target.Name)
	}
	if len(toInstall) == 0 && len(toUpgrade) == 0 {
		return cook.Result{
			Succeeded: true,
			Changed:   false,
			Notes:     []fmt.Stringer{note("all packages are at latest version: %s", strings.Join(upToDate, ", "))},
		}, nil
	}
	var notes []fmt.Stringer
	if len(toInstall) > 0 {
		if test {
			notes = append(notes, note("would install: %s", strings.Join(toInstall, ", ")))
		} else {
			notes = append(notes, note("installed: %s", strings.Join(toInstall, ", ")))
		}
	}
	if len(toUpgrade) > 0 {
		if test {
			notes = append(notes, note("would upgrade: %s", strings.Join(toUpgrade, ", ")))
		} else {
			notes = append(notes, note("upgraded: %s", strings.Join(toUpgrade, ", ")))
		}
	}
	if test {
		return cook.Result{Succeeded: true, Changed: true, Notes: notes}, nil
	}
	opts := p.buildOptions()
	_, err = mgr.Install(ctx, targets, opts...)
	if err != nil {
		return failResult(err)
	}
	return cook.Result{Succeeded: true, Changed: true, Notes: notes}, nil
}
