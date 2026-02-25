package group

import (
	"context"
	"errors"
	"os/exec"
	"os/user"

	"github.com/gogrlx/grlx/v2/internal/cook"
)

func (g Group) present(ctx context.Context, test bool) (cook.Result, error) {
	var result cook.Result

	groupName, ok := g.params["name"].(string)
	if groupName == "" || !ok {
		result.Failed = true
		result.Succeeded = false
		return result, errors.New("invalid user; name must be a string")
	}
	args := []string{groupName}

	gid := ""
	if gidInter, ok := g.params["gid"]; ok {
		gid, ok = gidInter.(string)
	}
	if gid != "" {
		args = append(args, "-g"+gid)
	}

	groupByName, err := user.LookupGroup(groupName)
	if err != nil {
		cmd := exec.CommandContext(ctx, "groupadd", args...)
		if test {
			result.Succeeded = true
			result.Failed = false
			result.Changed = true
			result.Notes = append(result.Notes, cook.SimpleNote("would have added a group by executing: "+cmd.String()))
			return result, nil
		}
		err = cmd.Run()
		if err != nil {
			result.Failed = true
			result.Succeeded = false
			return result, err
		}
		result.Changed = true
		return result, nil
	}
	if groupByName == nil || groupByName.Gid != gid {
		cmd := exec.CommandContext(ctx, "groupmod", args...)
		if test {
			result.Succeeded = true
			result.Failed = false
			result.Changed = true
			result.Notes = append(result.Notes, cook.SimpleNote("would have modified the existing group by executing: "+cmd.String()))
			return result, nil
		}
		err = cmd.Run()
		if err != nil {
			result.Failed = true
			result.Succeeded = false
			return result, err
		}
		result.Changed = true
		return result, nil
	}
	result.Succeeded = true
	result.Failed = false
	result.Changed = false
	result.Notes = append(result.Notes, cook.SimpleNote("group already exists"))
	return result, nil
}
