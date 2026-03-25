package certs

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gogrlx/grlx/v2/internal/config"
)

// setupTLSConfigDir sets config globals to use a temp directory for TLS cert tests.
func setupTLSConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	config.RootCA = filepath.Join(dir, "rootCA.pem")
	config.RootCAPriv = filepath.Join(dir, "rootCA-key.pem")
	config.CertFile = filepath.Join(dir, "cert.pem")
	config.KeyFile = filepath.Join(dir, "key.pem")
	config.CertHosts = []string{"localhost", "127.0.0.1"}
	config.FarmerOrganization = "grlx-test"
	config.CertificateValidTime = 24 * time.Hour
	return dir
}

// setupNKeyConfigDir sets config globals to use a temp directory for NKey tests.
func setupNKeyConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	config.NKeyFarmerPrivFile = filepath.Join(dir, "farmer.nkey")
	config.NKeyFarmerPubFile = filepath.Join(dir, "farmer.pub")
	config.NKeySproutPrivFile = filepath.Join(dir, "sprout.nkey")
	config.NKeySproutPubFile = filepath.Join(dir, "sprout.pub")
	return dir
}

func TestPublicKeyRSA(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	pub := publicKey(key)
	if pub == nil {
		t.Fatal("publicKey returned nil for RSA key")
	}
	if _, ok := pub.(*rsa.PublicKey); !ok {
		t.Fatalf("expected *rsa.PublicKey, got %T", pub)
	}
}

func TestPublicKeyECDSA(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ECDSA key: %v", err)
	}
	pub := publicKey(key)
	if pub == nil {
		t.Fatal("publicKey returned nil for ECDSA key")
	}
	if _, ok := pub.(*ecdsa.PublicKey); !ok {
		t.Fatalf("expected *ecdsa.PublicKey, got %T", pub)
	}
}

func TestPublicKeyEd25519(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate Ed25519 key: %v", err)
	}
	pub := publicKey(priv)
	if pub == nil {
		t.Fatal("publicKey returned nil for Ed25519 key")
	}
	if _, ok := pub.(ed25519.PublicKey); !ok {
		t.Fatalf("expected ed25519.PublicKey, got %T", pub)
	}
}

func TestPublicKeyUnsupported(t *testing.T) {
	pub := publicKey("not a key")
	if pub != nil {
		t.Fatal("publicKey should return nil for unsupported type")
	}
}

func TestGenCACert(t *testing.T) {
	setupTLSConfigDir(t)

	if err := genCACert(); err != nil {
		t.Fatalf("genCACert failed: %v", err)
	}

	// Verify CA cert file was created
	certBytes, err := os.ReadFile(config.RootCA)
	if err != nil {
		t.Fatalf("failed to read CA cert: %v", err)
	}
	block, _ := pem.Decode(certBytes)
	if block == nil {
		t.Fatal("failed to decode CA cert PEM")
	}
	if block.Type != "CERTIFICATE" {
		t.Fatalf("expected CERTIFICATE PEM block, got %s", block.Type)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse CA certificate: %v", err)
	}
	if !cert.IsCA {
		t.Fatal("generated certificate should be a CA")
	}
	if cert.Subject.Organization[0] != "grlx-test" {
		t.Fatalf("expected org grlx-test, got %s", cert.Subject.Organization[0])
	}

	// Verify CA key file was created
	keyBytes, err := os.ReadFile(config.RootCAPriv)
	if err != nil {
		t.Fatalf("failed to read CA key: %v", err)
	}
	keyBlock, _ := pem.Decode(keyBytes)
	if keyBlock == nil {
		t.Fatal("failed to decode CA key PEM")
	}
	if keyBlock.Type != "PRIVATE KEY" {
		t.Fatalf("expected PRIVATE KEY PEM block, got %s", keyBlock.Type)
	}

	// Verify file permissions on private key
	info, err := os.Stat(config.RootCAPriv)
	if err != nil {
		t.Fatalf("failed to stat CA key file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected CA key permissions 0600, got %o", info.Mode().Perm())
	}
}

func TestGenCACertIdempotent(t *testing.T) {
	setupTLSConfigDir(t)

	if err := genCACert(); err != nil {
		t.Fatalf("genCACert failed: %v", err)
	}

	// Record the original cert content
	origCert, err := os.ReadFile(config.RootCA)
	if err != nil {
		t.Fatalf("failed to read original CA cert: %v", err)
	}

	// Call again — should not regenerate
	if err := genCACert(); err != nil {
		t.Fatalf("genCACert second call failed: %v", err)
	}

	newCert, err := os.ReadFile(config.RootCA)
	if err != nil {
		t.Fatalf("failed to read CA cert after second call: %v", err)
	}
	if string(origCert) != string(newCert) {
		t.Fatal("genCACert should be idempotent — cert changed on second call")
	}
}

func TestGenCACertUnwritableDir(t *testing.T) {
	dir := t.TempDir()
	readonlyDir := filepath.Join(dir, "readonly")
	if err := os.MkdirAll(readonlyDir, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	// Use 0o555 (r-x) so stat works (returns ENOENT) but create fails (no write).
	if err := os.Chmod(readonlyDir, 0o555); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}
	defer os.Chmod(readonlyDir, 0o755)

	config.RootCA = filepath.Join(readonlyDir, "rootCA.pem")
	config.RootCAPriv = filepath.Join(readonlyDir, "rootCA-key.pem")
	config.CertFile = filepath.Join(dir, "cert.pem")
	config.KeyFile = filepath.Join(dir, "key.pem")
	config.CertHosts = []string{"localhost"}
	config.FarmerOrganization = "grlx-test"
	config.CertificateValidTime = 24 * time.Hour

	err := genCACert()
	if err == nil {
		t.Fatal("genCACert should fail when directory is not writable")
	}
}

func TestGenCACertPrivKeyOnly(t *testing.T) {
	// Test the branch: RootCAPriv exists but RootCA does not → should regenerate.
	dir := setupTLSConfigDir(t)

	// Create only the private key file
	if err := os.WriteFile(config.RootCAPriv, []byte("existing-key"), 0o600); err != nil {
		t.Fatalf("failed to write priv key: %v", err)
	}

	if err := genCACert(); err != nil {
		t.Fatalf("genCACert should succeed when only priv key exists: %v", err)
	}

	// Both files should now exist
	if _, err := os.Stat(filepath.Join(dir, "rootCA.pem")); err != nil {
		t.Fatalf("rootCA.pem should exist: %v", err)
	}
}

func TestGenCert(t *testing.T) {
	setupTLSConfigDir(t)

	if err := GenCert(); err != nil {
		t.Fatalf("GenCert failed: %v", err)
	}

	// Verify server cert was created
	certBytes, err := os.ReadFile(config.CertFile)
	if err != nil {
		t.Fatalf("failed to read server cert: %v", err)
	}
	block, _ := pem.Decode(certBytes)
	if block == nil {
		t.Fatal("failed to decode server cert PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse server certificate: %v", err)
	}
	if cert.IsCA {
		t.Fatal("server certificate should not be a CA")
	}

	// Check SAN entries
	foundLocalhost := false
	for _, name := range cert.DNSNames {
		if name == "localhost" {
			foundLocalhost = true
		}
	}
	if !foundLocalhost {
		t.Fatal("server cert should have localhost in DNSNames")
	}

	foundIP := false
	for _, ip := range cert.IPAddresses {
		if ip.String() == "127.0.0.1" {
			foundIP = true
		}
	}
	if !foundIP {
		t.Fatal("server cert should have 127.0.0.1 in IPAddresses")
	}

	// Verify key was created
	keyBytes, err := os.ReadFile(config.KeyFile)
	if err != nil {
		t.Fatalf("failed to read server key: %v", err)
	}
	keyBlock, _ := pem.Decode(keyBytes)
	if keyBlock == nil {
		t.Fatal("failed to decode server key PEM")
	}
	if keyBlock.Type != "PRIVATE KEY" {
		t.Fatalf("expected PRIVATE KEY PEM block, got %s", keyBlock.Type)
	}

	// Verify CA signed it
	caBytes, err := os.ReadFile(config.RootCA)
	if err != nil {
		t.Fatalf("failed to read CA cert: %v", err)
	}
	caBlock, _ := pem.Decode(caBytes)
	caCert, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		t.Fatalf("failed to parse CA cert: %v", err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(caCert)
	_, err = cert.Verify(x509.VerifyOptions{
		Roots: roots,
	})
	if err != nil {
		t.Fatalf("server cert should verify against CA: %v", err)
	}
}

func TestGenCertIdempotent(t *testing.T) {
	setupTLSConfigDir(t)

	if err := GenCert(); err != nil {
		t.Fatalf("GenCert failed: %v", err)
	}

	origCert, err := os.ReadFile(config.CertFile)
	if err != nil {
		t.Fatalf("failed to read original cert: %v", err)
	}

	// Call again — should not regenerate
	if err := GenCert(); err != nil {
		t.Fatalf("GenCert second call failed: %v", err)
	}

	newCert, err := os.ReadFile(config.CertFile)
	if err != nil {
		t.Fatalf("failed to read cert after second call: %v", err)
	}
	if string(origCert) != string(newCert) {
		t.Fatal("GenCert should be idempotent — cert changed on second call")
	}
}

func TestGenCertDNSOnly(t *testing.T) {
	setupTLSConfigDir(t)
	config.CertHosts = []string{"example.com", "*.example.com"}

	if err := GenCert(); err != nil {
		t.Fatalf("GenCert failed: %v", err)
	}

	certBytes, err := os.ReadFile(config.CertFile)
	if err != nil {
		t.Fatalf("failed to read cert: %v", err)
	}
	block, _ := pem.Decode(certBytes)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse cert: %v", err)
	}
	if len(cert.IPAddresses) != 0 {
		t.Fatalf("expected no IP SANs for DNS-only hosts, got %v", cert.IPAddresses)
	}
	if len(cert.DNSNames) != 2 {
		t.Fatalf("expected 2 DNS SANs, got %d: %v", len(cert.DNSNames), cert.DNSNames)
	}
}

func TestGenCertIPOnly(t *testing.T) {
	setupTLSConfigDir(t)
	config.CertHosts = []string{"10.0.0.1", "::1"}

	if err := GenCert(); err != nil {
		t.Fatalf("GenCert failed: %v", err)
	}

	certBytes, err := os.ReadFile(config.CertFile)
	if err != nil {
		t.Fatalf("failed to read cert: %v", err)
	}
	block, _ := pem.Decode(certBytes)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse cert: %v", err)
	}
	if len(cert.DNSNames) != 0 {
		t.Fatalf("expected no DNS SANs for IP-only hosts, got %v", cert.DNSNames)
	}
	if len(cert.IPAddresses) != 2 {
		t.Fatalf("expected 2 IP SANs, got %d: %v", len(cert.IPAddresses), cert.IPAddresses)
	}
}

func TestGenCertUnwritableDir(t *testing.T) {
	dir := t.TempDir()
	readonlyDir := filepath.Join(dir, "readonly")
	if err := os.MkdirAll(readonlyDir, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	// Use 0o555 (r-x) so stat works but create fails.
	if err := os.Chmod(readonlyDir, 0o555); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}
	defer os.Chmod(readonlyDir, 0o755)

	config.RootCA = filepath.Join(readonlyDir, "rootCA.pem")
	config.RootCAPriv = filepath.Join(readonlyDir, "rootCA-key.pem")
	config.CertFile = filepath.Join(readonlyDir, "cert.pem")
	config.KeyFile = filepath.Join(readonlyDir, "key.pem")
	config.CertHosts = []string{"localhost"}
	config.FarmerOrganization = "grlx-test"
	config.CertificateValidTime = 24 * time.Hour

	err := GenCert()
	if err == nil {
		t.Fatal("GenCert should fail when directory is not writable")
	}
}

func TestGenCertCorruptCAPEM(t *testing.T) {
	setupTLSConfigDir(t)

	// Generate valid CA first
	if err := genCACert(); err != nil {
		t.Fatalf("genCACert failed: %v", err)
	}

	// Corrupt the CA cert
	if err := os.WriteFile(config.RootCA, []byte("not valid pem"), 0o644); err != nil {
		t.Fatalf("failed to corrupt CA cert: %v", err)
	}

	err := GenCert()
	if err == nil {
		t.Fatal("GenCert should fail when CA cert PEM is corrupt")
	}
}

func TestGenCertCorruptCAPrivKeyPEM(t *testing.T) {
	setupTLSConfigDir(t)

	// Generate valid CA first
	if err := genCACert(); err != nil {
		t.Fatalf("genCACert failed: %v", err)
	}

	// Corrupt the CA private key
	if err := os.WriteFile(config.RootCAPriv, []byte("not valid pem"), 0o600); err != nil {
		t.Fatalf("failed to corrupt CA priv key: %v", err)
	}

	err := GenCert()
	if err == nil {
		t.Fatal("GenCert should fail when CA private key PEM is corrupt")
	}
}

func TestGenCertCorruptCAPrivKeyDER(t *testing.T) {
	setupTLSConfigDir(t)

	// Generate valid CA first
	if err := genCACert(); err != nil {
		t.Fatalf("genCACert failed: %v", err)
	}

	// Write valid PEM wrapping invalid DER for the private key
	badPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: []byte("not valid DER"),
	})
	if err := os.WriteFile(config.RootCAPriv, badPEM, 0o600); err != nil {
		t.Fatalf("failed to write bad CA priv key: %v", err)
	}

	err := GenCert()
	if err == nil {
		t.Fatal("GenCert should fail when CA private key DER is invalid")
	}
}

func TestGenCertCorruptCADER(t *testing.T) {
	setupTLSConfigDir(t)

	// Generate valid CA first
	if err := genCACert(); err != nil {
		t.Fatalf("genCACert failed: %v", err)
	}

	// Write valid PEM wrapping invalid DER for the cert
	badPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: []byte("not valid DER"),
	})
	if err := os.WriteFile(config.RootCA, badPEM, 0o644); err != nil {
		t.Fatalf("failed to write bad CA cert: %v", err)
	}

	err := GenCert()
	if err == nil {
		t.Fatal("GenCert should fail when CA cert DER is invalid")
	}
}

func TestGenCertCertExistsKeyMissing(t *testing.T) {
	setupTLSConfigDir(t)

	// Generate certs normally
	if err := GenCert(); err != nil {
		t.Fatalf("GenCert failed: %v", err)
	}

	// Remove only the key file
	os.Remove(config.KeyFile)

	origCert, _ := os.ReadFile(config.CertFile)

	// Should regenerate since key is missing
	if err := GenCert(); err != nil {
		t.Fatalf("GenCert should succeed when key is missing: %v", err)
	}

	newCert, _ := os.ReadFile(config.CertFile)
	if string(origCert) == string(newCert) {
		t.Fatal("cert should be regenerated when key is missing")
	}
}

func TestGenCertKeyUnwritable(t *testing.T) {
	dir := setupTLSConfigDir(t)

	// Generate CA first
	if err := genCACert(); err != nil {
		t.Fatalf("genCACert failed: %v", err)
	}

	// Make the key file path point to a read-only directory
	readonlyDir := filepath.Join(dir, "readonly")
	if err := os.MkdirAll(readonlyDir, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	config.KeyFile = filepath.Join(readonlyDir, "key.pem")
	// CertFile stays writable
	if err := os.Chmod(readonlyDir, 0o444); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}
	defer os.Chmod(readonlyDir, 0o755)

	err := GenCert()
	if err == nil {
		t.Fatal("GenCert should fail when key file directory is not writable")
	}
}

func TestGenNKeyFarmer(t *testing.T) {
	setupNKeyConfigDir(t)

	if err := GenNKey(true); err != nil {
		t.Fatalf("GenNKey(true) failed: %v", err)
	}

	// Verify pub key file was created
	pubBytes, err := os.ReadFile(config.NKeyFarmerPubFile)
	if err != nil {
		t.Fatalf("failed to read farmer pub key: %v", err)
	}
	if len(pubBytes) == 0 {
		t.Fatal("farmer pub key file is empty")
	}
	if pubBytes[0] != 'U' {
		t.Fatalf("expected NATS user public key starting with 'U', got %c", pubBytes[0])
	}

	// Verify priv key file was created
	privBytes, err := os.ReadFile(config.NKeyFarmerPrivFile)
	if err != nil {
		t.Fatalf("failed to read farmer priv key: %v", err)
	}
	if len(privBytes) == 0 {
		t.Fatal("farmer priv key file is empty")
	}
	if privBytes[0] != 'S' {
		t.Fatalf("expected NATS seed starting with 'S', got %c", privBytes[0])
	}

	// Verify file permissions
	info, err := os.Stat(config.NKeyFarmerPrivFile)
	if err != nil {
		t.Fatalf("failed to stat farmer priv key: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected priv key permissions 0600, got %o", info.Mode().Perm())
	}
	info, err = os.Stat(config.NKeyFarmerPubFile)
	if err != nil {
		t.Fatalf("failed to stat farmer pub key: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected pub key permissions 0600, got %o", info.Mode().Perm())
	}
}

func TestGenNKeySprout(t *testing.T) {
	setupNKeyConfigDir(t)

	if err := GenNKey(false); err != nil {
		t.Fatalf("GenNKey(false) failed: %v", err)
	}

	pubBytes, err := os.ReadFile(config.NKeySproutPubFile)
	if err != nil {
		t.Fatalf("failed to read sprout pub key: %v", err)
	}
	if len(pubBytes) == 0 {
		t.Fatal("sprout pub key file is empty")
	}
	if pubBytes[0] != 'U' {
		t.Fatalf("expected NATS user public key starting with 'U', got %c", pubBytes[0])
	}

	privBytes, err := os.ReadFile(config.NKeySproutPrivFile)
	if err != nil {
		t.Fatalf("failed to read sprout priv key: %v", err)
	}
	if len(privBytes) == 0 {
		t.Fatal("sprout priv key file is empty")
	}
}

func TestGenNKeyIdempotent(t *testing.T) {
	setupNKeyConfigDir(t)

	if err := GenNKey(true); err != nil {
		t.Fatalf("GenNKey(true) failed: %v", err)
	}

	origPub, err := os.ReadFile(config.NKeyFarmerPubFile)
	if err != nil {
		t.Fatalf("failed to read original pub key: %v", err)
	}

	// Call again — should not regenerate
	if err := GenNKey(true); err != nil {
		t.Fatalf("GenNKey(true) second call failed: %v", err)
	}

	newPub, err := os.ReadFile(config.NKeyFarmerPubFile)
	if err != nil {
		t.Fatalf("failed to read pub key after second call: %v", err)
	}
	if string(origPub) != string(newPub) {
		t.Fatal("GenNKey should be idempotent — key changed on second call")
	}
}

func TestGenNKeyFarmerAndSproutDistinct(t *testing.T) {
	setupNKeyConfigDir(t)

	if err := GenNKey(true); err != nil {
		t.Fatalf("GenNKey(true) failed: %v", err)
	}
	if err := GenNKey(false); err != nil {
		t.Fatalf("GenNKey(false) failed: %v", err)
	}

	farmerPub, err := os.ReadFile(config.NKeyFarmerPubFile)
	if err != nil {
		t.Fatalf("failed to read farmer pub key: %v", err)
	}
	sproutPub, err := os.ReadFile(config.NKeySproutPubFile)
	if err != nil {
		t.Fatalf("failed to read sprout pub key: %v", err)
	}
	if string(farmerPub) == string(sproutPub) {
		t.Fatal("farmer and sprout NKeys should be distinct")
	}
}

func TestGenNKeyUnwritablePubDir(t *testing.T) {
	dir := t.TempDir()
	readonlyDir := filepath.Join(dir, "readonly")
	if err := os.MkdirAll(readonlyDir, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	// Use 0o555 so stat works but write fails.
	if err := os.Chmod(readonlyDir, 0o555); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}
	defer os.Chmod(readonlyDir, 0o755)

	config.NKeyFarmerPrivFile = filepath.Join(readonlyDir, "farmer.nkey")
	config.NKeyFarmerPubFile = filepath.Join(readonlyDir, "farmer.pub")

	err := GenNKey(true)
	if err == nil {
		t.Fatal("GenNKey should fail when pub key directory is not writable")
	}
}

func TestGenNKeyUnwritablePrivDir(t *testing.T) {
	dir := t.TempDir()
	writableDir := t.TempDir()
	readonlyDir := filepath.Join(dir, "readonly")
	if err := os.MkdirAll(readonlyDir, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	// Pub goes to writable dir, priv to read-only dir
	config.NKeyFarmerPubFile = filepath.Join(writableDir, "farmer.pub")
	config.NKeyFarmerPrivFile = filepath.Join(readonlyDir, "farmer.nkey")

	if err := os.Chmod(readonlyDir, 0o555); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}
	defer os.Chmod(readonlyDir, 0o755)

	err := GenNKey(true)
	if err == nil {
		t.Fatal("GenNKey should fail when priv key directory is not writable")
	}
}

func TestGetPubNKeyFarmer(t *testing.T) {
	setupNKeyConfigDir(t)

	if err := GenNKey(true); err != nil {
		t.Fatalf("GenNKey(true) failed: %v", err)
	}

	pubKey, err := GetPubNKey(true)
	if err != nil {
		t.Fatalf("GetPubNKey(true) failed: %v", err)
	}
	if len(pubKey) == 0 {
		t.Fatal("GetPubNKey returned empty string")
	}
	if pubKey[0] != 'U' {
		t.Fatalf("expected NATS user public key starting with 'U', got %c", pubKey[0])
	}
}

func TestGetPubNKeySprout(t *testing.T) {
	setupNKeyConfigDir(t)

	if err := GenNKey(false); err != nil {
		t.Fatalf("GenNKey(false) failed: %v", err)
	}

	pubKey, err := GetPubNKey(false)
	if err != nil {
		t.Fatalf("GetPubNKey(false) failed: %v", err)
	}
	if len(pubKey) == 0 {
		t.Fatal("GetPubNKey returned empty string")
	}
}

func TestGetPubNKeyMissing(t *testing.T) {
	setupNKeyConfigDir(t)

	_, err := GetPubNKey(true)
	if err == nil {
		t.Fatal("GetPubNKey should fail when key file doesn't exist")
	}
}

func TestRotateTLSCertsNoRotationNeeded(t *testing.T) {
	setupTLSConfigDir(t)
	config.CertificateValidTime = 24 * time.Hour

	if err := GenCert(); err != nil {
		t.Fatalf("GenCert failed: %v", err)
	}

	origCert, err := os.ReadFile(config.CertFile)
	if err != nil {
		t.Fatalf("failed to read cert: %v", err)
	}

	// Threshold is 23 hours — cert valid for 24h, so no rotation needed.
	rotated, err := RotateTLSCerts(23 * time.Hour)
	if err != nil {
		t.Fatalf("RotateTLSCerts error: %v", err)
	}
	if rotated {
		t.Fatal("expected no rotation when cert is still valid")
	}

	newCert, err := os.ReadFile(config.CertFile)
	if err != nil {
		t.Fatalf("failed to read cert after rotation check: %v", err)
	}
	if string(origCert) != string(newCert) {
		t.Fatal("cert should not have changed")
	}
}

func TestRotateTLSCertsRotationNeeded(t *testing.T) {
	setupTLSConfigDir(t)
	// Create a cert that's only valid for 1 hour.
	config.CertificateValidTime = 1 * time.Hour

	if err := GenCert(); err != nil {
		t.Fatalf("GenCert failed: %v", err)
	}

	origCert, err := os.ReadFile(config.CertFile)
	if err != nil {
		t.Fatalf("failed to read cert: %v", err)
	}

	// Reset validity for the new cert.
	config.CertificateValidTime = 24 * time.Hour

	// Threshold is 2 hours — cert expires in 1h, so rotation IS needed.
	rotated, err := RotateTLSCerts(2 * time.Hour)
	if err != nil {
		t.Fatalf("RotateTLSCerts error: %v", err)
	}
	if !rotated {
		t.Fatal("expected rotation when cert is about to expire")
	}

	newCert, err := os.ReadFile(config.CertFile)
	if err != nil {
		t.Fatalf("failed to read cert after rotation: %v", err)
	}
	if string(origCert) == string(newCert) {
		t.Fatal("cert should have changed after rotation")
	}

	// Verify new cert is valid for 24h.
	block, _ := pem.Decode(newCert)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse new cert: %v", err)
	}
	remaining := time.Until(cert.NotAfter)
	if remaining < 23*time.Hour {
		t.Fatalf("new cert should be valid for ~24h, got %v", remaining)
	}
}

func TestRotateTLSCertsMissingCert(t *testing.T) {
	setupTLSConfigDir(t)
	config.CertificateValidTime = 24 * time.Hour

	// No cert exists — should generate one.
	rotated, err := RotateTLSCerts(1 * time.Hour)
	if err != nil {
		t.Fatalf("RotateTLSCerts error: %v", err)
	}
	if !rotated {
		t.Fatal("expected rotation when cert doesn't exist")
	}

	// Verify cert was created.
	if _, err := os.Stat(config.CertFile); err != nil {
		t.Fatalf("cert file should exist after rotation: %v", err)
	}
}

func TestRotateTLSCertsInvalidDER(t *testing.T) {
	setupTLSConfigDir(t)
	config.CertificateValidTime = 24 * time.Hour

	// Generate CA first.
	if err := genCACert(); err != nil {
		t.Fatalf("genCACert failed: %v", err)
	}

	// Write valid PEM but with invalid DER content inside.
	badPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: []byte("this is not valid DER"),
	})
	if err := os.WriteFile(config.CertFile, badPEM, 0o644); err != nil {
		t.Fatalf("failed to write bad cert: %v", err)
	}

	rotated, err := RotateTLSCerts(1 * time.Hour)
	if err != nil {
		t.Fatalf("RotateTLSCerts error: %v", err)
	}
	if !rotated {
		t.Fatal("expected rotation when cert DER is invalid")
	}

	// Verify valid cert was generated.
	certBytes, err := os.ReadFile(config.CertFile)
	if err != nil {
		t.Fatalf("failed to read cert: %v", err)
	}
	block, _ := pem.Decode(certBytes)
	if block == nil {
		t.Fatal("new cert should be valid PEM")
	}
	_, err = x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("new cert should be parseable: %v", err)
	}
}

func TestForceRegenCert_RemoveCertError(t *testing.T) {
	setupTLSConfigDir(t)
	config.CertificateValidTime = 24 * time.Hour

	// Point CertFile to a path inside a non-writable directory.
	readonlyDir := filepath.Join(t.TempDir(), "readonly")
	if err := os.MkdirAll(readonlyDir, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	certPath := filepath.Join(readonlyDir, "cert.pem")
	if err := os.WriteFile(certPath, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("failed to write cert: %v", err)
	}
	if err := os.Chmod(readonlyDir, 0o444); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}
	defer os.Chmod(readonlyDir, 0o755)

	config.CertFile = certPath

	_, err := forceRegenCert()
	if err == nil {
		t.Fatal("expected error when cert cannot be removed")
	}
}

func TestForceRegenCert_RemoveKeyError(t *testing.T) {
	setupTLSConfigDir(t)
	config.CertificateValidTime = 24 * time.Hour

	readonlyDir := filepath.Join(t.TempDir(), "readonly")
	if err := os.MkdirAll(readonlyDir, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	keyPath := filepath.Join(readonlyDir, "key.pem")
	if err := os.WriteFile(keyPath, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("failed to write key: %v", err)
	}
	if err := os.Chmod(readonlyDir, 0o444); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}
	defer os.Chmod(readonlyDir, 0o755)

	config.KeyFile = keyPath

	_, err := forceRegenCert()
	if err == nil {
		t.Fatal("expected error when key cannot be removed")
	}
}

func TestGenNKeySeedData(t *testing.T) {
	setupNKeyConfigDir(t)

	if err := GenNKey(true); err != nil {
		t.Fatalf("GenNKey(true) failed: %v", err)
	}

	privBytes, err := os.ReadFile(config.NKeyFarmerPrivFile)
	if err != nil {
		t.Fatalf("failed to read priv key: %v", err)
	}
	if len(privBytes) < 4 {
		t.Fatal("private key seed too short")
	}
	if privBytes[0] != 'S' {
		t.Fatalf("expected seed starting with S, got %c", privBytes[0])
	}
	if privBytes[1] != 'U' {
		t.Fatalf("expected user seed (SU...), got S%c", privBytes[1])
	}
}

func TestGenCert_MultipleHosts(t *testing.T) {
	setupTLSConfigDir(t)
	config.CertHosts = []string{"localhost", "example.com", "127.0.0.1", "10.0.0.1", "::1"}

	if err := GenCert(); err != nil {
		t.Fatalf("GenCert failed: %v", err)
	}

	certBytes, err := os.ReadFile(config.CertFile)
	if err != nil {
		t.Fatalf("failed to read cert: %v", err)
	}
	block, _ := pem.Decode(certBytes)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse cert: %v", err)
	}
	if len(cert.DNSNames) != 2 {
		t.Fatalf("expected 2 DNS SANs, got %d: %v", len(cert.DNSNames), cert.DNSNames)
	}
	if len(cert.IPAddresses) != 3 {
		t.Fatalf("expected 3 IP SANs, got %d: %v", len(cert.IPAddresses), cert.IPAddresses)
	}
}

func TestGenCert_EmptyHosts(t *testing.T) {
	setupTLSConfigDir(t)
	config.CertHosts = []string{}

	if err := GenCert(); err != nil {
		t.Fatalf("GenCert failed: %v", err)
	}

	certBytes, err := os.ReadFile(config.CertFile)
	if err != nil {
		t.Fatalf("failed to read cert: %v", err)
	}
	block, _ := pem.Decode(certBytes)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse cert: %v", err)
	}
	if len(cert.DNSNames) != 0 {
		t.Fatalf("expected no DNS SANs, got %v", cert.DNSNames)
	}
	if len(cert.IPAddresses) != 0 {
		t.Fatalf("expected no IP SANs, got %v", cert.IPAddresses)
	}
}

func TestRotateTLSCertsCorruptPEM(t *testing.T) {
	setupTLSConfigDir(t)
	config.CertificateValidTime = 24 * time.Hour

	if err := genCACert(); err != nil {
		t.Fatalf("genCACert failed: %v", err)
	}

	if err := os.WriteFile(config.CertFile, []byte("not a pem"), 0o644); err != nil {
		t.Fatalf("failed to write corrupt cert: %v", err)
	}

	rotated, err := RotateTLSCerts(1 * time.Hour)
	if err != nil {
		t.Fatalf("RotateTLSCerts error: %v", err)
	}
	if !rotated {
		t.Fatal("expected rotation when cert PEM is corrupt")
	}

	certBytes, err := os.ReadFile(config.CertFile)
	if err != nil {
		t.Fatalf("failed to read cert: %v", err)
	}
	block, _ := pem.Decode(certBytes)
	if block == nil {
		t.Fatal("new cert should be valid PEM")
	}
}

func TestGenCertMissingCAAfterGen(t *testing.T) {
	setupTLSConfigDir(t)

	// Generate CA normally
	if err := genCACert(); err != nil {
		t.Fatalf("genCACert failed: %v", err)
	}

	// Delete the CA cert file (simulate corruption/removal)
	os.Remove(config.RootCA)

	// GenCert will call genCACert, which sees RootCAPriv exists but RootCA
	// doesn't — it regenerates. This should succeed.
	err := GenCert()
	if err != nil {
		t.Fatalf("GenCert should recover when CA cert is missing: %v", err)
	}
}

func TestGenCertMissingCAPrivAfterGen(t *testing.T) {
	setupTLSConfigDir(t)

	// Generate CA normally
	if err := genCACert(); err != nil {
		t.Fatalf("genCACert failed: %v", err)
	}

	// Delete the CA private key
	os.Remove(config.RootCAPriv)

	// genCACert will see RootCAPriv doesn't exist, regenerate everything
	err := GenCert()
	if err != nil {
		t.Fatalf("GenCert should recover when CA priv key is missing: %v", err)
	}
}

func TestGenCertCertDirUnwritable(t *testing.T) {
	dir := setupTLSConfigDir(t)

	// Generate CA first
	if err := genCACert(); err != nil {
		t.Fatalf("genCACert failed: %v", err)
	}

	// Make a subdirectory for the cert file that's not writable
	readonlyDir := filepath.Join(dir, "certdir")
	if err := os.MkdirAll(readonlyDir, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	config.CertFile = filepath.Join(readonlyDir, "cert.pem")
	if err := os.Chmod(readonlyDir, 0o555); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}
	defer os.Chmod(readonlyDir, 0o755)

	err := GenCert()
	if err == nil {
		t.Fatal("GenCert should fail when cert directory is not writable")
	}
}

func TestGenCACertKeyDirUnwritable(t *testing.T) {
	dir := t.TempDir()
	writableDir := t.TempDir()
	readonlyDir := filepath.Join(dir, "keydir")
	if err := os.MkdirAll(readonlyDir, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	// CA cert can be written, but CA key cannot
	config.RootCA = filepath.Join(writableDir, "rootCA.pem")
	config.RootCAPriv = filepath.Join(readonlyDir, "rootCA-key.pem")
	config.CertFile = filepath.Join(writableDir, "cert.pem")
	config.KeyFile = filepath.Join(writableDir, "key.pem")
	config.CertHosts = []string{"localhost"}
	config.FarmerOrganization = "grlx-test"
	config.CertificateValidTime = 24 * time.Hour

	if err := os.Chmod(readonlyDir, 0o555); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}
	defer os.Chmod(readonlyDir, 0o755)

	err := genCACert()
	if err == nil {
		t.Fatal("genCACert should fail when key directory is not writable")
	}
}

func TestGenCertCAUnreadable(t *testing.T) {
	setupTLSConfigDir(t)

	// Generate CA normally
	if err := genCACert(); err != nil {
		t.Fatalf("genCACert failed: %v", err)
	}

	// Make CA cert unreadable — genCACert will see it exists and return,
	// but GenCert will fail when trying to read it.
	if err := os.Chmod(config.RootCA, 0o000); err != nil {
		t.Fatalf("failed to chmod CA cert: %v", err)
	}
	defer os.Chmod(config.RootCA, 0o644)

	err := GenCert()
	if err == nil {
		t.Fatal("GenCert should fail when CA cert is unreadable")
	}
}

func TestGenCertCAPrivKeyUnreadable(t *testing.T) {
	setupTLSConfigDir(t)

	// Generate CA normally
	if err := genCACert(); err != nil {
		t.Fatalf("genCACert failed: %v", err)
	}

	// Make CA private key unreadable
	if err := os.Chmod(config.RootCAPriv, 0o000); err != nil {
		t.Fatalf("failed to chmod CA key: %v", err)
	}
	defer os.Chmod(config.RootCAPriv, 0o600)

	err := GenCert()
	if err == nil {
		t.Fatal("GenCert should fail when CA private key is unreadable")
	}
}

func TestRotateTLSCertsReadError(t *testing.T) {
	setupTLSConfigDir(t)
	config.CertificateValidTime = 24 * time.Hour

	// Make CertFile a directory instead of a file — causes a read error
	// that isn't IsNotExist.
	if err := os.MkdirAll(config.CertFile, 0o755); err != nil {
		t.Fatalf("failed to create dir as CertFile: %v", err)
	}

	_, err := RotateTLSCerts(1 * time.Hour)
	if err == nil {
		t.Fatal("expected error when cert file cannot be read")
	}
}
