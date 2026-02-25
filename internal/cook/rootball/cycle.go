package rootball

import (
	"errors"
	"fmt"

	"github.com/gogrlx/grlx/v2/internal/types"
)

// Pull in all included RecipieFiles first
// List all recipies across the RecipieFiles
// Starting with the RecipieFile HEAD, build out tree for each Recipe Dependent Graph

var (
	ProtoRecipe types.Step
	RecipeSet   []*types.Step
)

func ValidateTrees(allRecipies []*types.Step) ([]*types.Step, error) {
	// check for duplicates
	errorList := []error{}
	hasNoDups, dups := NoDuplicateIDs(allRecipies)
	if !hasNoDups {
		for _, dup := range dups {
			// TODO wrap this error
			errorList = append(errorList, fmt.Errorf("recipe identifier is not unique: %s", dup))
		}
		return []*types.Step{}, errors.Join(errorList...)
	}
	// check for undefined deps
	allDefined, missing := AllRequisitesDefined(allRecipies)
	if !allDefined {
		for _, dep := range missing {
			// TODO wrap this error
			errorList = append(errorList, fmt.Errorf("recipe identifier is required but not defined: %s", dep))
		}
		return []*types.Step{}, errors.Join(errorList...)
	}
	// check for cycles
	hasCycle, cycle := HasCycle(allRecipies)
	if hasCycle {
		errorList = append(errorList, fmt.Errorf("%w: %s", types.ErrDependencyCycleFound, PrintCycle(cycle)))
		return []*types.Step{}, errors.Join(errorList...)
	}
	// generate and return the roots
	recipeMap := make(map[types.StepID]*types.Step)
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

// Step 1: render the YAMLs (recipefiles)
// Step 2: recursively gather all recipefiles, adding each to a map[string]bool. Cycles between recipefiles are allowed.
// Step 3: make a list of all states, with Requisites attached, described by *unique* string identifiers
// Step 4: detect non-unique string identifiers, return an error for this
// Step 5: Pass in a list of all possible states, each identifying their Requisites as string IDs
// Step 6: For each of the recipes in the list, check for a dependency cycle using DFS (depth first search)
// Step 7: Build a dependency tree for each of the recipies in the cooked protorecipe
// Step 8: Scan for out-of-tree reicpies that need to be included
// Step 9: Build a dependency tree for each of the out-of-tree Requisites

// Start from step 4
func dfs(allSteps *map[types.StepID]*types.Step, current types.StepID, isVisited *map[types.StepID]bool, isValidated *map[types.StepID]bool) (bool, []types.StepID) {
	if (*isVisited)[current] {
		// TODO return the cycle
		return findCycle(allSteps, current, "", []types.StepID{})
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
	return false, []types.StepID{}
}

func findCycle(allRecipes *map[types.StepID]*types.Step, top types.StepID, current types.StepID, chain []types.StepID) (bool, []types.StepID) {
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
	return false, []types.StepID{}
}

func NoDuplicateIDs(allSteps []*types.Step) (bool, []types.StepID) {
	duplicates := []types.StepID{}
	stepMap := make(map[types.StepID]struct{})
	for _, step := range allSteps {
		if _, ok := stepMap[step.ID]; !ok {
			stepMap[step.ID] = struct{}{}
		} else {
			duplicates = append(duplicates, step.ID)
		}
	}
	return len(duplicates) == 0, duplicates
}

func AllRequisitesDefined(allRecipes []*types.Step) (bool, []types.StepID) {
	unresolved := []types.StepID{}
	recipeMap := make(map[types.StepID]*types.Step)
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

func HasCycle(allRecipes []*types.Step) (bool, []types.StepID) {
	isValidated := make(map[types.StepID]bool)
	isVisited := make(map[types.StepID]bool)
	recipeMap := make(map[types.StepID]*types.Step)
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
	return false, []types.StepID{}
}

func FindRoots(allRecipes []*types.Step) []*types.Step {
	roots := []*types.Step{}
	for _, recipe := range allRecipes {
		if !(*recipe).IsRequisite {
			roots = append(roots, recipe)
		}
	}
	return roots
}
