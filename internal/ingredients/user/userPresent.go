package user

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"os/user"
	"strings"

	"github.com/gogrlx/grlx/v2/internal/types"
)

func (u User) present(ctx context.Context, test bool) (types.Result, error) {
	var result types.Result

	userName, ok := u.params["name"].(string)
	if !ok {
		result.Failed = true
		result.Succeeded = false
		return result, errors.New("invalid user; name must be a string")
	}

	uid := ""
	gid := ""
	shell := ""
	groups := []string{}
	home := ""
	if uidInter, ok := u.params["uid"]; ok {
		uid, ok = uidInter.(string)
	}
	if gidInter, ok := u.params["gid"]; ok {
		gid, ok = gidInter.(string)
	}
	if shellInter, ok := u.params["shell"]; ok {
		shell, ok = shellInter.(string)
	}
	if groupsInter, ok := u.params["groups"]; ok {
		groups, ok = groupsInter.([]string)
	}
	if homeInter, ok := u.params["home"]; ok {
		home, ok = homeInter.(string)
	}
	userCmd := "usermod"
	user, err := user.Lookup(userName)
	if err != nil {
		userCmd = "useradd"
	}
	args := []string{userName}
	if uid != "" && user == nil || uid != user.Uid {
		args = append(args, "-u"+uid)
	}
	if gid != "" && user == nil || gid != user.Gid {
		args = append(args, "-g"+gid)
	}
	if shell != "" {
		args = append(args, "-s"+shell)
	}
	if home != "" && shell != user.HomeDir {
		args = append(args, "-d"+home)
	}
	if len(groups) > 0 {
		args = append(args, "-G"+strings.Join(groups, ","))
	}
	cmd := exec.CommandContext(ctx, userCmd, args...)
	if test {
		result.Notes = append(result.Notes,
			types.SimpleNote(fmt.Sprintf("would have updated user with command: %s", cmd.String())))
		result.Succeeded = true
		result.Failed = false
		result.Changed = true
		return result, nil
	}
	err = cmd.Run()
	if err != nil {
		result.Failed = true
		result.Succeeded = false
		result.Notes = append(result.Notes, types.SimpleNote(fmt.Sprintf("failed to update user: %s", err.Error())))
		return result, err
	}
	result.Failed = false
	result.Succeeded = true
	result.Changed = true
	result.Notes = append(result.Notes, types.SimpleNote(fmt.Sprintf("updated user with command: %s", cmd.String())))
	return result, nil
}
