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

var proxyStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the proxy service and daemon status",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := systemdAvailable(); err == nil {
			out, err := exec.Command("systemctl", "is-active", serviceName).Output()
			fmt.Printf("systemd unit %s: %s\n", serviceName, strings.TrimSpace(string(out)))
			if err != nil {
				fmt.Printf("  (not running; if installed and failing, inspect: journalctl -u %s -n 20 --no-pager)\n", serviceName)
			}
		}
		if internal.PingDaemon("http://127.0.0.1:80") {
			fmt.Println("Proxy daemon reachable on 127.0.0.1:80 — clean URLs ready (http://<app>.localhost)")
		} else if internal.PingDaemon("http://127.0.0.1:8080") {
			fmt.Println("Proxy daemon reachable on 127.0.0.1:8080 — URLs need :8080 until the service is installed on :80")
		} else {
			fmt.Println("No proxy daemon running (it will be spawned on :8080 on the next 'devmesh up')")
		}
		return nil
	},
}

func init() {
	proxyCmd.AddCommand(proxyStartCmd)
	proxyCmd.AddCommand(proxyStopCmd)
	proxyCmd.AddCommand(proxyRemoveCmd)
	proxyCmd.AddCommand(proxyStatusCmd)
}
