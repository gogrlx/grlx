package pkg

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/snack"
)

func (p Pkg) uptodate(ctx context.Context, test bool) (cook.Result, error) {
	mgr, err := getManager()
	if err != nil {
		return failResult(err)
	}
	if test {
		// If the manager supports version queries, list what would be upgraded.
		if vq, ok := mgr.(snack.VersionQuerier); ok {
			upgrades, err := vq.ListUpgrades(ctx)
			if err != nil {
				return failResult(err)
			}
			if len(upgrades) == 0 {
				return cook.Result{
					Succeeded: true,
					Changed:   false,
					Notes:     []fmt.Stringer{note("all packages are up to date")},
				}, nil
			}
			names := make([]string, len(upgrades))
			for i, pkg := range upgrades {
				names[i] = pkg.Name
			}
			return cook.Result{
				Succeeded: true,
				Changed:   true,
				Notes:     []fmt.Stringer{note("would upgrade %d packages: %s", len(upgrades), strings.Join(names, ", "))},
			}, nil
		}
		return cook.Result{
			Succeeded: true,
			Changed:   true,
			Notes:     []fmt.Stringer{note("would run system upgrade")},
		}, nil
	}
	opts := p.buildOptions()
	err = mgr.Upgrade(ctx, opts...)
	if err != nil {
		return failResult(err)
	}
	return cook.Result{
		Succeeded: true,
		Changed:   true,
		Notes:     []fmt.Stringer{note("system upgrade completed")},
	}, nil
}
