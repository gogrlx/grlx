package config

import (
	"time"

	. "github.com/gogrlx/grlx/types"
	nats_server "github.com/nats-io/nats-server/v2/server"
)

var DefaultTestOptions nats_server.Options
var BuildInfo Version

var ConfigRoot = "./etc/grlx/"
var FarmerPKI = ConfigRoot + "pki/farmer/"
var SproutPKI = ConfigRoot + "pki/sprout/"
var SproutRootCA = SproutPKI + "tls-rootca.pem"
var RootCA = FarmerPKI + "tls-rootca.pem"
var RootCAPriv = FarmerPKI + "tls-rootca-key.pem"
var CertFile = FarmerPKI + "tls-cert.pem"
var KeyFile = FarmerPKI + "tls-key.pem"
var NKeyFarmerPubFile = FarmerPKI + "farmer.nkey.pub"
var NKeyFarmerPrivFile = FarmerPKI + "farmer.nkey"
var NKeySproutPubFile = SproutPKI + "sprout.nkey.pub"
var NKeySproutPrivFile = SproutPKI + "sprout.nkey"
var Organization = "GRLX Development"
var FarmerInterface = "localhost"
var FarmerAPIPort = "5405"
var SproutID = ""

var CertificateValidTime = 365 * 24 * time.Hour

var CertHosts = []string{"localhost", "127.0.0.1", "farmer", "grlx"}

func init() {
	DefaultTestOptions = nats_server.Options{
		Host:                  "0.0.0.0",
		Port:                  4443,
		NoSigs:                true,
		MaxControlLine:        4096,
		DisableShortFirstPing: true,
		Trace:                 true,
		Debug:                 true,
	}
}

//TODO
func ReloadNKeys() error {
	return nil
}
