package jobs

import (
	"fmt"
	"os"
)

func removeDirIfEmpty(dir string) error {
	remaining, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading %s: %w", dir, err)
	}
	if len(remaining) > 0 {
		return nil
	}
	if err := os.Remove(dir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing empty dir %s: %w", dir, err)
	}
	return nil
}
