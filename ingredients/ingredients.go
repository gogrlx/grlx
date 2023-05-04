package ingredients

import (
	"os/sync"
	 "github.com/gogrlx/grlx/types"
)

var ( 
	ingTex sync.Mutex
	ingMap map[string] types.RecipeStep

)

func init(){
	ingMap = make(map[string]types.RecipeStep)
}

func RegisterAllMethods(step types.RecipeStep){
	ingMap.Lock()
	defer ingMap.Unlock()
	for _, method := range step.Methods(){
		ingMap[method]=step
	}
}
