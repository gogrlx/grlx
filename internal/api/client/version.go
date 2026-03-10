package client

import (
	"encoding/json"
	"fmt"

	"github.com/gogrlx/grlx/v2/internal/config"
)

func GetVersion() (config.Version, error) {
	var version config.Version
	resp, err := NatsRequest("version", nil)
	if err != nil {
		return version, err
	}
	if err := json.Unmarshal(resp, &version); err != nil {
		return version, fmt.Errorf("version: %w", err)
	}
	return version, nil
}
