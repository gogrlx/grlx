package cook

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/gogrlx/grlx/types"
)

func TestDeInterfaceRequisites(t *testing.T) {
	testCases := []struct {
		id              string
		requisiteString string
		Expected        types.RequisiteSet
		ReqType         types.ReqType
		Err             error
	}{
		{
			id:              "empty",
			requisiteString: `{"data":null}`, Expected: types.RequisiteSet{},
			ReqType: types.OnChanges, Err: ErrInvalidFormat,
		},
		{
			id: "onchanges", requisiteString: `{"data":["single dependency"]}`,
			Expected: types.RequisiteSet{types.Requisite{
				Condition: types.OnChanges,
				StepIDs:   []types.StepID{types.StepID("single dependency")},
			}}, ReqType: types.OnChanges,
			Err: nil,
		},
		{
			id: "two onchanges", requisiteString: `{"data":["one dependency", "another dependency"]}`,
			Expected: types.RequisiteSet{types.Requisite{
				Condition: types.OnChanges,
				StepIDs:   []types.StepID{types.StepID("one dependency"), types.StepID("another dependency")},
			}}, ReqType: types.OnChanges,
			Err: nil,
		},
		{
			id: "single string onchanges", requisiteString: `{"data":"single dependency"}`,
			Expected: types.RequisiteSet{types.Requisite{
				Condition: types.OnChanges,
				StepIDs:   []types.StepID{types.StepID("single dependency")},
			}}, ReqType: types.OnChanges,
			Err: nil,
		},
		{
			id: "onfail", requisiteString: `{"data":["one dependency", "another dependency"]}`,
			Expected: types.RequisiteSet{types.Requisite{
				Condition: types.OnFail,
				StepIDs:   []types.StepID{types.StepID("one dependency"), types.StepID("another dependency")},
			}}, ReqType: types.OnFail,
			Err: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			m := map[string]interface{}{}
			err := json.Unmarshal([]byte(tc.requisiteString), &m)
			if err != nil {
				t.Error(err)
			}
			reqs, err := deInterfaceRequisites(tc.ReqType, m["data"])
			if !errors.Is(err, tc.Err) {
				t.Error(err)
			}
			for _, r := range reqs {
				if r.Condition != tc.ReqType {
					t.Errorf("expected %v but got %v", tc.ReqType, r.Condition)
				}
			}
			if !reqs.Equals(tc.Expected) {
				t.Errorf("expected %v but got %v", tc.Expected, reqs)
			}
		})
	}
}

func TestExtractRequisites(t *testing.T) {
	testCases := []struct {
		id          string
		stepString  string
		ExpectedReq types.RequisiteSet
	}{{id: "empty", stepString: "{}", ExpectedReq: types.RequisiteSet{}}}
	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			m := make(map[string]interface{})
			err := json.Unmarshal([]byte(tc.stepString), &m)
			if err != nil {
				t.Error(err)
			}
			req, err := extractRequisites(m)
			if err != nil {
				t.Error(err)
			}
			if !req.Equals(tc.ExpectedReq) {
				t.Errorf("expected %v but got %v", tc.ExpectedReq, req)
			}
		})
	}
}

func TestExtractIncludes(t *testing.T) {
	testCases := []struct {
		id          string
		sprout      string
		basepath    string
		recipe      types.RecipeName
		mapContents []string
	}{
		{
			id:          "dev",
			sprout:      "testSprout",
			basepath:    getBasePath(),
			recipe:      "dev",
			mapContents: []string{"apache", "missing"},
		},
		{
			id:          "independent",
			sprout:      "testSprout",
			basepath:    getBasePath(),
			recipe:      "independent",
			mapContents: []string{},
		},
		{
			id:          "apache init",
			sprout:      "testSprout",
			basepath:    getBasePath(),
			recipe:      "apache",
			mapContents: []string{"apache"},
		},
		{
			id:          "apache slash init",
			sprout:      "testSprout",
			basepath:    getBasePath(),
			recipe:      "apache.init.grlx",
			mapContents: []string{"apache"},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			fp, err := ResolveRecipeFilePath(getBasePath(), tc.recipe)
			if err != nil {
				t.Error(err)
			}
			f, _ := os.ReadFile(fp)
			r, err := extractIncludes(tc.sprout, tc.basepath, string(tc.recipe), f)
			if err != nil {
				t.Error(err)
			}
			if len(r) != len(tc.mapContents) {
				t.Errorf("expected %v but got %v", tc.mapContents, r)
			}
			sort.Slice(r, func(i, j int) bool {
				return string(r[i]) < string(r[j])
			})
			sort.Strings(tc.mapContents)
			for i := range tc.mapContents {
				if string(r[i]) != tc.mapContents[i] {
					t.Errorf("expected %v but got %v", tc.mapContents, r)
				}
			}
		})
	}
}

func TestCollectAllIncludes(t *testing.T) {
	testCases := []struct {
		id     string
		recipe types.RecipeName
		sprout string
	}{{
		id:     "dev",
		recipe: "dev",
		sprout: "testSprout",
	}}
	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			recipes, err := collectAllIncludes(tc.sprout, getBasePath(), tc.recipe)
			// TODO actually test this
			fmt.Printf("%v, %v", recipes, err)
		})
	}
}

func TestRelativeRecipeToAbsolute(t *testing.T) {
	testCases := []struct {
		id              string
		recipe          types.RecipeName
		filepath        string
		err             error
		relatedFilepath string
	}{{
		id:              "file doesn't exist",
		recipe:          "",
		filepath:        "",
		err:             os.ErrNotExist,
		relatedFilepath: "",
	}, {
		id:              "valid missing recipe",
		recipe:          ".missing",
		filepath:        "missing",
		err:             nil,
		relatedFilepath: filepath.Join(getBasePath(), "dev.grlx"),
	}}
	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			filepath, err := relativeRecipeToAbsolute(getBasePath(), tc.relatedFilepath, tc.recipe)
			if string(filepath) != tc.filepath {
				t.Errorf("expected %s but got %s", tc.filepath, filepath)
			}
			if !errors.Is(err, tc.err) {
				t.Errorf("expected error %v but got %v", tc.err, err)
			}
		})
	}
}
