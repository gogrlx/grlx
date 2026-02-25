package cook

import (
	"errors"
	"fmt"
	"strings"
)

var ErrDependencyCycleFound = errors.New("found a dependency cycle")

type RecipeFile struct {
	Steps      []*Step
	Includes   []string
	includes   []*RecipeFile
	IsIncluded bool
	ID         string
}

var (
	ProtoRecipe Step
	RecipeSet   []*Step
)

func ValidateTrees(allRecipies []*Step) ([]*Step, error) {
	errorList := []error{}
	hasNoDups, dups := NoDuplicateIDs(allRecipies)
	if !hasNoDups {
		for _, dup := range dups {
			errorList = append(errorList, fmt.Errorf("recipe identifier is not unique: %s", dup))
		}
		return []*Step{}, errors.Join(errorList...)
	}
	allDefined, missing := AllRequisitesDefined(allRecipies)
	if !allDefined {
		for _, dep := range missing {
			errorList = append(errorList, fmt.Errorf("recipe identifier is required but not defined: %s", dep))
		}
		return []*Step{}, errors.Join(errorList...)
	}
	hasCycle, cycle := HasCycle(allRecipies)
	if hasCycle {
		errorList = append(errorList, fmt.Errorf("%w: %s", ErrDependencyCycleFound, PrintCycle(cycle)))
		return []*Step{}, errors.Join(errorList...)
	}
	recipeMap := make(map[StepID]*Step)
	for _, recipe := range allRecipies {
		recipeMap[recipe.ID] = recipe
	}
	for _, recipe := range recipeMap {
		for i := range recipe.Requisites {
			for _, v := range recipe.Requisites[i].StepIDs {
				recipe.Requisites[i].Steps = append(recipe.Requisites[i].Steps, recipeMap[v])
				(recipeMap[v]).IsRequisite = true
			}
		}
	}
	return FindRoots(allRecipies), nil
}

func dfs(allSteps *map[StepID]*Step, current StepID, isVisited *map[StepID]bool, isValidated *map[StepID]bool) (bool, []StepID) {
	if (*isVisited)[current] {
		return findCycle(allSteps, current, "", []StepID{})
	}
	(*isVisited)[current] = true
	for _, id := range (*allSteps)[current].Requisites.AllIDs() {
		hasCycle, cycle := dfs(allSteps, id, isVisited, isValidated)
		if hasCycle {
			return true, cycle
		}
	}
	(*isValidated)[current] = true
	(*isVisited)[current] = false
	return false, []StepID{}
}

func findCycle(allRecipes *map[StepID]*Step, top StepID, current StepID, chain []StepID) (bool, []StepID) {
	if current == top {
		chain = append(chain, current)
		return true, chain
	}
	if current == "" {
		current = top
	}
	chain = append(chain, current)
	for _, w := range (*allRecipes)[current].Requisites.AllIDs() {
		if w == top {
			chain = append(chain, w)
			return true, chain
		}
		isCycle, cchain := findCycle(allRecipes, top, w, chain)
		if isCycle {
			return true, cchain
		}
	}
	return false, []StepID{}
}

func NoDuplicateIDs(allSteps []*Step) (bool, []StepID) {
	duplicates := []StepID{}
	stepMap := make(map[StepID]struct{})
	for _, step := range allSteps {
		if _, ok := stepMap[step.ID]; !ok {
			stepMap[step.ID] = struct{}{}
		} else {
			duplicates = append(duplicates, step.ID)
		}
	}
	return len(duplicates) == 0, duplicates
}

func AllRequisitesDefined(allRecipes []*Step) (bool, []StepID) {
	unresolved := []StepID{}
	recipeMap := make(map[StepID]*Step)
	for _, recipe := range allRecipes {
		recipeMap[recipe.ID] = recipe
	}
	for _, recipe := range allRecipes {
		for _, dep := range recipe.Requisites.AllIDs() {
			if _, ok := recipeMap[dep]; !ok {
				unresolved = append(unresolved, dep)
			}
		}
	}
	return len(unresolved) == 0, unresolved
}

func HasCycle(allRecipes []*Step) (bool, []StepID) {
	isValidated := make(map[StepID]bool)
	isVisited := make(map[StepID]bool)
	recipeMap := make(map[StepID]*Step)
	for _, i := range allRecipes {
		isVisited[i.ID] = false
		isValidated[i.ID] = false
		recipeMap[i.ID] = i
	}
	for _, i := range allRecipes {
		if isValidated[i.ID] {
			continue
		}
		hasCycle, cycle := dfs(&recipeMap, i.ID, &isVisited, &isValidated)
		if hasCycle {
			return true, cycle
		}
	}
	return false, []StepID{}
}

func FindRoots(allRecipes []*Step) []*Step {
	roots := []*Step{}
	for _, recipe := range allRecipes {
		if !(*recipe).IsRequisite {
			roots = append(roots, recipe)
		}
	}
	return roots
}

func PrintTrees(roots []*Step) string {
	output := ""
	for _, recipe := range roots {
		output += printNode(recipe, 0, false) + "\n\n"
	}
	return output
}

func printNode(recipe *Step, depth int, isLast bool) string {
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

func PrintCycle(cycle []StepID) string {
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
