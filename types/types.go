package types

import "net/http"

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
