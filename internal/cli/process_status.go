package cli

import (
	"devmesh/internal"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var psCmd = &cobra.Command{
	Use:   "ps [name]",
	Short: "Show status of devmesh projects",
	Long:  `Display status of registered devmesh projects. With no argument, shows all projects. With a project name, shows detail for that one project.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			return showSingleProjectStatus(args[0])
		} else {
			return showAllProjectStatus()
		}
	},
}

func showAllProjectStatus() error {
	states, err := internal.ListAllProjectStates()
	if err != nil {
		return fmt.Errorf("failed to list projects: %w", err)
	}

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
				if err := internal.SaveProjectState(s); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to update stale state for %s: %v\n", s.Name, err)
				}
			}
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", s.Name, s.Domain, portStr, pidStr, statusStr)
	}
	w.Flush()
	return nil
}

func showSingleProjectStatus(projectName string) error {
	project, err := internal.LoadProjectState(projectName)
	if err != nil {
		return fmt.Errorf("failed to load project state: %w", err)
	}
	running := internal.IsProcessRunning(project.PID)
	statusStr := "○ stopped"
	pidStr := "—"
	portStr := "—"
	if project.Port > 0 {
		portStr = fmt.Sprintf("%d", project.Port)
	}
	if running {
		statusStr = "● running"
		pidStr = fmt.Sprintf("%d", project.PID)
	} else {
		if project.PID > 0 {
			project.PID = 0
			err = internal.SaveProjectState(project)
			if err != nil {
				return fmt.Errorf("failed to save project state: %w", err)
			}
		}
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
	fmt.Fprintf(w, "%s status\n", projectName)
	fmt.Fprintln(w, "____________________________")
	fmt.Fprintf(w, "Name:\t%s\n", project.Name)
	fmt.Fprintf(w, "Domain:\t%s\n", project.Domain)
	fmt.Fprintf(w, "Port:\t%s\n", portStr)
	fmt.Fprintf(w, "PID:\t%s\n", pidStr)
	fmt.Fprintf(w, "Command:\t%s\n", project.Cmd)
	fmt.Fprintf(w, "Directory:\t%s\n", project.Directory)
	fmt.Fprintf(w, "Current Status:\t%s", statusStr)

	w.Flush()
	return nil

}

func init() {
	rootCmd.AddCommand(psCmd)
}
