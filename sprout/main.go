package main

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"os"
	"time"

	log "github.com/taigrr/log-socket/log"

	certs "github.com/gogrlx/grlx/certs"
	. "github.com/gogrlx/grlx/config"
	"github.com/gogrlx/grlx/ingredients/cmd"
	"github.com/gogrlx/grlx/ingredients/test"
	"github.com/gogrlx/grlx/pki"

	nats "github.com/nats-io/nats.go"
)

func init() {
	log.SetLogLevel(log.LTrace)
	sproutID = pki.GetSproutID()
	createConfigRoot()
	pki.SetupPKISprout()
}

var (
	Authors   string
	BuildNo   string
	BuildTime string
	GitCommit string
	Package   string
	Tag       string
	sproutID  string
)

func main() {
	defer log.Flush()
	certs.GenNKey(false)
	for err := pki.LoadRootCA(); err != nil; err = pki.LoadRootCA() {
		log.Debugf("Error with RootCA: %v", err)
		time.Sleep(time.Minute * 5)
	}
	for err := pki.PutNKey(sproutID); err != nil; err = pki.PutNKey(sproutID) {
		log.Debugf("Error submitting NKey: %v", err)
		time.Sleep(time.Minute * 5)
	}
	go ConnectSprout()
	select {}

	// Generate nkey and save or read existing
	// Post user struct to mux
	// Attempt nats auth
	// Auth nats bus
	// Cli accept key, add to config file
	// Update auth users via api

}

func createConfigRoot() {
	_, err := os.Stat(ConfigRoot)
	if err == nil {
		return
	}
	if os.IsNotExist(err) {
		err = os.MkdirAll(ConfigRoot, os.ModePerm)
		if err != nil {
			log.Panicf(err.Error())
		}
	} else {
		//TODO: work out what the other errors could be here
		log.Panicf(err.Error())
	}
}

func ConnectSprout() {
	var connectionAttempts = 0
	var err error
	opt, err := nats.NkeyOptionFromSeed(NKeySproutPrivFile)
	if err != nil {
		//TODO: handle error
		log.Panic(err)
	}
	certPool := x509.NewCertPool()
	rootPEM, err := ioutil.ReadFile(SproutRootCA)
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
	nc, err := nats.Connect("tls://"+FarmerInterface+":4443", nats.Secure(config), opt,
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second*15),
		nats.DisconnectHandler(func(_ *nats.Conn) {
			connectionAttempts++
			log.Debugf("Reconnecting to Farmer, attempt: %d\n", connectionAttempts)
		}),
	)
	for err != nil {
		time.Sleep(time.Second * 15)
		nc, err = nats.Connect("tls://"+FarmerInterface+":4443", nats.Secure(config), opt,
			nats.MaxReconnects(-1),
			nats.ReconnectWait(time.Second*15),
			//TODO: Add a reconnect handler
			nats.DisconnectHandler(func(_ *nats.Conn) {
				connectionAttempts++
				log.Debugf("Reconnecting to Farmer, attempt: %d\n", connectionAttempts)
			}),
		)
	}
	if err != nil {
		log.Errorf("Got an error on Connect with Secure Options: %+v\n", err)
	}
	log.Debugf("Successfully connected to the Farmer")

	//	nc, err := nats.Connect(serverUrl, opt)
	//	if err != nil {
	//		//TODO: handle error
	//		panic(err)
	//	}
	ec, _ := nats.NewEncodedConn(nc, nats.JSON_ENCODER)
	test.RegisterEC(ec)
	cmd.RegisterEC(ec)
	natsInit(ec)
	defer ec.Close()
	select {}
}
