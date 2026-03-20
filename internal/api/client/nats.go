package client

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/gogrlx/grlx/v2/internal/auth"
	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/log"
)

// NatsConn is the shared NATS connection used by the CLI client.
var NatsConn *nats.Conn

// NatsRequestTimeout is the default timeout for NATS request/reply.
var NatsRequestTimeout = 30 * time.Second

// natsResponse is the envelope returned by NATS API handlers.
type natsResponse struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// NewNatsClient creates a new NATS connection authenticated via NKey.
func NewNatsClient() (*nats.Conn, error) {
	URL := config.FarmerBusURL
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
	tlsCfg := &tls.Config{
		ServerName: config.FarmerInterface,
		RootCAs:    certPool,
		MinVersion: tls.VersionTLS12,
	}

	connOpts := []nats.Option{nats.Name("grlx-cli"), nats.Nkey(pubkey, auth.Sign), nats.Secure(tlsCfg)}

	log.Tracef("Connecting to %s", URL)
	return nats.Connect(URL, connOpts...)
}

// ConnectNats establishes the shared NATS connection for the CLI.
func ConnectNats() error {
	nc, err := NewNatsClient()
	if err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}
	NatsConn = nc
	return nil
}

// NatsRequest sends a request to a NATS API method and returns the result.
// The method is appended to "grlx.api." to form the subject.
// params is marshaled to JSON; pass nil for no params.
// The local user's auth token is automatically injected into the JSON
// payload so the farmer can attribute the request to the invoking user.
func NatsRequest(method string, params any) (json.RawMessage, error) {
	if NatsConn == nil {
		return nil, fmt.Errorf("NATS connection not established")
	}

	subject := "grlx.api." + method

	var data []byte
	if params != nil {
		var err error
		data, err = json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal params: %w", err)
		}
	}

	// Inject the auth token into the JSON payload so the farmer can
	// identify the invoking user for attribution and RBAC checks.
	data = injectToken(data)

	msg, err := NatsConn.Request(subject, data, NatsRequestTimeout)
	if err != nil {
		return nil, fmt.Errorf("NATS request to %s failed: %w", subject, err)
	}

	var resp natsResponse
	if err := json.Unmarshal(msg.Data, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("%s", resp.Error)
	}

	return resp.Result, nil
}

// injectToken merges a "token" field into the JSON payload. If the
// payload is nil or empty, it creates a new JSON object with just
// the token. If the token cannot be generated, the payload is
// returned unchanged (the request will proceed unauthenticated).
func injectToken(data []byte) []byte {
	token, err := auth.NewToken()
	if err != nil {
		return data
	}
	tokenJSON, err := json.Marshal(token)
	if err != nil {
		return data
	}

	if len(data) == 0 {
		return []byte(fmt.Sprintf(`{"token":%s}`, tokenJSON))
	}

	// Inject token into existing JSON object by replacing the opening
	// brace with an opening brace + token field. This avoids
	// unmarshaling/remarshaling the entire payload.
	trimmed := data
	for len(trimmed) > 0 && trimmed[0] == ' ' {
		trimmed = trimmed[1:]
	}
	if len(trimmed) > 0 && trimmed[0] == '{' {
		return append([]byte(fmt.Sprintf(`{"token":%s,`, tokenJSON)), trimmed[1:]...)
	}

	return data
}
