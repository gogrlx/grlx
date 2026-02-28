package pkg

import (
	"context"
	"errors"
	"fmt"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/snack"
)

var errGroupNotSupported = errors.New("package manager does not support group operations")

func (p Pkg) groupInstalled(ctx context.Context, test bool) (cook.Result, error) {
	mgr, err := getManager()
	if err != nil {
		return failResult(err)
	}
	grouper, ok := mgr.(snack.Grouper)
	if !ok {
		return failResult(errGroupNotSupported)
	}
	installed, err := grouper.GroupIsInstalled(ctx, p.name)
	if err != nil {
		return failResult(err)
	}
	if installed {
		return cook.Result{
			Succeeded: true,
			Changed:   false,
			Notes:     []fmt.Stringer{note("group already installed: %s", p.name)},
		}, nil
	}
	if test {
		return cook.Result{
			Succeeded: true,
			Changed:   true,
			Notes:     []fmt.Stringer{note("would install group: %s", p.name)},
		}, nil
	}
	err = grouper.GroupInstall(ctx, p.name, snack.WithAssumeYes())
	if err != nil {
		return failResult(err)
	}
	return cook.Result{
		Succeeded: true,
		Changed:   true,
		Notes:     []fmt.Stringer{note("installed group: %s", p.name)},
	}, nil
}
