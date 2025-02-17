package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"runtime"
	"time"

	log "github.com/taigrr/log-socket/log"

	"github.com/gogrlx/grlx/api/handlers"
	"github.com/gogrlx/grlx/certs"
	"github.com/gogrlx/grlx/config"
	"github.com/gogrlx/grlx/cook"
	"github.com/gogrlx/grlx/ingredients/cmd"
	"github.com/gogrlx/grlx/ingredients/test"
	"github.com/gogrlx/grlx/jobs"
	"github.com/gogrlx/grlx/pki"
	"github.com/gogrlx/grlx/server"
	"github.com/gogrlx/grlx/types"

	nats_server "github.com/nats-io/nats-server/v2/server"
	nats "github.com/nats-io/nats.go"
)

func init() {
	config.LoadConfig("farmer")
	log.SetLogLevel(config.LogLevel)
}

var (
	s         *nats_server.Server
	GitCommit string
	Tag       string
)

func main() {
	config.LoadConfig("farmer")
	fmt.Printf("Starting Farmer with URL %s\n", config.FarmerBusURL)
	defer log.Flush()
	log := log.CreateClient()
	log.LogLevel = (config.LogLevel)
	createConfigRoot()
	pki.SetupPKIFarmer()
	certs.GenCert()
	certs.GenNKey(true)
	RunNATSServer()
	server.SetVersion(types.Version{
		Arch:      runtime.GOOS,
		Compiler:  runtime.Version(),
		GitCommit: GitCommit,
		Tag:       Tag,
	})
	certs.SetHttpServer(server.StartAPIServer())
	go ConnectFarmer()
	select {}

	// Generate nkey and save or read existing
	// Post user struct to mux
	// Attempt nats auth
	// Auth nats bus
	// Cli accept key, add to config file
	// Update auth users via api
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
			log.Panicf(err.Error())
		}
	} else {
		// TODO work out what the other errors could be here
		log.Panicf(err.Error())
	}
}

type logger struct{}

func (l logger) Debugf(format string, args ...interface{}) {
	log.Debugf(format, args...)
}

// RunNATSServer starts a new Go routine based server
func RunNATSServer() {
	// Optionally override for individual debugging of tests
	// err := opts.ProcessConfigFile("config.json")
	// if err != nil {
	//		log.Panicf("Error configuring server: %v", err)
	//	}
	var err error
	pki.ReloadNatsServer()
	opts := pki.ConfigureNats()
	s, err = nats_server.NewServer(&opts)
	if err != nil || s == nil {
		log.Panicf("No NATS Server object returned: %v", err)
	}
	if err != nil || s == nil {
		log.Panicf("No NATS Server object returned: %v", err)
	}
	// Run server in Go routine.
	go s.Start()
	var logger log.Logger
	logger.SetInfoDepth(6)
	s.SetLogger(logger, true, true)
	// Wait for accept loop(s) to be started
	if !s.ReadyForConnections(10 * time.Second) {
		log.Panicf("Unable to start NATS Server")
	}
	// s.ReloadOptions(opts)
	pki.SetNATSServer(s)
	pki.ReloadNatsServer()
}

func ConnectFarmer() {
	connectionAttempts := 1
	maxFarmerReconnect := 30
	RootCA := config.RootCA
	BusURL := config.FarmerBusURL
	FarmerInterface := config.FarmerInterface
	if FarmerInterface == "0.0.0.0" {
		FarmerInterface = "localhost"
	}
	var err error
	opt, err := nats.NkeyOptionFromSeed(config.NKeyFarmerPrivFile)
	_ = opt
	if err != nil {
		// TODO handle error
		log.Panic(err)
	}
	certPool := x509.NewCertPool()
	rootPEM, err := os.ReadFile(RootCA)
	if err != nil || rootPEM == nil {
		log.Panicf("nats: error loading or parsing rootCA file: %v", err)
	}
	ok := certPool.AppendCertsFromPEM(rootPEM)
	if !ok {
		log.Errorf("nats: failed to parse root certificate from %v", RootCA)
	}

	config := &tls.Config{
		ServerName: FarmerInterface,
		RootCAs:    certPool,
		MinVersion: tls.VersionTLS12,
	}
	log.Debug("Attempting to pair Farmer to NATS bus.")
	nc, err := nats.Connect(BusURL,
		nats.Secure(config),
		opt,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(maxFarmerReconnect),
		nats.ReconnectWait(time.Second*15),
		nats.DisconnectHandler(func(_ *nats.Conn) {
			connectionAttempts++
			log.Warnf("WARN: Reconnecting Farmer to NATS bus, attempt: %d\n", connectionAttempts)
		}),
	)
	if err != nil {
		log.Errorf("Got an error on Connect with Secure Options: %+v\n", err)
	}
	for !nc.IsConnected() {
		connectionAttempts++
		log.Debugf("Attempting to pair Farmer to NATS bus (attempt %d/%d).", connectionAttempts, maxFarmerReconnect)
		if connectionAttempts >= maxFarmerReconnect {
			log.Fatalf("Failed to connect Farmer to NATS %d times, exiting.", connectionAttempts)
		}
		time.Sleep(time.Second * 15)
	}
	connectionAttempts = 0
	log.Debugf("Successfully joined Farmer to NATS bus")

	_, err = nc.Subscribe("grlx.sprouts.announce.>", func(m *nats.Msg) {
		log.Infof("Received a join event: %s\n", string(m.Data))
	})
	if err != nil {
		log.Errorf("Got an error on Subscribe: %+v\n", err)
	}

	test.RegisterNatsConn(nc)
	cmd.RegisterNatsConn(nc)
	cook.RegisterNatsConn(nc)
	jobs.RegisterNatsConn(nc)
	handlers.RegisterNatsConn(nc)
	defer nc.Close()
	select {}
}
