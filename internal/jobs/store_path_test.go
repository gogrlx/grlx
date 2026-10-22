package jobs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreRejectsPathTraversal(t *testing.T) {
	for _, identifier := range []string{"../../outside", "/absolute", "../outside", "", ".", "..", "nested/name", `nested\name`} {
		t.Run(identifier, func(t *testing.T) {
			root := t.TempDir()
			logDir := filepath.Join(root, "jobs")
			if err := os.MkdirAll(filepath.Join(logDir, "sprout"), 0o700); err != nil {
				t.Fatal(err)
			}
			outside := filepath.Join(root, "outside.jsonl")
			if err := os.WriteFile(outside, []byte(""), 0o600); err != nil {
				t.Fatal(err)
			}
			store := NewStoreWithDir(logDir)
			if _, err := store.GetJob("sprout", identifier); !errors.Is(err, ErrInvalidJobID) {
				t.Error("GetJob accepted invalid job ID")
			}
			if _, err := store.FindJob(identifier); !errors.Is(err, ErrInvalidJobID) {
				t.Error("FindJob accepted invalid job ID")
			}
			if err := store.DeleteJob(identifier); !errors.Is(err, ErrInvalidJobID) {
				t.Error("DeleteJob accepted invalid job ID")
			}
			if _, err := store.GetJob(identifier, "outside"); !errors.Is(err, ErrInvalidJobID) {
				t.Error("GetJob accepted invalid sprout ID")
			}
			if _, err := store.ListJobsForSprout(identifier); !errors.Is(err, ErrInvalidJobID) {
				t.Error("ListJobsForSprout accepted invalid sprout ID")
			}
			if _, err := store.CountJobsForSprout(identifier); !errors.Is(err, ErrInvalidJobID) {
				t.Error("CountJobsForSprout accepted invalid sprout ID")
			}
			if _, err := os.Stat(outside); err != nil {
				t.Fatalf("outside file was removed: %v", err)
			}
		})
	}
}
