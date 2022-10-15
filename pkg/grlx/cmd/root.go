package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/gogrlx/grlx/config"
	"github.com/gogrlx/grlx/pkg/grlx/util"
	"github.com/gogrlx/grlx/pki"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile      string
	sproutTarget string
	outputMode   string
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
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	initConfig()
	// cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&outputMode, "out", "", "Format to print out response (where appropriate). Options are `json`, `yaml`, or `text`")
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		switch outputMode {
		case "json":
			fallthrough
		case "yaml":
			fallthrough
		case "":
			fallthrough
		case "text":
		default:
			fmt.Println("Valid --out modes: `json`, `yaml`, or `text`. Mode `" + outputMode + "` is invalid.")
			os.Exit(1)
		}
	}
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.grlx.yaml)")
	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	if !pki.RootCACached("grlx") {
		fmt.Print("The TLS certificate for this farmer is unknown. Would you like to download and trust it? ")
		shouldDownload, err := util.UserConfirmWithDefault(true)
		for err != nil {
			shouldDownload, err = util.UserConfirmWithDefault(true)
		}
		if !shouldDownload {
			fmt.Println("No certificate, exiting!")
			os.Exit(1)
		}
	}
	err := pki.LoadRootCA("grlx")
	if err != nil {
		fmt.Printf("%v", err)
		color.Red("The RootCA could not be loaded from %s. Exiting!", viper.GetString("GrlxRootCA"))
		os.Exit(1)
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		config.LoadConfig("grlx")
	}
	viper.AutomaticEnv() // read in environment variables that match
}
