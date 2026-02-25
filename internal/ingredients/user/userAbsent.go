package user

import (
	"context"
	"errors"
	"os/exec"

	"github.com/gogrlx/grlx/v2/internal/cook"
)

func (u User) absent(ctx context.Context, test bool) (cook.Result, error) {
	var result cook.Result
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
			return cook.Result{
				Succeeded: true,
				Failed:    false,
				Notes:     append(result.Notes, cook.SimpleNote("user "+userName+" would be deleted")),
			}, nil
		}
		cmd := exec.CommandContext(ctx, "userdel", userName)
		err := cmd.Run()
		if err != nil {
			result.Failed = true
			result.Succeeded = false
			result.Notes = append(result.Notes, cook.SimpleNote("user "+userName+" could not be deleted"))
			result.Notes = append(result.Notes, cook.SimpleNote(err.Error()))
			return result, err
		}
		if !userExists(userName) {
			result.Notes = append(result.Notes, cook.SimpleNote("user "+userName+" deleted"))
			return result, nil
		}
		result.Failed = true
		result.Succeeded = false
		result.Notes = append(result.Notes, cook.SimpleNote("user "+userName+" could not be deleted"))
		return result, errors.New("user " + userName + " could not be deleted")

	}
	result.Notes = append(result.Notes, cook.SimpleNote("user "+userName+" already absent, nothing to do"))
	return result, nil
}
