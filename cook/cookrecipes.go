package cook

import (
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
		err = yaml.Unmarshal(b, &recipe)
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

func ParseRecipeFile(recipeName types.RecipeName) []types.RecipeCooker {
	return nil
}

// TODO ensure ability to only run individual state (+ dependencies),
// i.e. start from a root of a given dependency tree
