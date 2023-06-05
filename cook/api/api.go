package api

import (
	"net/http"

	"github.com/gogrlx/grlx/types"
)

func CookHandler(w http.ResponseWriter, r *http.Request) {
	// ...
}

func CookClient(target string, cmdCook types.CmdCook) (types.CmdCook, error) {
	// ...
	return cmdCook, nil
}
