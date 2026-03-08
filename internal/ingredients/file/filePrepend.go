package file

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/ingredients"
)

// prependContent extracts the text content to prepend from params.
func (f File) prependContent() []byte {
	var buf bytes.Buffer
	if text, ok := f.params["text"].(string); ok && text != "" {
		buf.WriteString(text + "\n")
	} else if texti, ok := f.params["text"].([]interface{}); ok {
		for _, v := range texti {
			buf.WriteString(fmt.Sprintf("%v\n", v))
		}
	}
	return buf.Bytes()
}

func (f File) prepend(ctx context.Context, test bool) (cook.Result, error) {
	// "name": "string", "text": "[]string", "makedirs": "bool",
	// "source": "string", "source_hash": "string",
	// "template": "bool", "sources": "[]string",
	// "source_hashes": "[]string",
	var notes []fmt.Stringer
	name, ok := f.params["name"].(string)
	if !ok {
		return cook.Result{
			Succeeded: false, Failed: true, Notes: notes,
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
		}, ErrModifyRoot
	}
	res, _, err := f.contains(ctx, test)
	notes = append(notes, res.Notes...)
	if err == nil {
		// Content already present — nothing to do.
		return cook.Result{
			Succeeded: res.Succeeded, Failed: res.Failed,
			Changed: res.Changed, Notes: notes,
		}, nil
	}
	if os.IsNotExist(err) {
		content := f.prependContent()
		if test {
			notes = append(notes, cook.Snprintf("would create and prepend to %s", name))
			return cook.Result{
				Succeeded: true, Failed: false,
				Changed: true, Notes: notes,
			}, nil
		}
		if createErr := os.WriteFile(name, content, 0o644); createErr != nil {
			return cook.Result{
				Succeeded: false, Failed: true, Notes: notes,
			}, fmt.Errorf("failed to create %s: %w", name, createErr)
		}
		notes = append(notes, cook.Snprintf("prepended %v", name))
		return cook.Result{
			Succeeded: true, Failed: false,
			Changed: true, Notes: notes,
		}, nil
	}
	if errors.Is(err, ErrMissingContent) {
		content := f.prependContent()
		if test {
			notes = append(notes, cook.Snprintf("would prepend to %s", name))
			return cook.Result{
				Succeeded: true, Failed: false,
				Changed: true, Notes: notes,
			}, nil
		}
		// Read existing file contents.
		existing, readErr := os.ReadFile(name)
		if readErr != nil {
			return cook.Result{
				Succeeded: false, Failed: true, Notes: notes,
			}, fmt.Errorf("failed to read %s for prepending: %w", name, readErr)
		}
		// Build new content: prepended text + existing content.
		var combined bytes.Buffer
		combined.Write(content)
		combined.Write(existing)
		if writeErr := os.WriteFile(name, combined.Bytes(), 0o644); writeErr != nil {
			return cook.Result{
				Succeeded: false, Failed: true, Notes: notes,
			}, fmt.Errorf("failed to open %s for prepending: %w", name, writeErr)
		}
		notes = append(notes, cook.Snprintf("prepended %v", name))
		return cook.Result{
			Succeeded: true, Failed: false,
			Changed: true, Notes: notes,
		}, nil
	}
	return cook.Result{
		Succeeded: false, Failed: true,
		Changed: false, Notes: notes,
	}, err
}
