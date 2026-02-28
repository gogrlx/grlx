package pkg

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/snack"
)

func (p Pkg) removed(ctx context.Context, test bool) (cook.Result, error) {
	mgr, err := getManager()
	if err != nil {
		return failResult(err)
	}
	names := p.parseTargetNames()
	var toRemove []string
	for _, name := range names {
		installed, err := mgr.IsInstalled(ctx, name)
		if err != nil {
			return failResult(err)
		}
		if installed {
			toRemove = append(toRemove, name)
		}
	}
	if len(toRemove) == 0 {
		return cook.Result{
			Succeeded: true,
			Changed:   false,
			Notes:     []fmt.Stringer{note("no packages to remove; none are installed")},
		}, nil
	}
	if test {
		return cook.Result{
			Succeeded: true,
			Changed:   true,
			Notes:     []fmt.Stringer{note("would remove: %s", strings.Join(toRemove, ", "))},
		}, nil
	}
	targets := snack.Targets(toRemove...)
	_, err = mgr.Remove(ctx, targets, snack.WithAssumeYes())
	if err != nil {
		return failResult(err)
	}
	return cook.Result{
		Succeeded: true,
		Changed:   true,
		Notes:     []fmt.Stringer{note("removed: %s", strings.Join(toRemove, ", "))},
	}, nil
}
