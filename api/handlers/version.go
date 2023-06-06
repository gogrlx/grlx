package handlers

import (
	"encoding/json"
	"net/http"

	//. "github.com/gogrlx/grlx/config"

	"github.com/gogrlx/grlx/types"
	. "github.com/gogrlx/grlx/types"
	log "github.com/taigrr/log-socket/log"
)

var buildVersion types.Version

func SetBuildVersion(v types.Version) {
	buildVersion = v
}

func GetVersion(w http.ResponseWriter, _ *http.Request) {
	jr, err := json.Marshal(buildVersion)
	if err != nil {
		log.Error(err)
	}
	w.WriteHeader(http.StatusOK)
	w.Write(jr)
	return
}
