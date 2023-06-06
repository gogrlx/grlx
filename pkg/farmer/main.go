package main

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/spf13/viper"
	log "github.com/taigrr/log-socket/log"

	"github.com/gogrlx/grlx/api"
	"github.com/gogrlx/grlx/certs"
	"github.com/gogrlx/grlx/config"
	"github.com/gogrlx/grlx/cook"
	"github.com/gogrlx/grlx/ingredients/cmd"
	"github.com/gogrlx/grlx/ingredients/test"
	"github.com/gogrlx/grlx/pki"
	"github.com/gogrlx/grlx/types"

	nats_server "github.com/nats-io/nats-server/v2/server"
	nats "github.com/nats-io/nats.go"
)

func init() {
	config.LoadConfig("farmer")
	log.SetLogLevel(log.LDebug)
}

var (
	s         *nats_server.Server
	BuildTime string
	GitCommit string
	Tag       string
)

func main() {
	config.LoadConfig("farmer")
	defer log.Flush()
	createConfigRoot()
	pki.SetupPKIFarmer()
	certs.GenCert()
	certs.GenNKey(true)
	RunNATSServer()
	StartAPIServer()
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
	ConfigRoot := viper.GetString("ConfigRoot")
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
		// TODO: work out what the other errors could be here
		log.Panicf(err.Error())
	}
}

func StartAPIServer() {
	CertFile := viper.GetString("CertFile")
	FarmerInterface := viper.GetString("FarmerInterface")
	FarmerAPIPort := viper.GetString("FarmerAPIPort")
	KeyFile := viper.GetString("KeyFile")
	r := api.NewRouter(types.Version{
		Arch:      runtime.GOOS,
		BuildTime: BuildTime,
		Compiler:  runtime.Version(),
		GitCommit: GitCommit,
		Tag:       Tag,
	}, CertFile)
	srv := http.Server{
		// TODO: add all below settings to configuration
		Addr:         FarmerInterface + ":" + FarmerAPIPort,
		WriteTimeout: time.Second * 120,
		ReadTimeout:  time.Second * 120,
		IdleTimeout:  time.Second * 120,
		Handler:      r,
	}
	go func() {
		if err := srv.ListenAndServeTLS(CertFile, KeyFile); err != nil {
			log.Fatalf(err.Error())
		}
	}()

	log.Tracef("API Server started on %s\n", FarmerInterface+":"+FarmerAPIPort)
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
	pki.ReloadNKeys()
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
		// TODO handle case where nats server port is already taken
		log.Panicf("Unable to start NATS Server in Go Routine")
	}
	// s.ReloadOptions(opts)
	pki.SetNATSServer(s)
	pki.ReloadNKeys()
}

func ConnectFarmer() {
	connectionAttempts := 1
	maxFarmerReconnect := 30
	RootCA := viper.GetString("RootCA")
	FarmerInterface := viper.GetString("FarmerInterface")
	if FarmerInterface == "0.0.0.0" {
		FarmerInterface = "localhost"
	}
	var err error
	opt, err := nats.NkeyOptionFromSeed(viper.GetString("NKeyFarmerPrivFile"))
	_ = opt
	if err != nil {
		// TODO: handle error
		log.Panic(err)
	}
	certPool := x509.NewCertPool()
	rootPEM, err := ioutil.ReadFile(RootCA)
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
	_ = config
	log.Debug("Attempting to pair Farmer to NATS bus.")
	nc, err := nats.Connect("tls://"+FarmerInterface+":4443", // nats.RootCAs(RootCA),
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

	for !nc.IsConnected() {
		connectionAttempts++
		log.Debugf("Attempting to pair Farmer to NATS bus (attempt %d/%d).", connectionAttempts, maxFarmerReconnect)
		if connectionAttempts >= maxFarmerReconnect {
			log.Fatalf("Failed to connect Farmer to NATS %d times, exiting.", connectionAttempts)
		}
		time.Sleep(time.Second * 15)
	}
	connectionAttempts = 0
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
	test.RegisterEC(ec)
	cmd.RegisterEC(ec)
	cook.RegisterEC(ec)
	defer ec.Close()
	select {}
}
