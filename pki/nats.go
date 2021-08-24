package pki

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"

	. "github.com/gogrlx/grlx/config"
	log "github.com/taigrr/log-socket/log"

	//	. "github.com/gogrlx/grlx/types"
	nats_server "github.com/nats-io/nats-server/v2/server"
)

var NatsServer *nats_server.Server
var NatsOpts *nats_server.Options

var cert tls.Certificate
var certPool *x509.CertPool

func ConfigureNats() {
	DefaultTestOptions = nats_server.Options{
		Host:                  "0.0.0.0",
		Port:                  4443,
		NoSigs:                true,
		MaxControlLine:        4096,
		DisableShortFirstPing: true,
		Trace:                 true,
		Debug:                 true,
		//	TLSCert:               CertFile,
		//		TLSKey:                KeyFile,
		TLS:         true,
		AllowNonTLS: false,
		LogFile:     "nats.log",
		AuthTimeout: 10,
		//TLSCaCert:             RootCA,
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
		ServerName:   "localhost",
		RootCAs:      certPool,
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	DefaultTestOptions.TLSConfig = &config
}

func SetNATSServer(s *nats_server.Server) {
	NatsServer = s
}

//TODO
func ReloadNKeys() error {
	//AuthorizedKeys
	authorizedKeys := GetNKeysByType("accepted")
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
	for _, account := range authorizedKeys.Sprouts {
		log.Tracef("Adding accepted key `%s` to NATS", account.SproutID)
		sproutAccount := nats_server.Account{}
		sproutAccount.Name = account.SproutID
		key, err := GetNKey(account.SproutID)
		if err != nil {
			//TODO update panic to handle error
			panic(err)
		}
		account_subscribe := nats_server.SubjectPermission{Allow: []string{"grlx.sprouts." + account.SproutID + ".>"}}
		account_publish := nats_server.SubjectPermission{Allow: []string{"grlx.sprouts.announce." + account.SproutID + ".>"}}
		sproutAccount.Nkey = key
		sproutPermissions := nats_server.Permissions{}
		sproutPermissions.Publish = &account_publish
		sproutPermissions.Subscribe = &account_subscribe
		sproutUser := nats_server.NkeyUser{}
		sproutUser.Permissions = &sproutPermissions
		sproutUser.Nkey = key
		//farmerUser.Account = &farmerAccount
		nkeyUsers = append(nkeyUsers, &sproutUser)
	}
	log.Tracef("Completed adding authorized clients.")
	DefaultTestOptions.Nkeys = nkeyUsers
	config := tls.Config{
		ServerName:   "localhost",
		RootCAs:      certPool,
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	DefaultTestOptions.TLSConfig = &config

	//DefaultTestOptions.Accounts = append(DefaultTestOptions.Accounts, &farmerAccount)
	//DefaultTestOptions.Accounts
	if NatsServer != nil {
		err = NatsServer.ReloadOptions(&DefaultTestOptions)
		log.Error(err)
	}
	return err
}
