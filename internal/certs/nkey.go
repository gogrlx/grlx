package certs

import (
	"os"

	"github.com/nats-io/nkeys"
	log "github.com/taigrr/log-socket/log"

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

func GenNKey(isFarmer bool) {
	privFile := config.NKeySproutPrivFile
	pubFile := config.NKeySproutPubFile
	if isFarmer {
		privFile = config.NKeyFarmerPrivFile
		pubFile = config.NKeyFarmerPubFile
	}
	_, err := os.Stat(privFile)
	if err == nil {
		return
	}
	if os.IsNotExist(err) {
		kp, err := nkeys.CreateUser()
		if err != nil {
			log.Panic(err.Error())
		}
		pubKey, err := os.OpenFile(pubFile,
			os.O_WRONLY|os.O_TRUNC|os.O_CREATE,
			0o600,
		)
		if err != nil {
			log.Panic(err.Error())
		}
		defer pubKey.Close()
		key, err := kp.PublicKey()
		_, err = pubKey.Write([]byte(key))
		if err != nil {
			log.Panic(err.Error())
		}

		privKey, err := os.OpenFile(privFile,
			os.O_WRONLY|os.O_TRUNC|os.O_CREATE,
			0o600,
		)
		if err != nil {
			log.Panic(err.Error())
		}
		defer privKey.Close()
		pkey, err := kp.Seed()
		_, err = privKey.Write(pkey)
		if err != nil {
			log.Panic(err.Error())
		}
		return
	}
	log.Panic(err)
}
