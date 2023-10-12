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
	notes := []fmt.Stringer{}
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
		return types.Result{
			Succeeded: false, Failed: true,
			Notes: notes, Changed: false,
		}, types.ErrMissingName
	}
	name = filepath.Clean(name)
	if name == "" {
		return types.Result{
			Succeeded: false, Failed: true, Notes: notes,
		}, types.ErrMissingName
	}
	if name == "/" {
		return types.Result{
			Succeeded: false, Failed: true, Notes: notes,
		}, types.ErrDeleteRoot
	}
	d := dir{}
	// create the directory if it doesn't exist
	{
		// create the dir if "makeDirs" is true or not defined
		if val, ok := f.params["makedirs"].(bool); ok && val || !ok {
			d.makeDirs = true
			errCreate := os.MkdirAll(name, 0o755)
			notes = append(notes, types.Snprintf("creating directory %s", name))
			if errCreate != nil {
				return types.Result{
					Succeeded: false, Failed: true, Notes: notes,
				}, errCreate
			}

		}
		// TODO: Bug, this should be moved to potentially NOT create the directory
		if test {
			notes = append(notes, types.Snprintf("would create directory %s", name))
		}
	}
	// chown the directory to the named user
	{
		if val, ok := f.params["user"].(string); ok {
			d.user = val
			userL, lookupErr := user.Lookup(d.user)
			if lookupErr != nil {
				return types.Result{
					Succeeded: false, Failed: true, Notes: notes,
				}, lookupErr
			}
			uid, parseErr := strconv.ParseUint(userL.Uid, 10, 32)
			if parseErr != nil {
				return types.Result{
					Succeeded: false, Failed: true, Notes: notes,
				}, parseErr
			}

			if test {
				notes = append(notes, types.Snprintf("would chown %s to %s", name, d.user))
			} else {
				chownErr := os.Chown(name, int(uid), -1)
				if chownErr != nil {
					return types.Result{
						Succeeded: false, Failed: true, Notes: notes,
					}, chownErr
				}
				notes = append(notes, types.Snprintf("chown %s to %s", name, d.user))
			}
			if val, ok := f.params["recurse"].(bool); ok && val {
				walkErr := filepath.WalkDir(name, func(path string, d fs.DirEntry, err error) error {
					if test {
						notes = append(notes, types.Snprintf("would chown %s to %s", name, val))
						return nil
					}
					notes = append(notes, types.Snprintf("chown %s to %s", name, val))
					return os.Chown(path, int(uid), -1)
				})
				if walkErr != nil {
					return types.Result{
						Succeeded: false, Failed: true, Notes: notes,
					}, walkErr
				}
			}
		}
	}
	// chown the directory to the named group
	{
		if val, ok := f.params["group"].(string); ok {
			d.group = val
			group, lookupErr := user.LookupGroup(d.group)
			if lookupErr != nil {
				return types.Result{
					Succeeded: false, Failed: true, Notes: notes,
				}, lookupErr
			}
			gid, parseErr := strconv.ParseUint(group.Gid, 10, 32)
			if parseErr != nil {
				return types.Result{
					Succeeded: false, Failed: true, Notes: notes,
				}, parseErr
			}
			if test {
				notes = append(notes, types.Snprintf("would chown %s to %s", name, d.group))
			} else {
				chownErr := os.Chown(name, -1, int(gid))
				if chownErr != nil {
					return types.Result{
						Succeeded: false, Failed: true, Notes: notes,
					}, chownErr
				}
				notes = append(notes, types.Snprintf("chown %s to %s", name, d.group))
			}
			if val, ok := f.params["recurse"].(bool); ok && val {
				walkErr := filepath.WalkDir(name, func(path string, d fs.DirEntry, err error) error {
					if test {
						notes = append(notes, types.Snprintf("would chown %s to %s", name, val))
						return nil
					}
					notes = append(notes, types.Snprintf("chown %s to %s", name, val))
					return os.Chown(path, -1, int(gid))
				})
				if walkErr != nil {
					return types.Result{
						Succeeded: false, Failed: true, Notes: notes,
					}, walkErr
				}
			}
		}
	}
	// chmod the directory to the named dirmode if it is defined
	{
		// TODO: Bug, this should at least be able to return a successful result
		if val, ok := f.params["dir_mode"].(string); ok {
			d.dirMode = val
			modeVal, _ := strconv.ParseUint(d.dirMode, 8, 32)
			// "dir_mode": "string", "file_mode": "string"
			//"clean": "bool", "follow_symlinks": "bool", "force": "bool", "backupname": "string", "allow_symlink": "bool",
			if test {
				notes = append(notes, types.Snprintf("would chmod %s to %s", name, val))
			} else {
				err := os.Chmod(name, os.FileMode(modeVal))
				if err != nil {
					return types.Result{
						Succeeded: false, Failed: true, Notes: notes,
					}, err
				}
				notes = append(notes, types.Snprintf("chmod %s to %s", name, val))
			}
		}
	}
	// chmod the directory to the named dirmode if it is defined
	{
		if val, ok := f.params["file_mode"].(string); ok {
			d.fileMode = val
			modeVal, _ := strconv.ParseUint(d.fileMode, 8, 32)
			// "makedirs": "bool",
			//"clean": "bool", "follow_symlinks": "bool", "force": "bool", "backupname": "string", "allow_symlink": "bool",
			if test {
				notes = append(notes, types.Snprintf("would chmod %s to %s", name, val))
			} else {
				err := os.Chmod(name, os.FileMode(modeVal))
				if err != nil {
					return types.Result{
						Succeeded: false, Failed: true, Notes: notes,
					}, err
				}
			}
		}
	} // recurse the file_mode if it is defined
	{
		if val, ok := f.params["group"].(string); ok {
			d.group = val
			group, lookupErr := user.LookupGroup(d.group)
			if lookupErr != nil {
				return types.Result{
					Succeeded: false, Failed: true, Notes: notes,
				}, lookupErr
			}
			gid, parseErr := strconv.ParseUint(group.Gid, 10, 32)
			if parseErr != nil {
				return types.Result{
					Succeeded: false, Failed: true,
				}, parseErr
			}
			if test {
				notes = append(notes, types.Snprintf("would chown %s to %s", name, d.group))
			} else {
				chownErr := os.Chown(name, -1, int(gid))
				if chownErr != nil {
					return types.Result{
						Succeeded: false, Failed: true,
					}, chownErr
				}
				notes = append(notes, types.Snprintf("chown %s to %s", name, d.group))
			}
			if val, ok := f.params["recurse"].(bool); ok && val {
				walkErr := filepath.WalkDir(name, func(path string, d fs.DirEntry, err error) error {
					if test {
						notes = append(notes, types.Snprintf("would chown %s to %s", name, val))
						return nil
					}
					notes = append(notes, types.Snprintf("chown %s to %s", name, val))
					return os.Chown(path, -1, int(gid))
				})
				if walkErr != nil {
					return types.Result{
						Succeeded: false, Failed: true,
					}, walkErr
				}
			}
		}
	}

	// TODO: Bug, any directory operations will report as a failure and can never succeed
	out, err := f.undef()
	out.Notes = append(notes, out.Notes...)
	return out, err
}
