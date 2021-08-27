package config

import (
	"fmt"
	"sync"
	"time"

	. "github.com/gogrlx/grlx/types"
	nats_server "github.com/nats-io/nats-server/v2/server"
	"github.com/spf13/viper"
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
var FarmerURL = "https://" + FarmerInterface + ":" + FarmerAPIPort

var CertificateValidTime = 365 * 24 * time.Hour

var CertHosts = []string{"localhost", "127.0.0.1", "farmer", "grlx", "192.168.2.4"}

func init() {
	LoadConfig()
}

var configLoaded sync.Once

func LoadConfig() {
	configLoaded.Do(func() {
		viper.SetConfigName("config")         // name of config file (without extension)
		viper.SetConfigType("yaml")           // REQUIRED if the config file does not have the extension in the name
		viper.AddConfigPath("/etc/appname/")  // path to look for the config file in
		viper.AddConfigPath("$HOME/.appname") // call multiple times to add many search paths
		viper.AddConfigPath(".")              // optionally look for config in the working directory
		err := viper.ReadInConfig()           // Find and read the config file
		if err != nil {                       // Handle errors reading the config file
			panic(fmt.Errorf("Fatal error config file: %w \n", err))
		}
	})
}
