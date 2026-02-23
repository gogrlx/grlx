package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/taigrr/jety"
	"github.com/taigrr/log-socket/log"

	"github.com/gogrlx/grlx/v2/types"
)

const GrlxExt = "grlx"

var BuildInfo types.Version

var configLoaded sync.Once

var (
	AdminPubkeys         []string
	APIIdleTimeout       time.Duration
	APIReadTimeout       time.Duration
	APIWriteTimeout      time.Duration
	CacheDir             string
	CertFile             string
	CertHosts            []string
	CertificateValidTime time.Duration
	ConfigRoot           string
	FarmerAPIPort        string
	FarmerBusURL         string
	FarmerBusPort        string
	FarmerInterface      string
	FarmerOrganization   string
	FarmerPKI            string
	FarmerURL            string
	GrlxRootCA           string
	JobLogDir            string
	KeyFile              string
	LogLevel             log.Level
	NKeyFarmerPrivFile   string
	NKeyFarmerPubFile    string
	NKeySproutPrivFile   string
	NKeySproutPubFile    string
	// TODO the final path arg should be dynamic to allow for dev/prod/etc
	RecipeDir    = filepath.Join("/", "srv", "grlx", "recipes", "prod")
	RootCA       string
	RootCAPriv   string
	SproutID     string
	SproutPKI    string
	SproutRootCA string
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
		jety.SetDefault("loglevel", "info")
		jety.SetDefault("cachedir", "/var/cache/grlx/sprout/files/provided")
		jety.SetDefault("configroot", "/etc/grlx/")
		jety.SetDefault("farmerinterface", "localhost")
		jety.SetDefault("farmerapiport", "5405")
		jety.SetDefault("farmerbusport", "5406")
		switch binary {
		case "grlx":
			dirname, err := os.UserHomeDir()
			if err != nil {
				log.Fatal(err)
			}
			configDir := os.Getenv("XDG_CONFIG_HOME")
			if configDir == "" {
				configDir = filepath.Join(dirname, ".config")
			}
			certPath := filepath.Join(configDir, "grlx/tls-rootca.pem")
			jety.Set("grlxrootca", certPath)
		case "farmer":
			jety.SetDefault("apiwritetimeout", 120*time.Second)
			jety.SetDefault("apireadtimeout", 120*time.Second)
			jety.SetDefault("apiidletimeout", 120*time.Second)
			jety.SetDefault("certificatevalidtime", 365*24*time.Hour)
			jety.SetDefault("certfile", "/etc/grlx/pki/farmer/tls-cert.pem")
			jety.SetDefault("farmerpki", "/etc/grlx/pki/farmer/")
			jety.SetDefault("keyfile", "/etc/grlx/pki/farmer/tls-key.pem")
			jety.SetDefault("joblogdir", "/var/cache/grlx/farmer/jobs")
			jety.SetDefault("nkeyfarmerpubfile", "/etc/grlx/pki/farmer/farmer.nkey.pub")
			jety.SetDefault("nkeyfarmerprivfile", "/etc/grlx/pki/farmer/farmer.nkey")
			jety.SetDefault("rootca", "/etc/grlx/pki/farmer/tls-rootca.pem")
			jety.SetDefault("rootcapriv", "/etc/grlx/pki/farmer/tls-rootca-key.pem")
			jety.SetDefault("farmerorganization", "grlx farmer")
			JobLogDir = jety.GetString("joblogdir")
			CertHosts = jety.GetStringSlice("certhosts")

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
			AdminPubKeys = jety.GetStringMap("pubkeys")
			if len(CertHosts) == 0 {
				if hostList, found := os.LookupEnv("CERT_HOSTS"); found {
					hosts := strings.Split(hostList, ",")
					cleanHosts := []string{}
					for _, v := range hosts {
						if v != "" {
							cleanHosts = append(cleanHosts, v)
						}
					}
					jety.Set("certhosts", cleanHosts)
				}
			}
			if AdminPubKeys["admin"] != nil {
				anyKeys, ok := AdminPubKeys["admin"].([]any)
				if !ok {
					log.Fatal("pubkeys > admin is not a slice")
				}
				for _, v := range anyKeys {
					if v, ok := v.(string); ok {
						AdminPubkeys = append(AdminPubkeys, v)
					}
				}

			}
			hosts := map[string]bool{"localhost": true, "127.0.0.1": true, "farmer": true, "grlx": true}
			fi := jety.GetString("farmerinterface")
			if _, ok := hosts[fi]; fi != "" && !ok {
				hosts[fi] = true
			}
			chosts := []string{}
			for k := range hosts {
				chosts = append(chosts, k)
			}
			jety.SetDefault("certhosts", chosts)
			CertHosts = jety.GetStringSlice("certhosts")

		case "sprout":
			jety.SetDefault("sproutid", "")
			jety.SetDefault("sproutpki", "/etc/grlx/pki/sprout/")
			jety.SetDefault("sproutrootca", "/etc/grlx/pki/sprout/tls-rootca.pem")
			jety.SetDefault("nkeysproutpubfile", "/etc/grlx/pki/sprout/sprout.nkey.pub")
			jety.SetDefault("joblogdir", "/var/cache/grlx/sprout/jobs")
			jety.SetDefault("nkeysproutprivfile", "/etc/grlx/pki/sprout/sprout.nkey")
			jety.SetDefault("cachedir", "/var/cache/grlx/sprout/files/provided")

			JobLogDir = jety.GetString("joblogdir")
		}
		jety.WriteConfig()
	})
	logLevel := jety.GetString("loglevel")
	switch logLevel {
	case "debug":
		LogLevel = log.LDebug
	case "info":
		LogLevel = log.LInfo
	case "notice":
		LogLevel = log.LNotice
	case "warn":
		LogLevel = log.LWarn
	case "error":
		LogLevel = log.LError
	case "panic":
		LogLevel = log.LPanic
	case "fatal":
		LogLevel = log.LFatal
	default:
		LogLevel = log.LNotice
	}
	APIIdleTimeout = jety.GetDuration("apiidletimeout")
	APIReadTimeout = jety.GetDuration("apireadtimeout")
	APIWriteTimeout = jety.GetDuration("apiwritetimeout")
	CacheDir = jety.GetString("cachedir")
	CertFile = jety.GetString("certfile")
	CertHosts = jety.GetStringSlice("certhosts")
	CertificateValidTime = jety.GetDuration("certificatevalidtime")
	ConfigRoot = jety.GetString("configroot")
	FarmerAPIPort = jety.GetString("farmerapiport")
	FarmerBusURL = jety.GetString("farmerinterface") + ":" + jety.GetString("farmerbusport")
	FarmerBusPort = jety.GetString("farmerbusport")
	FarmerInterface = jety.GetString("farmerinterface")
	FarmerPKI = jety.GetString("farmerpki")
	FarmerURL = "https://" + jety.GetString("farmerinterface") + ":" + jety.GetString("farmerapiport")
	GrlxRootCA = jety.GetString("grlxrootca")
	KeyFile = jety.GetString("keyfile")
	NKeyFarmerPrivFile = jety.GetString("nkeyfarmerprivfile")
	NKeyFarmerPubFile = jety.GetString("nkeyfarmerpubfile")
	NKeySproutPrivFile = jety.GetString("nkeysproutprivfile")
	NKeySproutPubFile = jety.GetString("nkeysproutpubfile")
	FarmerOrganization = jety.GetString("farmerorganization")
	RootCA = jety.GetString("rootca")
	RootCAPriv = jety.GetString("rootcapriv")
	SproutID = jety.GetString("sproutid")
	SproutPKI = jety.GetString("sproutpki")
	SproutRootCA = jety.GetString("sproutrootca")
}

// TODO actually validate the base path exists
func BasePathValid() bool {
	return true
}

func Init() string {
	return jety.GetString("init")
}

func SetSproutID(id string) {
	jety.Set("sproutid", id)
	jety.WriteConfig()
}
