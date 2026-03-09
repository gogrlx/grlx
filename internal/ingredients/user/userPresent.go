package user

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"os/user"
	"strings"

	"github.com/gogrlx/grlx/v2/internal/cook"
)

func (u User) present(ctx context.Context, test bool) (cook.Result, error) {
	var result cook.Result

	userName, ok := u.params["name"].(string)
	if !ok || userName == "" {
		result.Failed = true
		return result, errors.New("invalid user; name must be a non-empty string")
	}

	uid := stringParam(u.params, "uid")
	gid := stringParam(u.params, "gid")
	shell := stringParam(u.params, "shell")
	home := stringParam(u.params, "home")
	comment := stringParam(u.params, "comment")
	groups := stringSliceParam(u.params, "groups")
	createHome := boolParam(u.params, "createhome", true)
	system := boolParam(u.params, "system", false)

	existing, err := user.Lookup(userName)
	if err != nil {
		// User does not exist — useradd
		args := buildUseraddArgs(userName, uid, gid, shell, home, comment, groups, createHome, system)
		cmd := exec.CommandContext(ctx, "useradd", args...)
		if test {
			result.Succeeded = true
			result.Changed = true
			result.Notes = append(result.Notes,
				cook.SimpleNote(fmt.Sprintf("would create user with command: %s", cmd.String())))
			return result, nil
		}
		if err := cmd.Run(); err != nil {
			result.Failed = true
			result.Notes = append(result.Notes,
				cook.SimpleNote(fmt.Sprintf("failed to create user: %s", err.Error())))
			return result, err
		}
		result.Succeeded = true
		result.Changed = true
		result.Notes = append(result.Notes,
			cook.SimpleNote(fmt.Sprintf("created user with command: %s", cmd.String())))
		return result, nil
	}

	// User exists — check if modifications are needed
	args := buildUsermodArgs(userName, uid, gid, shell, home, comment, groups, existing)
	if len(args) == 0 {
		// No changes needed
		result.Succeeded = true
		result.Notes = append(result.Notes,
			cook.SimpleNote(fmt.Sprintf("user %s is already in the desired state", userName)))
		return result, nil
	}

	cmd := exec.CommandContext(ctx, "usermod", args...)
	if test {
		result.Succeeded = true
		result.Changed = true
		result.Notes = append(result.Notes,
			cook.SimpleNote(fmt.Sprintf("would modify user with command: %s", cmd.String())))
		return result, nil
	}
	if err := cmd.Run(); err != nil {
		result.Failed = true
		result.Notes = append(result.Notes,
			cook.SimpleNote(fmt.Sprintf("failed to modify user: %s", err.Error())))
		return result, err
	}
	result.Succeeded = true
	result.Changed = true
	result.Notes = append(result.Notes,
		cook.SimpleNote(fmt.Sprintf("modified user with command: %s", cmd.String())))
	return result, nil
}

func buildUseraddArgs(name, uid, gid, shell, home, comment string, groups []string, createHome, system bool) []string {
	var args []string
	if uid != "" {
		args = append(args, "-u", uid)
	}
	if gid != "" {
		args = append(args, "-g", gid)
	}
	if shell != "" {
		args = append(args, "-s", shell)
	}
	if home != "" {
		args = append(args, "-d", home)
	}
	if comment != "" {
		args = append(args, "-c", comment)
	}
	if len(groups) > 0 {
		args = append(args, "-G", strings.Join(groups, ","))
	}
	if createHome {
		args = append(args, "-m")
	}
	if system {
		args = append(args, "-r")
	}
	args = append(args, name)
	return args
}

func buildUsermodArgs(name, uid, gid, shell, home, comment string, groups []string, existing *user.User) []string {
	var args []string
	if uid != "" && uid != existing.Uid {
		args = append(args, "-u", uid)
	}
	if gid != "" && gid != existing.Gid {
		args = append(args, "-g", gid)
	}
	if shell != "" {
		// user.User doesn't expose shell, so always set if specified
		args = append(args, "-s", shell)
	}
	if home != "" && home != existing.HomeDir {
		args = append(args, "-d", home)
	}
	if comment != "" {
		// user.User.Name is the GECOS field
		if comment != existing.Name {
			args = append(args, "-c", comment)
		}
	}
	if len(groups) > 0 {
		args = append(args, "-G", strings.Join(groups, ","))
	}
	if len(args) == 0 {
		return nil
	}
	args = append(args, name)
	return args
}

// stringParam extracts a string parameter from the params map.
func stringParam(params map[string]interface{}, key string) string {
	v, ok := params[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// stringSliceParam extracts a []string parameter, handling both []string
// and []interface{} (which is what JSON unmarshalling produces).
func stringSliceParam(params map[string]interface{}, key string) []string {
	v, ok := params[key]
	if !ok {
		return nil
	}
	switch vt := v.(type) {
	case []string:
		return vt
	case []interface{}:
		var out []string
		for _, item := range vt {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// boolParam extracts a bool parameter with a default value.
func boolParam(params map[string]interface{}, key string, defaultVal bool) bool {
	v, ok := params[key]
	if !ok {
		return defaultVal
	}
	b, ok := v.(bool)
	if !ok {
		return defaultVal
	}
	return b
}
