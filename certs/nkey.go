package certs

import (
	"os"

	. "github.com/gogrlx/grlx/config"
	"github.com/nats-io/nkeys"
	log "github.com/taigrr/log-socket/log"
)

func GetPubNKey(isFarmer bool) (string, error) {
	pubFile := NKeySproutPubFile
	if isFarmer {
		pubFile = NKeyFarmerPubFile
	}
	pubKeyBytes, err := os.ReadFile(pubFile)
	if err != nil {
		return "", err
	}
	return string(pubKeyBytes), nil
}
func GenNKey(isFarmer bool) {
	privFile := NKeySproutPrivFile
	pubFile := NKeySproutPubFile
	if isFarmer {
		privFile = NKeyFarmerPrivFile
		pubFile = NKeyFarmerPubFile
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
			0600,
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
			0600,
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
