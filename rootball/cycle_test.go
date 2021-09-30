package rootball

import (
	"errors"
	"testing"

	. "github.com/gogrlx/grlx/types"
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
	j  Recipe
	k  Recipe
	l  Recipe
	m  Recipe
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
	j.ID = "j"
	k.ID = "k"
	l.ID = "l"
	m.ID = "m"
	a.Dependencies = []string{"b", "c"}
	b.Dependencies = []string{"d"}
	e.Dependencies = []string{"a", "d"}
	g.Dependencies = []string{"h"}
	h.Dependencies = []string{"i"}
	i.Dependencies = []string{"g", "a", "e"}
	j.Dependencies = []string{"a", "b"}
	k.Dependencies = []string{"j", "b"}
	l.Dependencies = []string{"k", "j", "b"}
	m.Dependencies = []string{"j", "b", "a", "k"}

}

func TestGenerateTree(t *testing.T) {
	testCases := []struct {
		name       string
		recipeFile RecipeFile
		expected   string
		err        error
	}{{name: "simple test",
		recipeFile: RecipeFile{Recipes: []*Recipe{&a, &b, &d, &c},
			Includes: []string{}},
		expected: "a\n|\t├── b\n|\t|\t└── d\n|\t└── c\n\n\n"},
		{name: "deeply nested deps",
			recipeFile: RecipeFile{Recipes: []*Recipe{&a, &b, &d, &c, &j, &k, &l, &m},
				Includes: []string{}},

			expected: "l\n|\t├── k\n|\t|\t├── j\n|\t|\t|\t├── a\n|\t|\t|\t|\t├── b\n|\t|\t|\t|\t|\t└── d\n|\t|\t|\t|\t└── c\n|\t|\t|\t└── b\n|\t|\t|\t|\t└── d\n|\t|\t└── b\n|\t|\t|\t└── d\n|\t├── j\n|\t|\t├── a\n|\t|\t|\t├── b\n|\t|\t|\t|\t└── d\n|\t|\t|\t└── c\n|\t|\t└── b\n|\t|\t|\t└── d\n|\t└── b\n|\t|\t└── d\n\n\nm\n|\t├── j\n|\t|\t├── a\n|\t|\t|\t├── b\n|\t|\t|\t|\t└── d\n|\t|\t|\t└── c\n|\t|\t└── b\n|\t|\t|\t└── d\n|\t├── b\n|\t|\t└── d\n|\t├── a\n|\t|\t├── b\n|\t|\t|\t└── d\n|\t|\t└── c\n|\t└── k\n|\t|\t├── j\n|\t|\t|\t├── a\n|\t|\t|\t|\t├── b\n|\t|\t|\t|\t|\t└── d\n|\t|\t|\t|\t└── c\n|\t|\t|\t└── b\n|\t|\t|\t|\t└── d\n|\t|\t└── b\n|\t|\t|\t└── d\n\n\n"},
		//expected: "l\n|\t├── k\n|\t|\t├── j\n|\t|\t|\t├── a\n|\t|\t|\t|\t├── b\n|\t|\t|\t|\t|\t└── d\n|\t|\t|\t|\t└── c\n|\t|\t|\t└── b\n|\t|\t|\t|\t└── d\n|\t|\t└── b\n|\t|\t|\t└── d\n|\t├── j\n|\t|\t├── a\n|\t|\t|\t├── b\n|\t|\t|\t|\t└── d\n|\t|\t|\t└── c\n|\t|\t└── b\n|\t|\t|\t└── d\n|\t└── b\n|\t|\t└── d\n\nm\n|\t├── j\n|\t|\t├── a\n|\t|\t|\t├── b\n|\t|\t|\t|\t└── d\n|\t|\t|\t└── c\n|\t|\t└── b\n|\t|\t|\t└── d\n|\t├── b\n|\t|\t└── d\n|\t├── a\n|\t|\t├── b\n|\t|\t|\t└── d\n|\t|\t└── c\n|\t└── k\n|\t|\t├── j\n|\t|\t|\t├── a\n|\t|\t|\t|\t├── b\n|\t|\t|\t|\t|\t└── d\n|\t|\t|\t|\t└── c\n|\t|\t|\t└── b\n|\t|\t|\t|\t└── d\n|\t|\t└── b\n|\t|\t|\t└── d"},
		{name: "g-h-i cycle",
			recipeFile: RecipeFile{Recipes: []*Recipe{&g, &h, &i, &a, &b, &c, &d, &e}},
			expected:   "",
			err:        ErrDependencyCycleFound},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, recipe := range tc.recipeFile.Recipes {
				recipe.dependencies = []*Recipe{}
			}
			roots, errs := GenerateTrees(tc.recipeFile.Recipes)
			if len(errs) > 0 {
				for _, e := range errs {
					if !errors.Is(e, tc.err) {
						t.Error(e)
					}
				}
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
		{name: "a and d missing",
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

func TestBuildTrees(t *testing.T) {
}
