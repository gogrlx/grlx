package cmd

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/gogrlx/grlx/api/client"
	"github.com/gogrlx/grlx/types"
)

// testCmd represents the test command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "check the cli and server versions",
	Run: func(cmd *cobra.Command, _ []string) {
		grlxVersion := BuildInfo
		serverVersion, err := client.GetVersion()
		cv := types.CombinedVersion{
			CLI:    grlxVersion,
			Farmer: serverVersion,
			Error:  err.Error(),
		}
		switch outputMode {
		case "json":
			jw, _ := json.Marshal(cv)
			fmt.Println(string(jw))
			return
		case "":
			fallthrough
		case "text":
			formatter := "%s Version:\n\tTag: %s\n\tCommit: %s\n\tBuild Time: %s\n\tArch: %s\n\tCompiler: %s\n"
			fmt.Printf(formatter, "CLI", grlxVersion.Tag, grlxVersion.GitCommit, grlxVersion.BuildTime, grlxVersion.Arch, grlxVersion.Compiler)
			if err != nil {
				log.Println("Error fetching Farmer version: " + err.Error())
				return
			}
			fmt.Printf(formatter, "Farmer", serverVersion.Tag, serverVersion.GitCommit, serverVersion.BuildTime, serverVersion.Arch)
		case "yaml":
			// TODO implement YAML
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
