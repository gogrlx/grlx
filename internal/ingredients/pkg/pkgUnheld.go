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
	var toUnhold []string
	for _, name := range names {
		held, err := holder.IsHeld(ctx, name)
		if err != nil {
			return failResult(err)
		}
		if held {
			toUnhold = append(toUnhold, name)
		}
	}
	if len(toUnhold) == 0 {
		return cook.Result{
			Succeeded: true,
			Changed:   false,
			Notes:     []fmt.Stringer{note("no packages are held: %s", strings.Join(names, ", "))},
		}, nil
	}
	if test {
		return cook.Result{
			Succeeded: true,
			Changed:   true,
			Notes:     []fmt.Stringer{note("would unhold: %s", strings.Join(toUnhold, ", "))},
		}, nil
	}
	err = holder.Unhold(ctx, toUnhold)
	if err != nil {
		return failResult(err)
	}
	return cook.Result{
		Succeeded: true,
		Changed:   true,
		Notes:     []fmt.Stringer{note("unheld: %s", strings.Join(toUnhold, ", "))},
	}, nil
}
