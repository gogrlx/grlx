package natsapi

import (
	"encoding/json"

	"github.com/gogrlx/grlx/v2/internal/config"
)

var buildVersion config.Version

// SetBuildVersion sets the version info returned by the version handler.
func SetBuildVersion(v config.Version) {
	buildVersion = v
}

func handleVersion(_ json.RawMessage) (any, error) {
	return buildVersion, nil
}
