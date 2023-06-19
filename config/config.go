package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gogrlx/grlx/types"
	"github.com/spf13/viper"
)

const GrlxExt = "grlx"

var BuildInfo types.Version

var configLoaded sync.Once

// TODO move all references to viper to this file
// TODO use enum for binary as elsewhere
func LoadConfig(binary string) {
	configLoaded.Do(func() {
		// TODO if config doesn't exist, write out the default one
		viper.SetConfigType("yaml")
		switch binary {
		case "grlx":
			viper.SetConfigName("grlx")
			dirname, err := os.UserHomeDir()
			if err != nil {
				log.Fatal(err)
			}
			cfgPath := filepath.Join(dirname, ".config/grlx/")
			viper.AddConfigPath(cfgPath)
		case "farmer":
			viper.SetConfigName("farmer")
			viper.AddConfigPath("/etc/grlx/")
		case "sprout":
			viper.SetConfigName("sprout")
			viper.AddConfigPath("/etc/grlx/")
		}
		err := viper.ReadInConfig()

		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("Config file not found, will create default config")
			switch binary {
			case "grlx":
				dirname, errHomeDir := os.UserHomeDir()
				if err != nil {
					log.Fatal(errHomeDir)
				}
				cfgPath := filepath.Join(dirname, ".config/grlx/")
				os.MkdirAll(cfgPath, 0o755)
				cfgFile := filepath.Join(cfgPath, "grlx")
				_, err = os.Create(cfgFile)
				if err != nil {
					log.Fatal(err)
				}
			case "farmer":
				os.MkdirAll("/etc/grlx", 0o755)
				cfgFile := filepath.Join("/etc/grlx", "farmer")
				_, err = os.Create(cfgFile)
				if err != nil {
					log.Fatal(err)
				}
			case "sprout":
				os.MkdirAll("/etc/grlx", 0o755)
				cfgFile := filepath.Join("/etc/grlx", "sprout")
				_, err = os.Create(cfgFile)
				if err != nil {
					log.Fatal(err)
				}
			}
		} else if err != nil {
			// TODO create default config
			log.Printf("%T\n", err)
			panic(fmt.Errorf("fatal error config file: %w", err))
		}
		viper.Set("ConfigRoot", "/etc/grlx/")
		viper.SetDefault("FarmerInterface", "localhost")
		viper.SetDefault("FarmerAPIPort", "5405")
		switch binary {
		case "grlx":
			dirname, err := os.UserHomeDir()
			if err != nil {
				log.Fatal(err)
			}
			certPath := filepath.Join(dirname, ".config/grlx/tls-rootca.pem")
			viper.Set("GrlxRootCA", certPath)
			viper.SetDefault("FarmerBusPort", "5406")
			viper.SetDefault("FarmerBusInterface", viper.GetString("FarmerInterface")+":"+viper.GetString("FarmerBusPort"))
		case "farmer":
			viper.SetDefault("CertHosts", []string{"localhost", "127.0.0.1", "farmer", "grlx", viper.GetString("FarmerInterface")})
			viper.SetDefault("CertificateValidTime", 365*24*time.Hour)
			viper.Set("CertFile", "/etc/grlx/pki/farmer/tls-cert.pem")
			viper.Set("FarmerPKI", "/etc/grlx/pki/farmer/")
			viper.Set("KeyFile", "/etc/grlx/pki/farmer/tls-key.pem")
			viper.Set("NKeyFarmerPubFile", "/etc/grlx/pki/farmer/farmer.nkey.pub")
			viper.Set("NKeyFarmerPrivFile", "/etc/grlx/pki/farmer/farmer.nkey")
			viper.Set("RootCA", "/etc/grlx/pki/farmer/tls-rootca.pem")
			viper.Set("RootCAPriv", "/etc/grlx/pki/farmer/tls-rootca-key.pem")
			viper.SetDefault("Organization", "GRLX Development")
			viper.SetDefault("FarmerBusPort", "5406")
			viper.SetDefault("FarmerBusInterface", viper.GetString("FarmerURL")+":"+viper.GetString("FarmerBusPort"))
		case "sprout":
			viper.SetDefault("SproutID", "")
			viper.Set("SproutPKI", "/etc/grlx/pki/sprout/")
			viper.Set("SproutRootCA", "/etc/grlx/pki/sprout/tls-rootca.pem")
			viper.Set("NKeySproutPubFile", "/etc/grlx/pki/sprout/sprout.nkey.pub")
			viper.Set("NKeySproutPrivFile", "/etc/grlx/pki/sprout/sprout.nkey")
			viper.SetDefault("FarmerBusPort", "5406")
			viper.SetDefault("FarmerBusInterface", viper.GetString("FarmerURL")+":"+viper.GetString("FarmerBusPort"))
			viper.SetDefault("CacheDir", "/var/cache/grlx/sprout/files/provided")
		}
		viper.Set("FarmerURL", "https://"+viper.GetString("FarmerInterface")+":"+viper.GetString("FarmerAPIPort"))
		viper.WriteConfig()
	})
}

// TODO actually validate the base path exists
func BasePathValid() bool {
	return true
}

func CacheDir() string {
	return viper.GetString("CacheDir")
}

func Init() string {
	return viper.GetString("init")
}
