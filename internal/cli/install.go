package cli

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Unique-01/devmesh/internal"

	"github.com/spf13/cobra"
)

const (
	serviceName     = "devmesh-proxy"
	serviceUnitPath = "/etc/systemd/system/devmesh-proxy.service"
	serviceBinPath  = "/usr/local/bin/devmesh"
)

// installBinary copies the current devmesh binary to dst (typically
// /usr/local/bin/devmesh). It writes to a temp file and renames, so the copy
// succeeds even when dst is currently being executed by the proxy service
// (writing to a running binary directly fails with ETXTBSY; replacing it via
// rename is atomic and safe on Linux).
func installBinary(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", src, err)
	}
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0755); err != nil {
		return fmt.Errorf("failed to write %s: %w", tmp, err)
	}
	if err := os.Chmod(tmp, 0755); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("failed to set executable bit on %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("failed to replace %s: %w", dst, err)
	}
	return nil
}

// buildUnitContent renders the systemd unit for the proxy daemon. The service
// runs as the invoking user and receives only CAP_NET_BIND_SERVICE, so it can
// bind :80 without running as root and keeps the user's HOME/PATH/state.
func buildUnitContent(binPath, userName, homeDir string) string {
	return fmt.Sprintf(`[Unit]
Description=DevMesh reverse proxy daemon
After=network.target

[Service]
User=%s
Environment=HOME=%s
ExecStart=%s proxy --addr 127.0.0.1:80
Restart=on-failure
RestartSec=2
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
`, userName, homeDir, binPath)
}

// serviceIdentity returns the user the proxy service should run as: the sudo
// invoker when elevated, otherwise the current user.
func serviceIdentity() (name, home string, err error) {
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" && sudoUser != "root" {
		if u, lookupErr := user.Lookup(sudoUser); lookupErr == nil {
			return u.Username, u.HomeDir, nil
		}
	}
	current, err := user.Current()
	if err != nil {
		return "", "", err
	}
	return current.Username, current.HomeDir, nil
}

func systemdAvailable() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("this command is only supported on Linux (systemd) for now")
	}
	if _, err := os.Stat("/run/systemd/system"); err != nil {
		return fmt.Errorf("systemd not detected on this system")
	}
	return nil
}

// removeServiceUnit stops, disables, and deletes the proxy systemd unit.
func removeServiceUnit() error {
	_ = exec.Command("systemctl", "disable", "--now", serviceName).Run()
	if err := os.Remove(serviceUnitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove %s: %w", serviceUnitPath, err)
	}
	_ = exec.Command("systemctl", "daemon-reload").Run()
	return nil
}

func requireRoot(action string) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("%s requires root: run 'sudo devmesh %s'", action, action)
	}
	return nil
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install DevMesh system-wide: CLI to /usr/local/bin + always-on proxy service (requires sudo once)",
	Long: `One-time setup (run with sudo):

  1. Installs the CLI to /usr/local/bin/devmesh (callable from anywhere).
  2. Installs and starts the proxy as a systemd service on :80, running as
     your user with CAP_NET_BIND_SERVICE (binds :80 without root).

After this, no command needs elevated privileges. Re-run after rebuilding
devmesh to refresh the service binary.`,
	Annotations: map[string]string{"allowRoot": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := systemdAvailable(); err != nil {
			return err
		}
		if err := requireRoot("install"); err != nil {
			return err
		}

		binPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to resolve binary path: %w", err)
		}
		if resolved, err := filepath.EvalSymlinks(binPath); err == nil {
			binPath = resolved
		}

		// Install the binary into a stable system location. Systemd (and
		// SELinux on Fedora) refuse to execute binaries from user home
		// directories (user_home_t), so the service must point at a
		// bin_t-labeled path like /usr/local/bin.
		if binPath != serviceBinPath {
			if err := installBinary(binPath, serviceBinPath); err != nil {
				return fmt.Errorf("failed to install binary: %w", err)
			}
			fmt.Printf("Installed binary: %s -> %s\n", binPath, serviceBinPath)
		}
		if info, err := os.Stat(serviceBinPath); err != nil || info.IsDir() {
			return fmt.Errorf("service binary %s is missing or not executable", serviceBinPath)
		}

		userName, homeDir, err := serviceIdentity()
		if err != nil {
			return fmt.Errorf("failed to resolve service user: %w", err)
		}

		unit := buildUnitContent(serviceBinPath, userName, homeDir)
		if err := os.WriteFile(serviceUnitPath, []byte(unit), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", serviceUnitPath, err)
		}

		if out, err := exec.Command("systemctl", "daemon-reload").CombinedOutput(); err != nil {
			return fmt.Errorf("systemctl daemon-reload failed: %w: %s", err, strings.TrimSpace(string(out)))
		}
		if out, err := exec.Command("systemctl", "enable", serviceName).CombinedOutput(); err != nil {
			return fmt.Errorf("systemctl enable %s failed: %w: %s", serviceName, err, strings.TrimSpace(string(out)))
		}
		// restart covers both a stopped unit (fresh install) and a running
		// one (re-install after rebuilding the binary).
		if out, err := exec.Command("systemctl", "restart", serviceName).CombinedOutput(); err != nil {
			return fmt.Errorf("systemctl restart %s failed: %w: %s", serviceName, err, strings.TrimSpace(string(out)))
		}

		if internal.WaitForDaemon("http://127.0.0.1:80") == nil {
			fmt.Printf("DevMesh proxy service installed, enabled, and running on http://127.0.0.1:80 (user: %s)\n", userName)
			fmt.Println("Your apps are now reachable as http://<name>.localhost.")
		} else {
			fmt.Println("Service installed and enabled, but the proxy is not responding on :80 yet. Recent logs:")
			if out, jErr := exec.Command("journalctl", "-u", serviceName, "-n", "15", "--no-pager").CombinedOutput(); jErr == nil {
				fmt.Println(string(out))
			} else {
				fmt.Printf("  (could not read journal: %v) Check: journalctl -u %s -n 20\n", jErr, serviceName)
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
}
