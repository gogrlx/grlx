package pkg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/snack"
)

var errHoldNotSupported = errors.New("package manager does not support hold/unhold operations")

func (p Pkg) held(ctx context.Context, test bool) (cook.Result, error) {
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
			Notes:     []fmt.Stringer{note("would hold: %s", strings.Join(names, ", "))},
		}, nil
	}
	err = holder.Hold(ctx, names)
	if err != nil {
		return failResult(err)
	}
	return cook.Result{
		Succeeded: true,
		Changed:   true,
		Notes:     []fmt.Stringer{note("held: %s", strings.Join(names, ", "))},
	}, nil
}
