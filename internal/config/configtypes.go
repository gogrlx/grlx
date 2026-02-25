package config

type (
	Version struct {
		Arch      string `json:"arch"`
		Compiler  string `json:"compiler"`
		GitCommit string `json:"git_commit"`
		Tag       string `json:"tag"`
	}
	CombinedVersion struct {
		CLI    Version `json:"cli"`
		Farmer Version `json:"farmer"`
		Error  string  `json:"error"`
	}
	Startup struct {
		Version  Version `json:"version"`
		SproutID string  `json:"id"`
	}
	TriggerMsg struct {
		JID string `json:"jid"`
	}
)
