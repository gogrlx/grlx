package main

type RecipeName string

type Function string

type StateID string
type Ingredient map[string]interface{}

type recipe struct {
	Includes []RecipeName        `json:"include" yaml:"include"`
	States   []map[StateID]State `json:"states" yaml:"states"`
}
type Recipe map[string]interface{}

type State struct {
	Ingredient Ingredient `json:"ingredient" yaml:"ingredient"`
	ID         StateID
	Requisites []StateID
}

func main() {

}
