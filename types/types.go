package types

import (
	"net/http"
	"time"
)

type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
}
type Routes []Route
type Startup struct {
	Version  Version `json:"version"`
	SproutID string  `json:"id"`
}
type Version struct {
	Authors   string `json:"authors"`
	BuildNo   string `json:"build_no"`
	BuildTime string `json:"build_time"`
	GitCommit string `json:"git_commit"`
	Package_  string `json:"package"`
	Tag       string `json:"tag"`
}

type KeySubmission struct {
	NKey     string `json:"nkey"`
	SproutID string `json:"id"`
}
type KeyManager struct {
	SproutID string `json:"id"`
}
type KeySet struct {
	Sprouts []KeyManager `json:"sprouts"`
}
type KeysByType struct {
	Accepted   KeySet `json:"accepted"`
	Denied     KeySet `json:"denied"`
	Rejected   KeySet `json:"rejected"`
	Unaccepted KeySet `json:"unaccepted"`
}
type Inline200 struct {
	Success bool `json:"success"`
}

type PingPong struct {
	Ping bool `json:"ping"`
	Pong bool `json:"pong"`
}

type CmdRun struct {
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
}

type EnvVar map[string]string

type TargetedAction struct {
	Target []KeyManager `json:"target"`
	Action interface{}  `json:"action"`
}
type TargetedResults struct {
	Results map[KeyManager]interface{} `json:"results"`
}
