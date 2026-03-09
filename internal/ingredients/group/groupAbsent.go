package group

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	"github.com/gogrlx/grlx/v2/internal/cook"
)

func (g Group) absent(ctx context.Context, test bool) (cook.Result, error) {
	var result cook.Result

	groupName, ok := g.params["name"].(string)
	if !ok || groupName == "" {
		result.Failed = true
		return result, errors.New("invalid group; name must be a non-empty string")
	}

	if !groupExists(groupName) {
		result.Succeeded = true
		result.Notes = append(result.Notes,
			cook.SimpleNote("group "+groupName+" already absent, nothing to do"))
		return result, nil
	}

	if test {
		result.Succeeded = true
		result.Changed = true
		result.Notes = append(result.Notes,
			cook.SimpleNote("group "+groupName+" would be deleted"))
		return result, nil
	}

	cmd := exec.CommandContext(ctx, "groupdel", groupName)
	if err := cmd.Run(); err != nil {
		result.Failed = true
		result.Notes = append(result.Notes,
			cook.SimpleNote(fmt.Sprintf("failed to delete group %s: %s", groupName, err.Error())))
		return result, err
	}

	if groupExists(groupName) {
		result.Failed = true
		return result, errors.New("group " + groupName + " could not be deleted")
	}

	result.Succeeded = true
	result.Changed = true
	result.Notes = append(result.Notes,
		cook.SimpleNote("group "+groupName+" deleted"))
	return result, nil
}
