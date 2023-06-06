package auth

import (
	"errors"

	"github.com/nats-io/nkeys"
	"github.com/spf13/viper"
)

var (
	ErrInvalidPubkey = errors.New("invalid pubkey format in config")
	ErrMissingAdmin  = errors.New("no admin pubkey found in config")
	ErrNoPrivkey     = errors.New("no private key found in config")
	ErrNoPubkeys     = errors.New("no pubkeys found in config")
)

func GetPubkey() (string, error) {
	seed, err := getPrivateSeed()
	if err != nil {
		return "", err
	}
	kp, err := nkeys.FromSeed([]byte(seed))
	if err != nil {
		return "", err
	}
	pubkey, err := kp.PublicKey()
	if err != nil {
		return "", err
	}
	return pubkey, nil
}

func CreatePrivkey() error {
	_, err := createPrivateSeed()
	return err
}

func getPrivateSeed() (string, error) {
	seed := viper.GetString("privkey")
	if seed == "" {
		return "", ErrNoPrivkey
	}
	return seed, nil
}

func NewToken() (string, error) {
	seed, err := getPrivateSeed()
	if err != nil {
		return "", err
	}
	kp, err := nkeys.FromSeed([]byte(seed))
	if err != nil {
		return "", err
	}
	return createSignedToken(kp)
}

func TokenHasAccess(token string, method string) bool {
	ua, err := decodeToken(token)
	if err != nil {
		return false
	}
	pk, err := ua.IsValid()
	if err != nil {
		return false
	}
	return pubkeyHasAccess(pk, method)
}

func GetPubkeysByRole(role string) ([]string, error) {
	authKeySet := viper.GetStringMap("pubkeys")
	if authKeySet == nil {
		return []string{}, ErrNoPubkeys
	}
	i, ok := authKeySet[role]
	if !ok {
		return []string{}, ErrMissingAdmin
	}
	keys := []string{}
	if adminKey, ok := i.(string); !ok {
		if adminKeyList, ok := i.([]interface{}); ok {
			for _, k := range adminKeyList {
				if str, ok := k.(string); ok {
					keys = append(keys, str)
				} else {
					return []string{}, ErrInvalidPubkey
				}
			}
			return keys, nil
		} else {
			return []string{}, ErrInvalidPubkey
		}
	} else {
		return []string{adminKey}, nil
	}
}

// TODO allow for method-based access control
func pubkeyHasAccess(pubkey string, method string) bool {
	keys, _ := GetPubkeysByRole("admin")
	for _, k := range keys {
		if k == pubkey {
			return true
		}
	}
	return false
}

func createPrivateSeed() (string, error) {
	kp, err := nkeys.CreateAccount()
	if err != nil {
		return "", err
	}
	seed, err := kp.Seed()
	if err != nil {
		return "", err
	}
	viper.Set("privkey", string(seed))
	viper.WriteConfig()
	return string(seed), nil
}
