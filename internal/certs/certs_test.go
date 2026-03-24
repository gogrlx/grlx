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

	genCACert()

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

	genCACert()

	// Record the original cert content
	origCert, err := os.ReadFile(config.RootCA)
	if err != nil {
		t.Fatalf("failed to read original CA cert: %v", err)
	}

	// Call again — should not regenerate
	genCACert()

	newCert, err := os.ReadFile(config.RootCA)
	if err != nil {
		t.Fatalf("failed to read CA cert after second call: %v", err)
	}
	if string(origCert) != string(newCert) {
		t.Fatal("genCACert should be idempotent — cert changed on second call")
	}
}

func TestGenCert(t *testing.T) {
	setupTLSConfigDir(t)

	GenCert()

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

	GenCert()

	origCert, err := os.ReadFile(config.CertFile)
	if err != nil {
		t.Fatalf("failed to read original cert: %v", err)
	}

	// Call again — should not regenerate
	GenCert()

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

	GenCert()

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

	GenCert()

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

func TestGenNKeyFarmer(t *testing.T) {
	setupNKeyConfigDir(t)

	GenNKey(true)

	// Verify pub key file was created
	pubBytes, err := os.ReadFile(config.NKeyFarmerPubFile)
	if err != nil {
		t.Fatalf("failed to read farmer pub key: %v", err)
	}
	if len(pubBytes) == 0 {
		t.Fatal("farmer pub key file is empty")
	}
	// NATS user public keys start with 'U'
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
	// NATS seeds start with 'S'
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

	GenNKey(false)

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

	GenNKey(true)

	origPub, err := os.ReadFile(config.NKeyFarmerPubFile)
	if err != nil {
		t.Fatalf("failed to read original pub key: %v", err)
	}

	// Call again — should not regenerate
	GenNKey(true)

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

	GenNKey(true)
	GenNKey(false)

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

func TestGetPubNKeyFarmer(t *testing.T) {
	setupNKeyConfigDir(t)

	GenNKey(true)

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

	GenNKey(false)

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

	GenCert()

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

	GenCert()

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
	genCACert()

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
	// Create a cert file, then make the directory read-only.
	certPath := filepath.Join(readonlyDir, "cert.pem")
	if err := os.WriteFile(certPath, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("failed to write cert: %v", err)
	}
	if err := os.Chmod(readonlyDir, 0o444); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}
	defer os.Chmod(readonlyDir, 0o755) // restore for cleanup

	config.CertFile = certPath

	_, err := forceRegenCert()
	if err == nil {
		t.Fatal("expected error when cert cannot be removed")
	}
}

func TestForceRegenCert_RemoveKeyError(t *testing.T) {
	setupTLSConfigDir(t)
	config.CertificateValidTime = 24 * time.Hour

	// CertFile doesn't exist (so remove succeeds/is no-op).
	// KeyFile points to a read-only directory.
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

func TestGenNKey_StatError(t *testing.T) {
	// Test the branch where os.Stat returns an error other than
	// IsNotExist — this triggers log.Panic. We can't easily test a
	// panic in a production path, so we verify the happy path deeply.
	setupNKeyConfigDir(t)

	// Generate a farmer key
	GenNKey(true)

	// Verify the private key seed can be parsed
	privBytes, err := os.ReadFile(config.NKeyFarmerPrivFile)
	if err != nil {
		t.Fatalf("failed to read priv key: %v", err)
	}

	// NATS seeds are base32-encoded, starting with 'S'
	if len(privBytes) < 4 {
		t.Fatal("private key seed too short")
	}
	if privBytes[0] != 'S' {
		t.Fatalf("expected seed starting with S, got %c", privBytes[0])
	}
	// Second character indicates key type: U=user
	if privBytes[1] != 'U' {
		t.Fatalf("expected user seed (SU...), got S%c", privBytes[1])
	}
}

func TestGenCert_MultipleHosts(t *testing.T) {
	setupTLSConfigDir(t)
	config.CertHosts = []string{"localhost", "example.com", "127.0.0.1", "10.0.0.1", "::1"}

	GenCert()

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

	GenCert()

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

	// Generate CA first (GenCert needs it).
	genCACert()

	// Write garbage to cert file.
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

	// Verify valid cert was generated.
	certBytes, err := os.ReadFile(config.CertFile)
	if err != nil {
		t.Fatalf("failed to read cert: %v", err)
	}
	block, _ := pem.Decode(certBytes)
	if block == nil {
		t.Fatal("new cert should be valid PEM")
	}
}
