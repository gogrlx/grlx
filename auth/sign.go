package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/nats-io/nkeys"
)

type UserAuth struct {
	Expires string `json:"expires"`
	Pubkey  string `json:"pubkey"`
	Sig     string `json:"sig"`
}

var ErrExpired = errors.New("auth token expired")

// Sign adds a signature digest to the UserAuth struct using the provided
// KeyPair. The signature digest is base64 encoded.
func (u UserAuth) Sign(kp nkeys.KeyPair) (UserAuth, error) {
	b, err := kp.Sign([]byte(u.Expires))
	if err != nil {
		return u, err
	}
	u.Sig = base64.StdEncoding.EncodeToString(b)
	return u, nil
}

// IsValid checks if the token is valid. It returns the public key
// if valid, or an error if not.
func (u UserAuth) IsValid() (string, error) {
	exp, err := time.Parse(time.RFC3339, u.Expires)
	if err != nil {
		return "", err
	}
	if exp.Before(time.Now()) {
		return "", ErrExpired
	}
	kp, err := nkeys.FromPublicKey(u.Pubkey)
	if err != nil {
		return "", err
	}
	sig, err := base64.StdEncoding.DecodeString(u.Sig)
	if err != nil {
		return "", err
	}
	return u.Pubkey, kp.Verify([]byte(u.Expires), sig)
}

// decodeToken decodes a base64 encoded token and returns the UserAuth
// struct. The token is not validated.
func decodeToken(token string) (UserAuth, error) {
	var ua UserAuth
	b, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return ua, err
	}
	err = json.Unmarshal(b, &ua)
	return ua, err
}

// createSignedToken creates a signed token that can be used to authenticate
// with the server. The token is valid for 5 minutes, and is base64 encoded.
func createSignedToken(kp nkeys.KeyPair) (string, error) {
	pk, err := kp.PublicKey()
	if err != nil {
		log.Fatal("error getting public key", err)
	}

	ua := UserAuth{
		Expires: time.Now().Add(time.Duration(time.Minute) * 5).Format(time.RFC3339),
		Pubkey:  pk,
	}
	ua, err = ua.Sign(kp)
	if err != nil {
		log.Fatal("error signing", err)
	}
	b, err := json.Marshal(ua)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}
