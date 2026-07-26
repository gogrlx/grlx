package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	log "github.com/gogrlx/grlx/v2/internal/log"

	"github.com/gogrlx/grlx/v2/internal/api"
	"github.com/gogrlx/grlx/v2/internal/audit"
	"github.com/gogrlx/grlx/v2/internal/auth"
	"github.com/gogrlx/grlx/v2/internal/certs"
	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/facts"
	"github.com/gogrlx/grlx/v2/internal/ingredients/cmd"
	"github.com/gogrlx/grlx/v2/internal/ingredients/test"
	"github.com/gogrlx/grlx/v2/internal/jobs"
	"github.com/gogrlx/grlx/v2/internal/natsapi"
	"github.com/gogrlx/grlx/v2/internal/pki"
	"github.com/gogrlx/grlx/v2/internal/props"
	"github.com/gogrlx/grlx/v2/internal/rbac"

	nats_server "github.com/nats-io/nats-server/v2/server"
	nats "github.com/nats-io/nats.go"
)

func init() {
	config.LoadConfig("farmer")
	log.SetLogLevel(config.LogLevel)
}

var (
	// srvMu guards the s and apiServer package globals, which are read by the
	// shutdown path in main and written/read by handleSIGHUP concurrently.
	srvMu     sync.Mutex
	s         *nats_server.Server
	apiServer *http.Server
	GitCommit string
	Tag       string
)

func setNATSServer(v *nats_server.Server) {
	srvMu.Lock()
	s = v
	srvMu.Unlock()
}

func getNATSServer() *nats_server.Server {
	srvMu.Lock()
	defer srvMu.Unlock()
	return s
}

func setAPIServer(v *http.Server) {
	srvMu.Lock()
	apiServer = v
	srvMu.Unlock()
}

func getAPIServer() *http.Server {
	srvMu.Lock()
	defer srvMu.Unlock()
	return apiServer
}

func main() {
	config.LoadConfig("farmer")
	fmt.Printf("Starting Farmer with URL %s\n", config.FarmerBusURL)
	defer log.Flush()
	props.InitStore(config.PropsDir)
	props.LoadStaticProps(config.StaticProps())
	loadCohortRegistry()
	createConfigRoot()
	initAuditLogger()
	loadAuthPolicy()
	pki.SetupPKIFarmer()
	if err := certs.GenCert(); err != nil {
		log.Fatalf("failed to generate TLS certificates: %v", err)
	}
	if err := certs.GenNKey(true); err != nil {
		log.Fatalf("failed to generate farmer NKey: %v", err)
	}
	RunNATSServer()
	StartAPIServer()
	// ctx is cancelled on SIGINT/SIGTERM, driving a graceful shutdown of the
	// cohort refresher, job reaper, NATS connection, NATS server, and API server.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	natsapi.StartCohortRefresher(ctx, config.CohortRefreshInterval)
	farmerDone := make(chan struct{})
	sighupDone := make(chan struct{})
	go ConnectFarmer(ctx, farmerDone)
	go handleSIGHUP(ctx, sighupDone)

	<-ctx.Done()
	stop()
	log.Info("Shutdown signal received, stopping farmer...")
	// Stop the SIGHUP handler first so it can't restart the API server
	// concurrently with the shutdown below (bounded, with a warning).
	select {
	case <-sighupDone:
	case <-time.After(20 * time.Second):
		log.Warn("timed out waiting for SIGHUP handler to stop")
	}
	// Wait for ConnectFarmer to close the NATS client (bounded).
	select {
	case <-farmerDone:
	case <-time.After(10 * time.Second):
		log.Warn("timed out waiting for NATS client to close")
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if srv := getAPIServer(); srv != nil {
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Errorf("API server shutdown error: %v", err)
		}
	}
	if srv := getNATSServer(); srv != nil {
		srv.Shutdown()
	}
	log.Info("Farmer stopped")
}

func initAuditLogger() {
	auditDir := config.AuditLogDir
	if auditDir == "" {
		auditDir = "/var/log/grlx/audit"
	}
	logger, err := audit.NewLogger(auditDir)
	if err != nil {
		log.Errorf("Failed to initialize audit logger at %s: %v", auditDir, err)
		return
	}
	audit.SetGlobal(logger)
	audit.SetIdentityResolver(auth.WhoAmI)
	level := audit.ParseLevel(config.AuditLevel)
	audit.SetLevel(level)
	log.Infof("Audit logging enabled: %s (level: %s)", auditDir, level)
}

func loadAuthPolicy() {
	if auth.DangerouslyAllowRoot() {
		log.Warn("WARNING: dangerously_allow_root is enabled — ALL auth checks are bypassed. Do not use in production!")
	}

	if err := auth.LoadPolicy(); err != nil {
		log.Errorf("Failed to load auth policy: %v", err)
	} else {
		roles := auth.ListRoles()
		users := auth.ListAllUsers()
		log.Infof("Auth policy loaded: %d role(s), %d user(s)", len(roles), len(users))
	}
}

func loadCohortRegistry() {
	registry, err := rbac.LoadCohortsFromConfig()
	if err != nil {
		log.Errorf("Failed to load cohort config: %v", err)
		registry = rbac.NewRegistry()
	}
	if err := registry.ValidateReferences(); err != nil {
		log.Errorf("Cohort reference validation failed: %v", err)
	}
	natsapi.SetCohortRegistry(registry)
	names := registry.List()
	if len(names) > 0 {
		log.Infof("Loaded %d cohort(s): %v", len(names), names)
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

// StartAPIServer starts the farmer's HTTPS server. It handles PKI
// bootstrap (certificate distribution and NKey registration), file
// serving for recipe downloads (farmer:// scheme), and a health
// endpoint for monitoring and automated tooling.
func StartAPIServer() {
	CertFile := config.CertFile
	FarmerInterface := config.FarmerInterface
	FarmerAPIPort := config.FarmerAPIPort
	KeyFile := config.KeyFile
	r := api.NewRouter(CertFile)
	srv := &http.Server{
		Addr:         FarmerInterface + ":" + FarmerAPIPort,
		WriteTimeout: config.APIWriteTimeout,
		ReadTimeout:  config.APIReadTimeout,
		IdleTimeout:  config.APIIdleTimeout,
		Handler:      r,
	}
	setAPIServer(srv)
	go func() {
		if err := srv.ListenAndServeTLS(CertFile, KeyFile); err != nil && err != http.ErrServerClosed {
			log.Fatalf("API server failed: %v", err)
		}
	}()

	log.Tracef("API server started on %s\n", FarmerInterface+":"+FarmerAPIPort)
}

// handleSIGHUP listens for SIGHUP signals and reloads the API server
// and NATS server configuration. This allows certificate rotation and
// configuration changes to take effect without a full restart.
func handleSIGHUP(ctx context.Context, done chan<- struct{}) {
	defer close(done)
	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)
	defer signal.Stop(sighup)
	for {
		select {
		case <-ctx.Done():
			return
		case <-sighup:
		}
		// Re-check cancellation: the select above may have chosen the sighup
		// case even though shutdown was also requested. Don't start a new
		// server if we're shutting down.
		if ctx.Err() != nil {
			return
		}
		log.Info("Received SIGHUP, reloading servers...")

		// Reload NATS server NKeys (picks up new sprout keys, config changes)
		if err := pki.ReloadNKeys(); err != nil {
			log.Errorf("Failed to reload NKeys: %v", err)
		} else {
			log.Info("NATS NKeys reloaded successfully")
		}

		// Reload the NATS server configuration
		if srv := getNATSServer(); srv != nil {
			if err := srv.Reload(); err != nil {
				log.Errorf("Failed to reload NATS server: %v", err)
			} else {
				log.Info("NATS server reloaded successfully")
			}
		}

		// Gracefully shut down the API server and restart it
		// so it picks up any new TLS certificates
		if srv := getAPIServer(); srv != nil {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			if err := srv.Shutdown(shutdownCtx); err != nil {
				log.Errorf("Failed to gracefully shut down API server: %v", err)
			}
			cancel()
			log.Info("API server shut down, restarting...")
		}

		// Reload config before restarting the API server
		config.LoadConfig("farmer")
		props.ClearStaticProps()
		props.LoadStaticProps(config.StaticProps())
		loadCohortRegistry()
		loadAuthPolicy()
		// Don't restart the API server if shutdown was requested while we
		// were reloading.
		if ctx.Err() != nil {
			return
		}
		StartAPIServer()
		log.Info("Servers reloaded successfully")
	}
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
	srv, err := nats_server.NewServer(&opts)
	if err != nil || srv == nil {
		log.Panicf("No NATS Server object returned: %v", err)
	}
	// Run server in Go routine.
	go srv.Start()
	var natsLogger log.Logger
	srv.SetLogger(natsLogger, true, true)
	// Wait for accept loop(s) to be started
	if !srv.ReadyForConnections(10 * time.Second) {
		log.Panicf("Unable to start NATS Server")
	}
	setNATSServer(srv)
	pki.SetNATSServer(srv)
	pki.ReloadNKeys()
}

func ConnectFarmer(ctx context.Context, done chan<- struct{}) {
	defer close(done)
	var connectionAttempts atomic.Int64
	connectionAttempts.Store(1)
	maxFarmerReconnect := 30
	RootCA := config.RootCA
	BusURL := config.FarmerBusURL
	FarmerInterface := config.FarmerInterface
	if FarmerInterface == "0.0.0.0" {
		FarmerInterface = "localhost"
	}
	var err error
	opt, err := nats.NkeyOptionFromSeed(config.NKeyFarmerPrivFile)

	if err != nil {
		// NKey seed is critical for NATS authentication
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

	tlsCfg := &tls.Config{
		ServerName: FarmerInterface,
		RootCAs:    certPool,
		MinVersion: tls.VersionTLS12,
	}
	log.Debug("Attempting to pair Farmer to NATS bus.")
	nc, err := nats.Connect(BusURL,
		nats.Secure(tlsCfg),
		opt,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(maxFarmerReconnect),
		nats.ReconnectWait(time.Second*15),
		nats.DisconnectHandler(func(_ *nats.Conn) {
			log.Warnf("WARN: Reconnecting Farmer to NATS bus, attempt: %d\n", connectionAttempts.Add(1))
		}),
	)
	if err != nil {
		log.Errorf("Got an error on Connect with Secure Options: %+v\n", err)
	}
	if nc == nil {
		log.Fatalf("Failed to connect Farmer to NATS bus: %v", err)
	}
	for !nc.IsConnected() {
		attempts := connectionAttempts.Add(1)
		log.Debugf("Attempting to pair Farmer to NATS bus (attempt %d/%d).", attempts, maxFarmerReconnect)
		if attempts >= int64(maxFarmerReconnect) {
			log.Fatalf("Failed to connect Farmer to NATS %d times, exiting.", attempts)
		}
		select {
		case <-ctx.Done():
			nc.Close()
			return
		case <-time.After(time.Second * 15):
		}
	}
	connectionAttempts.Store(0)
	log.Debugf("Successfully joined Farmer to NATS bus")

	if err := log.ConnectNATS(BusURL); err != nil {
		log.Errorf("Failed to connect log-nats backend: %v", err)
	}

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
	facts.RegisterFarmerListener(nc)

	// Set version info and subscribe NATS API handlers.
	natsapi.SetBuildVersion(config.Version{
		Arch:      runtime.GOOS,
		Compiler:  runtime.Version(),
		GitCommit: GitCommit,
		Tag:       Tag,
	})
	if err := natsapi.Subscribe(nc); err != nil {
		log.Errorf("Failed to subscribe NATS API handlers: %v", err)
	} else {
		log.Info("NATS API handlers registered")
	}
	// Start the job log reaper to clean up old job files.
	jobStore := jobs.NewStore()
	jobStore.StartReaperCtx(ctx, config.JobLogTTL)
	<-ctx.Done()
	nc.Close()
}
