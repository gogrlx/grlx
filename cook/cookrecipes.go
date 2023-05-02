package cook

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gogrlx/grlx/config"
	"github.com/gogrlx/grlx/types"
	"gopkg.in/yaml.v3"
)

var funcMap template.FuncMap

func init() {
	funcMap = make(template.FuncMap)
	// funcMap["props"] = props
	// funcMap["secrets"] = secrets
	// funcMap["hostname"] = hostname
	// funcMap["id"] = id
}

func collectAllIncludes(basepath string, recipeID types.RecipeName) ([]types.RecipeName, error) {
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
	starterIncludes, err := extractIncludes(basepath, recipeFilePath, f)
	if err != nil {
		return []types.RecipeName{}, err
	}
	includeSet := make(map[types.RecipeName]bool)
	for _, si := range starterIncludes {
		includeSet[si] = false
	}
	includeSet, err = collectIncludes(basepath, includeSet)
	if err != nil {
		return []types.RecipeName{}, err
	}
	includes := []types.RecipeName{}
	for inc := range includeSet {
		includes = append(includes, inc)
	}
	return includes, nil
}

func Cook(recipeID types.RecipeName) error {
	basepath := getBasePath()
	includes, err := collectAllIncludes(basepath, recipeID)
	if err != nil {
		return err
	}
	recipesteps := make(map[string]interface{})
	for _, inc := range includes {
		// load all imported files into recipefile list
		fp, err := ResolveRecipeFilePath(basepath, inc)
		if err != nil {
			// TODO: wrap this error and explain file existed but no longer exists
			return err
		}
		f, err := os.ReadFile(fp)
		if err != nil {
			return err
		}
		b, err := renderRecipeTemplate(fp, f)
		if err != nil {
			return err
		}
		var recipe map[string]interface{}
		err = json.Unmarshal(b, &recipe)
		if err != nil {
			return err
		}
		m, err := stepsFromMap(recipe)
		if err != nil {
			return err
		}
		// range over all keys under each recipe ID for matching ingredients
		recipesteps, err = joinMaps(recipesteps, m)
		if err != nil {
			return err
		}
	}
	for id, step := range recipesteps {
		switch s := step.(type) {
		case map[string]interface{}:
			if len(s) != 1 {
				return fmt.Errorf("recipe %s must have one directive, but has %d", id, len(s))
			}
		default:
			return fmt.Errorf("recipe %s must me a map[string]interface{} but found %T", id, step)
		}
	}
	// split on periods in ingredient name, fail and error if no matching ingredient module
	// generate ingredient ID
	// based on Recipe ID + basename of ingredient module
	// Load all ingredients into trees
	// test all ingredients for missing, loops, duplicates, etc.
	// run all ingredients in goroutine waitgroups, sending success codes via channels
	// use reasonable timeouts for each ingredient cook
	return nil
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

func ResolveRecipeFilepath(basepath, path string) (types.RecipeName, error) {
	basepath = filepath.Clean(basepath)
	path = filepath.Clean(path)
	path = strings.TrimPrefix(path, basepath)
	strings.ReplaceAll(path, "/", ".")
	recipeExtFile := string(path) + "." + config.GrlxExt
	initFile := filepath.Join(path, "init."+config.GrlxExt)
	stat, err := os.Stat(initFile)
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
		return types.RecipeName(strings.TrimSuffix(recipeExtFile, ".grlx")), nil
	}
	// TODO allow for init.grlx types etc. in the future
	if stat.IsDir() {
		// TODO standardize this error type
		return "", errors.New("init.grlx cannot be a directory")
	}
	return types.RecipeName(strings.TrimSuffix(initFile, ".grlx")), nil
}

func relativeRecipeToAbsolute(basepath, relatedRecipePath string, recipeID types.RecipeName) (types.RecipeName, error) {
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
		return types.RecipeName(recipeExtFile), nil
	}
	// TODO allow for init.grlx types etc. in the future
	if stat.IsDir() {
		// TODO standardize this error type
		return "", errors.New("init.grlx cannot be a directory")
	}
	return types.RecipeName(initFile), nil
}

func ResolveRecipeFilePath(basePath string, recipeID types.RecipeName) (string, error) {
	// open file if found, error out if missing , also allow for .grlx extensions
	// split the ID on periods, resolve to basename directory
	if filepath.Ext(string(recipeID)) == config.GrlxExt {
		recipeID = types.RecipeName(strings.TrimSuffix(string(recipeID), "."+config.GrlxExt))
	}
	// TODO check if basepath is completely empty first
	if !config.BasePathValid() {
		// TODO create an error type for this, wrap and return it
		return "", errors.New("")
	}
	dirList := strings.Split(string(recipeID), ".")
	currentDir := filepath.Join(basePath)
	for depth := 0; depth < len(dirList)-1; depth++ {
		currentDir = filepath.Join(currentDir, dirList[depth])
	}
	stat, err := os.Stat(currentDir)
	// TODO check all possible errors here
	if os.IsNotExist(err) {
		return "", err
	}
	if !stat.IsDir() {
		// TODO standardize this error type
		return "", errors.New("path provided is not to a directory")
	}
	recipeExtFile := dirList[len(dirList)-1] + "." + config.GrlxExt
	recipeExtFile = filepath.Join(currentDir, recipeExtFile)
	initFile := filepath.Join(dirList[len(dirList)-1], "init."+config.GrlxExt)
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

func getBasePath() string {
	// TODO Get basepath from environment
	return "/home/tai/code/foss/grlx/testing/recipes"
}

func ParseRecipeFile(recipeName types.RecipeName) []types.RecipeStep {
	return nil
}

func extractIncludes(basepath, recipePath string, file []byte) ([]types.RecipeName, error) {
	recipeBytes, err := renderRecipeTemplate(recipePath, file)
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

func renderRecipeTemplate(recipeName string, file []byte) ([]byte, error) {
	temp := template.New(recipeName)
	gFuncs := make(template.FuncMap)
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

func collectIncludes(basepath string, starter map[types.RecipeName]bool) (map[types.RecipeName]bool, error) {
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
				eIncludes, err := extractIncludes(basepath, recipeFilePath, f)
				if err != nil {
					return starter, err
				}
				for _, inc := range eIncludes {
					if _, ok := starter[inc]; !ok {
						starter[inc] = false
					}
				}

				newIncludes, err := collectIncludes(basepath, starter)
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
	if includes, ok := recipe["includes"]; ok {
		switch i := includes.(type) {
		case []string:
			inc := []types.RecipeName{}
			for _, v := range i {
				inc = append(inc, types.RecipeName(v))
			}
			return inc, nil
		default:
			return []types.RecipeName{}, fmt.Errorf("include must be a slice of strings, but found type %T", i)
		}
	}
	return []types.RecipeName{}, nil
}

// TODO ensure ability to only run individual state (+ dependencies),
// i.e. start from a root of a given dependency tree
