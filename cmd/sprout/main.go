package main

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"time"

	log "github.com/taigrr/log-socket/log"

	certs "github.com/gogrlx/grlx/v2/internal/certs"
	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/ingredients"
	"github.com/gogrlx/grlx/v2/internal/ingredients/cmd"
	"github.com/gogrlx/grlx/v2/internal/ingredients/test"
	"github.com/gogrlx/grlx/v2/internal/pki"

	nats "github.com/nats-io/nats.go"

	"github.com/taigrr/jety"
)

func init() {
	config.LoadConfig("sprout")
	log.SetLogLevel(config.LogLevel)
	sproutID = pki.GetSproutID()
	createConfigRoot()
	pki.SetupPKISprout()
	cook.NewRecipeCooker = ingredients.NewRecipeCooker
}

var (
	BuildTime string
	GitCommit string
	Tag       string
	sproutID  string
)

func main() {
	if err := os.MkdirAll(config.CacheDir, os.ModeDir); err != nil {
		log.Fatalf("failed to create cache directory %s: %v", config.CacheDir, err)
	}
	config.LoadConfig("sprout")
	defer log.Flush()
	certs.GenNKey(false)
	rootCARetryDelay := jety.GetDuration("rootca_retry_delay")
	for err := pki.LoadRootCA("sprout"); err != nil; err = pki.LoadRootCA("sprout") {
		log.Debugf("Error with RootCA: %v", err)
		time.Sleep(rootCARetryDelay)
	}
	nkeyRetryDelay := jety.GetDuration("nkey_retry_delay")
	for err := pki.PutNKey(sproutID); err != nil; err = pki.PutNKey(sproutID) {
		log.Debugf("Error submitting NKey: %v", err)
		time.Sleep(nkeyRetryDelay)
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
			log.Panicf("failed to create config directory: %v", err)
		}
	} else {
		log.Panicf("unexpected error checking config directory: %v", err)
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
		log.Panicf("failed to load NKey seed: %v", err)
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
			nats.DisconnectHandler(func(_ *nats.Conn) {
				connectionAttempts++
				log.Debugf("Reconnecting to Farmer, attempt: %d\n", connectionAttempts)
			}),
		)
	}
	log.Debugf("Successfully connected to the Farmer")

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
