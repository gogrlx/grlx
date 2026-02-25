package handlers

import (
	"encoding/json"
	"net/http"

	log "github.com/taigrr/log-socket/log"

	"github.com/gogrlx/grlx/v2/internal/config"
)

var buildVersion config.Version

func SetBuildVersion(v config.Version) {
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
