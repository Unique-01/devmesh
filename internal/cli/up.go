package cli

import (
	"context"
	"fmt"
	"os"

	"devmesh/internal/process"
	"devmesh/proxy"

	"github.com/spf13/cobra"
)

var (
	cmdFlag string
	portFlag int
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start development command with automatic PORT assignment and proxy routing",
	Long:  `Start development command with automatic PORT assignment (injected as PORT environment variable) and proxy routing.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if cmdFlag == "" {
			return fmt.Errorf("required flag \"cmd\" not set (e.g. devmesh up --cmd \"pnpm dev\")")
		}

		// Allocate port
		port, err := proxy.AllocatePort(portFlag)
		if err != nil {
			return fmt.Errorf("failed to allocate port: %w", err)
		}

		fmt.Printf("Starting DevMesh development service on port %d for command: %s\n", port, cmdFlag)

		mgr := process.NewManager(cmdFlag, port)
		fmt.Printf("Process assigned PID tracker. Spawning...\n")

		ctx := context.Background()
		if err := mgr.Run(ctx, nil, os.Stdout, os.Stderr); err != nil {
			return fmt.Errorf("process execution failed: %w", err)
		}

		return nil
	},
}

func init() {
	upCmd.Flags().StringVar(&cmdFlag, "cmd", "", "Development command to run (e.g. \"pnpm dev\")")
	upCmd.Flags().IntVar(&portFlag, "port", 0, "Preferred port number (optional)")
	rootCmd.AddCommand(upCmd)
}
