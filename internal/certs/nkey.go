package certs

import (
	"fmt"
	"os"

	"github.com/nats-io/nkeys"

	"github.com/gogrlx/grlx/v2/internal/config"
)

func GetPubNKey(isFarmer bool) (string, error) {
	pubFile := config.NKeySproutPubFile
	if isFarmer {
		pubFile = config.NKeyFarmerPubFile
	}
	pubKeyBytes, err := os.ReadFile(pubFile)
	if err != nil {
		return "", err
	}
	return string(pubKeyBytes), nil
}

func GenNKey(isFarmer bool) error {
	privFile := config.NKeySproutPrivFile
	pubFile := config.NKeySproutPubFile
	if isFarmer {
		privFile = config.NKeyFarmerPrivFile
		pubFile = config.NKeyFarmerPubFile
	}
	_, err := os.Stat(privFile)
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat NKey file %s: %w", privFile, err)
	}
	kp, err := nkeys.CreateUser()
	if err != nil {
		return fmt.Errorf("failed to create NKey user: %w", err)
	}
	key, err := kp.PublicKey()
	if err != nil {
		return fmt.Errorf("failed to get NKey public key: %w", err)
	}
	if err := os.WriteFile(pubFile, []byte(key), 0o600); err != nil {
		return fmt.Errorf("failed to write NKey public key to %s: %w", pubFile, err)
	}
	seed, err := kp.Seed()
	if err != nil {
		return fmt.Errorf("failed to get NKey seed: %w", err)
	}
	if err := os.WriteFile(privFile, seed, 0o600); err != nil {
		return fmt.Errorf("failed to write NKey private key to %s: %w", privFile, err)
	}
	return nil
}
