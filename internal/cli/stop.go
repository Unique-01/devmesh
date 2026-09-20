package cli

import (
	"devmesh/internal"
	"fmt"
	"os"
	"runtime"
	"syscall"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop [name]",
	Short: "Stop a project and remove its active proxy route",
	Long: `Stops a running project and removes its active proxy route while preserving its .devmesh.yaml and state. 
	Specify a project name to stop it from anywhere, or omit the name to stop the current project`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return runStop()
		}
		return runStopNamed(args[0])
	},
}

// stopProjectState terminates a project's process group, deregisters its
// proxy routes, and persists PID=0. Shared by down/stop/remove flows.
func stopProjectState(state internal.ProjectState) {
	if state.PID > 0 && internal.IsProcessRunning(state.PID) {
		fmt.Printf("Stopping process group for project %q...\n", state.Name)
		// Use negative PID to signal process group on Unix
		if runtime.GOOS != "windows" {
			_ = syscall.Kill(-state.PID, syscall.SIGTERM)
		} else {
			proc, err := os.FindProcess(state.PID)
			if err == nil && proc != nil {
				_ = proc.Kill()
			}
		}
	}

	// Deregister from running proxy if it's up
	_ = internal.DeregisterRoute("127.0.0.1:80", state.Domain)
	_ = internal.DeregisterRoute("127.0.0.1:8080", state.Domain)

	// Update state: PID = 0, keep config and registration
	state.PID = 0
	_ = internal.SaveProjectState(state)
}

func runStop() error {
	cfg, err := loadProjectConfig()
	if err != nil {
		return fmt.Errorf("error loading project config: %w", err)
	}

	ident, err := internal.ResolveIdentity("", cfg.Name)
	if err != nil {
		return fmt.Errorf("failed to resolve project identity: %w", err)
	}

	// Load state if exists
	state, err := internal.LoadProjectState(ident.Name)
	if err != nil {
		state = internal.ProjectState{
			Name:   ident.Name,
			Domain: ident.Domain,
		}
	}

	stopProjectState(state)

	fmt.Printf("Project %q is now stopped. .devmesh.yaml retained.\n", ident.Name)
	return nil
}

// runStopNamed stops a previously-run project by its saved name.
func runStopNamed(name string) error {
	state, err := internal.LoadProjectState(name)
	if err != nil {
		return fmt.Errorf("no saved project named %q (run 'devmesh status' to list saved projects)", name)
	}
	stopProjectState(state)
	fmt.Printf("Project %q is now stopped. .devmesh.yaml retained.\n", state.Name)
	return nil
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
