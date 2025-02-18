package user

import (
	"context"
	"errors"
	"os/exec"

	"github.com/gogrlx/grlx/v2/types"
)

func (u User) absent(ctx context.Context, test bool) (types.Result, error) {
	var result types.Result
	result.Succeeded = true
	result.Failed = false
	userName, ok := u.params["name"].(string)
	if !ok {
		result.Failed = true
		result.Succeeded = false
		return result, errors.New("invalid user; name must be a string")
	}
	if userExists(userName) {
		if test {
			return types.Result{
				Succeeded: true,
				Failed:    false,
				Notes:     append(result.Notes, types.SimpleNote("user "+userName+" would be deleted")),
			}, nil
		}
		cmd := exec.CommandContext(ctx, "userdel", userName)
		err := cmd.Run()
		if err != nil {
			result.Failed = true
			result.Succeeded = false
			result.Notes = append(result.Notes, types.SimpleNote("user "+userName+" could not be deleted"))
			result.Notes = append(result.Notes, types.SimpleNote(err.Error()))
			return result, err
		}
		if !userExists(userName) {
			result.Notes = append(result.Notes, types.SimpleNote("user "+userName+" deleted"))
			return result, nil
		}
		result.Failed = true
		result.Succeeded = false
		result.Notes = append(result.Notes, types.SimpleNote("user "+userName+" could not be deleted"))
		return result, errors.New("user " + userName + " could not be deleted")

	}
	result.Notes = append(result.Notes, types.SimpleNote("user "+userName+" already absent, nothing to do"))
	return result, nil
}
