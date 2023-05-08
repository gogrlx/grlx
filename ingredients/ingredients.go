package ingredients

import (
	"os/sync"

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
	ingMap.Lock()
	defer ingMap.Unlock()
	for _, method := range step.Methods() {
		ingMap[method] = step
	}
}
