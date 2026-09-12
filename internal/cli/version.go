package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of DevMesh",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("DevMesh CLI v0.1.1")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
