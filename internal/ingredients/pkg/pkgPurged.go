package pkg

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/snack"
)

func (p Pkg) purged(ctx context.Context, test bool) (cook.Result, error) {
	mgr, err := getManager()
	if err != nil {
		return failResult(err)
	}
	names := p.parseTargetNames()
	var toPurge []string
	for _, name := range names {
		installed, err := mgr.IsInstalled(ctx, name)
		if err != nil {
			return failResult(err)
		}
		if installed {
			toPurge = append(toPurge, name)
		}
	}
	if len(toPurge) == 0 {
		return cook.Result{
			Succeeded: true,
			Changed:   false,
			Notes:     []fmt.Stringer{note("no packages to purge; none are installed")},
		}, nil
	}
	if test {
		return cook.Result{
			Succeeded: true,
			Changed:   true,
			Notes:     []fmt.Stringer{note("would purge: %s", strings.Join(toPurge, ", "))},
		}, nil
	}
	targets := snack.Targets(toPurge...)
	err = mgr.Purge(ctx, targets, snack.WithAssumeYes())
	if err != nil {
		return failResult(err)
	}
	return cook.Result{
		Succeeded: true,
		Changed:   true,
		Notes:     []fmt.Stringer{note("purged: %s", strings.Join(toPurge, ", "))},
	}, nil
}
