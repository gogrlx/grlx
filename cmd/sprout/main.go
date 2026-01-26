package main

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"time"

	log "github.com/taigrr/log-socket/log"

	certs "github.com/gogrlx/grlx/v2/certs"
	"github.com/gogrlx/grlx/v2/config"
	"github.com/gogrlx/grlx/v2/cook"
	"github.com/gogrlx/grlx/v2/ingredients/cmd"
	"github.com/gogrlx/grlx/v2/ingredients/test"
	"github.com/gogrlx/grlx/v2/pki"

	nats "github.com/nats-io/nats.go"
)

func init() {
	config.LoadConfig("sprout")
	log.SetLogLevel(config.LogLevel)
	sproutID = pki.GetSproutID()
	createConfigRoot()
	pki.SetupPKISprout()
}

var (
	BuildTime string
	GitCommit string
	Tag       string
	sproutID  string
)

func main() {
	os.MkdirAll(config.CacheDir, os.ModeDir)
	config.LoadConfig("sprout")
	defer log.Flush()
	certs.GenNKey(false)
	for err := pki.LoadRootCA("sprout"); err != nil; err = pki.LoadRootCA("sprout") {
		log.Debugf("Error with RootCA: %v", err)
		// TODO make this delay configurable
		time.Sleep(time.Second * 5)
	}
	for err := pki.PutNKey(sproutID); err != nil; err = pki.PutNKey(sproutID) {
		log.Debugf("Error submitting NKey: %v", err)

		// TODO make this delay configurable
		time.Sleep(time.Second * 5)
	}
	go ConnectSprout()
	select {}
}

func createConfigRoot() {
	ConfigRoot := config.ConfigRoot
	_, err := os.Stat(ConfigRoot)
	if err == nil {
		return
	}
	if os.IsNotExist(err) {
		err = os.MkdirAll(ConfigRoot, os.ModePerm)
		if err != nil {
			log.Panicf("%s", err.Error())
		}
	} else {
		// TODO: work out what the other errors could be here
		log.Panicf("%s", err.Error())
	}
}

func ConnectSprout() {
	connectionAttempts := 0
	var err error
	SproutRootCA := config.SproutRootCA
	FarmerInterface := config.FarmerInterface
	FarmerBusURL := config.FarmerBusURL
	opt, err := nats.NkeyOptionFromSeed(config.NKeySproutPrivFile)
	if err != nil {
		// TODO: handle error
		log.Panic(err)
	}
	certPool := x509.NewCertPool()
	rootPEM, err := os.ReadFile(SproutRootCA)
	if err != nil || rootPEM == nil {
		log.Panicf("nats: error loading or parsing rootCA file: %v", err)
	}
	ok := certPool.AppendCertsFromPEM(rootPEM)
	if !ok {
		log.Errorf("nats: failed to parse root certificate from %q", SproutRootCA)
	}
	config := &tls.Config{
		ServerName: FarmerInterface,
		RootCAs:    certPool,
		MinVersion: tls.VersionTLS12,
	}
	nc, err := nats.Connect(FarmerBusURL, nats.Secure(config), opt,
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second*15),
		nats.DisconnectHandler(func(_ *nats.Conn) {
			connectionAttempts++
			log.Debugf("Reconnecting to Farmer, attempt: %d\n", connectionAttempts)
		}),
	)
	for err != nil {
		time.Sleep(time.Second * 15)
		nc, err = nats.Connect(FarmerBusURL, nats.Secure(config), opt,
			nats.MaxReconnects(-1),
			nats.ReconnectWait(time.Second*15),
			// TODO: Add a reconnect handler
			nats.DisconnectHandler(func(_ *nats.Conn) {
				connectionAttempts++
				log.Debugf("Reconnecting to Farmer, attempt: %d\n", connectionAttempts)
			}),
		)
	}
	log.Debugf("Successfully connected to the Farmer")

	//	nc, err := nats.Connect(serverUrl, opt)
	//	if err != nil {
	//		//TODO: handle error
	//		panic(err)
	//	}
	test.RegisterNatsConn(nc)
	cmd.RegisterNatsConn(nc)
	cook.RegisterNatsConn(nc)
	err = natsInit(nc)
	if err != nil {
		log.Panicf("Error with natsInit: %v", err)
	}
	defer nc.Close()
	select {}
}
