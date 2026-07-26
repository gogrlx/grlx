package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	log "github.com/gogrlx/grlx/v2/internal/log"

	certs "github.com/gogrlx/grlx/v2/internal/certs"
	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/ingredients"
	"github.com/gogrlx/grlx/v2/internal/ingredients/cmd"
	"github.com/gogrlx/grlx/v2/internal/ingredients/test"
	"github.com/gogrlx/grlx/v2/internal/jobs"
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
	if err := os.MkdirAll(config.CacheDir, 0o755); err != nil {
		log.Fatalf("failed to create cache directory %s: %v", config.CacheDir, err)
	}
	config.LoadConfig("sprout")
	defer log.Flush()
	if err := certs.GenNKey(false); err != nil {
		log.Fatalf("failed to generate sprout NKey: %v", err)
	}
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
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	done := make(chan struct{})
	go ConnectSprout(ctx, done)
	<-ctx.Done()
	stop()
	log.Info("Shutdown signal received, stopping sprout...")
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		log.Warn("timed out waiting for NATS client to close")
	}
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

func ConnectSprout(ctx context.Context, done chan<- struct{}) {
	defer close(done)
	var connectionAttempts atomic.Int64
	var err error
	SproutRootCA := config.SproutRootCA
	FarmerInterface := config.FarmerInterface
	FarmerBusURL := config.FarmerBusURL
	// Capture job-log settings before the local tls.Config below shadows the
	// config package identifier.
	jobLogDir := config.JobLogDir
	jobLogTTL := config.JobLogTTL
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
			log.Debugf("Reconnecting to Farmer, attempt: %d\n", connectionAttempts.Add(1))
		}),
	)
	for err != nil {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second * 15):
		}
		nc, err = nats.Connect(FarmerBusURL, nats.Secure(config), opt,
			nats.MaxReconnects(-1),
			nats.ReconnectWait(time.Second*15),
			nats.DisconnectHandler(func(_ *nats.Conn) {
				log.Debugf("Reconnecting to Farmer, attempt: %d\n", connectionAttempts.Add(1))
			}),
		)
	}
	log.Debugf("Successfully connected to the Farmer")

	if err := log.ConnectNATS(FarmerBusURL); err != nil {
		log.Errorf("Failed to connect log-nats backend: %v", err)
	}

	test.RegisterNatsConn(nc)
	cmd.RegisterNatsConn(nc)
	cook.RegisterNatsConn(nc)
	err = natsInit(nc)
	if err != nil {
		log.Panicf("Error with natsInit: %v", err)
	}
	// Expire old local job logs written by cook runs on this sprout.
	jobs.StartSproutReaper(ctx, jobLogDir, jobLogTTL)
	<-ctx.Done()
	nc.Close()
}
