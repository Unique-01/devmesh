package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/Unique-01/devmesh/internal"

	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove [name]",
	Short: "Completely remove project (stop, clean routes, delete .devmesh.yaml)",
	Long:  `Stops project if running, removes active route, deletes .devmesh.yaml, and deletes state after confirmation.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := ".devmesh.yaml"

		var ident internal.ProjectIdentity
		var err error
		if len(args) == 0 {
			cfg, err := loadProjectConfig()
			if err != nil {
				return fmt.Errorf("error loading project config: %w", err)
			}

			ident, err = internal.ResolveIdentity("", cfg.Name)
			if err != nil {
				return fmt.Errorf("failed to resolve project identity: %w", err)
			}

		} else {
			ident, err = internal.ResolveIdentity("", args[0])
			if err != nil {
				return fmt.Errorf("failed to resolve project identity: %w", err)
			}
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
		err = runStopNamed(ident.Name)
		if err != nil {
			return fmt.Errorf("failed to stop project: %w", err)
		}

		// 2. Delete .devmesh.yaml
		if _, err := os.Stat(configPath); err == nil {
			if err := os.Remove(configPath); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to delete %s: %v\n", configPath, err)
			} else {
				fmt.Printf("Deleted %s\n", configPath)
			}
		}

		// 3. Delete state file
		err = internal.RemoveProjectState(ident.Name)
		if err != nil {
			return err
		}

		fmt.Printf("Project %q successfully removed.\n", ident.Name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)
}
