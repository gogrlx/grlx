package group

import (
	"context"
	"errors"
	"os/user"

	"github.com/gogrlx/grlx/v2/internal/cook"
)

func (g Group) exists(ctx context.Context, test bool) (cook.Result, error) {
	var result cook.Result
	result.Succeeded = true
	result.Failed = false

	groupName, ok := g.params["name"].(string)
	if !ok {
		result.Failed = true
		result.Succeeded = false
		return result, errors.New("invalid group; name must be a string")
	}
	if !groupExists(groupName) {
		result.Failed = true
		result.Succeeded = false
		result.Notes = append(result.Notes, cook.SimpleNote("group "+groupName+" does not exist"))
		return result, nil
	}
	result.Notes = append(result.Notes, cook.SimpleNote("group "+groupName+" exists"))
	return result, nil
}

func groupExists(name string) bool {
	_, err := user.LookupGroup(name)
	return err == nil
}
