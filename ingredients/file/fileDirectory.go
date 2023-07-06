package file

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	"github.com/gogrlx/grlx/types"
)

func (f File) directory(ctx context.Context, test bool) (types.Result, error) {
	changes := []fmt.Stringer{}
	type dir struct {
		user           string
		group          string
		recurse        bool
		maxDepth       int
		dirMode        string
		fileMode       string
		makeDirs       bool
		clean          bool
		followSymlinks bool
		force          bool
		backupName     string
		allowSymlink   bool
	}
	name, ok := f.params["name"].(string)
	if !ok {
		return types.Result{Succeeded: false, Failed: true, Notes: changes, Changed: changes != nil}, types.ErrMissingName
	}
	name = filepath.Clean(name)
	if name == "" {
		return types.Result{Succeeded: false, Failed: true}, types.ErrMissingName
	}
	if name == "/" {
		return types.Result{Succeeded: false, Failed: true}, fmt.Errorf("refusing to delete root")
	}
	d := dir{}
	// create the directory if it doesn't exist
	{
		// create the dir if "makeDirs" is true or not defined
		if val, ok := f.params["makedirs"].(bool); ok && val || !ok {
			d.makeDirs = true
			errCreate := os.MkdirAll(name, 0o755)
			if errCreate != nil {
				return types.Result{Succeeded: false, Failed: true}, errCreate
			}

		}
	}
	// chown the directory to the named user
	// TODO add test apply path here
	{
		if val, ok := f.params["user"].(string); ok {
			d.user = val
			userL, lookupErr := user.Lookup(d.user)
			if lookupErr != nil {
				return types.Result{Succeeded: false, Failed: true}, lookupErr
			}
			uid, parseErr := strconv.ParseUint(userL.Uid, 10, 32)
			if parseErr != nil {
				return types.Result{Succeeded: false, Failed: true}, parseErr
			}

			chownErr := os.Chown(name, int(uid), -1)
			if chownErr != nil {
				return types.Result{Succeeded: false, Failed: true}, chownErr
			}
			if val, ok := f.params["recurse"].(bool); ok && val {
				walkErr := filepath.WalkDir(name, func(path string, d fs.DirEntry, err error) error {
					return os.Chown(path, int(uid), -1)
				})
				if walkErr != nil {
					return types.Result{Succeeded: false, Failed: true}, walkErr
				}
			}
		}
	}
	// chown the directory to the named group
	// TODO add test apply path here
	{
		if val, ok := f.params["group"].(string); ok {
			d.group = val
			group, lookupErr := user.LookupGroup(d.group)
			if lookupErr != nil {
				return types.Result{Succeeded: false, Failed: true}, lookupErr
			}
			gid, parseErr := strconv.ParseUint(group.Gid, 10, 32)
			if parseErr != nil {
				return types.Result{Succeeded: false, Failed: true}, parseErr
			}
			chownErr := os.Chown(name, -1, int(gid))
			if chownErr != nil {
				return types.Result{Succeeded: false, Failed: true}, chownErr
			}
			if val, ok := f.params["recurse"].(bool); ok && val {
				walkErr := filepath.WalkDir(name, func(path string, d fs.DirEntry, err error) error {
					return os.Chown(path, -1, int(gid))
				})
				if walkErr != nil {
					return types.Result{Succeeded: false, Failed: true}, walkErr
				}
			}
		}
	}
	// chmod the directory to the named dirmode if it is defined
	// TODO add test apply path here
	{
		if val, ok := f.params["dir_mode"].(string); ok {
			d.dirMode = val
			modeVal, _ := strconv.ParseUint(d.dirMode, 8, 32)
			// "dir_mode": "string", "file_mode": "string"
			//"clean": "bool", "follow_symlinks": "bool", "force": "bool", "backupname": "string", "allow_symlink": "bool",
			err := os.Chmod(name, os.FileMode(modeVal))
			if err != nil {
				return types.Result{Succeeded: false, Failed: true}, err
			}
		}
	}
	// chmod the directory to the named dirmode if it is defined
	// TODO add test apply path here
	{
		if val, ok := f.params["file_mode"].(string); ok {
			d.fileMode = val
			modeVal, _ := strconv.ParseUint(d.fileMode, 8, 32)
			// "makedirs": "bool",
			//"clean": "bool", "follow_symlinks": "bool", "force": "bool", "backupname": "string", "allow_symlink": "bool",
			err := os.Chmod(name, os.FileMode(modeVal))
			if err != nil {
				return types.Result{Succeeded: false, Failed: true}, err
			}
		}
	} // recurse the file_mode if it is defined
	// TODO add test apply path here
	{
		if val, ok := f.params["group"].(string); ok {
			d.group = val
			group, lookupErr := user.LookupGroup(d.group)
			if lookupErr != nil {
				return types.Result{Succeeded: false, Failed: true}, lookupErr
			}
			gid, parseErr := strconv.ParseUint(group.Gid, 10, 32)
			if parseErr != nil {
				return types.Result{Succeeded: false, Failed: true}, parseErr
			}
			chownErr := os.Chown(name, -1, int(gid))
			if chownErr != nil {
				return types.Result{Succeeded: false, Failed: true}, chownErr
			}
			if val, ok := f.params["recurse"].(bool); ok && val {
				walkErr := filepath.WalkDir(name, func(path string, d fs.DirEntry, err error) error {
					return os.Chown(path, -1, int(gid))
				})
				if walkErr != nil {
					return types.Result{Succeeded: false, Failed: true}, walkErr
				}
			}
		}
	}

	return f.undef()
}
