package jobs

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/gogrlx/grlx/v2/internal/log"
)

// StartReaper launches a background goroutine that periodically removes job
// log files older than the given TTL. It checks once per hour. A TTL of 0
// disables expiration entirely.
func (s *Store) StartReaper(ttl time.Duration) {
	if ttl <= 0 {
		log.Notice("job log expiration disabled (ttl <= 0)")
		return
	}
	log.Noticef("job log reaper started: ttl=%s", ttl)
	go func() {
		// Run once immediately on startup, then hourly.
		s.reap(ttl)
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			s.reap(ttl)
		}
	}()
}

// reap deletes job log files whose modification time is older than the TTL.
func (s *Store) reap(ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-ttl)
	sprouts, err := s.listSproutDirsUnlocked()
	if err != nil {
		log.Errorf("reaper: listing sprout dirs: %v", err)
		return
	}

	removed := 0
	for _, sproutID := range sprouts {
		sproutDir := filepath.Join(s.logDir, sproutID)
		entries, readErr := os.ReadDir(sproutDir)
		if readErr != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
				continue
			}
			info, infoErr := entry.Info()
			if infoErr != nil {
				continue
			}
			if info.ModTime().Before(cutoff) {
				jobFile := filepath.Join(sproutDir, entry.Name())
				if rmErr := os.Remove(jobFile); rmErr != nil {
					log.Errorf("reaper: removing %s: %v", jobFile, rmErr)
				} else {
					removed++
				}
				// Remove companion metadata file if it exists.
				jid := strings.TrimSuffix(entry.Name(), ".jsonl")
				metaFile := filepath.Join(sproutDir, jid+".meta.json")
				os.Remove(metaFile) // ignore error — file may not exist
			}
		}
		// Remove empty sprout directories.
		remaining, _ := os.ReadDir(sproutDir)
		if len(remaining) == 0 {
			os.Remove(sproutDir)
		}
	}

	if removed > 0 {
		log.Noticef("reaper: removed %d expired job log(s)", removed)
	}
}

// listSproutDirsUnlocked is the same as listSproutDirs but assumes the caller
// already holds the lock.
func (s *Store) listSproutDirsUnlocked() ([]string, error) {
	if s.logDir == "" {
		return nil, ErrInvalidJobDir
	}

	entries, err := os.ReadDir(s.logDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var sprouts []string
	for _, entry := range entries {
		if entry.IsDir() {
			sprouts = append(sprouts, entry.Name())
		}
	}
	return sprouts, nil
}
