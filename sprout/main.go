package main

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"os"
	"time"

	log "github.com/taigrr/log-socket/logger"

	certs "github.com/gogrlx/grlx/certs"
	. "github.com/gogrlx/grlx/config"
	"github.com/gogrlx/grlx/pki"

	nats "github.com/nats-io/nats.go"
)

func init() {
	log.SetLogLevel(log.LTrace)
	createConfigRoot()
	pki.SetupPKISprout()
}

func main() {
	defer log.Flush()
	certs.GenNKey(false)
	for err := pki.FetchRootCA(); err != nil; err = pki.FetchRootCA() {
		log.Debugf("Error with RootCA: %v", err)
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
		log.Errorf("nats: failed to parse root certificate from %q", CertFile)
	}
	config := &tls.Config{
		ServerName: "localhost",
		RootCAs:    certPool,
		MinVersion: tls.VersionTLS12,
	}

	nc, err := nats.Connect("nats://localhost:4443", nats.Secure(config), opt,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(1),
		nats.ReconnectWait(time.Second),

		nats.ReconnectHandler(func(_ *nats.Conn) {
			connectionAttempts++
			log.Warnf("WARN: Reconnecting Farmer to NATS bus, attempt: %d\n", connectionAttempts)
		}),
	)
	if err != nil {
		log.Errorf("Got an error on Connect with Secure Options: %+v\n", err)
	}
	log.Debugf("Successfully joined Farmer to NATS bus")

	//	nc, err := nats.Connect(serverUrl, opt)
	//	if err != nil {
	//		//TODO: handle error
	//		panic(err)
	//	}
	ec, _ := nats.NewEncodedConn(nc, nats.JSON_ENCODER)
	defer ec.Close()
	select {}
}
