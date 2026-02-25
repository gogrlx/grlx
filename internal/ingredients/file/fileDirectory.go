package file

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/ingredients"
)

func (f File) directory(ctx context.Context, test bool) (cook.Result, error) {
	notes := []fmt.Stringer{}
	type dir struct {
		user     string
		group    string
		dirMode  string
		fileMode string
		makeDirs bool
	}
	name, ok := f.params["name"].(string)
	if !ok {
		return cook.Result{
			Succeeded: false, Failed: true,
			Notes: notes, Changed: false,
		}, ingredients.ErrMissingName
	}
	name = filepath.Clean(name)
	if name == "" {
		return cook.Result{
			Succeeded: false, Failed: true, Notes: notes,
		}, ingredients.ErrMissingName
	}
	if name == "/" {
		return cook.Result{
			Succeeded: false, Failed: true, Notes: notes,
		}, ErrDeleteRoot
	}
	d := dir{}
	// create the directory if it doesn't exist
	{
		// create the dir if "makeDirs" is true or not defined
		if val, ok := f.params["makedirs"].(bool); ok && val || !ok {
			d.makeDirs = true
			st, statErr := os.Stat(name)
			dirExists := statErr == nil && st.IsDir()
			if test {
				if dirExists {
					notes = append(notes, cook.Snprintf("directory %s already exists", name))
				} else {
					notes = append(notes, cook.Snprintf("would create directory %s", name))
				}
			} else {
				if dirExists {
					notes = append(notes, cook.Snprintf("directory %s already exists", name))
				} else {
					errCreate := os.MkdirAll(name, 0o755)
					if errCreate != nil {
						notes = append(notes, cook.Snprintf("failed to create directory %s", name))
						return cook.Result{
							Succeeded: false, Failed: true, Notes: notes,
						}, errCreate
					}
					notes = append(notes, cook.Snprintf("created directory %s", name))
				}
			}

		}
	}
	// chown the directory to the named user
	{
		if val, ok := f.params["user"].(string); ok {
			d.user = val
			userL, lookupErr := user.Lookup(d.user)
			if lookupErr != nil {
				return cook.Result{
					Succeeded: false, Failed: true, Notes: notes,
				}, lookupErr
			}
			uid, parseErr := strconv.ParseUint(userL.Uid, 10, 32)
			if parseErr != nil {
				return cook.Result{
					Succeeded: false, Failed: true, Notes: notes,
				}, parseErr
			}

			if test {
				notes = append(notes, cook.Snprintf("would chown %s to %s", name, d.user))
			} else {
				chownErr := os.Chown(name, int(uid), -1)
				if chownErr != nil {
					return cook.Result{
						Succeeded: false, Failed: true, Notes: notes,
					}, chownErr
				}
				notes = append(notes, cook.Snprintf("chown %s to %s", name, d.user))
			}
			if val, ok := f.params["recurse"].(bool); ok && val {
				walkErr := filepath.WalkDir(name, func(path string, d fs.DirEntry, err error) error {
					if test {
						notes = append(notes, cook.Snprintf("would chown %s to %s", name, val))
						return nil
					}
					notes = append(notes, cook.Snprintf("chown %s to %s", name, val))
					return os.Chown(path, int(uid), -1)
				})
				if walkErr != nil {
					return cook.Result{
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
				return cook.Result{
					Succeeded: false, Failed: true, Notes: notes,
				}, lookupErr
			}
			gid, parseErr := strconv.ParseUint(group.Gid, 10, 32)
			if parseErr != nil {
				return cook.Result{
					Succeeded: false, Failed: true, Notes: notes,
				}, parseErr
			}
			if test {
				notes = append(notes, cook.Snprintf("would chown %s to %s", name, d.group))
			} else {
				chownErr := os.Chown(name, -1, int(gid))
				if chownErr != nil {
					return cook.Result{
						Succeeded: false, Failed: true, Notes: notes,
					}, chownErr
				}
				notes = append(notes, cook.Snprintf("chown %s to %s", name, d.group))
			}
			if val, ok := f.params["recurse"].(bool); ok && val {
				walkErr := filepath.WalkDir(name, func(path string, d fs.DirEntry, err error) error {
					if test {
						notes = append(notes, cook.Snprintf("would chown %s to %s", name, val))
						return nil
					}
					notes = append(notes, cook.Snprintf("chown %s to %s", name, val))
					return os.Chown(path, -1, int(gid))
				})
				if walkErr != nil {
					return cook.Result{
						Succeeded: false, Failed: true, Notes: notes,
					}, walkErr
				}
			}
		}
	}
	// chmod the directory to the named dirmode if it is defined
	{
		if val, ok := f.params["dir_mode"].(string); ok {
			d.dirMode = val
			modeVal, parseErr := strconv.ParseUint(d.dirMode, 8, 32)
			if parseErr != nil {
				return cook.Result{
					Succeeded: false, Failed: true, Notes: notes,
				}, fmt.Errorf("invalid dir_mode %q: %w", d.dirMode, parseErr)
			}
			if test {
				notes = append(notes, cook.Snprintf("would chmod %s to %s", name, val))
			} else {
				err := os.Chmod(name, os.FileMode(modeVal))
				if err != nil {
					return cook.Result{
						Succeeded: false, Failed: true, Notes: notes,
					}, err
				}
				notes = append(notes, cook.Snprintf("chmod %s to %s", name, val))
			}
		}
	}
	// chmod files to the named file_mode if it is defined
	{
		if val, ok := f.params["file_mode"].(string); ok {
			d.fileMode = val
			modeVal, parseErr := strconv.ParseUint(d.fileMode, 8, 32)
			if parseErr != nil {
				return cook.Result{
					Succeeded: false, Failed: true, Notes: notes,
				}, fmt.Errorf("invalid file_mode %q: %w", d.fileMode, parseErr)
			}
			if test {
				notes = append(notes, cook.Snprintf("would chmod %s to %s", name, val))
			} else {
				err := os.Chmod(name, os.FileMode(modeVal))
				if err != nil {
					return cook.Result{
						Succeeded: false, Failed: true, Notes: notes,
					}, err
				}
			}
		}
	}

	return cook.Result{
		Succeeded: true, Failed: false, Notes: notes,
	}, nil
}
