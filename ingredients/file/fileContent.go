package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gogrlx/grlx/types"
)

func (f File) content(ctx context.Context, test bool) (types.Result, error) {
	// TODO
	// "text": "[]string",
	// "makedirs": "bool", "source": "string",
	// "source_hash": "string", "template": "bool",
	// "sources": "[]string", "source_hashes": "[]string",
	name := ""
	makedirs := false
	source := ""
	sourceHash := ""
	text := []string{}
	template := false
	sources := []string{}
	sourceHashes := []string{}
	skipVerify := false
	foundSource := false
	notedChanges := []fmt.Stringer{}
	var ok bool
	{
		name, ok = f.params["name"].(string)
		if !ok {
			return types.Result{Succeeded: false, Failed: true}, types.ErrMissingName
		}
		name = filepath.Clean(name)
		if name == "" {
			return types.Result{Succeeded: false, Failed: true}, types.ErrMissingName
		}
		if name == "/" {
			return types.Result{Succeeded: false, Failed: true}, types.ErrModifyRoot
		}
	}
	{
		makedirs, _ = f.params["makedirs"].(bool)
		dir := filepath.Dir(name)
		_, statErr := os.Stat(dir)
		if os.IsNotExist(statErr) && makedirs {
			err := os.MkdirAll(dir, 0o755)
			if err != nil {
				return types.Result{
					Succeeded: false, Failed: true,
					Changed: false, Notes: nil,
				}, err
			}
			notedChanges = append(notedChanges, types.SimpleNote(fmt.Sprintf("created directory %s", dir)))
		} else if statErr != nil {
			return types.Result{
				Succeeded: false, Failed: true,
				Changed: false, Notes: nil,
			}, statErr
		}
	}
	{
		skipVerify, _ = f.params["skip_verify"].(bool)
		if skipVerify {
			_, statErr := os.Stat(name)
			if statErr == nil {
				return types.Result{
					Succeeded: false, Failed: true,
					Changed: len(notedChanges) != 0, Notes: notedChanges,
				}, nil
			} else if !os.IsNotExist(statErr) {
				return types.Result{
					Succeeded: false, Failed: true,
					Changed: len(notedChanges) != 0, Notes: notedChanges,
				}, statErr
			}
		}
	}
	{
		source, _ = f.params["source"].(string)
		sourceHash, _ = f.params["source_hash"].(string)
		if source != "" && sourceHash == "" && !skipVerify {
			return types.Result{Succeeded: false, Failed: true}, types.ErrMissingHash
		} else if source != "" {
			foundSource = true
		}
	}
	{
		if texts, ok := f.params["text"].(string); ok && texts != "" {
			text = []string{texts}
			foundSource = true
		} else if texti, ok := f.params["text"].([]interface{}); ok {
			for _, v := range texti {
				// need to make sure it's a string and not yaml parsing as an int
				text = append(text, fmt.Sprintf("%v", v))
			}
			foundSource = true
		}
	}

	return f.undef()
}
