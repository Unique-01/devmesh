package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of DevMesh",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("DevMesh CLI v%s\n", version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
