package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"

	"github.com/gogrlx/grlx/types"
)

const GrlxExt = "grlx"

var BuildInfo types.Version

var configLoaded sync.Once

var (
	AdminPubkeys         []string
	CacheDir             string
	CertFile             string
	CertHosts            []string
	CertificateValidTime time.Duration
	ConfigRoot           string
	FarmerAPIPort        string
	FarmerBusInterface   string
	FarmerBusPort        string
	FarmerInterface      string
	FarmerPKI            string
	FarmerURL            string
	GrlxRootCA           string
	KeyFile              string
	NKeyFarmerPrivFile   string
	NKeyFarmerPubFile    string
	NKeySproutPrivFile   string
	NKeySproutPubFile    string
	Organization         []string
	RootCA               string
	RootCAPriv           string
	SproutID             string
	SproutPKI            string
	SproutRootCA         string
)

func SetConfigFile(path string) {
	viper.SetConfigFile(path)
}

// TODO use enum for binary as elsewhere
func LoadConfig(binary string) {
	configLoaded.Do(func() {
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
		viper.AutomaticEnv()
		err := viper.ReadInConfig()

		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("Config file not found, will create default config")
			switch binary {
			case "grlx":
				dirname, errHomeDir := os.UserHomeDir()
				if errHomeDir != nil {
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
			CertHosts = viper.GetStringSlice("CertHosts")
			AdminPubKeys := viper.GetStringMap("pubkeys")
			envSet := os.Environ()
			for _, v := range envSet {
				pair := strings.SplitN(v, "=", 2)
				if len(AdminPubKeys) == 0 {
					if pair[0] == "ADMIN_PUBKEYS" {
						keyList := pair[1]
						pubkeys := strings.Split(keyList, ",")
						adminSet := make(map[string][]string)
						adminSet["admin"] = []string{}
						for _, v := range pubkeys {
							if v != "" {
								adminSet["admin"] = append(adminSet["admin"], v)
							}
						}
						viper.Set("pubkeys", adminSet)
					}
				}
				if CertHosts == nil {
					if pair[0] == "CERT_HOSTS" {
						hostList := pair[1]
						hosts := strings.Split(hostList, ",")
						cleanHosts := []string{}
						for _, v := range hosts {
							if v != "" {
								cleanHosts = append(cleanHosts, v)
							}
						}
						viper.Set("CertHosts", cleanHosts)
					}
				}
			}
			viper.SetDefault("CertHosts", []string{"localhost", "127.0.0.1", "farmer", "grlx", viper.GetString("FarmerInterface")})

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

	CacheDir = viper.GetString("CacheDir")
	CertFile = viper.GetString("CertFile")
	CertificateValidTime = viper.GetDuration("CertificateValidTime")
	ConfigRoot = viper.GetString("ConfigRoot")
	FarmerAPIPort = viper.GetString("FarmerAPIPort")
	FarmerBusInterface = viper.GetString("FarmerBusInterface")
	FarmerBusPort = viper.GetString("FarmerBusPort")
	FarmerInterface = viper.GetString("FarmerInterface")
	FarmerPKI = viper.GetString("FarmerPKI")
	FarmerURL = viper.GetString("FarmerURL")
	GrlxRootCA = viper.GetString("GrlxRootCA")
	KeyFile = viper.GetString("KeyFile")
	NKeyFarmerPrivFile = viper.GetString("NKeyFarmerPrivFile")
	NKeyFarmerPubFile = viper.GetString("NKeyFarmerPubFile")
	NKeySproutPrivFile = viper.GetString("NKeySproutPrivFile")
	NKeySproutPubFile = viper.GetString("NKeySproutPubFile")
	Organization = viper.GetStringSlice("Organization")
	RootCA = viper.GetString("RootCA")
	RootCAPriv = viper.GetString("RootCAPriv")
	SproutID = viper.GetString("SproutID")
	SproutPKI = viper.GetString("SproutPKI")
	SproutRootCA = viper.GetString("SproutRootCA")
}

// TODO actually validate the base path exists
func BasePathValid() bool {
	return true
}

func Init() string {
	return viper.GetString("init")
}

func SetSproutID(id string) {
	viper.Set("SproutID", id)
	viper.WriteConfig()
}
