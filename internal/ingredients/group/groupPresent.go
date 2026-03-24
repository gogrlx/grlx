package group

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"os/user"
	"strings"

	"github.com/gogrlx/grlx/v2/internal/cook"
)

// Function variables for system operations — replaceable in tests.
var (
	execCommand = func(ctx context.Context, name string, args ...string) error {
		return exec.CommandContext(ctx, name, args...).Run()
	}
	lookupGroup   = user.LookupGroup
	groupExistsBy = func(name string) bool {
		_, err := user.LookupGroup(name)
		return err == nil
	}
)

func (g Group) present(ctx context.Context, test bool) (cook.Result, error) {
	var result cook.Result

	groupName, ok := g.params["name"].(string)
	if groupName == "" || !ok {
		result.Failed = true
		return result, errors.New("invalid group; name must be a non-empty string")
	}

	gid := stringParam(g.params, "gid")
	system := boolParam(g.params, "system", false)
	members := stringSliceParam(g.params, "members")

	existing, err := lookupGroup(groupName)
	if err != nil {
		// Group does not exist — groupadd
		args := buildGroupaddArgs(groupName, gid, system)
		cmdStr := fmt.Sprintf("groupadd %s", strings.Join(args, " "))
		if test {
			result.Succeeded = true
			result.Changed = true
			result.Notes = append(result.Notes,
				cook.SimpleNote(fmt.Sprintf("would create group with command: %s", cmdStr)))
			if len(members) > 0 {
				result.Notes = append(result.Notes,
					cook.SimpleNote(fmt.Sprintf("would set members to: %s", strings.Join(members, ", "))))
			}
			return result, nil
		}
		if err := execCommand(ctx, "groupadd", args...); err != nil {
			result.Failed = true
			result.Notes = append(result.Notes,
				cook.SimpleNote(fmt.Sprintf("failed to create group: %s", err.Error())))
			return result, err
		}
		result.Succeeded = true
		result.Changed = true
		result.Notes = append(result.Notes,
			cook.SimpleNote(fmt.Sprintf("created group with command: %s", cmdStr)))

		// Set members if specified
		if len(members) > 0 {
			if err := setGroupMembers(ctx, groupName, members); err != nil {
				result.Notes = append(result.Notes,
					cook.SimpleNote(fmt.Sprintf("failed to set members: %s", err.Error())))
				result.Failed = true
				result.Succeeded = false
				return result, err
			}
			result.Notes = append(result.Notes,
				cook.SimpleNote(fmt.Sprintf("set members to: %s", strings.Join(members, ", "))))
		}
		return result, nil
	}

	// Group exists — check if modifications needed
	var changes []string

	if gid != "" && existing.Gid != gid {
		changes = append(changes, fmt.Sprintf("gid %s→%s", existing.Gid, gid))
	}

	if len(changes) > 0 {
		args := buildGroupmodArgs(groupName, gid)
		cmdStr := fmt.Sprintf("groupmod %s", strings.Join(args, " "))
		if test {
			result.Succeeded = true
			result.Changed = true
			result.Notes = append(result.Notes,
				cook.SimpleNote(fmt.Sprintf("would modify group with command: %s", cmdStr)))
			return result, nil
		}
		if err := execCommand(ctx, "groupmod", args...); err != nil {
			result.Failed = true
			result.Notes = append(result.Notes,
				cook.SimpleNote(fmt.Sprintf("failed to modify group: %s", err.Error())))
			return result, err
		}
		result.Succeeded = true
		result.Changed = true
		result.Notes = append(result.Notes,
			cook.SimpleNote(fmt.Sprintf("modified group: %s", strings.Join(changes, ", "))))
	}

	// Handle members
	if len(members) > 0 {
		if test {
			if !result.Changed {
				result.Succeeded = true
			}
			result.Changed = true
			result.Notes = append(result.Notes,
				cook.SimpleNote(fmt.Sprintf("would set members to: %s", strings.Join(members, ", "))))
			return result, nil
		}
		if err := setGroupMembers(ctx, groupName, members); err != nil {
			result.Failed = true
			result.Succeeded = false
			result.Notes = append(result.Notes,
				cook.SimpleNote(fmt.Sprintf("failed to set members: %s", err.Error())))
			return result, err
		}
		result.Changed = true
		result.Notes = append(result.Notes,
			cook.SimpleNote(fmt.Sprintf("set members to: %s", strings.Join(members, ", "))))
	}

	if !result.Changed {
		result.Succeeded = true
		result.Notes = append(result.Notes, cook.SimpleNote("group already exists"))
	} else if !result.Failed {
		result.Succeeded = true
	}

	return result, nil
}

func buildGroupaddArgs(name, gid string, system bool) []string {
	var args []string
	if gid != "" {
		args = append(args, "-g", gid)
	}
	if system {
		args = append(args, "-r")
	}
	args = append(args, name)
	return args
}

func buildGroupmodArgs(name, gid string) []string {
	var args []string
	if gid != "" {
		args = append(args, "-g", gid)
	}
	args = append(args, name)
	return args
}

// setGroupMembers uses gpasswd to set the exact member list for a group.
func setGroupMembers(ctx context.Context, groupName string, members []string) error {
	return execCommand(ctx, "gpasswd", "-M", strings.Join(members, ","), groupName)
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
