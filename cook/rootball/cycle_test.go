package rootball

import (
	"errors"
	"testing"

	. "github.com/gogrlx/grlx/types"
)

var (
	aa Step
	a  Step
	b  Step
	c  Step
	d  Step
	e  Step
	f  Step
	g  Step
	h  Step
	i  Step
	j  Step
	k  Step
	l  Step
	m  Step
)

func createSteps() {
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
	a.Requisites = RequisiteSet{Requisite{StepIDs: []StepID{"b", "c"}}}
	b.Requisites = RequisiteSet{Requisite{StepIDs: []StepID{"d"}}}
	e.Requisites = RequisiteSet{Requisite{StepIDs: []StepID{"a", "d"}}}
	g.Requisites = RequisiteSet{Requisite{StepIDs: []StepID{"h"}}}
	h.Requisites = RequisiteSet{Requisite{StepIDs: []StepID{"i"}}}
	i.Requisites = RequisiteSet{Requisite{StepIDs: []StepID{"g", "a", "e"}}}
	j.Requisites = RequisiteSet{Requisite{StepIDs: []StepID{"a", "b"}}}
	k.Requisites = RequisiteSet{Requisite{StepIDs: []StepID{"j", "b"}}}
	l.Requisites = RequisiteSet{Requisite{StepIDs: []StepID{"k", "j", "b"}}}
	m.Requisites = RequisiteSet{Requisite{StepIDs: []StepID{"j", "b", "a", "k"}}}
}

func TestGenerateTree(t *testing.T) {
	testCases := []struct {
		name       string
		recipeFile RecipeFile
		expected   string
		err        error
	}{
		{
			name: "simple test",
			recipeFile: RecipeFile{
				Steps:    []*Step{&a, &b, &d, &c},
				Includes: []string{},
			},
			expected: "a\n|\t├── b\n|\t|\t└── d\n|\t└── c\n\n\n",
		},
		{
			name: "deeply nested deps",
			recipeFile: RecipeFile{
				Steps:    []*Step{&a, &b, &d, &c, &j, &k, &l, &m},
				Includes: []string{},
			},

			expected: "l\n|\t├── k\n|\t|\t├── j\n|\t|\t|\t├── a\n|\t|\t|\t|\t├── b\n|\t|\t|\t|\t|\t└── d\n|\t|\t|\t|\t└── c\n|\t|\t|\t└── b\n|\t|\t|\t|\t└── d\n|\t|\t└── b\n|\t|\t|\t└── d\n|\t├── j\n|\t|\t├── a\n|\t|\t|\t├── b\n|\t|\t|\t|\t└── d\n|\t|\t|\t└── c\n|\t|\t└── b\n|\t|\t|\t└── d\n|\t└── b\n|\t|\t└── d\n\n\nm\n|\t├── j\n|\t|\t├── a\n|\t|\t|\t├── b\n|\t|\t|\t|\t└── d\n|\t|\t|\t└── c\n|\t|\t└── b\n|\t|\t|\t└── d\n|\t├── b\n|\t|\t└── d\n|\t├── a\n|\t|\t├── b\n|\t|\t|\t└── d\n|\t|\t└── c\n|\t└── k\n|\t|\t├── j\n|\t|\t|\t├── a\n|\t|\t|\t|\t├── b\n|\t|\t|\t|\t|\t└── d\n|\t|\t|\t|\t└── c\n|\t|\t|\t└── b\n|\t|\t|\t|\t└── d\n|\t|\t└── b\n|\t|\t|\t└── d\n\n\n",
		},
		{
			name:       "g-h-i cycle",
			recipeFile: RecipeFile{Steps: []*Step{&g, &h, &i, &a, &b, &c, &d, &e}},
			expected:   "",
			err:        ErrDependencyCycleFound,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			//			for _, recipe := range tc.recipeFile.Steps {
			//				recipe.Requisites = RequisiteSet{}
			//			}
			createSteps()
			roots, errs := GenerateTrees(tc.recipeFile.Steps)
			if len(errs) > 0 {
				for _, e := range errs {
					if !errors.Is(e, tc.err) {
						t.Error(e)
					}
				}
			}
			out := PrintTrees(roots)
			if out != tc.expected {
				t.Errorf("Expected:\n%s  but got:\n%s", tc.expected, out)
			}
		})
	}
}

func TestAllRequisitesDefined(t *testing.T) {
	testCases := []struct {
		name      string
		recipes   []*Step
		defined   bool
		undefined []StepID
	}{
		{
			name:      "all present",
			recipes:   []*Step{&a, &b, &c, &d},
			defined:   true,
			undefined: nil,
		},
		{
			name:      "a and d missing",
			recipes:   []*Step{&e},
			defined:   false,
			undefined: []StepID{"d", "a"},
		},
		{
			name:      "empty",
			recipes:   []*Step{},
			defined:   true,
			undefined: []StepID{},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			createSteps()
			allDefined, missing := AllRequisitesDefined(tc.recipes)
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
}

func TestNoDuplicateIDs(t *testing.T) {
	testCases := []struct {
		name         string
		recipes      []*Step
		noDuplicates bool
		duplicates   []StepID
	}{
		{
			name:         "no duplicates",
			recipes:      []*Step{&a, &b, &c, &d},
			noDuplicates: true,
			duplicates:   []StepID{},
		},
		{
			name:         "a duplicated",
			recipes:      []*Step{&a, &aa},
			noDuplicates: false,
			duplicates:   []StepID{"a"},
		},
		{
			name:         "empty set",
			recipes:      []*Step{},
			noDuplicates: true,
			duplicates:   []StepID{},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			createSteps()
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
				t.Errorf("Expected:\n%v  but got:\n%v", tc.noDuplicates, noDuplicates)
			}
		})
	}
}

func TestHasCycle(t *testing.T) {
	testCases := []struct {
		name         string
		recipes      []*Step
		noDuplicates bool
		duplicates   []StepID
	}{
		{
			name:         "no duplicates",
			recipes:      []*Step{&a, &b, &c, &d},
			noDuplicates: true,
			duplicates:   []StepID{},
		},
		{
			name:         "a duplicated",
			recipes:      []*Step{&a, &aa},
			noDuplicates: false,
			duplicates:   []StepID{"a"},
		},
		{
			name:         "empty set",
			recipes:      []*Step{},
			noDuplicates: true,
			duplicates:   []StepID{},
		},
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
				t.Errorf("Expected:\n%v  but got:\n%v", tc.noDuplicates, noDuplicates)
			}
		})
	}
}
