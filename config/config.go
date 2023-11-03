package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	jety "github.com/taigrr/jety"

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

// TODO use enum for binary as elsewhere
func LoadConfig(binary string) {
	configLoaded.Do(func() {
		jety.SetConfigType("yaml")
		switch binary {
		case "grlx":
			dirname, err := os.UserHomeDir()
			if err != nil {
				log.Fatal(err)
			}
			cfgPath := filepath.Join(dirname, ".config/grlx/")
			jety.SetConfigFile(filepath.Join(cfgPath, "grlx"))
		case "farmer":
			jety.SetConfigFile("/etc/grlx/farmer")
		case "sprout":
			jety.SetConfigFile("/etc/grlx/sprout")
		}
		err := jety.ReadInConfig()

		if errors.Is(err, jety.ErrConfigFileNotFound) || errors.Is(err, jety.ErrConfigFileEmpty) {
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
		jety.SetDefault("ConfigRoot", "/etc/grlx/")
		jety.SetDefault("FarmerInterface", "localhost")
		jety.SetDefault("FarmerAPIPort", "5405")
		jety.SetDefault("FarmerBusPort", "5406")
		FarmerURL = "https://" + jety.GetString("FarmerInterface") + ":" + jety.GetString("FarmerAPIPort")
		switch binary {
		case "grlx":
			dirname, err := os.UserHomeDir()
			if err != nil {
				log.Fatal(err)
			}
			certPath := filepath.Join(dirname, ".config/grlx/tls-rootca.pem")
			jety.Set("GrlxRootCA", certPath)
		case "farmer":
			jety.SetDefault("CertificateValidTime", 365*24*time.Hour)
			jety.SetDefault("CertFile", "/etc/grlx/pki/farmer/tls-cert.pem")
			jety.SetDefault("FarmerPKI", "/etc/grlx/pki/farmer/")
			jety.SetDefault("KeyFile", "/etc/grlx/pki/farmer/tls-key.pem")
			jety.SetDefault("NKeyFarmerPubFile", "/etc/grlx/pki/farmer/farmer.nkey.pub")
			jety.SetDefault("NKeyFarmerPrivFile", "/etc/grlx/pki/farmer/farmer.nkey")
			jety.SetDefault("RootCA", "/etc/grlx/pki/farmer/tls-rootca.pem")
			jety.SetDefault("RootCAPriv", "/etc/grlx/pki/farmer/tls-rootca-key.pem")
			jety.SetDefault("Organization", "GRLX Development")
			jety.SetDefault("FarmerBusInterface", jety.GetString("FarmerInterface"))
			CertHosts = jety.GetStringSlice("CertHosts")
			AdminPubKeys := jety.GetStringMap("pubkeys")
			if len(AdminPubKeys) == 0 {
				if keyList, found := os.LookupEnv("ADMIN_PUBKEYS"); found {
					pubkeys := strings.Split(keyList, ",")
					adminSet := make(map[string][]string)
					adminSet["admin"] = []string{}
					for _, v := range pubkeys {
						if v != "" {
							adminSet["admin"] = append(adminSet["admin"], v)
						}
					}
					jety.Set("pubkeys", adminSet)
				}
			}
			if CertHosts == nil {
				if hostList, found := os.LookupEnv("CERT_HOSTS"); found {
					hosts := strings.Split(hostList, ",")
					cleanHosts := []string{}
					for _, v := range hosts {
						if v != "" {
							cleanHosts = append(cleanHosts, v)
						}
					}
					jety.Set("CertHosts", cleanHosts)
				}
			}
			hosts := map[string]bool{"localhost": true, "127.0.0.1": true, "farmer": true, "grlx": true}
			fi := jety.GetString("FarmerInterface")
			if _, ok := hosts[fi]; fi != "" && !ok {
				hosts[fi] = true
			}
			chosts := []string{}
			for k := range hosts {
				chosts = append(chosts, k)
			}
			jety.SetDefault("CertHosts", chosts)

		case "sprout":
			jety.SetDefault("SproutID", "")
			jety.SetDefault("SproutPKI", "/etc/grlx/pki/sprout/")
			jety.SetDefault("SproutRootCA", "/etc/grlx/pki/sprout/tls-rootca.pem")
			jety.SetDefault("NKeySproutPubFile", "/etc/grlx/pki/sprout/sprout.nkey.pub")
			jety.SetDefault("NKeySproutPrivFile", "/etc/grlx/pki/sprout/sprout.nkey")
			jety.SetDefault("FarmerBusInterface", FarmerInterface+":"+jety.GetString("FarmerBusPort"))
			jety.SetDefault("CacheDir", "/var/cache/grlx/sprout/files/provided")
		}
		jety.WriteConfig()
	})

	CacheDir = jety.GetString("CacheDir")
	CertFile = jety.GetString("CertFile")
	CertHosts = jety.GetStringSlice("CertHosts")
	CertificateValidTime = jety.GetDuration("CertificateValidTime")
	ConfigRoot = jety.GetString("ConfigRoot")
	FarmerAPIPort = jety.GetString("FarmerAPIPort")
	FarmerBusInterface = jety.GetString("FarmerBusInterface")
	FarmerBusPort = jety.GetString("FarmerBusPort")
	FarmerInterface = jety.GetString("FarmerInterface")
	FarmerPKI = jety.GetString("FarmerPKI")
	GrlxRootCA = jety.GetString("GrlxRootCA")
	KeyFile = jety.GetString("KeyFile")
	NKeyFarmerPrivFile = jety.GetString("NKeyFarmerPrivFile")
	NKeyFarmerPubFile = jety.GetString("NKeyFarmerPubFile")
	NKeySproutPrivFile = jety.GetString("NKeySproutPrivFile")
	NKeySproutPubFile = jety.GetString("NKeySproutPubFile")
	Organization = jety.GetStringSlice("Organization")
	RootCA = jety.GetString("RootCA")
	RootCAPriv = jety.GetString("RootCAPriv")
	SproutID = jety.GetString("SproutID")
	SproutPKI = jety.GetString("SproutPKI")
	SproutRootCA = jety.GetString("SproutRootCA")
}

// TODO actually validate the base path exists
func BasePathValid() bool {
	return true
}

func Init() string {
	return jety.GetString("init")
}

func SetSproutID(id string) {
	jety.Set("SproutID", id)
	jety.WriteConfig()
}
