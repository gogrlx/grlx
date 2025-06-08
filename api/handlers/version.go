package handlers

import (
	"encoding/json"
	"net/http"

	log "github.com/taigrr/log-socket/log"

	"github.com/gogrlx/grlx/v2/types"
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
}
