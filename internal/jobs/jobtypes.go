package jobs

import "github.com/gogrlx/grlx/v2/internal/cook"

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
)
