package cook

import (
	"context"
	"fmt"
)

const (
	OnChanges ReqType = "onchanges"
	OnFail    ReqType = "onfail"
	Require   ReqType = "require"

	OnChangesAny ReqType = "onchanges_any"
	OnFailAny    ReqType = "onfail_any"
	RequireAny   ReqType = "require_any"
)

const (
	StepNotStarted CompletionStatus = iota
	StepInProgress
	StepCompleted
	StepFailed
)

type (
	CompletionStatus     int
	SproutStepCompletion struct {
		SproutID      string
		CompletedStep StepCompletion
	}
	StepCompletion struct {
		ID               StepID
		CompletionStatus CompletionStatus
		ChangesMade      bool
		Changes          []string
		Error            error
	}
	RecipeCooker interface {
		Apply(context.Context) (Result, error)
		Test(context.Context) (Result, error)
		Properties() (map[string]interface{}, error)
		Parse(id, method string, properties map[string]interface{}) (RecipeCooker, error)
		Methods() (string, []string)
		PropertiesForMethod(method string) (map[string]string, error)
	}
	RecipeEnvelope struct {
		JobID string
		Steps []Step
		Test  bool
	}
	Ack struct {
		Acknowledged bool
		JobID        string
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
		Notes     []fmt.Stringer
	}
	CookSummary struct {
		Succeeded int
		Failures  int
		Changed   int
		Notes     []fmt.Stringer
	}
	Summary struct {
		Succeeded  int
		InProgress bool
		Failures   int
		Changes    int
		Errors     []error
	}
	SimpleNote string
	ReqType    string
)

// CookerFactory is a function type for creating RecipeCooker instances.
// It is set by the ingredients package to break the import cycle.
var NewRecipeCooker func(id StepID, ingredient Ingredient, method string, params map[string]interface{}) (RecipeCooker, error)

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

func (r RequisiteSet) Equals(other RequisiteSet) bool {
	if len(r) != len(other) {
		return false
	}
	rmap := make(map[ReqType]Requisite)
	omap := make(map[ReqType]Requisite)
	for _, req := range r {
		rmap[req.Condition] = req
	}
	for _, req := range other {
		omap[req.Condition] = req
	}
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

func (s SimpleNote) String() string {
	return string(s)
}

func Snprintf(format string, a ...any) SimpleNote {
	return SimpleNote(fmt.Sprintf(string(format), a...))
}
