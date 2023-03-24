package main

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
	RecipeName string
	Function   string
	StateID    string
	Ingredient map[string]interface{}
	recipe     struct {
		Includes []RecipeName        `json:"include" yaml:"include"`
		States   []map[StateID]State `json:"states" yaml:"states"`
	}
	Recipe map[string]interface{}
	State  struct {
		Ingredient Ingredient `json:"ingredient" yaml:"ingredient"`
		ID         StateID
		Requisites []StateID
	}
	Targets   []StateID
	Requisite struct {
		Condition ReqType
	}
)

func main() {
}
