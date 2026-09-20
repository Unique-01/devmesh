package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const banner = `
░       ░░        ░  ░░░░  ░  ░░░░  ░░        ░░      ░░  ░░░░  ░
▒  ▒▒▒▒  ▒  ▒▒▒▒▒▒▒  ▒▒▒▒  ▒   ▒▒   ▒▒  ▒▒▒▒▒▒▒  ▒▒▒▒▒▒▒  ▒▒▒▒  ▒
▓  ▓▓▓▓  ▓      ▓▓▓▓  ▓▓  ▓▓        ▓▓      ▓▓▓▓      ▓▓        ▓
█  ████  █  █████████    ███  █  █  ██  █████████████  █  ████  █
█       ██        ████  ████  ████  ██        ██      ██  ████  █
                                                                      `

var rootCmd = &cobra.Command{
	Use:   "devmesh",
	Short: "DevMesh is a local development networking CLI that gives projects stable local domains while automatically managing their underlying ports.",
	Long:  `DevMesh is a local development networking CLI that gives projects stable local domains while automatically managing their underlying ports.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if isTerminal() {
			fmt.Println(banner)
		}
		if _, ok := cmd.Annotations["allowRoot"]; ok {
			return nil
		}
		if os.Getuid() == 0 {
			return fmt.Errorf("command does not support running as root — try again without sudo")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Welcome to DevMesh CLI! Use --help to see available commands.")
	},
}

func isTerminal() bool {
	stat, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
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
