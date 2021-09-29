package rootball

import (
	"testing"
)

var (
	aa Recipe
	a  Recipe
	b  Recipe
	c  Recipe
	d  Recipe
	e  Recipe
	f  Recipe
	g  Recipe
	h  Recipe
	i  Recipe
)

func init() {
	a.ID = "a"
	aa.ID = "a"
	b.ID = "b"
	c.ID = "c"
	d.ID = "d"
	e.ID = "e"
	f.ID = "f"
	g.ID = "g"
	h.ID = "h"
	i.ID = "i"
	a.Dependencies = []string{"b", "c"}
	b.Dependencies = []string{"d"}
	e.Dependencies = []string{"a", "d"}
	g.Dependencies = []string{"h"}
	h.Dependencies = []string{"i"}
	i.Dependencies = []string{"g", "a", "e"}

}

func TestGenerateTree(t *testing.T) {
	testCases := []struct {
		name       string
		recipeFile RecipeFile
		expected   string
		err        error
	}{{name: "simple test",
		recipeFile: RecipeFile{Recipes: []*Recipe{&a},
			Includes: []string{}},
		expected: ""},
		{name: "g-h-i cycle",
			recipeFile: RecipeFile{Recipes: []*Recipe{&g, &h, &i}},
			expected:   ""},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			roots, errs := GenerateTrees(tc.recipeFile.Recipes)
			if len(errs) > 0 {
				for _, e := range errs {
					t.Error(e)
				}
				// compare the error types here
			}
			out := PrintTrees(roots)
			if out != tc.expected {
				t.Errorf("Expected: %s  but got: %s", tc.expected, out)
			}
		})
	}
	return
}

func TestAllDependenciesDefined(t *testing.T) {
	testCases := []struct {
		name      string
		recipes   []*Recipe
		defined   bool
		undefined []string
	}{{name: "all present",
		recipes:   []*Recipe{&a, &b, &c, &d},
		defined:   true,
		undefined: nil},
		{name: "a missing",
			recipes:   []*Recipe{&e},
			defined:   false,
			undefined: []string{"d", "a"}},
		{name: "empty",
			recipes:   []*Recipe{},
			defined:   true,
			undefined: []string{}},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			allDefined, missing := AllDependenciesDefined(tc.recipes)
			if len(missing) > 0 {
				for _, e := range missing {
					found := false
					for _, ex := range tc.undefined {
						if ex == e {
							found = true
						}
					}
					if !found {
						t.Errorf("%s was found but should not have been.", e)
					}
				}
				// compare the error types here
			}
			for _, e := range tc.undefined {
				found := false
				for _, ex := range missing {
					if ex == e {
						found = true
					}
				}
				if !found {
					t.Errorf("%s was not found but should have been.", e)
				}
			}
			if allDefined != tc.defined {
				t.Errorf("Expected: %v  but got: %v", tc.defined, allDefined)
			}
		})
	}
	return
}

func TestNoDuplicateIDs(t *testing.T) {
	testCases := []struct {
		name         string
		recipes      []*Recipe
		noDuplicates bool
		duplicates   []string
	}{{name: "no duplicates",
		recipes:      []*Recipe{&a, &b, &c, &d},
		noDuplicates: true,
		duplicates:   []string{}},
		{name: "a duplicated",
			recipes:      []*Recipe{&a, &aa},
			noDuplicates: false,
			duplicates:   []string{"a"}},
		{name: "empty set",
			recipes:      []*Recipe{},
			noDuplicates: true,
			duplicates:   []string{}},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			noDuplicates, duplicates := NoDuplicateIDs(tc.recipes)
			if len(duplicates) > 0 {
				for _, e := range duplicates {
					found := false
					for _, ex := range tc.duplicates {
						if ex == e {
							found = true
						}
					}
					if !found {
						t.Errorf("%s was found but should not have been.", e)
					}
				}
				// compare the error types here
			}
			for _, e := range tc.duplicates {
				found := false
				for _, ex := range duplicates {
					if ex == e {
						found = true
					}
				}
				if !found {
					t.Errorf("%s was not found but should have been.", e)
				}
			}
			if noDuplicates != tc.noDuplicates {
				t.Errorf("Expected: %v  but got: %v", tc.noDuplicates, noDuplicates)
			}
		})
	}
	return
}
func TestHasCycle(t *testing.T) {
	testCases := []struct {
		name         string
		recipes      []*Recipe
		noDuplicates bool
		duplicates   []string
	}{{name: "no duplicates",
		recipes:      []*Recipe{&a, &b, &c, &d},
		noDuplicates: true,
		duplicates:   []string{}},
		{name: "a duplicated",
			recipes:      []*Recipe{&a, &aa},
			noDuplicates: false,
			duplicates:   []string{"a"}},
		{name: "empty set",
			recipes:      []*Recipe{},
			noDuplicates: true,
			duplicates:   []string{}},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			noDuplicates, duplicates := HasCycle(tc.recipes)
			if len(duplicates) > 0 {
				for _, e := range duplicates {
					found := false
					for _, ex := range tc.duplicates {
						if ex == e {
							found = true
						}
					}
					if !found {
						t.Errorf("%s was found but should not have been.", e)
					}
				}
				// compare the error types here
			}
			for _, e := range tc.duplicates {
				found := false
				for _, ex := range duplicates {
					if ex == e {
						found = true
					}
				}
				if !found {
					t.Errorf("%s was not found but should have been.", e)
				}
			}
			if noDuplicates != tc.noDuplicates {
				t.Errorf("Expected: %v  but got: %v", tc.noDuplicates, noDuplicates)
			}
		})
	}
	return
}

func TestBuildTrees(t *testing.T) {
}
