package cmd

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/gogrlx/grlx/v2/internal/api/client"
	"github.com/gogrlx/grlx/v2/internal/config"
)

// testCmd represents the test command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Check the cli and farmer versions",
	Run: func(cmd *cobra.Command, _ []string) {
		grlxVersion := BuildInfo
		serverVersion, err := client.GetVersion()
		cv := config.CombinedVersion{
			CLI:    grlxVersion,
			Farmer: serverVersion,
		}
		if err != nil {
			cv.Error = err.Error()
		}
		switch outputMode {
		case "json":
			jw, _ := json.Marshal(cv)
			fmt.Println(string(jw))
			return
		case "":
			fallthrough
		case "text":
			formatter := "%s Version:\n\tTag: %s\n\tCommit: %s\n\tArch: %s\n\tCompiler: %s\n"
			fmt.Printf(formatter, "CLI", grlxVersion.Tag, grlxVersion.GitCommit, grlxVersion.Arch, grlxVersion.Compiler)
			if err != nil {
				log.Println("Error fetching Farmer version: " + err.Error())
				return
			}
			fmt.Printf(formatter, "Farmer", serverVersion.Tag, serverVersion.GitCommit, serverVersion.Arch, serverVersion.Compiler)
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
