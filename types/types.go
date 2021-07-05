package types

import "net/http"

type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
}
type Routes []Route
type Version struct {
	Authors   string `json:"authors"`
	BuildNo   string `json:"build_no"`
	BuildTime string `json:"build_time"`
	GitCommit string `json:"git_commit"`
	Package_  string `json:"package"`
	Tag       string `json:"tag"`
}

type KeySubmission struct {
	NKey       string `json:"nkey"`
	SeedlingID string `json:"seedling_id"`
}
