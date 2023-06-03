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
	// TODO: implement secrets and other template functions
	//	v["secrets"] = secrets.GetSecretFunc(sproutID)
	return v
}

func Cook(sproutID string, recipeID types.RecipeName, JID string) error {
	basepath := getBasePath()
	includes, err := collectAllIncludes(sproutID, basepath, recipeID)
	if err != nil {
		return err
	}
	recipesteps := make(map[string]interface{})
	for _, inc := range includes {
		// load all imported files into recipefile list
		fp, fpErr := ResolveRecipeFilePath(basepath, inc)
		if fpErr != nil {
			return errors.Join(ErrNoRecipe, fpErr)
		}
		f, fpErr := os.ReadFile(fp)
		if fpErr != nil {
			return fpErr
		}
		b, renderErr := renderRecipeTemplate(sproutID, fp, f)
		if renderErr != nil {
			return renderErr
		}
		var recipe map[string]interface{}
		marshallErr := yaml.Unmarshal(b, &recipe)
		if marshallErr != nil {
			return marshallErr
		}
		m, loadErr := stepsFromMap(recipe)
		if loadErr != nil {
			return loadErr
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
				return errors.Join(ErrInvalidFormat, fmt.Errorf("recipe %s must have one directive, but has %d", id, len(s)))
			}

		default:
			return errors.Join(ErrInvalidFormat, fmt.Errorf("recipe %s must me a map[string]interface{} but found %T", id, step))
		}
	}
	steps, err := makeRecipeSteps(recipesteps)
	if err != nil {
		return err
	}
	tree, err := validateRecipeTree(steps)
	if err != nil {
		return err
	}
	validSteps := []types.Step{}
	for _, step := range tree {
		validSteps = append(validSteps, *step)
	}
	fmt.Println(validSteps)

	// here, send out the tree to be executed to the sprout over NATS, and send back the JobID

	return nil
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

// func ParseRecipeFile(recipeName types.RecipeName) []types.RecipeCooker {
// 	return nil
// }

// TODO ensure ability to only run individual state (+ dependencies),
// i.e. start from a root of a given dependency tree
