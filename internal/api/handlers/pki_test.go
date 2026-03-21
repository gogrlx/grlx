package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"

	"github.com/nats-io/nkeys"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/pki"
)

// setupPKIDirs creates a temp PKI tree for testing and sets config.FarmerPKI.
// It also creates a fake farmer NKey pub file so that pki.ReloadNKeys()
// (called by defer in AcceptNKey, DenyNKey, etc.) doesn't log.Fatal.
func setupPKIDirs(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	config.FarmerPKI = dir + "/"
	for _, state := range []string{"accepted", "unaccepted", "denied", "rejected"} {
		if err := os.MkdirAll(filepath.Join(dir, "sprouts", state), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}

	// Create a fake farmer NKey pub file so ReloadNKeys doesn't crash.
	farmerKP, err := nkeys.CreateUser()
	if err != nil {
		t.Fatalf("create farmer nkey: %v", err)
	}
	farmerPub, err := farmerKP.PublicKey()
	if err != nil {
		t.Fatalf("get farmer pub: %v", err)
	}
	farmerPubFile := filepath.Join(dir, "farmer.pub")
	if err := os.WriteFile(farmerPubFile, []byte(farmerPub), 0o644); err != nil {
		t.Fatalf("write farmer pub: %v", err)
	}
	config.NKeyFarmerPubFile = farmerPubFile

	// Set minimal config values so ConfigureNats() doesn't panic.
	config.FarmerBusPort = "0"
	config.FarmerInterface = "127.0.0.1"

	// Create self-signed TLS certs for ReloadNKeys -> ConfigureNats.
	createTestTLSCerts(t, dir)
	config.RootCA = filepath.Join(dir, "rootCA.pem")
	config.CertFile = filepath.Join(dir, "cert.pem")
	config.KeyFile = filepath.Join(dir, "key.pem")

	return dir
}

// writeSproutKey writes a fake NKey file into the PKI tree.
func writeSproutKey(t *testing.T, dir, state, id, nkey string) {
	t.Helper()
	p := filepath.Join(dir, "sprouts", state, id)
	if err := os.WriteFile(p, []byte(nkey), 0o644); err != nil {
		t.Fatalf("write sprout key: %v", err)
	}
}

// --- PutNKey tests ---

func TestPutNKey_InvalidJSON(t *testing.T) {
	setupPKIDirs(t)

	req := httptest.NewRequest(http.MethodPut, "/nkey", bytes.NewReader([]byte("bad json")))
	w := httptest.NewRecorder()
	PutNKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

func TestPutNKey_InvalidSproutID(t *testing.T) {
	setupPKIDirs(t)

	body, _ := json.Marshal(pki.KeySubmission{
		SproutID: "INVALID-CAPS",
		NKey:     "UABC123",
	})
	req := httptest.NewRequest(http.MethodPut, "/nkey", bytes.NewReader(body))
	w := httptest.NewRecorder()
	PutNKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid sprout ID, got %d", w.Code)
	}
}

func TestPutNKey_SproutIDWithUnderscore(t *testing.T) {
	setupPKIDirs(t)

	body, _ := json.Marshal(pki.KeySubmission{
		SproutID: "bad_id",
		NKey:     "UABC123",
	})
	req := httptest.NewRequest(http.MethodPut, "/nkey", bytes.NewReader(body))
	w := httptest.NewRecorder()
	PutNKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for underscore in ID, got %d", w.Code)
	}
}

func TestPutNKey_InvalidNKey(t *testing.T) {
	setupPKIDirs(t)

	body, _ := json.Marshal(pki.KeySubmission{
		SproutID: "good-sprout",
		NKey:     "not-a-real-nkey",
	})
	req := httptest.NewRequest(http.MethodPut, "/nkey", bytes.NewReader(body))
	w := httptest.NewRecorder()
	PutNKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid NKey, got %d", w.Code)
	}
}

func TestPutNKey_NewSprout(t *testing.T) {
	dir := setupPKIDirs(t)

	// Generate a real NKey user key pair for testing
	nkey := generateTestUserNKey(t)

	body, _ := json.Marshal(pki.KeySubmission{
		SproutID: "new-sprout",
		NKey:     nkey,
	})
	req := httptest.NewRequest(http.MethodPut, "/nkey", bytes.NewReader(body))
	w := httptest.NewRecorder()
	PutNKey(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for new sprout, got %d: %s", w.Code, w.Body.String())
	}

	var resp apitypes.Inline
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Success {
		t.Error("expected success=true")
	}

	// Verify the key was saved in unaccepted
	keyPath := filepath.Join(dir, "sprouts", "unaccepted", "new-sprout")
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Error("expected key to be saved in unaccepted directory")
	}
}

func TestPutNKey_AlreadyKnownExact(t *testing.T) {
	dir := setupPKIDirs(t)

	nkey := generateTestUserNKey(t)
	writeSproutKey(t, dir, "accepted", "known-sprout", nkey)

	body, _ := json.Marshal(pki.KeySubmission{
		SproutID: "known-sprout",
		NKey:     nkey,
	})
	req := httptest.NewRequest(http.MethodPut, "/nkey", bytes.NewReader(body))
	w := httptest.NewRecorder()
	PutNKey(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for re-submitting same key, got %d", w.Code)
	}

	var resp apitypes.Inline
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Success {
		t.Error("expected success=true for already known key")
	}
}

func TestPutNKey_SameIDDifferentKey(t *testing.T) {
	dir := setupPKIDirs(t)

	existingKey := generateTestUserNKey(t)
	newKey := generateTestUserNKey(t)
	writeSproutKey(t, dir, "accepted", "conflict-sprout", existingKey)

	body, _ := json.Marshal(pki.KeySubmission{
		SproutID: "conflict-sprout",
		NKey:     newKey,
	})
	req := httptest.NewRequest(http.MethodPut, "/nkey", bytes.NewReader(body))
	w := httptest.NewRecorder()
	PutNKey(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (saved as conflict), got %d: %s", w.Code, w.Body.String())
	}

	// Should have been saved as conflict-sprout_1 in rejected
	rejPath := filepath.Join(dir, "sprouts", "rejected", "conflict-sprout_1")
	if _, err := os.Stat(rejPath); os.IsNotExist(err) {
		t.Error("expected conflicting key to be saved as conflict-sprout_1 in rejected")
	}
}

// --- GetCertificate tests ---

func TestGetCertificate(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "rootCA.pem")
	certContent := []byte("-----BEGIN CERTIFICATE-----\nfake\n-----END CERTIFICATE-----\n")
	if err := os.WriteFile(tmpFile, certContent, 0o644); err != nil {
		t.Fatal(err)
	}
	config.RootCA = tmpFile

	req := httptest.NewRequest(http.MethodGet, "/certificate", nil)
	w := httptest.NewRecorder()
	GetCertificate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("BEGIN CERTIFICATE")) {
		t.Error("expected certificate content in response")
	}
}

func TestGetCertificate_MissingFile(t *testing.T) {
	config.RootCA = "/nonexistent/rootCA.pem"

	req := httptest.NewRequest(http.MethodGet, "/certificate", nil)
	w := httptest.NewRecorder()
	GetCertificate(w, req)

	if w.Code == http.StatusOK {
		t.Fatal("expected non-200 for missing cert file")
	}
}

// --- AcceptNKey tests ---

func TestAcceptNKey_Success(t *testing.T) {
	dir := setupPKIDirs(t)
	writeSproutKey(t, dir, "unaccepted", "pending-sprout", "UKEY123")

	body, _ := json.Marshal(pki.KeyManager{SproutID: "pending-sprout"})
	req := httptest.NewRequest(http.MethodPost, "/nkey/accept", bytes.NewReader(body))
	w := httptest.NewRecorder()
	AcceptNKey(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp apitypes.Inline
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Success {
		t.Error("expected success=true")
	}
}

func TestAcceptNKey_NotFound(t *testing.T) {
	setupPKIDirs(t)

	body, _ := json.Marshal(pki.KeyManager{SproutID: "nonexistent"})
	req := httptest.NewRequest(http.MethodPost, "/nkey/accept", bytes.NewReader(body))
	w := httptest.NewRecorder()
	AcceptNKey(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestAcceptNKey_InvalidBody(t *testing.T) {
	setupPKIDirs(t)

	req := httptest.NewRequest(http.MethodPost, "/nkey/accept", bytes.NewReader([]byte("bad")))
	w := httptest.NewRecorder()
	AcceptNKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- GetNKey tests ---

func TestGetNKey_Success(t *testing.T) {
	dir := setupPKIDirs(t)
	writeSproutKey(t, dir, "accepted", "web-server", "UPUBKEY999")

	body, _ := json.Marshal(pki.KeyManager{SproutID: "web-server"})
	req := httptest.NewRequest(http.MethodPost, "/nkey/get", bytes.NewReader(body))
	w := httptest.NewRecorder()
	GetNKey(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp pki.KeySubmission
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.NKey != "UPUBKEY999" {
		t.Errorf("expected nkey UPUBKEY999, got %q", resp.NKey)
	}
	if resp.SproutID != "web-server" {
		t.Errorf("expected sprout ID web-server, got %q", resp.SproutID)
	}
}

func TestGetNKey_NotFound(t *testing.T) {
	setupPKIDirs(t)

	body, _ := json.Marshal(pki.KeyManager{SproutID: "missing"})
	req := httptest.NewRequest(http.MethodPost, "/nkey/get", bytes.NewReader(body))
	w := httptest.NewRecorder()
	GetNKey(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetNKey_InvalidID(t *testing.T) {
	setupPKIDirs(t)

	body, _ := json.Marshal(pki.KeyManager{SproutID: "INVALID"})
	req := httptest.NewRequest(http.MethodPost, "/nkey/get", bytes.NewReader(body))
	w := httptest.NewRecorder()
	GetNKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetNKey_InvalidBody(t *testing.T) {
	setupPKIDirs(t)

	req := httptest.NewRequest(http.MethodPost, "/nkey/get", bytes.NewReader([]byte("nope")))
	w := httptest.NewRecorder()
	GetNKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- RejectNKey tests ---

func TestRejectNKey_Success(t *testing.T) {
	dir := setupPKIDirs(t)
	writeSproutKey(t, dir, "accepted", "reject-me", "UKEY111")

	body, _ := json.Marshal(pki.KeyManager{SproutID: "reject-me"})
	req := httptest.NewRequest(http.MethodPost, "/nkey/reject", bytes.NewReader(body))
	w := httptest.NewRecorder()
	RejectNKey(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRejectNKey_InvalidBody(t *testing.T) {
	setupPKIDirs(t)

	req := httptest.NewRequest(http.MethodPost, "/nkey/reject", bytes.NewReader([]byte("nope")))
	w := httptest.NewRecorder()
	RejectNKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestRejectNKey_InvalidID(t *testing.T) {
	setupPKIDirs(t)

	body, _ := json.Marshal(pki.KeyManager{SproutID: "CAPS"})
	req := httptest.NewRequest(http.MethodPost, "/nkey/reject", bytes.NewReader(body))
	w := httptest.NewRecorder()
	RejectNKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestRejectNKey_NotFound(t *testing.T) {
	setupPKIDirs(t)

	body, _ := json.Marshal(pki.KeyManager{SproutID: "ghost"})
	req := httptest.NewRequest(http.MethodPost, "/nkey/reject", bytes.NewReader(body))
	w := httptest.NewRecorder()
	RejectNKey(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// --- ListNKey tests ---

func TestListNKey_Empty(t *testing.T) {
	setupPKIDirs(t)

	req := httptest.NewRequest(http.MethodGet, "/nkey", nil)
	w := httptest.NewRecorder()
	ListNKey(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp pki.KeysByType
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
}

func TestListNKey_WithSprouts(t *testing.T) {
	dir := setupPKIDirs(t)
	writeSproutKey(t, dir, "accepted", "sprout-a", "UKEYA")
	writeSproutKey(t, dir, "denied", "sprout-b", "UKEYB")
	writeSproutKey(t, dir, "unaccepted", "sprout-c", "UKEYC")

	req := httptest.NewRequest(http.MethodGet, "/nkey", nil)
	w := httptest.NewRecorder()
	ListNKey(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp pki.KeysByType
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(resp.Accepted.Sprouts) != 1 {
		t.Errorf("expected 1 accepted, got %d", len(resp.Accepted.Sprouts))
	}
	if len(resp.Denied.Sprouts) != 1 {
		t.Errorf("expected 1 denied, got %d", len(resp.Denied.Sprouts))
	}
	if len(resp.Unaccepted.Sprouts) != 1 {
		t.Errorf("expected 1 unaccepted, got %d", len(resp.Unaccepted.Sprouts))
	}
}

// --- DenyNKey tests ---

func TestDenyNKey_Success(t *testing.T) {
	dir := setupPKIDirs(t)
	writeSproutKey(t, dir, "unaccepted", "deny-me", "UKEYDENY")

	body, _ := json.Marshal(pki.KeyManager{SproutID: "deny-me"})
	req := httptest.NewRequest(http.MethodPost, "/nkey/deny", bytes.NewReader(body))
	w := httptest.NewRecorder()
	DenyNKey(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDenyNKey_InvalidBody(t *testing.T) {
	setupPKIDirs(t)

	req := httptest.NewRequest(http.MethodPost, "/nkey/deny", bytes.NewReader([]byte("nope")))
	w := httptest.NewRecorder()
	DenyNKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDenyNKey_NotFound(t *testing.T) {
	setupPKIDirs(t)

	body, _ := json.Marshal(pki.KeyManager{SproutID: "no-such-sprout"})
	req := httptest.NewRequest(http.MethodPost, "/nkey/deny", bytes.NewReader(body))
	w := httptest.NewRecorder()
	DenyNKey(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestDenyNKey_InvalidID(t *testing.T) {
	setupPKIDirs(t)

	body, _ := json.Marshal(pki.KeyManager{SproutID: "BAD!"})
	req := httptest.NewRequest(http.MethodPost, "/nkey/deny", bytes.NewReader(body))
	w := httptest.NewRecorder()
	DenyNKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- UnacceptNKey tests ---

func TestUnacceptNKey_Success(t *testing.T) {
	dir := setupPKIDirs(t)
	writeSproutKey(t, dir, "accepted", "unaccept-me", "UKEYUN")

	body, _ := json.Marshal(pki.KeyManager{SproutID: "unaccept-me"})
	req := httptest.NewRequest(http.MethodPost, "/nkey/unaccept", bytes.NewReader(body))
	w := httptest.NewRecorder()
	UnacceptNKey(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUnacceptNKey_InvalidBody(t *testing.T) {
	setupPKIDirs(t)

	req := httptest.NewRequest(http.MethodPost, "/nkey/unaccept", bytes.NewReader([]byte("bad")))
	w := httptest.NewRecorder()
	UnacceptNKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUnacceptNKey_NotFound(t *testing.T) {
	setupPKIDirs(t)

	body, _ := json.Marshal(pki.KeyManager{SproutID: "nope"})
	req := httptest.NewRequest(http.MethodPost, "/nkey/unaccept", bytes.NewReader(body))
	w := httptest.NewRecorder()
	UnacceptNKey(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestUnacceptNKey_InvalidID(t *testing.T) {
	setupPKIDirs(t)

	body, _ := json.Marshal(pki.KeyManager{SproutID: "NOPE"})
	req := httptest.NewRequest(http.MethodPost, "/nkey/unaccept", bytes.NewReader(body))
	w := httptest.NewRecorder()
	UnacceptNKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- DeleteNKey tests ---

func TestDeleteNKey_Success(t *testing.T) {
	dir := setupPKIDirs(t)
	writeSproutKey(t, dir, "accepted", "delete-me", "UKEYDEL")

	body, _ := json.Marshal(pki.KeyManager{SproutID: "delete-me"})
	req := httptest.NewRequest(http.MethodPost, "/nkey/delete", bytes.NewReader(body))
	w := httptest.NewRecorder()
	DeleteNKey(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteNKey_InvalidBody(t *testing.T) {
	setupPKIDirs(t)

	req := httptest.NewRequest(http.MethodPost, "/nkey/delete", bytes.NewReader([]byte("bad")))
	w := httptest.NewRecorder()
	DeleteNKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDeleteNKey_NotFound(t *testing.T) {
	setupPKIDirs(t)

	body, _ := json.Marshal(pki.KeyManager{SproutID: "gone"})
	req := httptest.NewRequest(http.MethodPost, "/nkey/delete", bytes.NewReader(body))
	w := httptest.NewRecorder()
	DeleteNKey(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestDeleteNKey_InvalidID(t *testing.T) {
	setupPKIDirs(t)

	body, _ := json.Marshal(pki.KeyManager{SproutID: "BAD!"})
	req := httptest.NewRequest(http.MethodPost, "/nkey/delete", bytes.NewReader(body))
	w := httptest.NewRecorder()
	DeleteNKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// createTestTLSCerts generates a self-signed CA and server cert/key in dir.
func createTestTLSCerts(t *testing.T, dir string) {
	t.Helper()

	// Generate CA key and cert
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create CA cert: %v", err)
	}

	caCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertDER})
	if err := os.WriteFile(filepath.Join(dir, "rootCA.pem"), caCertPEM, 0o644); err != nil {
		t.Fatalf("write rootCA: %v", err)
	}

	// Generate server key and cert signed by CA
	srvKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate server key: %v", err)
	}

	srvTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	srvCertDER, err := x509.CreateCertificate(rand.Reader, srvTemplate, caTemplate, &srvKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create server cert: %v", err)
	}

	srvCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srvCertDER})
	if err := os.WriteFile(filepath.Join(dir, "cert.pem"), srvCertPEM, 0o644); err != nil {
		t.Fatalf("write cert: %v", err)
	}

	srvKeyDER, err := x509.MarshalECPrivateKey(srvKey)
	if err != nil {
		t.Fatalf("marshal server key: %v", err)
	}
	srvKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: srvKeyDER})
	if err := os.WriteFile(filepath.Join(dir, "key.pem"), srvKeyPEM, 0o644); err != nil {
		t.Fatalf("write key: %v", err)
	}
}

// generateTestUserNKey creates a valid NATS user public key for testing.
func generateTestUserNKey(t *testing.T) string {
	t.Helper()
	kp, err := nkeys.CreateUser()
	if err != nil {
		t.Fatalf("failed to create test user nkey: %v", err)
	}
	pub, err := kp.PublicKey()
	if err != nil {
		t.Fatalf("failed to get public key: %v", err)
	}
	return pub
}
