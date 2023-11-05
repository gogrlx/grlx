package client

import (
	"crypto/tls"
	"crypto/x509"
	"os"

	"github.com/nats-io/nats.go"
	"github.com/taigrr/log-socket/log"

	"github.com/gogrlx/grlx/auth"
	"github.com/gogrlx/grlx/config"
)

func NewNatsClient() (*nats.Conn, error) {
	URL := config.FarmerBusInterface
	pubkey, err := auth.GetPubkey()
	if err != nil {
		return nil, err
	}
	auth.NewToken()
	rootCA := config.GrlxRootCA
	certPool := x509.NewCertPool()
	rootPEM, err := os.ReadFile(rootCA)
	if err != nil || rootPEM == nil {
		log.Panicf("nats: error loading or parsing rootCA file: %v", err)
	}
	ok := certPool.AppendCertsFromPEM(rootPEM)
	if !ok {
		log.Errorf("nats: failed to parse root certificate from %q", rootCA)
	}
	config := &tls.Config{
		ServerName: config.FarmerInterface,
		RootCAs:    certPool,
		MinVersion: tls.VersionTLS12,
	}

	// TODO: add a disconnect handler to reconnect
	connOpts := []nats.Option{nats.Name("grlx-cli"), nats.Nkey(pubkey, auth.Sign), nats.Secure(config)}

	log.Tracef("Connecting to %s", URL)
	return nats.Connect("nats://"+URL, connOpts...)
}
