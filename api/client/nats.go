package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/nats-io/nats.go"
	"github.com/spf13/viper"
	"github.com/taigrr/log-socket/log"

	"github.com/gogrlx/grlx/auth"
)

func NewNatsClient() (*nats.Conn, error) {
	URL := viper.GetString("FarmerBusInterface")
	pubkey, err := auth.GetPubkey()
	if err != nil {
		return nil, err
	}
	auth.NewToken()
	rootCA := viper.GetString("GrlxRootCA")
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
		ServerName: viper.GetString("FarmerInterface"),
		RootCAs:    certPool,
		MinVersion: tls.VersionTLS12,
	}

	// TODO: add a disconnect handler to reconnect
	connOpts := []nats.Option{nats.Name("grlx-cli"), nats.Nkey(pubkey, auth.Sign), nats.Secure(config)}

	fmt.Println("Connecting to", URL)
	return nats.Connect("nats://"+URL, connOpts...)
}