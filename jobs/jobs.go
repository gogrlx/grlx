package jobs

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gogrlx/grlx/v2/types"
)

var (
	ingTex sync.Mutex
	ingMap map[types.Ingredient]map[string]types.RecipeCooker
)

func init() {
	ingMap = make(map[types.Ingredient]map[string]types.RecipeCooker)
}

type MethodProps struct {
	Key   string
	Type  string
	IsReq bool
}

type JobRecord struct {
	JID        string
	SproutID   string
	Timestamp  time.Time
	Executor   types.Executor
	Completion types.CompletionStatus
}
type RecordKeeper interface{}

type MethodPropsSet []MethodProps

func PropMapToPropSet(pmap map[string]string) (MethodPropsSet, error) {
	propset := MethodPropsSet{}
	for k, v := range pmap {
		if v == "" {
			return nil, fmt.Errorf("empty value for key %s", k)
		}
		split := strings.Split(v, ",")
		if len(split) > 2 {
			return nil, fmt.Errorf("invalid value for key %s", k)
		}
		isReq := false
		if len(split) == 2 {
			if split[1] == "req" {
				isReq = true
			} else if split[1] != "opt" {
				return nil, fmt.Errorf("invalid value for key %s", k)
			}
		}
		switch split[0] {
		case "string":
			fallthrough
		case "[]string":
			fallthrough
		case "bool":
			propset = append(propset, MethodProps{Key: k, Type: split[0], IsReq: isReq})
		default:
			return nil, fmt.Errorf("invalid Type value for key %s", k)
		}

	}
	return propset, nil
}

func (m MethodPropsSet) ToMap() map[string]string {
	ret := make(map[string]string)
	for _, v := range m {
		ret[v.Key] = v.Type
		if v.IsReq {
			ret[v.Key] = ret[v.Key] + ",req"
		} else {
			ret[v.Key] = ret[v.Key] + ",opt"
		}
	}
	return ret
}

func RegisterAllMethods(step types.RecipeCooker) {
	ingTex.Lock()
	defer ingTex.Unlock()
	name, methods := step.Methods()
	_, ok := ingMap[types.Ingredient(name)]
	if !ok {
		ingMap[types.Ingredient(name)] = make(map[string]types.RecipeCooker)
	}
	for _, method := range methods {
		ingMap[types.Ingredient(name)][method] = step
	}
}

var (
	ErrUnknownIngredient = errors.New("unknown ingredient")
	ErrUnknownMethod     = errors.New("unknown method")
)

func NewRecipeCooker(id types.StepID, ingredient types.Ingredient, method string, params map[string]interface{}) (types.RecipeCooker, error) {
	fmt.Printf("cooking %s %s %s\n", id, ingredient, method)
	ingTex.Lock()
	defer ingTex.Unlock()
	fmt.Printf("%v\n", ingMap)
	if r, ok := ingMap[ingredient]; ok {
		if ing, ok := r[method]; ok {
			return ing.Parse(string(id), method, params)
		}
		return nil, ErrUnknownMethod
	}
	return nil, ErrUnknownIngredient
}
