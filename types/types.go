package types

import (
	"net/http"
	"time"
)

type (
	Result struct {
		Succeeded bool
		Failed    bool
		Changed   bool
		Changes   any
	}
	Route struct {
		Name        string
		Method      string
		Pattern     string
		HandlerFunc http.HandlerFunc
	}
	Routes  []Route
	Startup struct {
		Version  Version `json:"version"`
		SproutID string  `json:"id"`
	}
	Version struct {
		Authors   string `json:"authors"`
		BuildNo   string `json:"build_no"`
		BuildTime string `json:"build_time"`
		GitCommit string `json:"git_commit"`
		Package_  string `json:"package"`
		Tag       string `json:"tag"`
	}

	KeySubmission struct {
		NKey     string `json:"nkey"`
		SproutID string `json:"id"`
	}
	FilePath struct {
		Name string `json:"name"`
	}
	KeyManager struct {
		SproutID string `json:"id"`
	}
	KeySet struct {
		Sprouts []KeyManager `json:"sprouts"`
	}
	KeysByType struct {
		Accepted   KeySet `json:"accepted,omitempty"`
		Denied     KeySet `json:"denied,omitempty"`
		Rejected   KeySet `json:"rejected,omitempty"`
		Unaccepted KeySet `json:"unaccepted,omitempty"`
	}
	Inline struct {
		Success bool  `json:"success"`
		Error   error `json:"error"`
	}
	PingPong struct {
		Ping  bool  `json:"ping"`
		Pong  bool  `json:"pong"`
		Error error `json:"error"`
	}
	CmdRun struct {
		Command string        `json:"command"`
		Args    []string      `json:"args"`
		Path    string        `json:"path"` // path is prepended to the command's normal path
		CWD     string        `json:"cwd"`
		RunAs   string        `json:"runas"`
		Env     EnvVar        `json:"env"`
		Timeout time.Duration `json:"timeout"`

		Stdout   string        `json:"stdout"`
		Stderr   string        `json:"stderr"`
		Duration time.Duration `json:"duration"`
		ErrCode  int           `json:"errcode"`

		Error error `json:"error"`
	}

	EnvVar map[string]string

	TargetedAction struct {
		Target []KeyManager `json:"target"`
		Action interface{}  `json:"action"`
	}
	TargetedResults struct {
		Results map[string]interface{} `json:"results"`
	}
)
