package cli

import (


	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart [name]",
	Short: "Restart a project by name from anywhere (or the current project if no name)",
	Long:  `Stops the project's running processes and routes, then starts it again.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			// Current directory: down, then up
			err := runStop()
			if err != nil {
				return err
			}
			return upCmd.RunE(upCmd, args)
		}
		if err := runStopNamed(args[0]); err != nil {
			return err
		}
		return runStartNamed(args[0])
	},
}



func init() {
	rootCmd.AddCommand(restartCmd)
}
