package auth

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/nkeys"
)

// helper: create an nkeys account keypair for testing.
func mustCreateKeyPair(t *testing.T) nkeys.KeyPair {
	t.Helper()
	kp, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatalf("failed to create keypair: %v", err)
	}
	return kp
}

func TestUserAuthSignAndIsValid(t *testing.T) {
	kp := mustCreateKeyPair(t)
	pk, err := kp.PublicKey()
	if err != nil {
		t.Fatal(err)
	}

	ua := UserAuth{
		Expires: time.Now().Add(5 * time.Minute).Format(time.RFC3339),
		Pubkey:  pk,
	}

	signed, err := ua.Sign(kp)
	if err != nil {
		t.Fatalf("Sign() error: %v", err)
	}
	if signed.Sig == "" {
		t.Fatal("Sign() produced empty signature")
	}
	if signed.Sig == ua.Sig {
		t.Error("Sign() should have changed the Sig field")
	}

	// IsValid should succeed for a freshly signed, non-expired token.
	gotPK, err := signed.IsValid()
	if err != nil {
		t.Fatalf("IsValid() error: %v", err)
	}
	if gotPK != pk {
		t.Errorf("IsValid() returned pubkey %q, want %q", gotPK, pk)
	}
}

func TestUserAuthIsValidExpired(t *testing.T) {
	kp := mustCreateKeyPair(t)
	pk, _ := kp.PublicKey()

	ua := UserAuth{
		Expires: time.Now().Add(-1 * time.Minute).Format(time.RFC3339),
		Pubkey:  pk,
	}
	signed, err := ua.Sign(kp)
	if err != nil {
		t.Fatalf("Sign() error: %v", err)
	}

	_, err = signed.IsValid()
	if err == nil {
		t.Error("IsValid() should fail for expired token")
	}
	if err != ErrExpired {
		t.Errorf("IsValid() error = %v, want ErrExpired", err)
	}
}

func TestUserAuthIsValidBadSignature(t *testing.T) {
	kp := mustCreateKeyPair(t)
	pk, _ := kp.PublicKey()

	ua := UserAuth{
		Expires: time.Now().Add(5 * time.Minute).Format(time.RFC3339),
		Pubkey:  pk,
	}
	signed, _ := ua.Sign(kp)

	// Corrupt the signature.
	sigBytes, _ := base64.StdEncoding.DecodeString(signed.Sig)
	sigBytes[0] ^= 0xFF
	signed.Sig = base64.StdEncoding.EncodeToString(sigBytes)

	_, err := signed.IsValid()
	if err == nil {
		t.Error("IsValid() should fail for corrupted signature")
	}
}

func TestUserAuthIsValidBadExpiresFormat(t *testing.T) {
	ua := UserAuth{
		Expires: "not-a-date",
		Pubkey:  "ATEST",
		Sig:     "dGVzdA==",
	}
	_, err := ua.IsValid()
	if err == nil {
		t.Error("IsValid() should fail for unparseable expires")
	}
}

func TestUserAuthIsValidBadPubkey(t *testing.T) {
	ua := UserAuth{
		Expires: time.Now().Add(5 * time.Minute).Format(time.RFC3339),
		Pubkey:  "NOTAVALIDNKEY",
		Sig:     base64.StdEncoding.EncodeToString([]byte("test")),
	}
	_, err := ua.IsValid()
	if err == nil {
		t.Error("IsValid() should fail for invalid pubkey")
	}
}

func TestUserAuthIsValidBadSigEncoding(t *testing.T) {
	kp := mustCreateKeyPair(t)
	pk, _ := kp.PublicKey()

	ua := UserAuth{
		Expires: time.Now().Add(5 * time.Minute).Format(time.RFC3339),
		Pubkey:  pk,
		Sig:     "%%%not-base64%%%",
	}
	_, err := ua.IsValid()
	if err == nil {
		t.Error("IsValid() should fail for invalid base64 signature")
	}
}

func TestUserAuthIsValidWrongKey(t *testing.T) {
	// Sign with one key, but set Pubkey to a different key.
	kp1 := mustCreateKeyPair(t)
	kp2 := mustCreateKeyPair(t)
	pk2, _ := kp2.PublicKey()

	ua := UserAuth{
		Expires: time.Now().Add(5 * time.Minute).Format(time.RFC3339),
		Pubkey:  pk2,
	}
	// Sign with kp1 but the token claims to be from pk2.
	signed, err := ua.Sign(kp1)
	if err != nil {
		t.Fatalf("Sign() error: %v", err)
	}

	_, err = signed.IsValid()
	if err == nil {
		t.Error("IsValid() should fail when signature doesn't match pubkey")
	}
}

func TestCreateSignedTokenAndDecodeToken(t *testing.T) {
	kp := mustCreateKeyPair(t)

	token, err := createSignedToken(kp)
	if err != nil {
		t.Fatalf("createSignedToken() error: %v", err)
	}
	if token == "" {
		t.Fatal("createSignedToken() returned empty token")
	}

	// Decode and validate.
	ua, err := decodeToken(token)
	if err != nil {
		t.Fatalf("decodeToken() error: %v", err)
	}

	pk, _ := kp.PublicKey()
	if ua.Pubkey != pk {
		t.Errorf("decoded pubkey = %q, want %q", ua.Pubkey, pk)
	}
	if ua.Sig == "" {
		t.Error("decoded token has empty signature")
	}
	if ua.Expires == "" {
		t.Error("decoded token has empty expires")
	}

	// The decoded token should be valid.
	gotPK, err := ua.IsValid()
	if err != nil {
		t.Fatalf("IsValid() after decode error: %v", err)
	}
	if gotPK != pk {
		t.Errorf("IsValid() pubkey = %q, want %q", gotPK, pk)
	}
}

func TestDecodeTokenInvalidBase64(t *testing.T) {
	_, err := decodeToken("%%%not-base64%%%")
	if err == nil {
		t.Error("decodeToken() should fail for invalid base64")
	}
}

func TestDecodeTokenInvalidJSON(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("not json"))
	_, err := decodeToken(encoded)
	if err == nil {
		t.Error("decodeToken() should fail for invalid JSON")
	}
}

func TestDecodeTokenValidJSON(t *testing.T) {
	ua := UserAuth{
		Expires: "2099-01-01T00:00:00Z",
		Pubkey:  "ATESTKEY",
		Sig:     "dGVzdA==",
	}
	data, _ := json.Marshal(ua)
	token := base64.StdEncoding.EncodeToString(data)

	decoded, err := decodeToken(token)
	if err != nil {
		t.Fatalf("decodeToken() error: %v", err)
	}
	if decoded.Pubkey != "ATESTKEY" {
		t.Errorf("Pubkey = %q, want ATESTKEY", decoded.Pubkey)
	}
	if decoded.Expires != "2099-01-01T00:00:00Z" {
		t.Errorf("Expires = %q", decoded.Expires)
	}
}

func TestSignPreservesOtherFields(t *testing.T) {
	kp := mustCreateKeyPair(t)
	pk, _ := kp.PublicKey()

	expires := time.Now().Add(10 * time.Minute).Format(time.RFC3339)
	ua := UserAuth{
		Expires: expires,
		Pubkey:  pk,
	}

	signed, err := ua.Sign(kp)
	if err != nil {
		t.Fatal(err)
	}
	if signed.Expires != expires {
		t.Errorf("Sign changed Expires: got %q, want %q", signed.Expires, expires)
	}
	if signed.Pubkey != pk {
		t.Errorf("Sign changed Pubkey: got %q, want %q", signed.Pubkey, pk)
	}
}
