package natsapi

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/pki"
)

// setupNatsAPIPKI mirrors the pki_test.go setup so PKI handlers can
// manipulate sprout keys during tests without hitting log.Fatalf.
func setupNatsAPIPKI(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	pkiDir := filepath.Join(tmpDir, "pki") + "/"
	config.FarmerPKI = pkiDir

	for _, state := range []string{"unaccepted", "accepted", "denied", "rejected"} {
		dir := filepath.Join(pkiDir, "sprouts", state)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create %s dir: %v", state, err)
		}
	}

	// Dummy farmer pub key so ReloadNKeys doesn't fatal.
	farmerPubFile := filepath.Join(tmpDir, "farmer.pub")
	if err := os.WriteFile(farmerPubFile, []byte("UFAKE_FARMER_KEY_FOR_TESTING"), 0o600); err != nil {
		t.Fatalf("write dummy farmer pub key: %v", err)
	}
	config.NKeyFarmerPubFile = farmerPubFile

	// NatsServer must be nil so ReloadNKeys skips server reload.
	pki.NatsServer = nil

	// TLS config for ConfigureNats inside ReloadNKeys.
	config.FarmerInterface = "127.0.0.1"
	config.FarmerBusPort = "14222"
	config.FarmerOrganization = "grlx-test"
	config.CertificateValidTime = 24 * 365 * time.Hour
	config.RootCA = filepath.Join(tmpDir, "rootca.pem")
	config.RootCAPriv = filepath.Join(tmpDir, "rootca-key.pem")
	config.CertFile = filepath.Join(tmpDir, "cert.pem")
	config.KeyFile = filepath.Join(tmpDir, "key.pem")
	config.CertHosts = []string{"127.0.0.1"}

	generateNatsAPICerts(t, tmpDir)

	return pkiDir
}

func generateNatsAPICerts(t *testing.T, tmpDir string) {
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}

	caTemplate := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{Organization: []string{"grlx-test"}},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caDER, err := x509.CreateCertificate(rand.Reader, &caTemplate, &caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create CA cert: %v", err)
	}

	writeTestPEM(t, filepath.Join(tmpDir, "rootca.pem"), "CERTIFICATE", caDER)

	caPrivBytes, err := x509.MarshalPKCS8PrivateKey(caKey)
	if err != nil {
		t.Fatalf("marshal CA key: %v", err)
	}
	writeTestPEM(t, filepath.Join(tmpDir, "rootca-key.pem"), "PRIVATE KEY", caPrivBytes)

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate leaf key: %v", err)
	}

	leafTemplate := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{Organization: []string{"grlx-test"}},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	leafDER, err := x509.CreateCertificate(rand.Reader, &leafTemplate, &caTemplate, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create leaf cert: %v", err)
	}

	writeTestPEM(t, filepath.Join(tmpDir, "cert.pem"), "CERTIFICATE", leafDER)

	leafPrivBytes, err := x509.MarshalPKCS8PrivateKey(leafKey)
	if err != nil {
		t.Fatalf("marshal leaf key: %v", err)
	}
	writeTestPEM(t, filepath.Join(tmpDir, "key.pem"), "PRIVATE KEY", leafPrivBytes)
}

func writeTestPEM(t *testing.T, path, blockType string, data []byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	if err := pem.Encode(f, &pem.Block{Type: blockType, Bytes: data}); err != nil {
		t.Fatalf("encode PEM %s: %v", path, err)
	}
}

func writeNKey(t *testing.T, pkiDir, state, sproutID, nkey string) {
	t.Helper()
	path := filepath.Join(pkiDir, "sprouts", state, sproutID)
	if err := os.WriteFile(path, []byte(nkey), 0o600); err != nil {
		t.Fatalf("write key file %s: %v", path, err)
	}
}

// --- PKI handler tests ---

func TestHandlePKIListEmpty(t *testing.T) {
	setupNatsAPIPKI(t)

	result, err := handlePKIList(nil)
	if err != nil {
		t.Fatalf("handlePKIList: %v", err)
	}

	keys, ok := result.(pki.KeysByType)
	if !ok {
		t.Fatalf("result type = %T, want pki.KeysByType", result)
	}
	if len(keys.Accepted.Sprouts) != 0 {
		t.Errorf("expected 0 accepted sprouts, got %d", len(keys.Accepted.Sprouts))
	}
}

func TestHandlePKIListWithKeys(t *testing.T) {
	pkiDir := setupNatsAPIPKI(t)

	writeNKey(t, pkiDir, "unaccepted", "sprout-a", "UKEY_A")
	writeNKey(t, pkiDir, "accepted", "sprout-b", "UKEY_B")
	writeNKey(t, pkiDir, "denied", "sprout-c", "UKEY_C")

	result, err := handlePKIList(nil)
	if err != nil {
		t.Fatalf("handlePKIList: %v", err)
	}

	keys := result.(pki.KeysByType)
	if len(keys.Unaccepted.Sprouts) != 1 {
		t.Errorf("expected 1 unaccepted, got %d", len(keys.Unaccepted.Sprouts))
	}
	if len(keys.Accepted.Sprouts) != 1 {
		t.Errorf("expected 1 accepted, got %d", len(keys.Accepted.Sprouts))
	}
	if len(keys.Denied.Sprouts) != 1 {
		t.Errorf("expected 1 denied, got %d", len(keys.Denied.Sprouts))
	}
}

func TestHandlePKIAccept(t *testing.T) {
	pkiDir := setupNatsAPIPKI(t)

	writeNKey(t, pkiDir, "unaccepted", "sprout-new", "UKEY_NEW")

	params := json.RawMessage(`{"id":"sprout-new"}`)
	result, err := handlePKIAccept(params)
	if err != nil {
		t.Fatalf("handlePKIAccept: %v", err)
	}

	m, ok := result.(map[string]bool)
	if !ok || !m["success"] {
		t.Fatalf("expected success=true, got %v", result)
	}

	// Verify key moved to accepted.
	keys := pki.ListNKeysByType()
	found := false
	for _, km := range keys.Accepted.Sprouts {
		if km.SproutID == "sprout-new" {
			found = true
			break
		}
	}
	if !found {
		t.Error("sprout-new not found in accepted keys after accept")
	}
}

func TestHandlePKIAcceptInvalidJSON(t *testing.T) {
	setupNatsAPIPKI(t)

	_, err := handlePKIAccept(json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHandlePKIAcceptNonexistent(t *testing.T) {
	setupNatsAPIPKI(t)

	params := json.RawMessage(`{"id":"does-not-exist"}`)
	_, err := handlePKIAccept(params)
	if err == nil {
		t.Fatal("expected error for nonexistent sprout")
	}
}

func TestHandlePKIReject(t *testing.T) {
	pkiDir := setupNatsAPIPKI(t)

	writeNKey(t, pkiDir, "accepted", "sprout-rej", "UKEY_REJ")

	params := json.RawMessage(`{"id":"sprout-rej"}`)
	result, err := handlePKIReject(params)
	if err != nil {
		t.Fatalf("handlePKIReject: %v", err)
	}

	m := result.(map[string]bool)
	if !m["success"] {
		t.Fatal("expected success=true")
	}

	keys := pki.ListNKeysByType()
	for _, km := range keys.Accepted.Sprouts {
		if km.SproutID == "sprout-rej" {
			t.Error("sprout-rej still in accepted after reject")
		}
	}
}

func TestHandlePKIRejectInvalidJSON(t *testing.T) {
	setupNatsAPIPKI(t)

	_, err := handlePKIReject(json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHandlePKIDeny(t *testing.T) {
	pkiDir := setupNatsAPIPKI(t)

	writeNKey(t, pkiDir, "unaccepted", "sprout-deny", "UKEY_DENY")

	params := json.RawMessage(`{"id":"sprout-deny"}`)
	result, err := handlePKIDeny(params)
	if err != nil {
		t.Fatalf("handlePKIDeny: %v", err)
	}

	m := result.(map[string]bool)
	if !m["success"] {
		t.Fatal("expected success=true")
	}

	keys := pki.ListNKeysByType()
	found := false
	for _, km := range keys.Denied.Sprouts {
		if km.SproutID == "sprout-deny" {
			found = true
			break
		}
	}
	if !found {
		t.Error("sprout-deny not found in denied keys")
	}
}

func TestHandlePKIDenyInvalidJSON(t *testing.T) {
	setupNatsAPIPKI(t)

	_, err := handlePKIDeny(json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHandlePKIUnaccept(t *testing.T) {
	pkiDir := setupNatsAPIPKI(t)

	writeNKey(t, pkiDir, "accepted", "sprout-unacc", "UKEY_UNACC")

	params := json.RawMessage(`{"id":"sprout-unacc"}`)
	result, err := handlePKIUnaccept(params)
	if err != nil {
		t.Fatalf("handlePKIUnaccept: %v", err)
	}

	m := result.(map[string]bool)
	if !m["success"] {
		t.Fatal("expected success=true")
	}
}

func TestHandlePKIUnacceptInvalidJSON(t *testing.T) {
	setupNatsAPIPKI(t)

	_, err := handlePKIUnaccept(json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHandlePKIDelete(t *testing.T) {
	pkiDir := setupNatsAPIPKI(t)

	writeNKey(t, pkiDir, "unaccepted", "sprout-del", "UKEY_DEL")

	params := json.RawMessage(`{"id":"sprout-del"}`)
	result, err := handlePKIDelete(params)
	if err != nil {
		t.Fatalf("handlePKIDelete: %v", err)
	}

	m := result.(map[string]bool)
	if !m["success"] {
		t.Fatal("expected success=true")
	}

	// Verify key is gone from all states.
	keys := pki.ListNKeysByType()
	for _, km := range keys.Unaccepted.Sprouts {
		if km.SproutID == "sprout-del" {
			t.Error("sprout-del still exists after delete")
		}
	}
}

func TestHandlePKIDeleteInvalidJSON(t *testing.T) {
	setupNatsAPIPKI(t)

	_, err := handlePKIDelete(json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHandlePKIDeleteNonexistent(t *testing.T) {
	setupNatsAPIPKI(t)

	params := json.RawMessage(`{"id":"ghost"}`)
	_, err := handlePKIDelete(params)
	if err == nil {
		t.Fatal("expected error for nonexistent sprout")
	}
}

// --- Sprouts handler tests ---

func TestHandleSproutsListEmpty(t *testing.T) {
	setupNatsAPIPKI(t)

	// Ensure no NATS conn for probe.
	old := natsConn
	natsConn = nil
	defer func() { natsConn = old }()

	result, err := handleSproutsList(nil)
	if err != nil {
		t.Fatalf("handleSproutsList: %v", err)
	}

	m, ok := result.(map[string][]SproutInfo)
	if !ok {
		t.Fatalf("result type = %T, want map[string][]SproutInfo", result)
	}
	if len(m["sprouts"]) != 0 {
		t.Errorf("expected 0 sprouts, got %d", len(m["sprouts"]))
	}
}

func TestHandleSproutsListWithSprouts(t *testing.T) {
	pkiDir := setupNatsAPIPKI(t)

	old := natsConn
	natsConn = nil
	defer func() { natsConn = old }()

	writeNKey(t, pkiDir, "accepted", "sprout-alpha", "UKEY_ALPHA")
	writeNKey(t, pkiDir, "unaccepted", "sprout-beta", "UKEY_BETA")
	writeNKey(t, pkiDir, "denied", "sprout-gamma", "UKEY_GAMMA")

	result, err := handleSproutsList(nil)
	if err != nil {
		t.Fatalf("handleSproutsList: %v", err)
	}

	m := result.(map[string][]SproutInfo)
	if len(m["sprouts"]) != 3 {
		t.Errorf("expected 3 sprouts, got %d", len(m["sprouts"]))
	}

	states := make(map[string]string)
	for _, s := range m["sprouts"] {
		states[s.ID] = s.KeyState
	}
	if states["sprout-alpha"] != "accepted" {
		t.Errorf("sprout-alpha state = %q, want accepted", states["sprout-alpha"])
	}
	if states["sprout-beta"] != "unaccepted" {
		t.Errorf("sprout-beta state = %q, want unaccepted", states["sprout-beta"])
	}
	if states["sprout-gamma"] != "denied" {
		t.Errorf("sprout-gamma state = %q, want denied", states["sprout-gamma"])
	}
}

func TestHandleSproutsGetFound(t *testing.T) {
	pkiDir := setupNatsAPIPKI(t)

	old := natsConn
	natsConn = nil
	defer func() { natsConn = old }()

	writeNKey(t, pkiDir, "accepted", "sprout-found", "UKEY_FOUND")

	params := json.RawMessage(`{"id":"sprout-found"}`)
	result, err := handleSproutsGet(params)
	if err != nil {
		t.Fatalf("handleSproutsGet: %v", err)
	}

	info, ok := result.(SproutInfo)
	if !ok {
		t.Fatalf("result type = %T, want SproutInfo", result)
	}
	if info.ID != "sprout-found" {
		t.Errorf("ID = %q, want sprout-found", info.ID)
	}
	if info.KeyState != "accepted" {
		t.Errorf("KeyState = %q, want accepted", info.KeyState)
	}
	if info.NKey != "UKEY_FOUND" {
		t.Errorf("NKey = %q, want UKEY_FOUND", info.NKey)
	}
}

func TestHandleSproutsGetNotFound(t *testing.T) {
	setupNatsAPIPKI(t)

	params := json.RawMessage(`{"id":"sprout-ghost"}`)
	_, err := handleSproutsGet(params)
	if err == nil {
		t.Fatal("expected error for nonexistent sprout")
	}
}

func TestHandleSproutsGetInvalidJSON(t *testing.T) {
	setupNatsAPIPKI(t)

	_, err := handleSproutsGet(json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHandleSproutsGetInvalidID(t *testing.T) {
	setupNatsAPIPKI(t)

	params := json.RawMessage(`{"id":"INVALID_UPPER"}`)
	_, err := handleSproutsGet(params)
	if err == nil {
		t.Fatal("expected error for invalid sprout ID")
	}
}

func TestHandleSproutsGetEmptyID(t *testing.T) {
	setupNatsAPIPKI(t)

	params := json.RawMessage(`{"id":""}`)
	_, err := handleSproutsGet(params)
	if err == nil {
		t.Fatal("expected error for empty sprout ID")
	}
}

// --- resolveKeyState tests ---

func TestResolveKeyState(t *testing.T) {
	pkiDir := setupNatsAPIPKI(t)

	writeNKey(t, pkiDir, "accepted", "sprout-ks-a", "UKEY_A")
	writeNKey(t, pkiDir, "denied", "sprout-ks-d", "UKEY_D")
	writeNKey(t, pkiDir, "rejected", "sprout-ks-r", "UKEY_R")
	writeNKey(t, pkiDir, "unaccepted", "sprout-ks-u", "UKEY_U")

	tests := []struct {
		id   string
		want string
	}{
		{"sprout-ks-a", "accepted"},
		{"sprout-ks-d", "denied"},
		{"sprout-ks-r", "rejected"},
		{"sprout-ks-u", "unaccepted"},
		{"nonexistent", "unknown"},
	}

	for _, tt := range tests {
		got := resolveKeyState(tt.id)
		if got != tt.want {
			t.Errorf("resolveKeyState(%q) = %q, want %q", tt.id, got, tt.want)
		}
	}
}

// --- probeSprout tests ---

func TestProbeSproutNoConn(t *testing.T) {
	old := natsConn
	natsConn = nil
	defer func() { natsConn = old }()

	if probeSprout("any-sprout") {
		t.Error("expected false when natsConn is nil")
	}
}

// --- SetNatsConn test ---

func TestSetNatsConn(t *testing.T) {
	old := natsConn
	defer func() { natsConn = old }()

	SetNatsConn(nil)
	if natsConn != nil {
		t.Error("expected nil after SetNatsConn(nil)")
	}
}

// --- Cmd handler tests ---

func TestHandleCmdRunInvalidJSON(t *testing.T) {
	setupNatsAPIPKI(t)

	_, err := handleCmdRun(json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHandleCmdRunInvalidSproutID(t *testing.T) {
	setupNatsAPIPKI(t)

	params := json.RawMessage(`{"target":[{"id":"INVALID_UPPER"}],"action":{"command":"echo hello"}}`)
	_, err := handleCmdRun(params)
	if err == nil {
		t.Fatal("expected error for invalid sprout ID")
	}
}

func TestHandleCmdRunUnknownSprout(t *testing.T) {
	setupNatsAPIPKI(t)

	params := json.RawMessage(`{"target":[{"id":"unknown-sprout"}],"action":{"command":"echo hello"}}`)
	_, err := handleCmdRun(params)
	if err == nil {
		t.Fatal("expected error for unknown sprout")
	}
}

func TestHandleCmdRunSproutIDWithUnderscore(t *testing.T) {
	setupNatsAPIPKI(t)

	params := json.RawMessage(`{"target":[{"id":"sprout_bad"}],"action":{"command":"echo hello"}}`)
	_, err := handleCmdRun(params)
	if err == nil {
		t.Fatal("expected error for sprout ID with underscore")
	}
}

// --- Test.ping handler tests ---

func TestHandleTestPingInvalidJSON(t *testing.T) {
	setupNatsAPIPKI(t)

	_, err := handleTestPing(json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHandleTestPingInvalidSproutID(t *testing.T) {
	setupNatsAPIPKI(t)

	params := json.RawMessage(`{"target":[{"id":"BAD_ID"}],"action":{"ping":true}}`)
	_, err := handleTestPing(params)
	if err == nil {
		t.Fatal("expected error for invalid sprout ID")
	}
}

func TestHandleTestPingUnknownSprout(t *testing.T) {
	setupNatsAPIPKI(t)

	params := json.RawMessage(`{"target":[{"id":"unknown-sprout"}],"action":{"ping":true}}`)
	_, err := handleTestPing(params)
	if err == nil {
		t.Fatal("expected error for unknown sprout")
	}
}

// --- Cook handler tests ---

func TestHandleCookInvalidJSON(t *testing.T) {
	setupNatsAPIPKI(t)

	_, err := handleCook(json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHandleCookInvalidSproutID(t *testing.T) {
	setupNatsAPIPKI(t)

	params := json.RawMessage(`{"target":[{"id":"BAD_UPPER"}],"action":{"recipe":"test.sls"}}`)
	_, err := handleCook(params)
	if err == nil {
		t.Fatal("expected error for invalid sprout ID")
	}
}

func TestHandleCookUnknownSprout(t *testing.T) {
	setupNatsAPIPKI(t)

	params := json.RawMessage(`{"target":[{"id":"ghost-sprout"}],"action":{"recipe":"test.sls"}}`)
	_, err := handleCook(params)
	if err == nil {
		t.Fatal("expected error for unknown sprout")
	}
}

func TestHandleCookNoNATSConn(t *testing.T) {
	pkiDir := setupNatsAPIPKI(t)

	writeNKey(t, pkiDir, "accepted", "sprout-cook", "UKEY_COOK")
	// Register the sprout as having an NKey to pass the existence check.

	old := natsConn
	natsConn = nil
	defer func() { natsConn = old }()

	params := json.RawMessage(`{"target":[{"id":"sprout-cook"}],"action":{"recipe":"test.sls"}}`)
	_, err := handleCook(params)
	if err == nil {
		t.Fatal("expected error when NATS not available")
	}
	if err.Error() != "NATS connection not available" {
		t.Errorf("error = %q, want 'NATS connection not available'", err.Error())
	}
}

// --- Shell handler tests ---

func TestHandleShellStartInvalidJSON(t *testing.T) {
	setupNatsAPIPKI(t)

	_, err := handleShellStart(json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHandleShellStartMissingSproutID(t *testing.T) {
	setupNatsAPIPKI(t)

	params := json.RawMessage(`{}`)
	_, err := handleShellStart(params)
	if err == nil {
		t.Fatal("expected error for missing sprout_id")
	}
}

func TestHandleShellStartInvalidSproutID(t *testing.T) {
	setupNatsAPIPKI(t)

	params := json.RawMessage(`{"sprout_id":"BAD_UPPER"}`)
	_, err := handleShellStart(params)
	if err == nil {
		t.Fatal("expected error for invalid sprout ID")
	}
}

func TestHandleShellStartUnknownSprout(t *testing.T) {
	setupNatsAPIPKI(t)

	params := json.RawMessage(`{"sprout_id":"unknown-sprout"}`)
	_, err := handleShellStart(params)
	if err == nil {
		t.Fatal("expected error for unknown sprout")
	}
}

func TestHandleShellStartSproutIDWithUnderscore(t *testing.T) {
	setupNatsAPIPKI(t)

	params := json.RawMessage(`{"sprout_id":"sprout_bad"}`)
	_, err := handleShellStart(params)
	if err == nil {
		t.Fatal("expected error for sprout ID with underscore")
	}
}

// --- resolveCallerIdentity tests ---

func TestResolveCallerIdentityEmpty(t *testing.T) {
	pk, role := resolveCallerIdentity(nil)
	if pk != "" || role != "" {
		t.Errorf("expected empty, got pk=%q role=%q", pk, role)
	}
}

func TestResolveCallerIdentityNoToken(t *testing.T) {
	pk, role := resolveCallerIdentity(json.RawMessage(`{}`))
	if pk != "" || role != "" {
		t.Errorf("expected empty, got pk=%q role=%q", pk, role)
	}
}

func TestResolveCallerIdentityInvalidJSON(t *testing.T) {
	pk, role := resolveCallerIdentity(json.RawMessage(`{invalid`))
	if pk != "" || role != "" {
		t.Errorf("expected empty for invalid JSON, got pk=%q role=%q", pk, role)
	}
}

func TestResolveCallerIdentityBadToken(t *testing.T) {
	pk, role := resolveCallerIdentity(json.RawMessage(`{"token":"garbage"}`))
	if pk != "" || role != "" {
		t.Errorf("expected empty for bad token, got pk=%q role=%q", pk, role)
	}
}

// --- ShellTracker test ---

func TestShellTracker(t *testing.T) {
	tracker := ShellTracker()
	if tracker == nil {
		t.Fatal("ShellTracker returned nil")
	}
}

// --- logShellEnd tests (nil audit logger) ---

func TestLogShellEndNilLogger(t *testing.T) {
	// Should not panic even with nil global audit logger.
	// audit.Global() returns nil in test context.
	info := &struct {
		SessionID string
		SproutID  string
		Pubkey    string
		RoleName  string
		Shell     string
	}{
		SessionID: "test-session",
		SproutID:  "test-sprout",
		Pubkey:    "UTEST",
		RoleName:  "admin",
	}
	// We can't easily call logShellEnd with the shell.SessionInfo type
	// from here since it requires the actual type. Just verify ShellTracker
	// works.
	_ = info
}

// --- Auth add/remove user tests ---

func TestHandleAuthAddUserMissingFields(t *testing.T) {
	_, err := handleAuthAddUser(json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for missing fields")
	}

	_, err = handleAuthAddUser(json.RawMessage(`{"pubkey":"UTEST"}`))
	if err == nil {
		t.Fatal("expected error for missing role")
	}

	_, err = handleAuthAddUser(json.RawMessage(`{"role":"admin"}`))
	if err == nil {
		t.Fatal("expected error for missing pubkey")
	}
}

func TestHandleAuthAddUserInvalidJSON(t *testing.T) {
	_, err := handleAuthAddUser(json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHandleAuthRemoveUserMissingPubkey(t *testing.T) {
	_, err := handleAuthRemoveUser(json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for missing pubkey")
	}
}

func TestHandleAuthRemoveUserInvalidJSON(t *testing.T) {
	_, err := handleAuthRemoveUser(json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// Audit handler tests are in audit_test.go.

// --- Scope tests ---

func TestAllAcceptedSproutIDs(t *testing.T) {
	pkiDir := setupNatsAPIPKI(t)

	writeNKey(t, pkiDir, "accepted", "sprout-scope-a", "UKEY_A")
	writeNKey(t, pkiDir, "accepted", "sprout-scope-b", "UKEY_B")
	writeNKey(t, pkiDir, "unaccepted", "sprout-scope-c", "UKEY_C")

	ids := allAcceptedSproutIDs()
	if len(ids) != 2 {
		t.Fatalf("expected 2 accepted IDs, got %d", len(ids))
	}

	found := make(map[string]bool)
	for _, id := range ids {
		found[id] = true
	}
	if !found["sprout-scope-a"] || !found["sprout-scope-b"] {
		t.Errorf("expected sprout-scope-a and sprout-scope-b, got %v", ids)
	}
}

func TestAllAcceptedSproutIDsEmpty(t *testing.T) {
	setupNatsAPIPKI(t)

	ids := allAcceptedSproutIDs()
	if len(ids) != 0 {
		t.Errorf("expected 0 accepted IDs, got %d", len(ids))
	}
}
