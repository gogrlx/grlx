package ingredients

import (
	"sync"

	"github.com/gogrlx/grlx/types"
)

var (
	ingTex sync.Mutex
	ingMap map[string]types.RecipeCooker
)

func init() {
	ingMap = make(map[string]types.RecipeCooker)
}

func RegisterAllMethods(step types.RecipeCooker) {
	ingTex.Lock()
	defer ingTex.Unlock()
	for _, method := range step.Methods() {
		ingMap[method] = step
	}
}
