package rootball

type RecipeFile struct {
	Recipes    []*Recipe
	Includes   []string
	includes   []*RecipeFile
	IsIncluded bool
	ID         string
}

type Recipe struct {
	Dependencies []string
	dependencies []*Recipe
	dependents   []*Recipe
	isRequisite  bool
	ID           string
}
