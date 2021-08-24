package pki

import (
	. "github.com/gogrlx/grlx/config"
	log "github.com/taigrr/log-socket/logger"

	//	. "github.com/gogrlx/grlx/types"
	nats_server "github.com/nats-io/nats-server/v2/server"
)

var NatsServer *nats_server.Server

func ConfigureNats() {
	DefaultTestOptions = nats_server.Options{
		Host:                  "0.0.0.0",
		Port:                  4443,
		NoSigs:                true,
		MaxControlLine:        4096,
		DisableShortFirstPing: true,
		Trace:                 true,
		Debug:                 true,
		TLSCert:               CertFile,
		TLSKey:                KeyFile,
		TLS:                   true,
		TLSCaCert:             RootCA,
	}
}

func SetNATSServer(s *nats_server.Server) {
	NatsServer = s
}

//TODO
func ReloadNKeys() error {
	//AuthorizedKeys
	//authorizedKeys := GetNKeysByType("authorized")
	farmerKey, err := GetPubNKey(true)
	log.Tracef("Loaded farmer's public key: %s", farmerKey)
	if err != nil {
		log.Fatalf("Could not load the Farmer's NKey, aborting!")
	}
	farmerAccount := nats_server.Account{}
	farmerAccount.Name = "Farmer"
	farmerAccount.Nkey = farmerKey
	nkeyUsers := []*nats_server.NkeyUser{}
	allowAll := nats_server.SubjectPermission{Allow: []string{"grlx.>"}}
	farmerPermissions := nats_server.Permissions{}
	farmerPermissions.Publish = &allowAll
	farmerPermissions.Subscribe = &allowAll
	farmerUser := nats_server.NkeyUser{}
	farmerUser.Permissions = &farmerPermissions
	farmerUser.Nkey = farmerKey
	//farmerUser.Account = &farmerAccount
	nkeyUsers = append(nkeyUsers, &farmerUser)
	DefaultTestOptions.Nkeys = nkeyUsers
	//DefaultTestOptions.Accounts = append(DefaultTestOptions.Accounts, &farmerAccount)
	//DefaultTestOptions.Accounts
	// err = NatsServer.ReloadOptions(&DefaultTestOptions)
	return err
}
