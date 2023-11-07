package group

import (
	"context"
	"errors"
	"os/user"

	"github.com/gogrlx/grlx/types"
)

func (g Group) exists(ctx context.Context, test bool) (types.Result, error) {
	var result types.Result
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
		result.Notes = append(result.Notes, types.SimpleNote("group "+groupName+" does not exist"))
	}
	return result, nil
}

func groupExists(name string) bool {
	_, err := user.LookupGroup(name)
	return err == nil
}
