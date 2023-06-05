package auth

import (
	"github.com/nats-io/nkeys"
	"github.com/spf13/viper"
)

func getPrivateSeed() (string, error) {
	seed := viper.GetString("privkey")
	if seed == "" {
		return createPrivateSeed()
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

// TODO allow for method-based access control
func pubkeyHasAccess(pubkey string, method string) bool {
	authKeySet := viper.GetStringMap("pubkeys")
	if authKeySet == nil {
		return false
	}
	i, ok := authKeySet["admin"]
	if !ok {
		return false
	}
	adminKey, ok := i.(string)
	if !ok {
		if adminKeyList, ok := i.([]interface{}); ok {
			for _, k := range adminKeyList {
				if str, ok := k.(string); ok {
					if str == pubkey {
						return true
					}
				} else {
					return false
				}
			}
			return false
		} else {
			return false
		}
	} else {
		return adminKey == pubkey
	}
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
	viper.Set("privkey", seed)
	viper.WriteConfig()
	return string(seed), nil
}
