package jobs

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/gogrlx/grlx/v2/internal/log"
)

// StartSproutReaper launches a background goroutine that periodically removes
// job log files older than ttl from a flat job log directory (the sprout-side
// layout, where files live directly under logDir as <jid>.jsonl rather than in
// per-sprout subdirectories). It checks once per hour and stops when ctx is
// cancelled. A ttl of 0 or negative disables expiration entirely.
func StartSproutReaper(ctx context.Context, logDir string, ttl time.Duration) {
	if ttl <= 0 {
		log.Notice("sprout job log expiration disabled (ttl <= 0)")
		return
	}
	if logDir == "" {
		return
	}
	log.Noticef("sprout job log reaper started: ttl=%s", ttl)
	go func() {
		reapFlatDir(logDir, ttl)
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Notice("sprout job log reaper stopped")
				return
			case <-ticker.C:
				reapFlatDir(logDir, ttl)
			}
		}
	}()
}

// reapFlatDir deletes *.jsonl files (and companion *.meta.json files) directly
// under logDir whose modification time is older than ttl.
func reapFlatDir(logDir string, ttl time.Duration) {
	cutoff := time.Now().Add(-ttl)
	entries, err := os.ReadDir(logDir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Errorf("sprout reaper: reading %s: %v", logDir, err)
		}
		return
	}
	removed := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			jobFile := filepath.Join(logDir, entry.Name())
			if rmErr := os.Remove(jobFile); rmErr != nil {
				log.Errorf("sprout reaper: removing %s: %v", jobFile, rmErr)
				continue
			}
			jobID := strings.TrimSuffix(entry.Name(), ".jsonl")
			metaFile := filepath.Join(logDir, jobID+".meta.json")
			os.Remove(metaFile)
			removed++
		}
	}
	if removed > 0 {
		log.Noticef("sprout reaper: removed %d expired job log(s)", removed)
	}
}
