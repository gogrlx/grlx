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

	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/cook"
)

var (
	ErrJobNotFound   = errors.New("job not found")
	ErrSproutNoJobs  = errors.New("no jobs found for sprout")
	ErrInvalidJobDir = errors.New("invalid job log directory")
)

// JobStatus represents the aggregate status of a job across all its steps.
type JobStatus int

const (
	JobPending   JobStatus = iota // Job created but no steps completed
	JobRunning                    // At least one step in progress
	JobSucceeded                  // All steps completed successfully
	JobFailed                     // At least one step failed
	JobPartial                    // Mix of completed and not-started steps
)

func (s JobStatus) String() string {
	switch s {
	case JobPending:
		return "pending"
	case JobRunning:
		return "running"
	case JobSucceeded:
		return "succeeded"
	case JobFailed:
		return "failed"
	case JobPartial:
		return "partial"
	default:
		return "unknown"
	}
}

// MarshalJSON implements json.Marshaler for JobStatus.
func (s JobStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// UnmarshalJSON implements json.Unmarshaler for JobStatus.
func (s *JobStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	switch str {
	case "pending":
		*s = JobPending
	case "running":
		*s = JobRunning
	case "succeeded":
		*s = JobSucceeded
	case "failed":
		*s = JobFailed
	case "partial":
		*s = JobPartial
	default:
		return fmt.Errorf("unknown job status: %s", str)
	}
	return nil
}

// JobSummary provides an overview of a job's execution.
type JobSummary struct {
	JID       string                `json:"jid"`
	SproutID  string                `json:"sprout_id"`
	Status    JobStatus             `json:"status"`
	Steps     []cook.StepCompletion `json:"steps"`
	StartedAt time.Time             `json:"started_at"`
	Duration  time.Duration         `json:"duration"`
	Succeeded int                   `json:"succeeded"`
	Failed    int                   `json:"failed"`
	Skipped   int                   `json:"skipped"`
	Total     int                   `json:"total"`
}

// Store provides methods for retrieving job data from the flat-file store.
type Store struct {
	mu     sync.RWMutex
	logDir string
}

// NewStore creates a new job Store using the configured job log directory.
func NewStore() *Store {
	return &Store{
		logDir: config.JobLogDir,
	}
}

// NewStoreWithDir creates a Store using a custom directory (useful for testing).
func NewStoreWithDir(dir string) *Store {
	return &Store{
		logDir: dir,
	}
}

// GetJob retrieves a job by its JID and sprout ID.
func (s *Store) GetJob(sproutID, jid string) (*JobSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobFile := filepath.Join(s.logDir, sproutID, fmt.Sprintf("%s.jsonl", jid))
	steps, err := readJobFile(jobFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrJobNotFound
		}
		return nil, fmt.Errorf("reading job file: %w", err)
	}

	return buildSummary(jid, sproutID, steps), nil
}

// FindJob searches all sprouts for a job with the given JID.
func (s *Store) FindJob(jid string) (*JobSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sprouts, err := s.listSproutDirs()
	if err != nil {
		return nil, err
	}

	for _, sproutID := range sprouts {
		jobFile := filepath.Join(s.logDir, sproutID, fmt.Sprintf("%s.jsonl", jid))
		steps, readErr := readJobFile(jobFile)
		if readErr != nil {
			continue
		}
		return buildSummary(jid, sproutID, steps), nil
	}

	return nil, ErrJobNotFound
}

// ListJobsForSprout returns all job summaries for a specific sprout,
// sorted by start time (most recent first).
func (s *Store) ListJobsForSprout(sproutID string) ([]JobSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sproutDir := filepath.Join(s.logDir, sproutID)
	entries, err := os.ReadDir(sproutDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrSproutNoJobs
		}
		return nil, fmt.Errorf("reading sprout job dir: %w", err)
	}

	var summaries []JobSummary
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		jid := strings.TrimSuffix(entry.Name(), ".jsonl")
		jobFile := filepath.Join(sproutDir, entry.Name())
		steps, readErr := readJobFile(jobFile)
		if readErr != nil {
			continue
		}
		summaries = append(summaries, *buildSummary(jid, sproutID, steps))
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].StartedAt.After(summaries[j].StartedAt)
	})

	return summaries, nil
}

// ListAllJobs returns job summaries across all sprouts,
// sorted by start time (most recent first). Limit of 0 means no limit.
func (s *Store) ListAllJobs(limit int) ([]JobSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sprouts, err := s.listSproutDirs()
	if err != nil {
		return nil, err
	}

	var allSummaries []JobSummary
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
			jid := strings.TrimSuffix(entry.Name(), ".jsonl")
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

// ListSprouts returns the IDs of all sprouts that have job records.
func (s *Store) ListSprouts() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.listSproutDirs()
}

// CountJobsForSprout returns the number of jobs recorded for a sprout.
func (s *Store) CountJobsForSprout(sproutID string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sproutDir := filepath.Join(s.logDir, sproutID)
	entries, err := os.ReadDir(sproutDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("reading sprout job dir: %w", err)
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".jsonl") {
			count++
		}
	}
	return count, nil
}

// listSproutDirs returns directory names under the job log dir (each is a sprout ID).
func (s *Store) listSproutDirs() ([]string, error) {
	if s.logDir == "" {
		return nil, ErrInvalidJobDir
	}

	entries, err := os.ReadDir(s.logDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading job log dir: %w", err)
	}

	var sprouts []string
	for _, entry := range entries {
		if entry.IsDir() {
			sprouts = append(sprouts, entry.Name())
		}
	}
	return sprouts, nil
}

// readJobFile reads and parses a JSONL job file into step completions.
func readJobFile(path string) ([]cook.StepCompletion, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var steps []cook.StepCompletion
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var step cook.StepCompletion
		if unmarshalErr := json.Unmarshal([]byte(line), &step); unmarshalErr != nil {
			return nil, fmt.Errorf("parsing step completion: %w", unmarshalErr)
		}
		steps = append(steps, step)
	}
	return steps, nil
}

// buildSummary aggregates step completions into a JobSummary.
func buildSummary(jid, sproutID string, steps []cook.StepCompletion) *JobSummary {
	summary := &JobSummary{
		JID:      jid,
		SproutID: sproutID,
		Steps:    steps,
		Total:    len(steps),
	}

	var earliest time.Time
	var latestEnd time.Time

	for _, step := range steps {
		switch step.CompletionStatus {
		case cook.StepCompleted:
			summary.Succeeded++
		case cook.StepFailed:
			summary.Failed++
		case cook.StepSkipped:
			summary.Skipped++
		}

		if !step.Started.IsZero() {
			if earliest.IsZero() || step.Started.Before(earliest) {
				earliest = step.Started
			}
			end := step.Started.Add(step.Duration)
			if end.After(latestEnd) {
				latestEnd = end
			}
		}
	}

	summary.StartedAt = earliest
	if !earliest.IsZero() && !latestEnd.IsZero() {
		summary.Duration = latestEnd.Sub(earliest)
	}

	// Determine overall status
	summary.Status = determineJobStatus(steps)

	return summary
}

// determineJobStatus computes the aggregate job status from step completions.
func determineJobStatus(steps []cook.StepCompletion) JobStatus {
	if len(steps) == 0 {
		return JobPending
	}

	hasInProgress := false
	hasFailed := false
	hasNotStarted := false
	hasCompleted := false

	for _, step := range steps {
		switch step.CompletionStatus {
		case cook.StepNotStarted:
			hasNotStarted = true
		case cook.StepInProgress:
			hasInProgress = true
		case cook.StepCompleted, cook.StepSkipped:
			hasCompleted = true
		case cook.StepFailed:
			hasFailed = true
		}
	}

	if hasInProgress {
		return JobRunning
	}
	if hasFailed {
		return JobFailed
	}
	if hasNotStarted && hasCompleted {
		return JobPartial
	}
	if hasNotStarted {
		return JobPending
	}
	return JobSucceeded
}
