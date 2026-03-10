package facts

import (
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"
	log "github.com/gogrlx/grlx/v2/internal/log"

	"github.com/gogrlx/grlx/v2/internal/props"
)

// RegisterFarmerListener subscribes to sprout facts publications and stores
// them as props on the farmer side.
func RegisterFarmerListener(nc *nats.Conn) {
	_, err := nc.Subscribe("grlx.sprouts.*.facts", func(msg *nats.Msg) {
		var sf SystemFacts
		if unmarshalErr := json.Unmarshal(msg.Data, &sf); unmarshalErr != nil {
			log.Errorf("facts: failed to unmarshal: %v", unmarshalErr)
			return
		}
		if sf.SproutID == "" {
			log.Error("facts: received facts with empty sprout ID")
			return
		}
		storeFacts(sf)
		log.Noticef("facts: received system facts from %s (os=%s arch=%s)", sf.SproutID, sf.OS, sf.Arch)
	})
	if err != nil {
		log.Errorf("facts: failed to subscribe: %v", err)
	}
}

// storeFacts writes system facts into the props store.
func storeFacts(sf SystemFacts) {
	sid := sf.SproutID
	props.SetProp(sid, "os", sf.OS)
	props.SetProp(sid, "arch", sf.Arch)
	props.SetProp(sid, "hostname", sf.Hostname)
	props.SetProp(sid, "go_version", sf.GoVersion)
	props.SetProp(sid, "num_cpu", fmt.Sprintf("%d", sf.NumCPU))
	if len(sf.IPAddresses) > 0 {
		ipsJSON, _ := json.Marshal(sf.IPAddresses)
		props.SetProp(sid, "ip_addresses", string(ipsJSON))
	}
}
