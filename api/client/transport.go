package client

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/spf13/viper"
	"github.com/taigrr/log-socket/log"

	"github.com/gogrlx/grlx/config"
	"github.com/gogrlx/grlx/pki"
	"github.com/gogrlx/grlx/types"
)

var APIClient *http.Client

func init() {
	APIClient = &http.Client{}
	config.LoadConfig("grlx")
	err := pki.LoadRootCA("grlx")
	if err != nil {
		log.Error(err)
	}
	RootCA := viper.GetString("GrlxRootCA")
	certPool := x509.NewCertPool()
	rootPEM, err := os.ReadFile(RootCA)
	if err != nil || rootPEM == nil {
		log.Error(err)
	}
	ok := certPool.AppendCertsFromPEM(rootPEM)
	if !ok {
		log.Errorf("apiClient: failed to parse root certificate from %q", RootCA)
		log.Error(types.ErrCannotParseRootCA)
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
}
