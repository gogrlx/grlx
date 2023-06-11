package ingredients

import (
	"errors"
	"sync"

	"github.com/gogrlx/grlx/types"
)

var (
	ingTex sync.Mutex
	ingMap map[types.Ingredient]map[string]types.RecipeCooker
)

func init() {
	ingMap = make(map[types.Ingredient]map[string]types.RecipeCooker)
}

func RegisterAllMethods(step types.RecipeCooker) {
	ingTex.Lock()
	defer ingTex.Unlock()
	name, methods := step.Methods()
	ingMap[types.Ingredient(name)] = make(map[string]types.RecipeCooker)
	for _, method := range methods {
		ingMap[types.Ingredient(name)][method] = step
	}
}

var (
	ErrUnknownIngredient = errors.New("unknown ingredient")
	ErrUnknownMethod     = errors.New("unknown method")
)

func NewRecipeCooker(id types.StepID, ingredient types.Ingredient, method string, params map[string]interface{}) (types.RecipeCooker, error) {
	ingTex.Lock()
	defer ingTex.Unlock()
	if r, ok := ingMap[ingredient]; ok {
		if ing, ok := r[method]; ok {
			return ing.Parse(string(id), method, params)
		}
		return nil, ErrUnknownMethod
	}
	return nil, ErrUnknownIngredient
}
