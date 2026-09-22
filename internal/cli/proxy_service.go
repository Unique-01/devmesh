package cli

import (
	"fmt"
	"os/exec"
	"strings"

	"devmesh/internal"

	"github.com/spf13/cobra"
)

// Proxy service lifecycle subcommands (devmesh proxy start|stop|remove|status).
// One-time setup lives in the top-level `devmesh install`; full teardown in
// `devmesh uninstall`.

var proxyStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the background proxy service (requires sudo)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := systemdAvailable(); err != nil {
			return err
		}
		if err := requireRoot("proxy start"); err != nil {
			return err
		}
		if out, err := exec.Command("systemctl", "start", serviceName).CombinedOutput(); err != nil {
			return fmt.Errorf("systemctl start %s failed: %w: %s", serviceName, err, strings.TrimSpace(string(out)))
		}
		if internal.PingDaemon("http://127.0.0.1:80") {
			fmt.Println("DevMesh proxy service started on http://127.0.0.1:80")
		} else {
			fmt.Printf("Service start command issued, but the proxy is not responding on :80 yet. Check: journalctl -u %s -n 20\n", serviceName)
		}
		return nil
	},
}

var proxyStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the background proxy service without removing it (requires sudo)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := systemdAvailable(); err != nil {
			return err
		}
		if err := requireRoot("proxy stop"); err != nil {
			return err
		}
		if out, err := exec.Command("systemctl", "stop", serviceName).CombinedOutput(); err != nil {
			return fmt.Errorf("systemctl stop %s failed: %w: %s", serviceName, err, strings.TrimSpace(string(out)))
		}
		fmt.Println("DevMesh proxy service stopped (unit kept; start again with 'sudo devmesh proxy start').")
		return nil
	},
}

var proxyRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove the background proxy service, keeping the CLI and data (requires sudo)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := systemdAvailable(); err != nil {
			return err
		}
		if err := requireRoot("proxy remove"); err != nil {
			return err
		}

		if err := removeServiceUnit(); err != nil {
			return err
		}

		fmt.Println("DevMesh proxy service removed (CLI binary and ~/.devmesh data kept).")
		fmt.Println("For complete removal, run 'sudo devmesh uninstall'.")
		return nil
	},
}



func init() {
	proxyCmd.AddCommand(proxyStartCmd)
	proxyCmd.AddCommand(proxyStopCmd)
	proxyCmd.AddCommand(proxyRemoveCmd)
}
