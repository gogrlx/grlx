package file

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestAbsentSymlink(t *testing.T) {
	for _, dangling := range []bool{false, true} {
		for _, dryRun := range []bool{false, true} {
			t.Run(fmt.Sprintf("dangling=%t/test=%t", dangling, dryRun), func(t *testing.T) {
				directory := t.TempDir()
				target := filepath.Join(directory, "target")
				link := filepath.Join(directory, "link")
				if !dangling {
					if err := os.WriteFile(target, []byte("preserve me"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.Symlink(target, link); err != nil {
					t.Fatal(err)
				}
				file := File{method: "absent", params: map[string]interface{}{"name": link}}
				result, err := file.absent(context.Background(), dryRun)
				if err != nil || !result.Succeeded || result.Failed || !result.Changed {
					t.Fatalf("absent = %+v, %v; want successful change", result, err)
				}
				_, err = os.Lstat(link)
				if dryRun && err != nil {
					t.Fatalf("dry run removed link: %v", err)
				}
				if !dryRun && !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("link still exists: %v", err)
				}
				if !dangling {
					content, err := os.ReadFile(target)
					if err != nil || string(content) != "preserve me" {
						t.Fatalf("target modified: %q, %v", content, err)
					}
				}
			})
		}
	}
}
