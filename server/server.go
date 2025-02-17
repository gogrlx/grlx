package server

import (
	"context"
	"net/http"
	"time"

	"github.com/gogrlx/grlx/api"
	"github.com/gogrlx/grlx/config"
	"github.com/gogrlx/grlx/types"
	"github.com/taigrr/log-socket/log"
)

var (
	version types.Version
	server  *http.Server
)

func SetVersion(v types.Version) {
	version = v
}

func StopAPIServer(ctx context.Context) error {
	if server != nil {
		return server.Shutdown(ctx)
	}
	return nil
}

func StartAPIServer() *http.Server {
	CertFile := config.CertFile
	FarmerInterface := config.FarmerInterface
	FarmerAPIPort := config.FarmerAPIPort
	KeyFile := config.KeyFile
	r := api.NewRouter(version, CertFile)
	srv := http.Server{
		// TODO add all below settings to configuration
		Addr:         FarmerInterface + ":" + FarmerAPIPort,
		WriteTimeout: time.Second * 120,
		ReadTimeout:  time.Second * 120,
		IdleTimeout:  time.Second * 120,
		Handler:      r,
	}
	go func() {
		if err := srv.ListenAndServeTLS(CertFile, KeyFile); err != nil {
			if err == http.ErrServerClosed {
				log.Debugf("API Server Shutting down, standby...")
				return
			}
			log.Panicf("API Server failure: %v", err)
		}
	}()

	log.Tracef("API Server started on %s\n", FarmerInterface+":"+FarmerAPIPort)
	server = &srv
	return &srv
}
