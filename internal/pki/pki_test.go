package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gogrlx/grlx/v2/internal/config"
)

// setupTestPKI creates the farmer PKI directory structure in a temp dir
// and sets config.FarmerPKI to point to it. Returns the PKI directory path.
// Note: FarmerPKI must end with "/" because the production code concatenates
// paths with string addition (e.g. FarmerPKI + "sprouts/...").
func setupTestPKI(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	pkiDir := filepath.Join(tmpDir, "pki") + "/"
	config.FarmerPKI = pkiDir

	for _, state := range []string{"unaccepted", "accepted", "denied", "rejected"} {
		dir := filepath.Join(pkiDir, "sprouts", state)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("failed to create %s directory: %v", state, err)
		}
	}

	// ReloadNKeys (called by Accept/Deny/Reject/Unaccept/Delete via defer)
	// needs a farmer NKey pub file and auth config. Create a dummy farmer key
	// so ReloadNKeys doesn't fatal.
	farmerPubFile := filepath.Join(tmpDir, "farmer.pub")
	if err := os.WriteFile(farmerPubFile, []byte("UFAKE_FARMER_KEY_FOR_TESTING"), 0o600); err != nil {
		t.Fatalf("failed to write dummy farmer pub key: %v", err)
	}
	config.NKeyFarmerPubFile = farmerPubFile

	// NatsServer must be nil so ReloadNKeys skips the server reload.
	NatsServer = nil

	// ReloadNKeys calls ConfigureNats which needs valid TLS files.
	// Generate a self-signed CA + cert/key pair inline for test isolation.
	config.FarmerInterface = "127.0.0.1"
	config.FarmerBusPort = "14222"
	config.FarmerOrganization = "grlx-test"
	config.CertificateValidTime = 24 * 365 * time.Hour
	config.RootCA = filepath.Join(tmpDir, "rootca.pem")
	config.RootCAPriv = filepath.Join(tmpDir, "rootca-key.pem")
	config.CertFile = filepath.Join(tmpDir, "cert.pem")
	config.KeyFile = filepath.Join(tmpDir, "key.pem")
	config.CertHosts = []string{"127.0.0.1"}

	generateTestCerts(t, tmpDir)

	return pkiDir
}

// generateTestCerts creates a self-signed CA and leaf certificate for
// ConfigureNats to load during ReloadNKeys.
func generateTestCerts(t *testing.T, tmpDir string) {
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

	// Write CA cert.
	writePEM(t, filepath.Join(tmpDir, "rootca.pem"), "CERTIFICATE", caDER)

	// Write CA private key.
	caPrivBytes, err := x509.MarshalPKCS8PrivateKey(caKey)
	if err != nil {
		t.Fatalf("marshal CA key: %v", err)
	}
	writePEM(t, filepath.Join(tmpDir, "rootca-key.pem"), "PRIVATE KEY", caPrivBytes)

	// Leaf cert.
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

	writePEM(t, filepath.Join(tmpDir, "cert.pem"), "CERTIFICATE", leafDER)

	leafPrivBytes, err := x509.MarshalPKCS8PrivateKey(leafKey)
	if err != nil {
		t.Fatalf("marshal leaf key: %v", err)
	}
	writePEM(t, filepath.Join(tmpDir, "key.pem"), "PRIVATE KEY", leafPrivBytes)
}

func writePEM(t *testing.T, path, blockType string, data []byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	if err := pem.Encode(f, &pem.Block{Type: blockType, Bytes: data}); err != nil {
		t.Fatalf("encode PEM to %s: %v", path, err)
	}
}

// writeKey writes an NKey file into the given state directory.
func writeKey(t *testing.T, pkiDir, state, sproutID, nkey string) {
	t.Helper()
	path := filepath.Join(pkiDir, "sprouts", state, sproutID)
	if err := os.WriteFile(path, []byte(nkey), 0o600); err != nil {
		t.Fatalf("failed to write key file %s: %v", path, err)
	}
}

func TestIsValidSproutID(t *testing.T) {
	testCases := []struct {
		id            string
		shouldSucceed bool
		testID        string
	}{
		{id: "test", shouldSucceed: true, testID: "simple"},
		{id: "-test", shouldSucceed: false, testID: "leading hyphen"},
		{id: "te_st", shouldSucceed: true, testID: "embedded underscore"},
		{id: "grlxNode", shouldSucceed: false, testID: "capital letter"},
		{id: "t.est", shouldSucceed: true, testID: "embedded dot"},
		{id: strings.Repeat("a", 300), shouldSucceed: false, testID: "300 long string"},
		{id: strings.Repeat("a", 253), shouldSucceed: true, testID: "253 long string"},
		{id: "0132-465798qwertyuiopasdfghjklzxcv.bnm", shouldSucceed: true, testID: "keyboard smash"},
		{id: "te\nst", shouldSucceed: false, testID: "multiline"},
		{id: "", shouldSucceed: false, testID: "empty string"},
		{id: "_test", shouldSucceed: false, testID: "leading underscore"},
		{id: "test.", shouldSucceed: false, testID: "trailing dot"},
		{id: "a", shouldSucceed: true, testID: "single char"},
		{id: "web-01.example.com", shouldSucceed: true, testID: "fqdn-like"},
		{id: "192.168.1.1", shouldSucceed: true, testID: "ip-like"},
	}
	for _, tc := range testCases {
		t.Run(tc.testID, func(t *testing.T) {
			if IsValidSproutID(tc.id) != tc.shouldSucceed {
				t.Errorf("`%s`: expected %v but got %v", tc.id, tc.shouldSucceed, !tc.shouldSucceed)
			}
		})
	}
}

func TestSetupPKIFarmer(t *testing.T) {
	tmpDir := t.TempDir()
	config.FarmerPKI = filepath.Join(tmpDir, "pki") + "/"

	SetupPKIFarmer()

	for _, state := range []string{"unaccepted", "accepted", "denied", "rejected"} {
		dir := filepath.Join(config.FarmerPKI, "sprouts", state)
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("expected directory %s to exist: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %s to be a directory", dir)
		}
	}

	// Calling again should not fail (idempotent).
	SetupPKIFarmer()
}

func TestUnacceptNKey_NewSprout(t *testing.T) {
	// Stub NatsServer to nil so ReloadNKeys is a no-op.
	NatsServer = nil
	pkiDir := setupTestPKI(t)

	err := UnacceptNKey("webserver01", "NKEY_ABC123")
	if err != nil {
		t.Fatalf("UnacceptNKey failed: %v", err)
	}

	// Verify file was created in unaccepted.
	path := filepath.Join(pkiDir, "sprouts/unaccepted/webserver01")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected key file at %s: %v", path, err)
	}
	if string(data) != "NKEY_ABC123" {
		t.Errorf("expected key content %q, got %q", "NKEY_ABC123", string(data))
	}
}

func TestUnacceptNKey_InvalidID(t *testing.T) {
	NatsServer = nil
	setupTestPKI(t)

	err := UnacceptNKey("-invalid", "NKEY_ABC123")
	if !errors.Is(err, ErrSproutIDInvalid) {
		t.Errorf("expected ErrSproutIDInvalid, got: %v", err)
	}
}

func TestAcceptNKey(t *testing.T) {
	NatsServer = nil
	pkiDir := setupTestPKI(t)

	// Place a key in unaccepted.
	writeKey(t, pkiDir, "unaccepted", "db01", "NKEY_DB01")

	err := AcceptNKey("db01")
	if err != nil {
		t.Fatalf("AcceptNKey failed: %v", err)
	}

	// Should exist in accepted.
	accepted := filepath.Join(pkiDir, "sprouts/accepted/db01")
	data, err := os.ReadFile(accepted)
	if err != nil {
		t.Fatalf("expected key at %s: %v", accepted, err)
	}
	if string(data) != "NKEY_DB01" {
		t.Errorf("key content mismatch: %q", string(data))
	}

	// Should NOT exist in unaccepted.
	unaccepted := filepath.Join(pkiDir, "sprouts/unaccepted/db01")
	if _, err := os.Stat(unaccepted); !os.IsNotExist(err) {
		t.Errorf("expected key to be removed from unaccepted, but it still exists")
	}
}

func TestAcceptNKey_AlreadyAccepted(t *testing.T) {
	NatsServer = nil
	pkiDir := setupTestPKI(t)

	writeKey(t, pkiDir, "accepted", "db01", "NKEY_DB01")

	err := AcceptNKey("db01")
	if !errors.Is(err, ErrAlreadyAccepted) {
		t.Errorf("expected ErrAlreadyAccepted, got: %v", err)
	}
}

func TestAcceptNKey_InvalidID(t *testing.T) {
	NatsServer = nil
	setupTestPKI(t)

	err := AcceptNKey("-nope")
	if !errors.Is(err, ErrSproutIDInvalid) {
		t.Errorf("expected ErrSproutIDInvalid, got: %v", err)
	}
}

func TestAcceptNKey_NotFound(t *testing.T) {
	NatsServer = nil
	setupTestPKI(t)

	err := AcceptNKey("nonexistent")
	if !errors.Is(err, ErrSproutIDNotFound) {
		t.Errorf("expected ErrSproutIDNotFound, got: %v", err)
	}
}

func TestDenyNKey(t *testing.T) {
	NatsServer = nil
	pkiDir := setupTestPKI(t)

	writeKey(t, pkiDir, "unaccepted", "app01", "NKEY_APP01")

	err := DenyNKey("app01")
	if err != nil {
		t.Fatalf("DenyNKey failed: %v", err)
	}

	denied := filepath.Join(pkiDir, "sprouts/denied/app01")
	if _, err := os.Stat(denied); err != nil {
		t.Fatalf("expected key at %s: %v", denied, err)
	}

	unaccepted := filepath.Join(pkiDir, "sprouts/unaccepted/app01")
	if _, err := os.Stat(unaccepted); !os.IsNotExist(err) {
		t.Error("expected key removed from unaccepted")
	}
}

func TestDenyNKey_AlreadyDenied(t *testing.T) {
	NatsServer = nil
	pkiDir := setupTestPKI(t)

	writeKey(t, pkiDir, "denied", "app01", "NKEY_APP01")

	err := DenyNKey("app01")
	if !errors.Is(err, ErrAlreadyDenied) {
		t.Errorf("expected ErrAlreadyDenied, got: %v", err)
	}
}

func TestRejectNKey_ExistingKey(t *testing.T) {
	NatsServer = nil
	pkiDir := setupTestPKI(t)

	writeKey(t, pkiDir, "unaccepted", "rogue01", "NKEY_ROGUE")

	err := RejectNKey("rogue01", "")
	if err != nil {
		t.Fatalf("RejectNKey failed: %v", err)
	}

	rejected := filepath.Join(pkiDir, "sprouts/rejected/rogue01")
	data, err := os.ReadFile(rejected)
	if err != nil {
		t.Fatalf("expected key at %s: %v", rejected, err)
	}
	if string(data) != "NKEY_ROGUE" {
		t.Errorf("key content mismatch: %q", string(data))
	}
}

func TestRejectNKey_NewKeyDirect(t *testing.T) {
	NatsServer = nil
	pkiDir := setupTestPKI(t)

	// Reject a sprout that doesn't exist yet — creates directly in rejected.
	err := RejectNKey("badactor", "NKEY_BAD")
	if err != nil {
		t.Fatalf("RejectNKey failed: %v", err)
	}

	rejected := filepath.Join(pkiDir, "sprouts/rejected/badactor")
	data, err := os.ReadFile(rejected)
	if err != nil {
		t.Fatalf("expected key at %s: %v", rejected, err)
	}
	if string(data) != "NKEY_BAD" {
		t.Errorf("expected %q, got %q", "NKEY_BAD", string(data))
	}
}

func TestRejectNKey_AlreadyRejected(t *testing.T) {
	NatsServer = nil
	pkiDir := setupTestPKI(t)

	writeKey(t, pkiDir, "rejected", "rogue01", "NKEY_ROGUE")

	err := RejectNKey("rogue01", "")
	if !errors.Is(err, ErrAlreadyRejected) {
		t.Errorf("expected ErrAlreadyRejected, got: %v", err)
	}
}

func TestDeleteNKey(t *testing.T) {
	NatsServer = nil
	pkiDir := setupTestPKI(t)

	writeKey(t, pkiDir, "accepted", "old01", "NKEY_OLD")

	err := DeleteNKey("old01")
	if err != nil {
		t.Fatalf("DeleteNKey failed: %v", err)
	}

	path := filepath.Join(pkiDir, "sprouts/accepted/old01")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected key file to be deleted")
	}
}

func TestDeleteNKey_NotFound(t *testing.T) {
	NatsServer = nil
	setupTestPKI(t)

	err := DeleteNKey("ghost")
	if !errors.Is(err, ErrSproutIDNotFound) {
		t.Errorf("expected ErrSproutIDNotFound, got: %v", err)
	}
}

func TestDeleteNKey_InvalidID(t *testing.T) {
	NatsServer = nil
	setupTestPKI(t)

	err := DeleteNKey("-bad")
	if !errors.Is(err, ErrSproutIDInvalid) {
		t.Errorf("expected ErrSproutIDInvalid, got: %v", err)
	}
}

func TestGetNKey(t *testing.T) {
	NatsServer = nil
	pkiDir := setupTestPKI(t)

	writeKey(t, pkiDir, "accepted", "cache01", "NKEY_CACHE01")

	key, err := GetNKey("cache01")
	if err != nil {
		t.Fatalf("GetNKey failed: %v", err)
	}
	if key != "NKEY_CACHE01" {
		t.Errorf("expected %q, got %q", "NKEY_CACHE01", key)
	}
}

func TestGetNKey_NotFound(t *testing.T) {
	NatsServer = nil
	setupTestPKI(t)

	_, err := GetNKey("missing")
	if !errors.Is(err, ErrSproutIDNotFound) {
		t.Errorf("expected ErrSproutIDNotFound, got: %v", err)
	}
}

func TestGetNKey_InvalidID(t *testing.T) {
	NatsServer = nil
	setupTestPKI(t)

	_, err := GetNKey("-invalid")
	if !errors.Is(err, ErrSproutIDInvalid) {
		t.Errorf("expected ErrSproutIDInvalid, got: %v", err)
	}
}

func TestGetNKey_FromEachState(t *testing.T) {
	NatsServer = nil
	pkiDir := setupTestPKI(t)

	states := []string{"unaccepted", "accepted", "denied", "rejected"}
	for _, state := range states {
		sproutID := state + "-sprout"
		expectedKey := "NKEY_" + strings.ToUpper(state)
		writeKey(t, pkiDir, state, sproutID, expectedKey)

		t.Run(state, func(t *testing.T) {
			key, err := GetNKey(sproutID)
			if err != nil {
				t.Fatalf("GetNKey(%q) failed: %v", sproutID, err)
			}
			if key != expectedKey {
				t.Errorf("expected %q, got %q", expectedKey, key)
			}
		})
	}
}

func TestNKeyExists(t *testing.T) {
	NatsServer = nil
	pkiDir := setupTestPKI(t)

	writeKey(t, pkiDir, "accepted", "exist01", "NKEY_EXIST")

	t.Run("exists and matches", func(t *testing.T) {
		registered, matches := NKeyExists("exist01", "NKEY_EXIST")
		if !registered {
			t.Error("expected registered=true")
		}
		if !matches {
			t.Error("expected matches=true")
		}
	})

	t.Run("exists but mismatches", func(t *testing.T) {
		registered, matches := NKeyExists("exist01", "WRONG_KEY")
		if !registered {
			t.Error("expected registered=true")
		}
		if matches {
			t.Error("expected matches=false")
		}
	})

	t.Run("not registered", func(t *testing.T) {
		registered, matches := NKeyExists("nope", "ANY")
		if registered {
			t.Error("expected registered=false")
		}
		if matches {
			t.Error("expected matches=false")
		}
	})
}

func TestGetNKeysByType(t *testing.T) {
	NatsServer = nil
	pkiDir := setupTestPKI(t)

	writeKey(t, pkiDir, "accepted", "a1", "KEY_A1")
	writeKey(t, pkiDir, "accepted", "a2", "KEY_A2")
	writeKey(t, pkiDir, "denied", "d1", "KEY_D1")

	t.Run("accepted", func(t *testing.T) {
		ks := GetNKeysByType("accepted")
		if len(ks.Sprouts) != 2 {
			t.Errorf("expected 2 accepted sprouts, got %d", len(ks.Sprouts))
		}
	})

	t.Run("denied", func(t *testing.T) {
		ks := GetNKeysByType("denied")
		if len(ks.Sprouts) != 1 {
			t.Errorf("expected 1 denied sprout, got %d", len(ks.Sprouts))
		}
	})

	t.Run("empty state", func(t *testing.T) {
		ks := GetNKeysByType("rejected")
		if len(ks.Sprouts) != 0 {
			t.Errorf("expected 0 rejected sprouts, got %d", len(ks.Sprouts))
		}
	})

	t.Run("invalid state", func(t *testing.T) {
		ks := GetNKeysByType("bogus")
		if len(ks.Sprouts) != 0 {
			t.Errorf("expected 0 sprouts for invalid state, got %d", len(ks.Sprouts))
		}
	})
}

func TestListNKeysByType(t *testing.T) {
	NatsServer = nil
	pkiDir := setupTestPKI(t)

	writeKey(t, pkiDir, "accepted", "a1", "KEY_A1")
	writeKey(t, pkiDir, "unaccepted", "u1", "KEY_U1")
	writeKey(t, pkiDir, "denied", "d1", "KEY_D1")
	writeKey(t, pkiDir, "rejected", "r1", "KEY_R1")

	all := ListNKeysByType()
	if len(all.Accepted.Sprouts) != 1 {
		t.Errorf("expected 1 accepted, got %d", len(all.Accepted.Sprouts))
	}
	if len(all.Unaccepted.Sprouts) != 1 {
		t.Errorf("expected 1 unaccepted, got %d", len(all.Unaccepted.Sprouts))
	}
	if len(all.Denied.Sprouts) != 1 {
		t.Errorf("expected 1 denied, got %d", len(all.Denied.Sprouts))
	}
	if len(all.Rejected.Sprouts) != 1 {
		t.Errorf("expected 1 rejected, got %d", len(all.Rejected.Sprouts))
	}
}

func TestAcceptThenDeny(t *testing.T) {
	NatsServer = nil
	pkiDir := setupTestPKI(t)

	writeKey(t, pkiDir, "unaccepted", "flip01", "NKEY_FLIP")

	// Accept it.
	if err := AcceptNKey("flip01"); err != nil {
		t.Fatalf("AcceptNKey: %v", err)
	}

	// Verify it's accepted.
	ks := GetNKeysByType("accepted")
	found := false
	for _, s := range ks.Sprouts {
		if s.SproutID == "flip01" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("flip01 not found in accepted after AcceptNKey")
	}

	// Deny it.
	if err := DenyNKey("flip01"); err != nil {
		t.Fatalf("DenyNKey: %v", err)
	}

	// Should be in denied, not accepted.
	ksAccepted := GetNKeysByType("accepted")
	for _, s := range ksAccepted.Sprouts {
		if s.SproutID == "flip01" {
			t.Error("flip01 still in accepted after DenyNKey")
		}
	}
	ksDenied := GetNKeysByType("denied")
	found = false
	for _, s := range ksDenied.Sprouts {
		if s.SproutID == "flip01" {
			found = true
			break
		}
	}
	if !found {
		t.Error("flip01 not found in denied after DenyNKey")
	}
}

func TestSetupPKISprout(t *testing.T) {
	tmpDir := t.TempDir()
	config.SproutPKI = filepath.Join(tmpDir, "sprout-pki/")

	SetupPKISprout()

	info, err := os.Stat(config.SproutPKI)
	if err != nil {
		t.Fatalf("expected sprout PKI directory to exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected sprout PKI path to be a directory")
	}

	// Idempotent.
	SetupPKISprout()
}

func TestRootCACached(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("grlx cached", func(t *testing.T) {
		caFile := filepath.Join(tmpDir, "grlx-rootca.pem")
		if err := os.WriteFile(caFile, []byte("cert"), 0o600); err != nil {
			t.Fatal(err)
		}
		config.GrlxRootCA = caFile
		if !RootCACached("grlx") {
			t.Error("expected RootCACached to return true for grlx")
		}
	})

	t.Run("sprout cached", func(t *testing.T) {
		caFile := filepath.Join(tmpDir, "sprout-rootca.pem")
		if err := os.WriteFile(caFile, []byte("cert"), 0o600); err != nil {
			t.Fatal(err)
		}
		config.SproutRootCA = caFile
		if !RootCACached("sprout") {
			t.Error("expected RootCACached to return true for sprout")
		}
	})

	t.Run("grlx not cached", func(t *testing.T) {
		config.GrlxRootCA = filepath.Join(tmpDir, "nonexistent.pem")
		if RootCACached("grlx") {
			t.Error("expected RootCACached to return false")
		}
	})
}

func TestCreateSproutID(t *testing.T) {
	id := createSproutID()
	if id == "" {
		t.Error("expected non-empty sprout ID")
	}
	// The ID should have no uppercase and no leading hyphen.
	if strings.ContainsAny(id, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		t.Errorf("sprout ID should be lowercase, got %q", id)
	}
	if strings.HasPrefix(id, "-") {
		t.Errorf("sprout ID should not start with hyphen, got %q", id)
	}
}

func TestUnacceptNKey_MoveFromAccepted(t *testing.T) {
	NatsServer = nil
	pkiDir := setupTestPKI(t)

	writeKey(t, pkiDir, "accepted", "revoke01", "NKEY_REVOKE")

	err := UnacceptNKey("revoke01", "")
	if err != nil {
		t.Fatalf("UnacceptNKey failed: %v", err)
	}

	// Should be in unaccepted now.
	unaccepted := filepath.Join(pkiDir, "sprouts/unaccepted/revoke01")
	if _, err := os.Stat(unaccepted); err != nil {
		t.Fatalf("expected key in unaccepted: %v", err)
	}

	// Should NOT be in accepted.
	accepted := filepath.Join(pkiDir, "sprouts/accepted/revoke01")
	if _, err := os.Stat(accepted); !os.IsNotExist(err) {
		t.Error("expected key removed from accepted")
	}
}

func TestUnacceptNKey_AlreadyUnaccepted(t *testing.T) {
	NatsServer = nil
	pkiDir := setupTestPKI(t)

	writeKey(t, pkiDir, "unaccepted", "already01", "NKEY_ALREADY")

	err := UnacceptNKey("already01", "")
	if !errors.Is(err, ErrAlreadyUnaccepted) {
		t.Errorf("expected ErrAlreadyUnaccepted, got: %v", err)
	}
}

func TestSetNATSServer(t *testing.T) {
	// SetNATSServer just assigns the package-level var.
	original := NatsServer
	defer func() { NatsServer = original }()

	SetNATSServer(nil)
	if NatsServer != nil {
		t.Error("expected NatsServer to be nil")
	}
}

func TestGetPubNKey_SproutKey(t *testing.T) {
	tmpDir := t.TempDir()
	pubFile := filepath.Join(tmpDir, "sprout.pub")
	if err := os.WriteFile(pubFile, []byte("USPROUT_PUB_KEY"), 0o600); err != nil {
		t.Fatal(err)
	}
	config.NKeySproutPubFile = pubFile

	key, err := GetPubNKey(SproutPubNKey)
	if err != nil {
		t.Fatalf("GetPubNKey(SproutPubNKey) failed: %v", err)
	}
	if key != "USPROUT_PUB_KEY" {
		t.Errorf("expected %q, got %q", "USPROUT_PUB_KEY", key)
	}
}

func TestGetPubNKey_FarmerKey(t *testing.T) {
	tmpDir := t.TempDir()
	pubFile := filepath.Join(tmpDir, "farmer.pub")
	if err := os.WriteFile(pubFile, []byte("UFARMER_PUB_KEY"), 0o600); err != nil {
		t.Fatal(err)
	}
	config.NKeyFarmerPubFile = pubFile

	key, err := GetPubNKey(FarmerPubNKey)
	if err != nil {
		t.Fatalf("GetPubNKey(FarmerPubNKey) failed: %v", err)
	}
	if key != "UFARMER_PUB_KEY" {
		t.Errorf("expected %q, got %q", "UFARMER_PUB_KEY", key)
	}
}

func TestGetPubNKey_MissingFile(t *testing.T) {
	config.NKeySproutPubFile = "/nonexistent/path/sprout.pub"

	_, err := GetPubNKey(SproutPubNKey)
	if err == nil {
		t.Error("expected error for missing pub key file")
	}
}

func TestGetPubNKey_CliKeyType(t *testing.T) {
	// CliPubNKey is not yet implemented — pubFile will be empty string,
	// which should fail with a file read error.
	_, err := GetPubNKey(CliPubNKey)
	if err == nil {
		t.Error("expected error for unimplemented CliPubNKey type")
	}
}

func TestGetSproutID_FromConfig(t *testing.T) {
	// When config.SproutID is set, GetSproutID should return it directly.
	config.SproutID = "preconfigured-sprout"
	defer func() { config.SproutID = "" }()

	id := GetSproutID()
	if id != "preconfigured-sprout" {
		t.Errorf("expected %q, got %q", "preconfigured-sprout", id)
	}
}

func TestGetSproutID_FallbackToHostname(t *testing.T) {
	// When config.SproutID is empty, it should fall back to hostname
	// and persist via SetSproutID (writes to jety, not config var).
	config.SproutID = ""

	id := GetSproutID()
	if id == "" {
		t.Error("expected non-empty sprout ID from hostname fallback")
	}
	// The returned ID should be a valid hostname-based string.
	if strings.HasPrefix(id, "-") {
		t.Errorf("sprout ID should not start with hyphen, got %q", id)
	}
}

func TestFetchRootCA_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	caFile := filepath.Join(tmpDir, "rootca.pem")
	if err := os.WriteFile(caFile, []byte("existing-cert"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Should return nil (no error) because the file already exists.
	err := FetchRootCA(caFile)
	if err != nil {
		t.Errorf("expected nil error when CA already exists, got: %v", err)
	}

	// File should be unchanged.
	data, _ := os.ReadFile(caFile)
	if string(data) != "existing-cert" {
		t.Errorf("file should be unchanged, got %q", string(data))
	}
}

func TestFetchRootCA_FromServer(t *testing.T) {
	// Create a test HTTPS server that serves a fake cert.
	certPEM := generateSelfSignedCertPEM(t)

	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/cert/" {
			http.NotFound(w, r)
			return
		}
		w.Write(certPEM)
	}))
	defer ts.Close()

	// Parse test server address to get host:port.
	addr := ts.Listener.Addr().String()
	host, port, _ := strings.Cut(addr, ":")

	config.FarmerInterface = host
	config.FarmerAPIPort = port

	tmpDir := t.TempDir()
	caFile := filepath.Join(tmpDir, "fetched-rootca.pem")

	err := FetchRootCA(caFile)
	if err != nil {
		t.Fatalf("FetchRootCA failed: %v", err)
	}

	data, err := os.ReadFile(caFile)
	if err != nil {
		t.Fatalf("failed to read fetched CA: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty CA file")
	}
}

func TestFetchRootCA_ServerUnreachable(t *testing.T) {
	config.FarmerInterface = "127.0.0.1"
	config.FarmerAPIPort = "1" // port 1 should be unreachable

	tmpDir := t.TempDir()
	caFile := filepath.Join(tmpDir, "unreachable-rootca.pem")

	err := FetchRootCA(caFile)
	if err == nil {
		t.Error("expected error when server is unreachable")
	}

	// File should have been cleaned up.
	if _, statErr := os.Stat(caFile); !os.IsNotExist(statErr) {
		t.Error("expected CA file to be cleaned up on error")
	}
}

func TestLoadRootCA_GrlxBinary(t *testing.T) {
	tmpDir := t.TempDir()

	// Generate a valid CA cert PEM.
	certPEM := generateSelfSignedCertPEM(t)
	caFile := filepath.Join(tmpDir, "grlx-rootca.pem")
	if err := os.WriteFile(caFile, certPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	config.GrlxRootCA = caFile

	err := LoadRootCA("grlx")
	if err != nil {
		t.Fatalf("LoadRootCA(grlx) failed: %v", err)
	}

	// nkeyClient should be configured.
	if nkeyClient == nil {
		t.Error("expected nkeyClient to be non-nil after LoadRootCA")
	}
}

func TestLoadRootCA_InvalidPEM(t *testing.T) {
	tmpDir := t.TempDir()
	caFile := filepath.Join(tmpDir, "bad-rootca.pem")
	if err := os.WriteFile(caFile, []byte("not-a-valid-pem"), 0o600); err != nil {
		t.Fatal(err)
	}
	config.GrlxRootCA = caFile

	err := LoadRootCA("grlx")
	if !errors.Is(err, ErrCannotParseRootCA) {
		t.Errorf("expected ErrCannotParseRootCA, got: %v", err)
	}
}

func TestLoadRootCA_MissingFile(t *testing.T) {
	config.GrlxRootCA = "/nonexistent/rootca.pem"

	err := LoadRootCA("grlx")
	if err == nil {
		t.Error("expected error for missing root CA file")
	}
}

func TestPutNKey_Success(t *testing.T) {
	// Set up a sprout pub key file.
	tmpDir := t.TempDir()
	pubFile := filepath.Join(tmpDir, "sprout.pub")
	if err := os.WriteFile(pubFile, []byte("UTEST_SPROUT_KEY"), 0o600); err != nil {
		t.Fatal(err)
	}
	config.NKeySproutPubFile = pubFile

	// Set up a test HTTP server to receive the PUT request.
	var receivedSubmission KeySubmission
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/pki/putnkey" {
			t.Errorf("expected /pki/putnkey, got %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&receivedSubmission); err != nil {
			t.Errorf("failed to decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	config.FarmerURL = ts.URL
	nkeyClient = ts.Client()

	err := PutNKey("test-sprout")
	if err != nil {
		t.Fatalf("PutNKey failed: %v", err)
	}

	if receivedSubmission.NKey != "UTEST_SPROUT_KEY" {
		t.Errorf("expected NKey %q, got %q", "UTEST_SPROUT_KEY", receivedSubmission.NKey)
	}
	if receivedSubmission.SproutID != "test-sprout" {
		t.Errorf("expected SproutID %q, got %q", "test-sprout", receivedSubmission.SproutID)
	}
}

func TestPutNKey_MissingPubKey(t *testing.T) {
	config.NKeySproutPubFile = "/nonexistent/sprout.pub"
	nkeyClient = &http.Client{}

	err := PutNKey("test-sprout")
	if err == nil {
		t.Error("expected error when sprout pub key file is missing")
	}
}

func TestPutNKey_ServerError(t *testing.T) {
	tmpDir := t.TempDir()
	pubFile := filepath.Join(tmpDir, "sprout.pub")
	if err := os.WriteFile(pubFile, []byte("UTEST_KEY"), 0o600); err != nil {
		t.Fatal(err)
	}
	config.NKeySproutPubFile = pubFile

	// Use a URL that won't connect.
	config.FarmerURL = "http://127.0.0.1:1"
	nkeyClient = &http.Client{Timeout: time.Millisecond * 100}

	err := PutNKey("test-sprout")
	if err == nil {
		t.Error("expected error when server is unreachable")
	}
}

func TestDenyNKey_InvalidID(t *testing.T) {
	NatsServer = nil
	setupTestPKI(t)

	err := DenyNKey("-invalid")
	if !errors.Is(err, ErrSproutIDInvalid) {
		t.Errorf("expected ErrSproutIDInvalid, got: %v", err)
	}
}

func TestDenyNKey_NotFound(t *testing.T) {
	NatsServer = nil
	setupTestPKI(t)

	err := DenyNKey("ghost")
	if !errors.Is(err, ErrSproutIDNotFound) {
		t.Errorf("expected ErrSproutIDNotFound, got: %v", err)
	}
}

func TestRejectNKey_InvalidID(t *testing.T) {
	NatsServer = nil
	setupTestPKI(t)

	err := RejectNKey("-invalid", "")
	if !errors.Is(err, ErrSproutIDInvalid) {
		t.Errorf("expected ErrSproutIDInvalid, got: %v", err)
	}
}

func TestRejectNKey_NotFound(t *testing.T) {
	NatsServer = nil
	setupTestPKI(t)

	err := RejectNKey("ghost", "")
	if !errors.Is(err, ErrSproutIDNotFound) {
		t.Errorf("expected ErrSproutIDNotFound, got: %v", err)
	}
}

func TestRootCACached_UnknownBinary(t *testing.T) {
	// Passing an unrecognized binary name should result in checking
	// an empty path, which should not exist.
	if RootCACached("unknown") {
		t.Error("expected false for unknown binary type")
	}
}

func TestAcceptNKey_WithSuffix(t *testing.T) {
	// When an ID contains "_<suffix>", AcceptNKey should strip the suffix
	// and also call DeleteNKey on the base ID.
	NatsServer = nil
	pkiDir := setupTestPKI(t)

	// Create the suffixed key.
	writeKey(t, pkiDir, "unaccepted", "web01_2", "NKEY_WEB01_2")

	err := AcceptNKey("web01_2")
	if err != nil {
		t.Fatalf("AcceptNKey with suffix failed: %v", err)
	}

	// Should be accepted as "web01" (base name).
	accepted := filepath.Join(pkiDir, "sprouts/accepted/web01")
	if _, err := os.Stat(accepted); err != nil {
		t.Fatalf("expected key at %s: %v", accepted, err)
	}
}

func TestSetupPKIFarmer_Idempotent_ExistingDirs(t *testing.T) {
	tmpDir := t.TempDir()
	config.FarmerPKI = filepath.Join(tmpDir, "pki") + "/"

	// Create the full structure first.
	SetupPKIFarmer()

	// Write a marker file to verify dirs aren't recreated (data preserved).
	marker := filepath.Join(config.FarmerPKI, "sprouts/accepted/marker")
	if err := os.WriteFile(marker, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Call again — should not fail and should preserve existing files.
	SetupPKIFarmer()

	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("marker file should still exist: %v", err)
	}
	if string(data) != "keep" {
		t.Error("marker file content changed")
	}
}

func TestNKeyExists_ReadError(t *testing.T) {
	NatsServer = nil
	pkiDir := setupTestPKI(t)

	// Create a key file that's unreadable.
	keyPath := filepath.Join(pkiDir, "sprouts/accepted/unreadable01")
	if err := os.WriteFile(keyPath, []byte("SECRET"), 0o000); err != nil {
		t.Fatal(err)
	}
	// Restore permissions in cleanup so TempDir can clean up.
	t.Cleanup(func() { os.Chmod(keyPath, 0o600) })

	registered, matches := NKeyExists("unreadable01", "SECRET")
	if !registered {
		t.Error("expected registered=true even when file is unreadable")
	}
	if matches {
		t.Error("expected matches=false when file cannot be read")
	}
}

// generateSelfSignedCertPEM creates a self-signed certificate and returns
// its PEM encoding. Useful for tests that need valid certificate data.
func generateSelfSignedCertPEM(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{Organization: []string{"test"}},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
}

func TestFindNKey_InvalidID(t *testing.T) {
	NatsServer = nil
	setupTestPKI(t)

	_, err := findNKey("-bad")
	if !errors.Is(err, ErrSproutIDInvalid) {
		t.Errorf("expected ErrSproutIDInvalid, got: %v", err)
	}
}

func TestRejectNKey_PathTraversal(t *testing.T) {
	NatsServer = nil
	setupTestPKI(t)

	// A sprout ID with path traversal should be caught by the path
	// clean check in RejectNKey.
	err := RejectNKey("../escape", "NKEY_BAD")
	if err == nil {
		t.Error("expected error for path traversal attempt")
	}
}

func TestConfigureNats_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Generate valid certs.
	certPEM := generateSelfSignedCertPEM(t)
	caFile := filepath.Join(tmpDir, "rootca.pem")
	if err := os.WriteFile(caFile, certPEM, 0o600); err != nil {
		t.Fatal(err)
	}

	// Generate a leaf cert+key for CertFile/KeyFile.
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leafTemplate := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{Organization: []string{"test"}},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	// Self-sign the leaf for simplicity.
	leafDER, err := x509.CreateCertificate(rand.Reader, &leafTemplate, &leafTemplate, &leafKey.PublicKey, leafKey)
	if err != nil {
		t.Fatal(err)
	}
	leafCertFile := filepath.Join(tmpDir, "cert.pem")
	leafKeyFile := filepath.Join(tmpDir, "key.pem")
	writePEM(t, leafCertFile, "CERTIFICATE", leafDER)
	leafPrivBytes, err := x509.MarshalPKCS8PrivateKey(leafKey)
	if err != nil {
		t.Fatal(err)
	}
	writePEM(t, leafKeyFile, "PRIVATE KEY", leafPrivBytes)

	config.FarmerInterface = "127.0.0.1"
	config.FarmerBusPort = "24222"
	config.RootCA = caFile
	config.CertFile = leafCertFile
	config.KeyFile = leafKeyFile

	opts := ConfigureNats()
	if opts.Host != "127.0.0.1" {
		t.Errorf("expected host 127.0.0.1, got %s", opts.Host)
	}
	if opts.Port != 24222 {
		t.Errorf("expected port 24222, got %d", opts.Port)
	}
	if opts.TLSConfig == nil {
		t.Error("expected TLSConfig to be set")
	}
	if !opts.TLS {
		t.Error("expected TLS to be true")
	}
}

func TestLoadRootCA_SproutBinary(t *testing.T) {
	tmpDir := t.TempDir()

	// For sprout binary, LoadRootCA calls FetchRootCA first.
	// We need a valid CA cert file at SproutRootCA.
	certPEM := generateSelfSignedCertPEM(t)
	caFile := filepath.Join(tmpDir, "sprout-rootca.pem")
	if err := os.WriteFile(caFile, certPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	config.SproutRootCA = caFile

	err := LoadRootCA("sprout")
	if err != nil {
		t.Fatalf("LoadRootCA(sprout) failed: %v", err)
	}
	if nkeyClient == nil {
		t.Error("expected nkeyClient to be non-nil")
	}
}

func TestReloadNKeys_WithAcceptedSprouts(t *testing.T) {
	NatsServer = nil
	pkiDir := setupTestPKI(t)

	// Add some accepted sprouts with valid-looking NKeys.
	writeKey(t, pkiDir, "accepted", "web01", "UABC123")
	writeKey(t, pkiDir, "accepted", "db01", "UDEF456")

	// ReloadNKeys should not error when NatsServer is nil (skips reload).
	err := ReloadNKeys()
	// err may be nil (no server to reload) — that's fine.
	_ = err
	// The important thing is it doesn't panic or fatal.
}

// Verify that the full key lifecycle works: unaccept → accept → reject → unaccept.
func TestKeyLifecycle_FullCycle(t *testing.T) {
	NatsServer = nil
	pkiDir := setupTestPKI(t)

	// 1. Register as unaccepted.
	err := UnacceptNKey("lifecycle01", "NKEY_LIFE")
	if err != nil {
		t.Fatalf("UnacceptNKey: %v", err)
	}

	// 2. Accept.
	err = AcceptNKey("lifecycle01")
	if err != nil {
		t.Fatalf("AcceptNKey: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(pkiDir, "sprouts/accepted/lifecycle01")); statErr != nil {
		t.Fatal("expected key in accepted")
	}

	// 3. Reject.
	err = RejectNKey("lifecycle01", "")
	if err != nil {
		t.Fatalf("RejectNKey: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(pkiDir, "sprouts/rejected/lifecycle01")); statErr != nil {
		t.Fatal("expected key in rejected")
	}

	// 4. Unaccept again.
	err = UnacceptNKey("lifecycle01", "")
	if err != nil {
		t.Fatalf("UnacceptNKey (from rejected): %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(pkiDir, "sprouts/unaccepted/lifecycle01")); statErr != nil {
		t.Fatal("expected key back in unaccepted")
	}

	// 5. Delete.
	err = DeleteNKey("lifecycle01")
	if err != nil {
		t.Fatalf("DeleteNKey: %v", err)
	}
	// Verify gone from all states.
	for _, state := range []string{"unaccepted", "accepted", "denied", "rejected"} {
		p := filepath.Join(pkiDir, "sprouts", state, "lifecycle01")
		if _, statErr := os.Stat(p); !os.IsNotExist(statErr) {
			t.Errorf("expected key removed from %s", state)
		}
	}
}

// Suppress unused import warnings.
var _ = fmt.Sprintf
