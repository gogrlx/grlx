package jobs

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gogrlx/grlx/v2/internal/cook"
)

// CLIJobMeta holds per-job metadata tracked on the CLI side,
// including the identity of the user who initiated the job.
type CLIJobMeta struct {
	JID       string    `json:"jid"`
	SproutID  string    `json:"sprout_id"`
	Recipe    string    `json:"recipe,omitempty"`
	UserKey   string    `json:"user_key"`
	CreatedAt time.Time `json:"created_at"`
}

var (
	ErrCLIStoreNotInitialized = errors.New("CLI job store not initialized")
	ErrMetaNotFound           = errors.New("job metadata not found")
)

// CLIStore provides local job storage on the CLI user's machine.
// Job data is stored under ~/.config/grlx/jobs/<sproutID>/<jid>.jsonl
// with a companion .meta.json file for per-user tracking.
type CLIStore struct {
	mu     sync.RWMutex
	logDir string
}

// NewCLIStore creates a CLIStore using the given base directory.
// Typically this is ~/.config/grlx/jobs/.
func NewCLIStore(dir string) (*CLIStore, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("creating CLI job store dir: %w", err)
	}
	return &CLIStore{logDir: dir}, nil
}

// DefaultCLIStorePath returns the default CLI job store directory.
func DefaultCLIStorePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "grlx", "jobs"), nil
}

// RecordJobStart creates the initial job files: a .meta.json with user
// identity and an empty .jsonl ready for step completions.
func (s *CLIStore) RecordJobStart(meta CLIJobMeta) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sproutDir := filepath.Join(s.logDir, meta.SproutID)
	if err := os.MkdirAll(sproutDir, 0o700); err != nil {
		return fmt.Errorf("creating sprout dir: %w", err)
	}

	// Write metadata file.
	metaFile := filepath.Join(sproutDir, fmt.Sprintf("%s.meta.json", meta.JID))
	metaData, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshaling job meta: %w", err)
	}
	if err := os.WriteFile(metaFile, metaData, 0o600); err != nil {
		return fmt.Errorf("writing job meta: %w", err)
	}

	// Create empty JSONL file if it doesn't exist.
	jobFile := filepath.Join(sproutDir, fmt.Sprintf("%s.jsonl", meta.JID))
	if _, statErr := os.Stat(jobFile); errors.Is(statErr, os.ErrNotExist) {
		f, createErr := os.Create(jobFile)
		if createErr != nil {
			return fmt.Errorf("creating job file: %w", createErr)
		}
		f.Close()
	}

	return nil
}

// AppendStep appends a step completion to the local JSONL file for a job.
func (s *CLIStore) AppendStep(sproutID, jid string, step cook.StepCompletion) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sproutDir := filepath.Join(s.logDir, sproutID)
	if err := os.MkdirAll(sproutDir, 0o700); err != nil {
		return fmt.Errorf("creating sprout dir: %w", err)
	}

	jobFile := filepath.Join(sproutDir, fmt.Sprintf("%s.jsonl", jid))
	f, err := os.OpenFile(jobFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("opening job file: %w", err)
	}
	defer f.Close()

	b, err := json.Marshal(step)
	if err != nil {
		return fmt.Errorf("marshaling step: %w", err)
	}
	if _, err := f.Write(b); err != nil {
		return fmt.Errorf("writing step: %w", err)
	}
	if _, err := f.WriteString("\n"); err != nil {
		return fmt.Errorf("writing newline: %w", err)
	}

	return nil
}

// GetJobMeta retrieves the metadata for a job, searching all sprout directories.
func (s *CLIStore) GetJobMeta(jid string) (*CLIJobMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sprouts, err := s.listSproutDirs()
	if err != nil {
		return nil, err
	}

	for _, sproutID := range sprouts {
		metaFile := filepath.Join(s.logDir, sproutID, fmt.Sprintf("%s.meta.json", jid))
		data, readErr := os.ReadFile(metaFile)
		if readErr != nil {
			continue
		}
		var meta CLIJobMeta
		if unmarshalErr := json.Unmarshal(data, &meta); unmarshalErr != nil {
			continue
		}
		return &meta, nil
	}

	return nil, ErrMetaNotFound
}

// GetJob retrieves a job summary from local storage.
func (s *CLIStore) GetJob(jid string) (*JobSummary, *CLIJobMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sprouts, err := s.listSproutDirs()
	if err != nil {
		return nil, nil, err
	}

	for _, sproutID := range sprouts {
		jobFile := filepath.Join(s.logDir, sproutID, fmt.Sprintf("%s.jsonl", jid))
		steps, readErr := readJobFile(jobFile)
		if readErr != nil {
			continue
		}

		summary := buildSummary(jid, sproutID, steps)

		// Try to load meta (optional).
		var meta *CLIJobMeta
		metaFile := filepath.Join(s.logDir, sproutID, fmt.Sprintf("%s.meta.json", jid))
		if metaData, metaErr := os.ReadFile(metaFile); metaErr == nil {
			var m CLIJobMeta
			if json.Unmarshal(metaData, &m) == nil {
				meta = &m
			}
		}

		return summary, meta, nil
	}

	return nil, nil, ErrJobNotFound
}

// ListJobs returns all locally stored job summaries, sorted by start time
// (most recent first). If userKey is non-empty, only jobs initiated by
// that user are returned. If sproutFilter is non-empty, only jobs for
// that sprout are returned.
func (s *CLIStore) ListJobs(limit int, userKey string, sproutFilter string) ([]JobSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sprouts, err := s.listSproutDirs()
	if err != nil {
		return nil, err
	}

	var allSummaries []JobSummary
	for _, sproutID := range sprouts {
		if sproutFilter != "" && sproutID != sproutFilter {
			continue
		}
		sproutDir := filepath.Join(s.logDir, sproutID)
		entries, readErr := os.ReadDir(sproutDir)
		if readErr != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
				continue
			}
			jid := strings.TrimSuffix(entry.Name(), ".jsonl")

			// Filter by user if requested.
			if userKey != "" {
				metaFile := filepath.Join(sproutDir, fmt.Sprintf("%s.meta.json", jid))
				if metaData, metaErr := os.ReadFile(metaFile); metaErr == nil {
					var meta CLIJobMeta
					if json.Unmarshal(metaData, &meta) == nil && meta.UserKey != userKey {
						continue
					}
				}
			}

			jobFile := filepath.Join(sproutDir, entry.Name())
			steps, fileErr := readJobFile(jobFile)
			if fileErr != nil {
				continue
			}
			allSummaries = append(allSummaries, *buildSummary(jid, sproutID, steps))
		}
	}

	sort.Slice(allSummaries, func(i, j int) bool {
		return allSummaries[i].StartedAt.After(allSummaries[j].StartedAt)
	})

	if limit > 0 && len(allSummaries) > limit {
		allSummaries = allSummaries[:limit]
	}

	return allSummaries, nil
}

// listSproutDirs returns directory names under the store dir.
func (s *CLIStore) listSproutDirs() ([]string, error) {
	entries, err := os.ReadDir(s.logDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading CLI job store: %w", err)
	}

	var sprouts []string
	for _, entry := range entries {
		if entry.IsDir() {
			sprouts = append(sprouts, entry.Name())
		}
	}
	return sprouts, nil
}
