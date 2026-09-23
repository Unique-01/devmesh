package cli

import (
	"fmt"
	"os"

	"github.com/Unique-01/devmesh/internal"

	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start [name]",
	Short: "Start a project by name from anywhere (or the current project if no name)",
	Long: `With a name: starts a previously-run project using its saved metadata
(working directory, command, identity) — no need to cd into the project folder.

Without a name: identical to 'devmesh up' (starts the project in the current
directory).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return upCmd.RunE(upCmd, []string{})
		}
		return runStartNamed(args[0])
	},
}

// runStartNamed starts a previously-run project by its saved name, from any
// directory. It reuses the up command with the project's saved metadata.
func runStartNamed(name string) error {
	state, err := internal.LoadProjectState(name)
	if err != nil {
		return fmt.Errorf("no saved project named %q (run 'devmesh ps' to list saved projects)", name)
	}
	if state.PID > 0 && internal.IsProcessRunning(state.PID) {
		return fmt.Errorf("project %q is already running (PID %d). Use 'devmesh stop %s' or 'devmesh restart %s'", name, state.PID, name, name)
	}
	if state.Cmd == "" {
		return fmt.Errorf("saved project %q has no command recorded", name)
	}
	if state.Directory == "" {
		return fmt.Errorf("saved project %q has no working directory recorded", name)
	}
	if _, err := os.Stat(state.Directory); err != nil {
		return fmt.Errorf("project directory %s no longer exists", state.Directory)
	}

	origDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	if err := os.Chdir(state.Directory); err != nil {
		return fmt.Errorf("failed to enter project directory %s: %w", state.Directory, err)
	}
	defer os.Chdir(origDir)

	fmt.Printf("Starting saved project %q from %s\n", name, state.Directory)

	// Reuse the up flow with the saved identity/command.
	cmdFlag = state.Cmd
	nameFlag = state.Name
	return upCmd.RunE(upCmd, []string{})
}

func init() {
	rootCmd.AddCommand(startCmd)
}
