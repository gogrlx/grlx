package cook

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/gogrlx/grlx/config"
)

func Cook(recipeID string) error {
	basePath := getBasePath()
	//TODO get git branch / tag from environment
	// pass in an ID to a Recipe
	recipeFilePath, err := ResolveRecipeFilePath(basePath, recipeID)
	_, _ = recipeFilePath, err

	// parse file imports
	// load all imported files into recipefile list
	// range over all keys under each recipe ID for matching ingredients
	// split on periods in ingredient name, fail and error if no matching ingredient module
	// generate ingredient ID based on Recipe ID + basename of ingredient module
	// Load all ingredients into trees
	// test all ingredients for missing, loops, duplicates, etc.
	// run all ingredients in goroutine waitgroups, sending success codes via channels
	// use reasonable timeouts for each ingredient cook
	return nil
}

func ResolveRecipeFilePath(basePath string, recipeID string) (string, error) {
	// open file if found, error out if missing , also allow for .grlx extensions
	// split the ID on periods, resolve to basename directory
	if filepath.Ext(basePath) == config.GrlxExt {
		basePath = strings.TrimSuffix(basePath, "."+config.GrlxExt)
	}
	//TODO check if basepath is completely empty first
	if !config.BasePathValid() {
		//TODO create an error type for this, wrap and return it
		return "", errors.New("")
	}
	dirList := strings.Split(recipeID, ".")
	currentDir := filepath.Join(basePath)
	for depth := 0; depth < len(dirList)-1; depth++ {
		currentDir = filepath.Join(currentDir, dirList[depth])
	}
	stat, err := os.Stat(currentDir)
	//TODO check all possible errors here
	if os.IsNotExist(err) {
		return "", err
	}
	if !stat.IsDir() {
		//TODO standardize this error type
		return "", errors.New("Path provided is not to a directory")
	}
	recipeFile := dirList[len(dirList)-1] + config.GrlxExt
	recipeFile = filepath.Join(currentDir, recipeFile)

	stat, err = os.Stat(recipeFile)
	//TODO check all possible errors here
	if os.IsNotExist(err) {
		return "", err
	}
	//TODO allow for init.grlx types etc. in the future
	if stat.IsDir() {
		//TODO standardize this error type
		return "", errors.New("Path provided is a directory")
	}
	return recipeFile, nil
}

func getBasePath() string {
	//TODO Get basepath from environment
	return "/home/tai/code/foss/grlx/testing/recipes"
}

func ParseRecipeFile() {

}

//TODO ensure ability to only run individual state (+ dependencies),
// i.e. start from a root of a given dependency tree
