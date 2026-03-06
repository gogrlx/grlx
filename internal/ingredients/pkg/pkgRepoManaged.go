package pkg

import (
	"context"
	"errors"
	"fmt"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/snack"
)

var errRepoNotSupported = errors.New("package manager does not support repository management")

func (p Pkg) repoManaged(ctx context.Context, test bool) (cook.Result, error) {
	mgr, err := getManager()
	if err != nil {
		return failResult(err)
	}
	repoMgr, ok := mgr.(snack.RepoManager)
	if !ok {
		return failResult(errRepoNotSupported)
	}
	absent := getBoolProp(p.properties, "absent", false)
	if absent {
		return p.repoRemoved(ctx, repoMgr, test)
	}
	return p.repoAdded(ctx, repoMgr, test)
}

func (p Pkg) repoAdded(ctx context.Context, repoMgr snack.RepoManager, test bool) (cook.Result, error) {
	// Check if repo already exists.
	repos, err := repoMgr.ListRepos(ctx)
	if err != nil {
		return failResult(err)
	}
	for _, r := range repos {
		if r.ID == p.name || r.Name == p.name {
			return cook.Result{
				Succeeded: true,
				Changed:   false,
				Notes:     []fmt.Stringer{note("repository already configured: %s", p.name)},
			}, nil
		}
	}
	if test {
		return cook.Result{
			Succeeded: true,
			Changed:   true,
			Notes:     []fmt.Stringer{note("would add repository: %s", p.name)},
		}, nil
	}
	repo := snack.Repository{
		ID:   p.name,
		Name: p.name,
	}
	if url, ok := p.properties["url"].(string); ok {
		repo.URL = url
	}
	err = repoMgr.AddRepo(ctx, repo)
	if err != nil {
		return failResult(err)
	}
	return cook.Result{
		Succeeded: true,
		Changed:   true,
		Notes:     []fmt.Stringer{note("added repository: %s", p.name)},
	}, nil
}

func (p Pkg) repoRemoved(ctx context.Context, repoMgr snack.RepoManager, test bool) (cook.Result, error) {
	// Check if repo exists.
	repos, err := repoMgr.ListRepos(ctx)
	if err != nil {
		return failResult(err)
	}
	found := false
	for _, r := range repos {
		if r.ID == p.name || r.Name == p.name {
			found = true
			break
		}
	}
	if !found {
		return cook.Result{
			Succeeded: true,
			Changed:   false,
			Notes:     []fmt.Stringer{note("repository already absent: %s", p.name)},
		}, nil
	}
	if test {
		return cook.Result{
			Succeeded: true,
			Changed:   true,
			Notes:     []fmt.Stringer{note("would remove repository: %s", p.name)},
		}, nil
	}
	err = repoMgr.RemoveRepo(ctx, p.name)
	if err != nil {
		return failResult(err)
	}
	return cook.Result{
		Succeeded: true,
		Changed:   true,
		Notes:     []fmt.Stringer{note("removed repository: %s", p.name)},
	}, nil
}
