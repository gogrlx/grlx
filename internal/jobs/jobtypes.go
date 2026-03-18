package jobs

import (
	"time"

	"github.com/gogrlx/grlx/v2/internal/cook"
)

type (
	Job struct {
		JID     string        `json:"jid"`
		ID      string        `json:"id"`
		Results []cook.Result `json:"results"`
		Sprout  string        `json:"sprout"`
		Summary cook.Summary  `json:"summary"`
	}
	Executor struct {
		PubKey string
	}
	// JobMeta stores metadata about a job, including who invoked it.
	// Written as <JID>.meta.json alongside the <JID>.jsonl step log.
	JobMeta struct {
		JID       string    `json:"jid"`
		InvokedBy string    `json:"invoked_by,omitempty"`
		CreatedAt time.Time `json:"created_at"`
	}
)
