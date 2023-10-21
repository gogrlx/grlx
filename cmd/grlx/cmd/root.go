package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/gogrlx/grlx/api/client"
	"github.com/gogrlx/grlx/cmd/grlx/util"
	"github.com/gogrlx/grlx/config"
	"github.com/gogrlx/grlx/pki"
	"github.com/gogrlx/grlx/types"
)

var (
	cfgFile      string
	sproutTarget string
	outputMode   string
	BuildInfo    types.Version
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "grlx",
	Short: "",
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(buildInfo types.Version) {
	BuildInfo = buildInfo
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	initConfig()
	// cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&outputMode, "out", "", "Format to print out response (where appropriate). Options are `json`, or `text`")
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		switch outputMode {
		case "json":
			fallthrough
		case "":
			fallthrough
		case "text":
		default:
			fmt.Println("Valid --out modes: `json`, or `text`. Mode `" + outputMode + "` is invalid.")
			os.Exit(1)
		}
	}
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/grlx/grlx)")
	noFailForCert := false
	if len(os.Args) > 1 {
		noFailForCert = os.Args[1] == "version" || os.Args[1] == "help" || os.Args[1] == "auth" || os.Args[1] == "init"
	}
	isInit := false
	if len(os.Args) > 1 {
		isInit = os.Args[1] == "init"
	}
	if !pki.RootCACached("grlx") && !isInit {
		fmt.Print("The TLS certificate for this farmer is unknown. Would you like to download and trust it? ")
		shouldDownload, err := util.UserConfirmWithDefault(true)
		for err != nil {
			shouldDownload, err = util.UserConfirmWithDefault(true)
		}
		if !shouldDownload && !noFailForCert {
			fmt.Println("No certificate, exiting!")
			os.Exit(1)
		}
		pki.FetchRootCA(config.GrlxRootCA)
	}
	err := pki.LoadRootCA("grlx")

	if err != nil && !noFailForCert {
		fmt.Printf("error: %v\n", err)
		color.Red("The RootCA could not be loaded from %s. Exiting!", config.GrlxRootCA)
		os.Exit(1)
	}
	err = client.CreateSecureTransport()
	if err != nil && !noFailForCert {
		if os.Args[1] != "version" {
			fmt.Printf("error: %v\n", err)
			color.Red("The API client could not be created. Exiting!")
			os.Exit(1)
		}
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		config.SetConfigFile(cfgFile)
	}
	config.LoadConfig("grlx")
	viper.AutomaticEnv() // read in environment variables that match
}
