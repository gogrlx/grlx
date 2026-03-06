package pkg

import (
	"context"
	"errors"
	"fmt"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/snack"
)

var errCleanNotSupported = errors.New("package manager does not support clean operations")

func (p Pkg) cleaned(ctx context.Context, test bool) (cook.Result, error) {
	mgr, err := getManager()
	if err != nil {
		return failResult(err)
	}
	cleaner, ok := mgr.(snack.Cleaner)
	if !ok {
		return failResult(errCleanNotSupported)
	}
	autoremove := getBoolProp(p.properties, "autoremove", false)
	if test {
		var notes []fmt.Stringer
		notes = append(notes, note("would clean package cache"))
		if autoremove {
			notes = append(notes, note("would autoremove orphaned packages"))
		}
		return cook.Result{Succeeded: true, Changed: true, Notes: notes}, nil
	}
	var notes []fmt.Stringer
	err = cleaner.Clean(ctx)
	if err != nil {
		return failResult(err)
	}
	notes = append(notes, note("cleaned package cache"))
	if autoremove {
		err = cleaner.Autoremove(ctx, snack.WithAssumeYes())
		if err != nil {
			return failResult(err)
		}
		notes = append(notes, note("autoremoved orphaned packages"))
	}
	return cook.Result{Succeeded: true, Changed: true, Notes: notes}, nil
}
