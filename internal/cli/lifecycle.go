package cli

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"
	"text/tabwriter"

	"devmesh/internal"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of devmesh projects",
	Long:  `Display status of registered devmesh projects across the system, showing project domain, port, PID, and running status.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		states, _ := internal.ListAllProjectStates()

		fmt.Println("DEV MESH")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "PROJECT\tDOMAIN\tPORT\tPID\tSTATUS")
		fmt.Fprintln(w, "---------------------------------------------------------")

		for _, s := range states {
			running := internal.IsProcessRunning(s.PID)
			statusStr := "○ stopped"
			pidStr := "—"
			portStr := "—"
			if s.Port > 0 {
				portStr = fmt.Sprintf("%d", s.Port)
			}
			if running {
				statusStr = "● running"
				pidStr = fmt.Sprintf("%d", s.PID)
			} else {
				if s.PID > 0 {
					s.PID = 0
					_ = internal.SaveProjectState(s)
				}
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", s.Name, s.Domain, portStr, pidStr, statusStr)
		}
		w.Flush()
		return nil
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered projects",
	Long:  `Shows all registered devmesh projects found in the system.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		states, _ := internal.ListAllProjectStates()

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
		fmt.Fprintln(w, "PROJECT\tDOMAIN")
		fmt.Fprintln(w, "-----------------------------")

		for _, s := range states {
			fmt.Fprintf(w, "%s\t%s\n", s.Name, s.Domain)
		}
		w.Flush()
		return nil
	},
}

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop the current project development process and remove its active route",
	Long:  `Finds the project, terminates running process, removes active route from proxy/hosts, while keeping .devmesh.yaml and state so 'devmesh up' works again.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := ".devmesh.yaml"
		var configName, configDomain string
		if _, err := os.Stat(configPath); err == nil {
			if data, err := os.ReadFile(configPath); err == nil {
				if cfg, err := parseConfigYaml(data); err == nil {
					configName = cfg.Name
					configDomain = cfg.Domain
				}
			}
		}

		ident, err := internal.ResolveIdentity("", "", configName, configDomain)
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

		// Terminate process group if PID is running
		if state.PID > 0 && internal.IsProcessRunning(state.PID) {
			fmt.Printf("Stopping process group for project %q...\n", ident.Name)
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

		// Remove hosts entry
		hostsMgr := internal.NewHostsManager("")
		_ = hostsMgr.RemoveEntry(ident.Domain)
		fmt.Printf("Removed /etc/hosts entry for %s\n", ident.Domain)

		// Deregister from running proxy if it's up
		_ = internal.DeregisterRoute("127.0.0.1:80", ident.Domain)
		_ = internal.DeregisterRoute("127.0.0.1:8080", ident.Domain)

		// Update state: PID = 0, keep config and registration
		state.PID = 0
		_ = internal.SaveProjectState(state)

		fmt.Printf("Project %q is now stopped. .devmesh.yaml retained.\n", ident.Name)
		return nil
	},
}

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the project (down followed by up)",
	Long:  `Stops the running project processes and routes, then starts it again using 'devmesh up'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Run down first (ignore errors if not running)
		_ = downCmd.RunE(downCmd, args)

		// Then run up
		return upCmd.RunE(upCmd, args)
	},
}

var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "Completely remove project (stop, clean routes/hosts, delete .devmesh.yaml)",
	Long:  `Stops project if running, removes active route, removes hosts entry, deletes .devmesh.yaml, and deletes state after confirmation.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := ".devmesh.yaml"
		var configName, configDomain string
		if _, err := os.Stat(configPath); err == nil {
			if data, err := os.ReadFile(configPath); err == nil {
				if cfg, err := parseConfigYaml(data); err == nil {
					configName = cfg.Name
					configDomain = cfg.Domain
				}
			}
		}

		ident, err := internal.ResolveIdentity("", "", configName, configDomain)
		if err != nil {
			return fmt.Errorf("failed to resolve project identity: %w", err)
		}

		fmt.Printf("Are you sure you want to completely remove project %q (%s)? This is destructive [y/N]: ", ident.Name, ident.Domain)
		var response string
		_, _ = fmt.Scanln(&response)
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Removal cancelled.")
			return nil
		}

		// 1. Stop project if running
		_ = downCmd.RunE(downCmd, args)

		// 2. Remove hosts entry
		hostsMgr := internal.NewHostsManager("")
		_ = hostsMgr.RemoveEntry(ident.Domain)

		// 3. Deregister from running proxy
		_ = internal.DeregisterRoute("127.0.0.1:80", ident.Domain)
		_ = internal.DeregisterRoute("127.0.0.1:8080", ident.Domain)

		// 4. Delete .devmesh.yaml
		if _, err := os.Stat(configPath); err == nil {
			if err := os.Remove(configPath); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to delete %s: %v\n", configPath, err)
			} else {
				fmt.Printf("Deleted %s\n", configPath)
			}
		}

		// 4. Delete state file
		_ = internal.RemoveProjectState(ident.Name)

		fmt.Printf("Project %q successfully removed.\n", ident.Name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(downCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(removeCmd)
}
