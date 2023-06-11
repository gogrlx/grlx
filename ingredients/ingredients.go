package ingredients

import (
	"errors"
	"sync"

	"github.com/gogrlx/grlx/types"
)

var (
	ingTex sync.Mutex
	ingMap map[string]map[string]types.RecipeCooker
)

func init() {
	ingMap = make(map[string]map[string]types.RecipeCooker)
}

func RegisterAllMethods(step types.RecipeCooker) {
	ingTex.Lock()
	defer ingTex.Unlock()
	name, methods := step.Methods()
	ingMap[name] = make(map[string]types.RecipeCooker)
	for _, method := range methods {
		ingMap[name][method] = step
	}
}

var (
	ErrUnknownIngredient = errors.New("unknown ingredient")
	ErrUnknownMethod     = errors.New("unknown method")
)

func NewRecipeCooker(id, ingredient, method string, params map[string]interface{}) (types.RecipeCooker, error) {
	ingTex.Lock()
	defer ingTex.Unlock()
	if r, ok := ingMap[ingredient]; ok {
		if ing, ok := r[method]; ok {
			return ing.Parse(id, method, params)
		}
		return nil, ErrUnknownMethod
	}
	return nil, ErrUnknownIngredient
}
