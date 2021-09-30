package rootball

import (
	"fmt"
	"strings"
)

func PrintTrees(roots []*Recipe) string {
	output := ""
	for _, recipe := range roots {
		output += printNode(recipe, 0, false) + "\n\n"
	}
	return output
}

func printNode(recipe *Recipe, depth int, isLast bool) string {
	nodeline := strings.Repeat("|\t", depth)
	if depth != 0 {
		if isLast {
			nodeline += "└── "
		} else {
			nodeline += "├── "
		}
	}
	nodeline += recipe.ID + "\n"
	for i, dep := range recipe.dependencies {
		if i == len(recipe.dependencies)-1 {
			nodeline += printNode(dep, depth+1, true)
		} else {
			nodeline += printNode(dep, depth+1, false)
		}
	}
	return nodeline
}

func PrintCycle(cycle []string) string {
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
