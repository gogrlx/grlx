package cook

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gogrlx/grlx/config"
	"github.com/gogrlx/grlx/props"
	"github.com/gogrlx/grlx/types"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

func populateFuncMap(sproutID string) template.FuncMap {
	v := template.FuncMap{}
	v["props"] = props.GetPropFunc(sproutID)
	//	v["secrets"] = secrets.GetSecretFunc(sproutID)
	return v
}

func Cook(sproutID string, recipeID types.RecipeName) (string, error) {
	basepath := getBasePath()
	includes, err := collectAllIncludes(sproutID, basepath, recipeID)
	if err != nil {
		return "", err
	}
	recipesteps := make(map[string]interface{})
	for _, inc := range includes {
		// load all imported files into recipefile list
		fp, err := ResolveRecipeFilePath(basepath, inc)
		if err != nil {
			// TODO: wrap this error and explain file existed but no longer exists
			return "", err
		}
		f, err := os.ReadFile(fp)
		if err != nil {
			return "", err
		}
		b, err := renderRecipeTemplate(sproutID, fp, f)
		if err != nil {
			return "", err
		}
		var recipe map[string]interface{}
		err = yaml.Unmarshal(b, &recipe)
		if err != nil {
			return "", err
		}
		m, err := stepsFromMap(recipe)
		if err != nil {
			return "", err
		}
		// range over all keys under each recipe ID for matching ingredients
		recipesteps, err = joinMaps(recipesteps, m)
		if err != nil {
			return "", err
		}
	}
	for id, step := range recipesteps {
		switch s := step.(type) {
		case map[string]interface{}:
			if len(s) != 1 {
				return "", fmt.Errorf("recipe %s must have one directive, but has %d", id, len(s))
			}

		default:
			return "", fmt.Errorf("recipe %s must me a map[string]interface{} but found %T", id, step)
		}
	}
	steps, err := makeRecipeSteps(recipesteps)
	tree, errs := getRecipeTree(steps)
	if len(errs) > 0 {
		for _, err := range errs {
			if err != nil {
				return "", err
			}
		}
	}
	jid := GenerateJobID()
	// here, send out the tree to be executed to the sprout over NATS, and send back the JobID
	_ = tree
	return jid, nil
}

func GenerateJobID() string {
	return uuid.New().String()
}

func ResolveRecipeFilePath(basepath string, recipeID types.RecipeName) (string, error) {
	path := string(recipeID)
	basepath = filepath.Clean(basepath)
	path = filepath.Clean(path)
	path = strings.TrimPrefix(path, basepath)
	path = filepath.Join(basepath, path)
	if strings.HasSuffix(path, "."+config.GrlxExt) {
		// swap out dot notation for slashes, but preserve extension
		path = strings.TrimSuffix(path, "."+config.GrlxExt)
		path = strings.ReplaceAll(path, ".", string(filepath.Separator))
		path = path + "." + config.GrlxExt

		stat, err := os.Stat(path)
		if os.IsNotExist(err) {
			return "", err
		}
		if stat.IsDir() {
			return "", errors.New("path provided is a directory, directory must not end in .grlx")
		}
		return path, nil
	}
	// at this point, we know the path doesn't end in .grlx

	path = strings.ReplaceAll(path, ".", string(filepath.Separator))
	// check if path is a directory and contains init.grlx
	initFile := filepath.Join(path, "init."+config.GrlxExt)
	stat, err := os.Stat(initFile)
	if err == nil {
		if stat.IsDir() {
			return "", errors.New("path provided is a directory, directory must not end in .grlx")
		}
		return initFile, nil
	}

	// check if path is a valid .grlx file
	extPath := path + "." + config.GrlxExt
	stat, err = os.Stat(extPath)
	if err == nil {
		if stat.IsDir() {
			// TODO standardize this error type
			return "", errors.New("init.grlx cannot be a directory")
		}
		return extPath, nil
	} else {
		return "", err
	}
}

func ParseRecipeFile(recipeName types.RecipeName) []types.RecipeCooker {
	return nil
}

// TODO ensure ability to only run individual state (+ dependencies),
// i.e. start from a root of a given dependency tree
