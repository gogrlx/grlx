package parser

type ReqType string

const (
	OnChanges ReqType = "onchanges"
	OnFail    ReqType = "onfail"
	Require   ReqType = "require"

	OnChangesAny ReqType = "onchanges_any"
	OnFailAny    ReqType = "onfail_any"
	RequireAny   ReqType = "require_any"
)

type (
	// Recipe interface{}
	Result struct {
		Suceeded bool
		Failed   bool
		Changed  bool
		Changes  any
	}
	RecipeName string
	Function   string
	StateID    string
	Ingredient map[string]interface{}
	recipe     struct {
		Includes []RecipeName        `json:"include" yaml:"include"`
		States   []map[StateID]State `json:"states" yaml:"states"`
	}
	State struct {
		Ingredient Ingredient `json:"ingredient" yaml:"ingredient"`
		ID         StateID
		Requisites []StateID
	}
	Targets   []StateID
	Requisite struct {
		Condition ReqType
	}
)
