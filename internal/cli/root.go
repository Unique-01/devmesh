package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "devmesh",
	Short: "DevMesh is a local development networking CLI that gives projects stable local domains while automatically managing their underlying ports.",
	Long:  `DevMesh is a local development networking CLI that gives projects stable local domains while automatically managing their underlying ports.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Welcome to DevMesh CLI! Use --help to see available commands.")
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
