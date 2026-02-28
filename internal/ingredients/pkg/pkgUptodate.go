package pkg

import (
	"context"
	"fmt"

	"github.com/gogrlx/grlx/v2/internal/cook"
)

func (p Pkg) uptodate(ctx context.Context, test bool) (cook.Result, error) {
	mgr, err := getManager()
	if err != nil {
		return failResult(err)
	}
	if test {
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
