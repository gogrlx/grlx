package pkg

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/snack"
)

func (p Pkg) unheld(ctx context.Context, test bool) (cook.Result, error) {
	mgr, err := getManager()
	if err != nil {
		return failResult(err)
	}
	holder, ok := mgr.(snack.Holder)
	if !ok {
		return failResult(errHoldNotSupported)
	}
	names := p.parseTargetNames()
	if test {
		return cook.Result{
			Succeeded: true,
			Changed:   true,
			Notes:     []fmt.Stringer{note("would unhold: %s", strings.Join(names, ", "))},
		}, nil
	}
	err = holder.Unhold(ctx, names)
	if err != nil {
		return failResult(err)
	}
	return cook.Result{
		Succeeded: true,
		Changed:   true,
		Notes:     []fmt.Stringer{note("unheld: %s", strings.Join(names, ", "))},
	}, nil
}
