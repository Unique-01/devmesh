package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "devmesh",
	Short: "DevMesh is a CLI tool for managing development environments and meshes",
	Long:  `A robust CLI application scaffolded in Go using Cobra, designed for managing your development workflow and mesh services.`,
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
