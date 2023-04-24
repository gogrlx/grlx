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

func Cook(recipeID types.RecipeName) error {
	basePath := getBasePath()
	// TODO get git branch / tag from environment
	// pass in an ID to a Recipe
	recipeFilePath, err := ResolveRecipeFilePath(basePath, recipeID)
	if err != nil {
		return err
	}
	f, err := os.ReadFile(recipeFilePath)
	if err != nil {
		return err
	}
	// parse file imports
	starterIncludes, err := extractIncludes(recipeFilePath, f)
	if err != nil {
		return err
	}
	allIncludes, err := collectAllIncludes(starterIncludes)
	if err != nil {
		return err
	}
	_ = starterIncludes
	includes := map[types.RecipeName]struct{}{}

	// load all imported files into recipefile list
	// range over all keys under each recipe ID for matching ingredients
	// split on periods in ingredient name, fail and error if no matching ingredient module
	// generate ingredient ID
	// based on Recipe ID + basename of ingredient module
	// Load all ingredients into trees
	// test all ingredients for missing, loops, duplicates, etc.
	// run all ingredients in goroutine waitgroups, sending success codes via channels
	// use reasonable timeouts for each ingredient cook
	return nil
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
	recipeExtFile := string(recipeID) + config.GrlxExt
	recipeExtFile = filepath.Join(relationBasePath, recipeExtFile)
	initFile := filepath.Join(relationBasePath, string(recipeID), "init"+config.GrlxExt)
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
	recipeExtFile := dirList[len(dirList)-1] + config.GrlxExt
	recipeExtFile = filepath.Join(currentDir, recipeExtFile)
	initFile := filepath.Join(dirList[len(dirList)-1], "init"+config.GrlxExt)
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

func extractIncludes(recipeName string, file []byte) ([]types.RecipeName, error) {
	recipeBytes, err := renderRecipeTemplate(recipeName, file)
	if err != nil {
		return []types.RecipeName{}, err
	}
	recipeMap, err := unmarshalRecipe(recipeBytes)
	if err != nil {
		return []types.RecipeName{}, err
	}
	return includesFromMap(recipeMap)
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

func collectAllIncludes(starter []types.RecipeName) ([]types.RecipeName, error) {
	includeMap := map[types.RecipeName]bool{}

	for _, s := range starter {
		includeMap[s] = false
	}
	names := []types.RecipeName{}
	for v := range includeMap {
		names = append(names, v)
	}
	return names, nil
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
