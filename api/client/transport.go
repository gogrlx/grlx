package client

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/gogrlx/grlx/config"
	"github.com/gogrlx/grlx/pki"
	"github.com/gogrlx/grlx/types"
)

var APIClient *http.Client

func CreateSecureTransport() error {
	APIClient = &http.Client{}
	config.LoadConfig("grlx")
	err := pki.LoadRootCA("grlx")
	if err != nil {
		return err
	}
	RootCA := config.GrlxRootCA
	certPool := x509.NewCertPool()
	rootPEM, err := os.ReadFile(RootCA)
	if err != nil || rootPEM == nil {
		return err
	}
	ok := certPool.AppendCertsFromPEM(rootPEM)
	if !ok {
		return errors.Join(types.ErrCannotParseRootCA, fmt.Errorf("apiClient: failed to parse root certificate from %q", RootCA))
	}
	var apiTransport http.RoundTripper = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			RootCAs:    certPool,
			MinVersion: tls.VersionTLS12,
		},
	}
	APIClient.Transport = apiTransport
	APIClient.Timeout = time.Second * 10
	return nil
}
