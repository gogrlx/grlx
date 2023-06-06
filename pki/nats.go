package pki

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"

	"github.com/spf13/viper"
	log "github.com/taigrr/log-socket/log"

	"github.com/gogrlx/grlx/auth"

	nats_server "github.com/nats-io/nats-server/v2/server"
)

var (
	NatsServer *nats_server.Server
	NatsOpts   *nats_server.Options
	cert       tls.Certificate
	certPool   *x509.CertPool
)

func ConfigureNats() nats_server.Options {
	var DefaultTestOptions nats_server.Options
	FarmerInterface := viper.GetString("FarmerInterface")
	RootCA := viper.GetString("RootCA")
	CertFile := viper.GetString("CertFile")
	KeyFile := viper.GetString("KeyFile")
	DefaultTestOptions = nats_server.Options{
		Host:                  FarmerInterface,
		Port:                  4443,
		NoSigs:                true,
		MaxControlLine:        4096,
		DisableShortFirstPing: true,
		Trace:                 true,
		Debug:                 true,
		TLS:                   true,
		AllowNonTLS:           false,
		LogFile:               "nats.log",
		AuthTimeout:           10,
		// TLSCert:               CertFile,
		// TLSKey:                KeyFile,
		// TLSCaCert:             RootCA,
	}
	certPool = x509.NewCertPool()
	rootPEM, err := ioutil.ReadFile(RootCA)
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
	DefaultTestOptions.TLSConfig = &config
	return DefaultTestOptions
}

func SetNATSServer(s *nats_server.Server) {
	NatsServer = s
}

// TODO
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
	accounts := []*nats_server.Account{}
	allowAll := nats_server.SubjectPermission{Allow: []string{"grlx.>", "_INBOX.>"}}

	farmerAccount := nats_server.NewAccount("Farmer")
	// farmerAccount.Nkey = farmerKey
	farmerPermissions := nats_server.Permissions{}
	farmerPermissions.Publish = &allowAll
	farmerPermissions.Subscribe = &allowAll
	farmerUser := nats_server.NkeyUser{}
	farmerUser.Permissions = &farmerPermissions
	farmerUser.Nkey = farmerKey
	farmerUser.Account = farmerAccount

	grlxAdminAccount := nats_server.NewAccount("grlxAdmin")
	grlxPermissions := nats_server.Permissions{}
	grlxPermissions.Publish = &allowAll
	grlxPermissions.Subscribe = &allowAll
	for _, key := range grlxKeys {
		grlxUser := nats_server.NkeyUser{}
		grlxUser.Permissions = &grlxPermissions
		grlxUser.Nkey = key
		grlxUser.Account = grlxAdminAccount
		nkeyUsers = append(nkeyUsers, &grlxUser)
	}

	nkeyUsers = append(nkeyUsers, &farmerUser)
	accounts = append(accounts, farmerAccount)
	accounts = append(accounts, grlxAdminAccount)
	for _, account := range authorizedKeys.Sprouts {
		log.Tracef("Adding accepted key `%s` to NATS", account.SproutID)
		sproutAccount := nats_server.NewAccount(account.SproutID)
		accounts = append(accounts, sproutAccount)
		// sproutAccount.Name = account.SproutID
		key, err := GetNKey(account.SproutID)
		if err != nil {
			// TODO update panic to handle error
			panic(err)
		}
		accountSubscribe := nats_server.SubjectPermission{Allow: []string{"grlx.sprouts." + account.SproutID + ".>"}}
		accountPublish := nats_server.SubjectPermission{Allow: []string{"grlx.sprouts.announce." + account.SproutID, "_INBOX.>"}}
		//	sproutAccount.Nkey = key
		sproutPermissions := nats_server.Permissions{}
		sproutPermissions.Publish = &accountPublish
		sproutPermissions.Subscribe = &accountSubscribe
		sproutUser := nats_server.NkeyUser{}
		sproutUser.Permissions = &sproutPermissions
		sproutUser.Nkey = key
		sproutUser.Account = sproutAccount
		// farmerUser.Account = &farmerAccount
		nkeyUsers = append(nkeyUsers, &sproutUser)
	}
	log.Tracef("Completed adding authorized clients.")
	optsCopy := ConfigureNats()
	optsCopy.Nkeys = nkeyUsers
	optsCopy.Accounts = accounts
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
		log.Error(err)
	}
	return err
}
