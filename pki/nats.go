package pki

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"strconv"

	log "github.com/taigrr/log-socket/log"

	"github.com/gogrlx/grlx/v2/auth"
	"github.com/gogrlx/grlx/v2/config"

	nats_server "github.com/nats-io/nats-server/v2/server"
)

var (
	NatsServer *nats_server.Server
	NatsOpts   *nats_server.Options
	cert       tls.Certificate
	certPool   *x509.CertPool
)

func ConfigureNats() nats_server.Options {
	var NatsConfig nats_server.Options
	FarmerInterface := config.FarmerInterface
	FBusPort := config.FarmerBusPort
	FarmerBusPort, err := strconv.Atoi(FBusPort)
	if err != nil {
		log.Panic(err)
	}
	RootCA := config.RootCA
	CertFile := config.CertFile
	KeyFile := config.KeyFile
	NatsConfig = nats_server.Options{
		Host:                  FarmerInterface,
		Port:                  FarmerBusPort,
		NoSigs:                true,
		MaxControlLine:        4096,
		DisableShortFirstPing: true,
		Trace:                 true,
		Debug:                 true,
		TLS:                   true,
		AllowNonTLS:           false,
		LogFile:               "nats.log",
		AuthTimeout:           10,
	}
	certPool = x509.NewCertPool()
	rootPEM, err := os.ReadFile(RootCA)
	if err != nil || rootPEM == nil {
		log.Panicf("nats: error loading or parsing rootCA file: %v", err)
	}
	ok := certPool.AppendCertsFromPEM(rootPEM)
	if !ok {
		log.Errorf("nats: failed to parse root certificate from %v", RootCA)
	}
	cert, err = tls.LoadX509KeyPair(CertFile, KeyFile)
	if err != nil {
		log.Panic(err)
	}
	config := tls.Config{
		ServerName:   FarmerInterface,
		RootCAs:      certPool,
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	NatsConfig.TLSConfig = &config
	return NatsConfig
}

func SetNATSServer(s *nats_server.Server) {
	NatsServer = s
}

func ReloadNKeys() error {
	// AuthorizedKeys
	authorizedKeys := GetNKeysByType("accepted")
	farmerKey, err := GetPubNKey(FarmerPubNKey)
	if err != nil {
		log.Fatalf("Could not load the Farmer's NKey, aborting")
	}
	log.Tracef("Loaded farmer's public key: %s", farmerKey)
	grlxKeys, err := auth.GetPubkeysByRole("admin")
	if err != nil {
		log.Errorf("Could not load the grlx cli's NKey(s), please edit the config")
	} else {
		log.Tracef("Loaded grlx cli's public key(s): %v", grlxKeys)
	}
	nkeyUsers := []*nats_server.NkeyUser{}
	allowAll := nats_server.SubjectPermission{Allow: []string{"grlx.>", "_INBOX.>"}}

	farmerPermissions := nats_server.Permissions{}
	farmerPermissions.Publish = &allowAll
	farmerPermissions.Subscribe = &allowAll
	farmerUser := nats_server.NkeyUser{}
	farmerUser.Permissions = &farmerPermissions
	farmerUser.Nkey = farmerKey

	grlxPermissions := nats_server.Permissions{}
	grlxPermissions.Publish = &allowAll
	grlxPermissions.Subscribe = &allowAll
	for _, key := range grlxKeys {
		grlxUser := nats_server.NkeyUser{}
		grlxUser.Permissions = &grlxPermissions
		grlxUser.Nkey = key
		nkeyUsers = append(nkeyUsers, &grlxUser)
	}

	nkeyUsers = append(nkeyUsers, &farmerUser)
	for _, account := range authorizedKeys.Sprouts {
		log.Tracef("Adding accepted key `%s` to NATS", account.SproutID)
		// sproutAccount.Name = account.SproutID
		key, errGet := GetNKey(account.SproutID)
		if errGet != nil {
			// TODO handle error
			panic(errGet)
		}
		accountSubscribe := nats_server.SubjectPermission{Allow: []string{"grlx.sprouts." + account.SproutID + ".>"}}
		accountPublish := nats_server.SubjectPermission{Allow: []string{"grlx.sprouts.announce." + account.SproutID, "_INBOX.>", "grlx.cook." + account.SproutID + ".>"}}
		sproutPermissions := nats_server.Permissions{}
		sproutPermissions.Publish = &accountPublish
		sproutPermissions.Subscribe = &accountSubscribe
		sproutUser := nats_server.NkeyUser{}
		sproutUser.Permissions = &sproutPermissions
		sproutUser.Nkey = key
		nkeyUsers = append(nkeyUsers, &sproutUser)
	}
	log.Tracef("Completed adding authorized clients.")
	optsCopy := ConfigureNats()
	optsCopy.Nkeys = nkeyUsers
	config := tls.Config{
		ServerName:   "localhost",
		RootCAs:      certPool,
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	optsCopy.TLSConfig = &config

	// DefaultTestOptions.Accounts = append(DefaultTestOptions.Accounts, &farmerAccount)
	// DefaultTestOptions.Accounts
	if NatsServer != nil {
		err = NatsServer.ReloadOptions(&optsCopy)
		if err != nil {
			log.Error(err)
		}
	}
	return err
}
