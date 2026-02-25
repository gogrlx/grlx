package group

import (
	"context"
	"errors"
	"os/exec"

	"github.com/gogrlx/grlx/v2/internal/cook"
)

func (g Group) absent(ctx context.Context, test bool) (cook.Result, error) {
	var result cook.Result
	result.Succeeded = true
	result.Failed = false
	groupName, ok := g.params["name"].(string)
	if !ok {
		result.Failed = true
		result.Succeeded = false
		return result, errors.New("invalid group; name must be a string")
	}
	if groupExists(groupName) {
		if test {
			return cook.Result{
				Succeeded: true,
				Failed:    false,
				Notes:     append(result.Notes, cook.SimpleNote("group "+groupName+" would be deleted")),
			}, nil
		}
		cmd := exec.CommandContext(ctx, "groupdel", groupName)
		err := cmd.Run()
		if err != nil {
			result.Failed = true
			result.Succeeded = false
			result.Notes = append(result.Notes, cook.SimpleNote("group "+groupName+" could not be deleted"))
			result.Notes = append(result.Notes, cook.SimpleNote(err.Error()))
			return result, err
		}
		if !groupExists(groupName) {
			result.Notes = append(result.Notes, cook.SimpleNote("group "+groupName+" deleted"))
			return result, nil
		}
		result.Failed = true
		result.Succeeded = false
		result.Notes = append(result.Notes, cook.SimpleNote("group "+groupName+" could not be deleted"))
		return result, errors.New("group " + groupName + " could not be deleted")

	}
	result.Notes = append(result.Notes, cook.SimpleNote("group "+groupName+" already absent, nothing to do"))
	return result, nil
}
