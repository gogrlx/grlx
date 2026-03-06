package pkg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/snack"
)

var errUpgradeNotSupported = errors.New("package manager does not support targeted package upgrades")

func (p Pkg) upgraded(ctx context.Context, test bool) (cook.Result, error) {
	mgr, err := getManager()
	if err != nil {
		return failResult(err)
	}
	upgrader, ok := mgr.(snack.PackageUpgrader)
	if !ok {
		return failResult(errUpgradeNotSupported)
	}
	targets := p.parseTargets()
	names := snack.TargetNames(targets)
	// Check which packages actually need upgrading.
	var toUpgrade []string
	var upToDate []string
	if vq, ok := mgr.(snack.VersionQuerier); ok {
		for _, name := range names {
			installed, err := mgr.IsInstalled(ctx, name)
			if err != nil {
				return failResult(err)
			}
			if !installed {
				continue
			}
			avail, err := vq.UpgradeAvailable(ctx, name)
			if err != nil {
				return failResult(err)
			}
			if avail {
				toUpgrade = append(toUpgrade, name)
			} else {
				upToDate = append(upToDate, name)
			}
		}
	} else {
		// Without VersionQuerier, assume all targets need upgrading.
		toUpgrade = names
	}
	if len(toUpgrade) == 0 {
		return cook.Result{
			Succeeded: true,
			Changed:   false,
			Notes:     []fmt.Stringer{note("all packages are at latest version: %s", strings.Join(upToDate, ", "))},
		}, nil
	}
	if test {
		return cook.Result{
			Succeeded: true,
			Changed:   true,
			Notes:     []fmt.Stringer{note("would upgrade: %s", strings.Join(toUpgrade, ", "))},
		}, nil
	}
	opts := p.buildOptions()
	_, err = upgrader.UpgradePackages(ctx, targets, opts...)
	if err != nil {
		return failResult(err)
	}
	return cook.Result{
		Succeeded: true,
		Changed:   true,
		Notes:     []fmt.Stringer{note("upgraded: %s", strings.Join(toUpgrade, ", "))},
	}, nil
}
