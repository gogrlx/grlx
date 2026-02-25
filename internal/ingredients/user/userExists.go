package user

import (
	"context"
	"errors"
	"os/user"

	"github.com/gogrlx/grlx/v2/internal/cook"
)

func (u User) exists(ctx context.Context, test bool) (cook.Result, error) {
	var result cook.Result

	userName, ok := u.params["name"].(string)
	if !ok {
		result.Failed = true
		result.Succeeded = false
		return result, errors.New("invalid user; name must be a string")
	}
	if !userExists(userName) {
		result.Failed = true
		result.Succeeded = false
		result.Notes = append(result.Notes, cook.SimpleNote("user "+userName+" does not exist"))
		return result, nil
	}
	result.Failed = false
	result.Succeeded = true
	result.Notes = append(result.Notes, cook.SimpleNote("user "+userName+" exists"))
	return result, nil
}

func userExists(name string) bool {
	_, err := user.Lookup(name)
	return err == nil
}
