package audit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// QueryParams controls which audit entries to return.
type QueryParams struct {
	// Date filters entries to a specific date (YYYY-MM-DD).
	// Empty means today.
	Date string `json:"date,omitempty"`

	// Action filters entries by action name (exact match).
	Action string `json:"action,omitempty"`

	// Pubkey filters entries by user pubkey (exact match).
	Pubkey string `json:"pubkey,omitempty"`

	// Limit caps the number of entries returned. 0 means 100.
	Limit int `json:"limit,omitempty"`

	// FailedOnly returns only entries where Success is false.
	FailedOnly bool `json:"failed_only,omitempty"`
}

// QueryResult is the response from a query.
type QueryResult struct {
	Date    string  `json:"date"`
	Entries []Entry `json:"entries"`
	Total   int     `json:"total"`
}

// DateSummary describes a single audit log file.
type DateSummary struct {
	Date       string `json:"date"`
	EntryCount int    `json:"entry_count"`
	SizeBytes  int64  `json:"size_bytes"`
}

// Query reads audit log entries matching the given parameters.
func (l *Logger) Query(params QueryParams) (QueryResult, error) {
	date := params.Date
	if date == "" {
		date = time.Now().UTC().Format("2006-01-02")
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 100
	}

	path := filepath.Join(l.dir, fmt.Sprintf("audit-%s.jsonl", date))
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return QueryResult{Date: date, Entries: []Entry{}, Total: 0}, nil
		}
		return QueryResult{}, fmt.Errorf("audit: open %s: %w", path, err)
	}
	defer f.Close()

	var all []Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry Entry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue // skip malformed lines
		}

		if params.Action != "" && entry.Action != params.Action {
			continue
		}
		if params.Pubkey != "" && entry.Pubkey != params.Pubkey {
			continue
		}
		if params.FailedOnly && entry.Success {
			continue
		}

		all = append(all, entry)
	}
	if err := scanner.Err(); err != nil {
		return QueryResult{}, fmt.Errorf("audit: scan %s: %w", path, err)
	}

	total := len(all)

	// Return most recent entries first.
	sort.Slice(all, func(i, j int) bool {
		return all[i].Timestamp.After(all[j].Timestamp)
	})

	if len(all) > limit {
		all = all[:limit]
	}

	return QueryResult{
		Date:    date,
		Entries: all,
		Total:   total,
	}, nil
}

// ListDates returns a summary of all available audit log files.
func (l *Logger) ListDates() ([]DateSummary, error) {
	pattern := filepath.Join(l.dir, "audit-*.jsonl")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("audit: glob: %w", err)
	}

	var summaries []DateSummary
	for _, path := range matches {
		base := filepath.Base(path)
		// Extract date from "audit-YYYY-MM-DD.jsonl"
		date := strings.TrimPrefix(base, "audit-")
		date = strings.TrimSuffix(date, ".jsonl")

		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		count := countLines(path)

		summaries = append(summaries, DateSummary{
			Date:       date,
			EntryCount: count,
			SizeBytes:  info.Size(),
		})
	}

	// Most recent first.
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Date > summaries[j].Date
	})

	if summaries == nil {
		summaries = []DateSummary{}
	}

	return summaries, nil
}

// countLines counts non-empty lines in a file.
func countLines(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) != "" {
			count++
		}
	}
	return count
}
