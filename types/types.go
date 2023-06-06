package types

import (
	"context"
	"time"
)

const (
	OnChanges ReqType = "onchanges"
	OnFail    ReqType = "onfail"
	Require   ReqType = "require"

	OnChangesAny ReqType = "onchanges_any"
	OnFailAny    ReqType = "onfail_any"
	RequireAny   ReqType = "require_any"
)

func (r RequisiteSet) AllIDs() []StepID {
	collection := []StepID{}
	for _, reqs := range r {
		collection = append(collection, reqs.StepIDs...)
	}
	return collection
}

func (r RequisiteSet) AllSteps() []*Step {
	collection := []*Step{}
	for _, reqs := range r {
		collection = append(collection, reqs.Steps...)
	}
	return collection
}

type (
	RecipeCooker interface {
		Apply(context.Context) (Result, error)
		Test(context.Context) (Result, error)
		Properties() (map[string]interface{}, error)
		Parse(id, method string, properties map[string]interface{}) (RecipeCooker, error)
		Methods() []string
		PropertiesForMethod(method string) (map[string]string, error)
	}
	RecipeName string
	Function   string
	StepID     string
	Ingredient string
	recipe     struct {
		Includes []RecipeName      `json:"include" yaml:"include"`
		States   []map[StepID]Step `json:"states" yaml:"states"`
	}
	RequisiteSet []Requisite
	Step         struct {
		Ingredient  Ingredient `json:"ingredient" yaml:"ingredient"`
		Method      string     `json:"method" yaml:"method"`
		ID          StepID
		Requisites  RequisiteSet
		Properties  map[string]interface{}
		IsRequisite bool
	}
	Targets   []StepID
	Requisite struct {
		Condition ReqType
		StepIDs   []StepID
		Steps     []*Step
	}
	Result struct {
		Succeeded bool
		Failed    bool
		Changed   bool
		Changes   any
	}
	Startup struct {
		Version  Version `json:"version"`
		SproutID string  `json:"id"`
	}
	CombinedVersion struct {
		CLI    Version `json:"cli"`
		Farmer Version `json:"farmer"`
		Error  string  `json:"error"`
	}
	Version struct {
		Arch      string `json:"arch"`
		Compiler  string `json:"compiler"`
		GitCommit string `json:"git_commit"`
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
	CmdCook struct {
		Async   bool          `json:"async"`
		Env     string        `json:"env"`
		Recipe  RecipeName    `json:"recipe"`
		Test    bool          `json:"test"`
		Timeout time.Duration `json:"timeout"`

		Errors map[string]error `json:"errors"`
		JID    string           `json:"jid"`
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
		Results map[string]interface{} `json:"results,omitempty"`
	}
	ReqType string
)

func (r RequisiteSet) Equals(other RequisiteSet) bool {
	if len(r) != len(other) {
		return false
	}
	rmap := make(map[ReqType]Requisite)
	omap := make(map[ReqType]Requisite)
	// load the slices into maps
	// to deduplicate the keys
	for _, req := range r {
		rmap[req.Condition] = req
	}
	for _, req := range other {
		omap[req.Condition] = req
	}
	// check that the keys are the same
	// across both maps
	for k, req := range rmap {
		oreq, ok := omap[k]
		if !ok {
			return false
		}
		if !req.Equals(oreq) {
			return false
		}
	}
	return true
}

func (r Requisite) Equals(other Requisite) bool {
	if r.Condition != other.Condition {
		return false
	}
	if len(r.StepIDs) != len(other.StepIDs) {
		return false
	}
	rmap := make(map[StepID]StepID)
	omap := make(map[StepID]StepID)
	for _, step := range r.StepIDs {
		rmap[step] = step
	}
	for _, step := range other.StepIDs {
		omap[step] = step
	}
	for k, step := range rmap {
		if omap[k] != step {
			return false
		}
	}
	return true
}
