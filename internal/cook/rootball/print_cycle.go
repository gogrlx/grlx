package rootball

import (
	"fmt"
	"strings"

	"github.com/gogrlx/grlx/v2/internal/types"
)

func PrintTrees(roots []*types.Step) string {
	output := ""
	for _, recipe := range roots {
		output += printNode(recipe, 0, false) + "\n\n"
	}
	return output
}

func printNode(recipe *types.Step, depth int, isLast bool) string {
	nodeline := strings.Repeat("|\t", depth)
	if depth != 0 {
		if isLast {
			nodeline += "└── "
		} else {
			nodeline += "├── "
		}
	}
	nodeline += string(recipe.ID + "\n")

	steps := recipe.Requisites.AllSteps()
	for i, step := range steps {
		if i == len(steps)-1 {
			nodeline += printNode(step, depth+1, true)
		} else {
			nodeline += printNode(step, depth+1, false)
		}
	}
	return nodeline
}

func PrintCycle(cycle []types.StepID) string {
	out := ""
	maxLength := 0
	for _, w := range cycle {
		if len(w) > maxLength {
			maxLength = len(w)
		}
	}
	for i := 0; i < len(cycle); i++ {
		switch i {
		case 0:
			out += fmt.Sprintf("> %s%s V\n", cycle[i], strings.Repeat(" ", maxLength-len(cycle[i])))
		case len(cycle) - 1:
			out += fmt.Sprintf("^ %s%s <\n", cycle[i], strings.Repeat(" ", maxLength-len(cycle[i])))
		default:
			out += fmt.Sprintf("|| %s%s||\n", cycle[i], strings.Repeat(" ", maxLength-len(cycle[i])))
		}
	}
	return out
}
