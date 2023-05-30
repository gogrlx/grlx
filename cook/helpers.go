package cook

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gogrlx/grlx/config"
	"github.com/gogrlx/grlx/cook/rootball"
	"github.com/gogrlx/grlx/types"
	"gopkg.in/yaml.v3"
)

func makeRecipeSteps(recipes map[string]interface{}) ([]*types.Step, error) {
	steps := []*types.Step{}
	for recipeName, recipe := range recipes {
		if _, ok := recipe.(map[string]interface{}); ok {
			step, err := recipeToStep(recipeName, recipe.(map[string]interface{}))
			if err != nil {
				return []*types.Step{}, err
			}
			steps = append(steps, &step)

		} else {
			return []*types.Step{}, fmt.Errorf("error: recipe %s must be a map", recipeName)
		}
	}
	return steps, nil
}

func recipeToStep(id string, recipe map[string]interface{}) (types.Step, error) {
	var step types.Step
	if len(recipe) != 1 {
		return step, errors.New("error: recipe must have exactly one key")
	}
	for k, v := range recipe {
		rp := strings.Split(k, ".")
		if len(rp) != 2 {
			return step, errors.New("error: recipe key must be in the form ingredient.method")
		}
		if m, ok := v.(map[string]interface{}); !ok {
			return types.Step{}, fmt.Errorf("error: %s must be a map", k)
		} else {
			reqs, err := extractRequisites(m)
			if err != nil {
				return types.Step{}, err
			}
			step = types.Step{
				ID:          types.StepID(id),
				Ingredient:  types.Ingredient(rp[0]),
				Method:      rp[1],
				Requisites:  reqs,
				Properties:  m,
				IsRequisite: false,
			}
			return step, nil
		}
	}
	// should be unreachable but need something to satisfy compiler
	return types.Step{}, errors.New("error: recipe must have exactly one key")
}

func collectAllIncludes(sproutID, basepath string, recipeID types.RecipeName) ([]types.RecipeName, error) {
	// TODO get git branch / tag from environment
	// pass in an ID to a Recipe
	recipeFilePath, err := ResolveRecipeFilePath(basepath, recipeID)
	if err != nil {
		return []types.RecipeName{}, err
	}
	f, err := os.ReadFile(recipeFilePath)
	if err != nil {
		return []types.RecipeName{}, err
	}
	// parse file imports
	starterIncludes, err := extractIncludes(sproutID, basepath, recipeFilePath, f)
	if err != nil {
		return []types.RecipeName{}, err
	}
	includeSet := make(map[types.RecipeName]bool)
	for _, si := range starterIncludes {
		includeSet[si] = false
	}
	includeSet, err = collectIncludesRecurse(sproutID, basepath, includeSet)
	if err != nil {
		return []types.RecipeName{}, err
	}
	fmt.Printf("includeSet: %v\n", includeSet)
	includes := []types.RecipeName{}
	for inc := range includeSet {
		includes = append(includes, inc)
	}
	return includes, nil
}

func deInterfaceRequisite(req types.ReqType, v interface{}) (types.RequisiteSet, error) {
	requisites := []types.Requisite{}
	switch v := v.(type) {
	case string:
		requisites = append(requisites, types.Requisite{StepIDs: []types.StepID{types.StepID(v)}, Condition: req})
	case []interface{}:
		ids := []types.StepID{}
		for i, id := range v {
			if id, ok := id.(string); ok {
				ids = append(ids, types.StepID(id))
			} else {
				return []types.Requisite{}, errors.New("error: " + string(req) + " must be a string or a list of strings, got " + fmt.Sprintf("%T", v[i]))
			}
		}
		requisites = append(requisites, types.Requisite{StepIDs: ids, Condition: types.OnChanges})
	default:
		return []types.Requisite{}, errors.New("error: " + string(req) + " must be a string or a list of strings, got " + fmt.Sprintf("%T", v))
	}
	return requisites, nil
}

func extractRequisites(step map[string]interface{}) (types.RequisiteSet, error) {
	rt, ok := step["requirements"]
	// if there isn't a requirements key, there aren't any requirements for this step
	if !ok {
		return []types.Requisite{}, nil
	}
	requisites := []types.Requisite{}
	// if there is a requirements key, it must be map[string]interface{} , i.e. map[string]string or map[string][]string
	if rti, ok := rt.(map[string]interface{}); !ok {
		return []types.Requisite{}, errors.New("error: requirements must be a map")
	} else {
		for k, v := range rti {
			switch types.ReqType(k) {
			case types.OnChanges, types.OnFail, types.Require:
				fallthrough
			case types.OnChangesAny, types.OnFailAny, types.RequireAny:
				reqs, err := deInterfaceRequisite(types.ReqType(k), v)
				if err != nil {
					return []types.Requisite{}, err
				}
				requisites = append(requisites, reqs...)
			default:
				return []types.Requisite{}, errors.New("error: unknown requisite type " + k)
			}
		}
	}
	return requisites, nil
}

func joinMaps(a, b map[string]interface{}) (map[string]interface{}, error) {
	c := make(map[string]interface{})
	for k, v := range a {
		c[k] = v
	}
	for k, v := range b {
		if _, ok := c[k]; ok {
			return make(map[string]interface{}), fmt.Errorf("error: key %s found in both maps", k)
		}
		c[k] = v
	}
	return c, nil
}

func resolveRelativeFilePath(relatedRecipePath string, recipeID types.RecipeName) (string, error) {
	if filepath.Ext(string(recipeID)) == config.GrlxExt {
		recipeID = types.RecipeName(strings.TrimSuffix(string(recipeID), "."+config.GrlxExt))
	}
	// TODO check if basepath is completely empty first
	relationBasePath := filepath.Dir(relatedRecipePath)
	stat, err := os.Stat(relatedRecipePath)
	// TODO check all possible errors here
	if os.IsNotExist(err) {
		return "", err
	}
	if !stat.IsDir() {
		// TODO standardize this error type
		return "", errors.New("path provided is not to a directory")
	}
	recipeExtFile := string(recipeID) + "." + config.GrlxExt
	recipeExtFile = filepath.Join(relationBasePath, recipeExtFile)
	initFile := filepath.Join(relationBasePath, string(recipeID), "init."+config.GrlxExt)
	stat, err = os.Stat(initFile)
	if os.IsNotExist(err) {
		stat, err = os.Stat(recipeExtFile)
		// TODO check all possible errors here
		if os.IsNotExist(err) {
			return "", err
		}
		// TODO allow for init.grlx types etc. in the future
		if stat.IsDir() {
			// TODO standardize this error type, this happend when the state points to a folder ending in .grlx
			return "", errors.New("path provided is a directory")
		}
		return recipeExtFile, nil
	}
	// TODO allow for init.grlx types etc. in the future
	if stat.IsDir() {
		// TODO standardize this error type
		return "", errors.New("init.grlx cannot be a directory")
	}
	return initFile, nil
}

// TODO implement reverse lookup
// note: because slash paths are valid,
// all that needs to be done is to check if the path contains
// the basepath and strip the extension
func pathToRecipeName(path string) (types.RecipeName, error) {
	path = strings.TrimSuffix(path, "."+config.GrlxExt)
	path = strings.TrimPrefix(path, getBasePath()+"/")
	return types.RecipeName(path), nil
}

// attaches a related path to the prefix of a recipe name
// makes no guarantees that the resultant path is valid

func relativeRecipeToAbsolute(basepath, relatedRecipePath string, recipeID types.RecipeName) (types.RecipeName, error) {
	path := string(recipeID)
	if !strings.HasPrefix(path, ".") {
		var err error
		path, err = ResolveRecipeFilePath(basepath, recipeID)
		if err != nil {
			return "", err
		}
		return pathToRecipeName(path)
	}
	path = strings.TrimPrefix(path, ".")

	relationBasePath := filepath.Dir(relatedRecipePath)

	path = filepath.Join(relationBasePath, path)
	return pathToRecipeName(path)
}

func getBasePath() string {
	// TODO Get basepath from environment
	return "/home/tai/code/foss/grlx/testing/recipes"
}

func extractIncludes(sproutID, basepath, recipePath string, file []byte) ([]types.RecipeName, error) {
	recipeBytes, err := renderRecipeTemplate(sproutID, recipePath, file)
	if err != nil {
		return []types.RecipeName{}, err
	}
	recipeMap, err := unmarshalRecipe(recipeBytes)
	if err != nil {
		return []types.RecipeName{}, err
	}
	includeList, err := includesFromMap(recipeMap)
	if err != nil {
		return []types.RecipeName{}, err
	}
	for i, inc := range includeList {
		tinc := string(inc)
		if strings.HasPrefix(tinc, ".") {

			rel, err := relativeRecipeToAbsolute(basepath, recipePath, inc)
			if err != nil {
				return []types.RecipeName{}, err
			}
			includeList[i] = rel
		}
	}
	return includeList, nil
}

func renderRecipeTemplate(sproutID, recipeName string, file []byte) ([]byte, error) {
	temp := template.New(recipeName)
	gFuncs := populateFuncMap(sproutID)
	temp.Funcs(gFuncs)
	rt, err := temp.Parse(string(file))
	if err != nil {
		return []byte{}, err
	}
	rt.Option("missingkey=error")
	buf := bytes.NewBuffer([]byte{})
	err = rt.Execute(buf, nil)
	if err != nil {
		return []byte{}, err
	}
	return buf.Bytes(), nil
}

func unmarshalRecipe(recipe []byte) (map[string]interface{}, error) {
	rmap := make(map[string]interface{})
	err := yaml.Unmarshal(recipe, &rmap)
	return rmap, err
}

func collectIncludesRecurse(sproutID, basepath string, starter map[types.RecipeName]bool) (map[types.RecipeName]bool, error) {
	allIncluded := false
	for !allIncluded {
		allIncluded = true
		for inc, done := range starter {
			if !done {
				allIncluded = false
				starter[inc] = true
				recipeFilePath, err := ResolveRecipeFilePath(basepath, inc)
				if err != nil {
					return starter, err
				}
				f, err := os.ReadFile(recipeFilePath)
				if err != nil {
					return starter, err
				}
				// parse file imports
				eIncludes, err := extractIncludes(sproutID, basepath, recipeFilePath, f)
				if err != nil {
					return starter, err
				}
				for _, inc := range eIncludes {
					if _, ok := starter[inc]; !ok {
						starter[inc] = false
					}
				}

				newIncludes, err := collectIncludesRecurse(sproutID, basepath, starter)
				if err != nil {
					return newIncludes, err
				}
				for inc, done := range newIncludes {
					starter[inc] = done
				}
			}
		}
	}
	return starter, nil
}

func stepsFromMap(recipe map[string]interface{}) (map[string]interface{}, error) {
	if steps, ok := recipe["steps"]; ok {
		switch s := steps.(type) {
		case map[string]interface{}:
			return s, nil
		default:
			return make(map[string]interface{}), fmt.Errorf("steps must be a map[string]interface{}, but found type %T", s)
		}
	}
	return make(map[string]interface{}), nil
}

func includesFromMap(recipe map[string]interface{}) ([]types.RecipeName, error) {
	if includes, ok := recipe["include"]; ok {
		switch i := includes.(type) {
		case []interface{}:
			inc := []types.RecipeName{}
			for _, v := range i {
				if s, ok := v.(string); ok {
					inc = append(inc, types.RecipeName(s))
				} else {
					return []types.RecipeName{}, fmt.Errorf("include must be a slice of strings, but found type %T in the slice", v)
				}
			}
			return inc, nil
		default:
			return []types.RecipeName{}, fmt.Errorf("include must be a slice of strings, but found type %T", i)
		}
	}

	return []types.RecipeName{}, nil
}

func getRecipeTree(recipes []*types.Step) ([]*types.Step, []error) {
	// TODO pick up here...
	return rootball.GenerateTrees(recipes)
}

// TODO ensure ability to only run individual state (+ dependencies),
// i.e. start from a root of a given dependency tree
