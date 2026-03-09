package user

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	"github.com/gogrlx/grlx/v2/internal/cook"
)

func (u User) absent(ctx context.Context, test bool) (cook.Result, error) {
	var result cook.Result

	userName, ok := u.params["name"].(string)
	if !ok || userName == "" {
		result.Failed = true
		return result, errors.New("invalid user; name must be a non-empty string")
	}

	if !userExists(userName) {
		result.Succeeded = true
		result.Notes = append(result.Notes,
			cook.SimpleNote("user "+userName+" already absent, nothing to do"))
		return result, nil
	}

	if test {
		result.Succeeded = true
		result.Changed = true
		result.Notes = append(result.Notes,
			cook.SimpleNote("user "+userName+" would be deleted"))
		return result, nil
	}

	args := []string{userName}
	purge := boolParam(u.params, "purge", false)
	if purge {
		args = []string{"-r", userName}
	}

	cmd := exec.CommandContext(ctx, "userdel", args...)
	if err := cmd.Run(); err != nil {
		result.Failed = true
		result.Notes = append(result.Notes,
			cook.SimpleNote(fmt.Sprintf("failed to delete user %s: %s", userName, err.Error())))
		return result, err
	}

	if userExists(userName) {
		result.Failed = true
		return result, errors.New("user " + userName + " could not be deleted")
	}

	result.Succeeded = true
	result.Changed = true
	result.Notes = append(result.Notes,
		cook.SimpleNote("user "+userName+" deleted"))
	return result, nil
}
