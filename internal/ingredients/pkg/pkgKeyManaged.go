package pkg

import (
	"context"
	"errors"
	"fmt"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/snack"
)

var errKeyNotSupported = errors.New("package manager does not support key management")

func (p Pkg) keyManaged(ctx context.Context, test bool) (cook.Result, error) {
	mgr, err := getManager()
	if err != nil {
		return failResult(err)
	}
	keyMgr, ok := mgr.(snack.KeyManager)
	if !ok {
		return failResult(errKeyNotSupported)
	}
	absent := getBoolProp(p.properties, "absent", false)
	if absent {
		return p.keyRemoved(ctx, keyMgr, test)
	}
	return p.keyAdded(ctx, keyMgr, test)
}

func (p Pkg) keyAdded(ctx context.Context, keyMgr snack.KeyManager, test bool) (cook.Result, error) {
	// Check if key already present.
	keys, err := keyMgr.ListKeys(ctx)
	if err != nil {
		return failResult(err)
	}
	for _, k := range keys {
		if k == p.name {
			return cook.Result{
				Succeeded: true,
				Changed:   false,
				Notes:     []fmt.Stringer{note("key already present: %s", p.name)},
			}, nil
		}
	}
	if test {
		return cook.Result{
			Succeeded: true,
			Changed:   true,
			Notes:     []fmt.Stringer{note("would add key: %s", p.name)},
		}, nil
	}
	err = keyMgr.AddKey(ctx, p.name)
	if err != nil {
		return failResult(err)
	}
	return cook.Result{
		Succeeded: true,
		Changed:   true,
		Notes:     []fmt.Stringer{note("added key: %s", p.name)},
	}, nil
}

func (p Pkg) keyRemoved(ctx context.Context, keyMgr snack.KeyManager, test bool) (cook.Result, error) {
	// Check if key exists.
	keys, err := keyMgr.ListKeys(ctx)
	if err != nil {
		return failResult(err)
	}
	found := false
	for _, k := range keys {
		if k == p.name {
			found = true
			break
		}
	}
	if !found {
		return cook.Result{
			Succeeded: true,
			Changed:   false,
			Notes:     []fmt.Stringer{note("key already absent: %s", p.name)},
		}, nil
	}
	if test {
		return cook.Result{
			Succeeded: true,
			Changed:   true,
			Notes:     []fmt.Stringer{note("would remove key: %s", p.name)},
		}, nil
	}
	err = keyMgr.RemoveKey(ctx, p.name)
	if err != nil {
		return failResult(err)
	}
	return cook.Result{
		Succeeded: true,
		Changed:   true,
		Notes:     []fmt.Stringer{note("removed key: %s", p.name)},
	}, nil
}
