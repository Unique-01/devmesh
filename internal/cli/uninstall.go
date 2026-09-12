package cli

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"devmesh/internal"

	"github.com/spf13/cobra"
)

var keepDataFlag bool

// uninstallInput is the source of the confirmation answer; injectable for tests.
var uninstallInput io.Reader = os.Stdin

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Completely remove DevMesh: proxy service, CLI binary, and saved state",
	Long: `Completely remove DevMesh from this machine:

  - the devmesh-proxy systemd service (sudo only)
  - /usr/local/bin/devmesh (sudo only)
  - ~/.devmesh saved project state (confirmation prompt unless --keep-data)

Project .devmesh.yaml files in your projects are never touched.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := serviceHomeDir()
		if err != nil {
			return err
		}
		isRoot := os.Geteuid() == 0

		if isRoot {
			// 1. Remove the proxy service (best effort; may not be installed)
			if systemdAvailable() == nil {
				if err := removeServiceUnit(); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
				} else {
					fmt.Printf("Removed systemd service %s\n", serviceName)
				}
			}

			// 2. Remove the installed CLI binary
			if err := os.Remove(serviceBinPath); err != nil && !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Warning: failed to remove %s: %v\n", serviceBinPath, err)
			} else if err == nil {
				fmt.Printf("Removed %s\n", serviceBinPath)
			}
		}

		// 3. Remove saved state (as the invoking user's home, also under sudo)
		stateDir := filepath.Join(home, ".devmesh")
		if _, err := os.Stat(stateDir); err == nil {
			if !keepDataFlag {
				fmt.Printf("This will delete %s (saved project states, daemon lock). This cannot be undone.\n", stateDir)
				if !confirmUninstall(uninstallInput) {
					fmt.Println("Skipping state deletion.")
				} else if err := removeStateDir(stateDir); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
				} else {
					fmt.Printf("Removed %s\n", stateDir)
				}
			} else {
				fmt.Printf("Keeping %s (--keep-data).\n", stateDir)
			}
		}

		if !isRoot {
			fmt.Println()
			fmt.Println("The systemd service and the CLI binary are root-owned and were not removed.")
			fmt.Println("For complete removal, run: sudo devmesh uninstall")
		} else {
			fmt.Println()
			fmt.Println("DevMesh fully uninstalled. Note: project .devmesh.yaml files were left untouched,")
			fmt.Println("and a ~/go/bin/devmesh copy from 'go install' (if present) is managed by the Go toolchain.")
		}
		return nil
	},
}

// serviceHomeDir resolves the home directory for state removal: the sudo
// invoker's home when elevated, otherwise the current $HOME.
func serviceHomeDir() (string, error) {
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" && sudoUser != "root" {
		if u, err := user.Lookup(sudoUser); err == nil {
			return u.HomeDir, nil
		}
	}
	return os.UserHomeDir()
}

func confirmUninstall(r io.Reader) bool {
	fmt.Print("Proceed? [y/N]: ")
	var resp string
	_, _ = fmt.Fscanln(r, &resp)
	resp = strings.TrimSpace(strings.ToLower(resp))
	return resp == "y" || resp == "yes"
}

// removeStateDir stops a CLI-spawned daemon if one is registered in the lock
// file, then deletes the state directory.
func removeStateDir(stateDir string) error {
	if data, err := os.ReadFile(filepath.Join(stateDir, "daemon.lock")); err == nil {
		if pid, perr := strconv.Atoi(strings.TrimSpace(string(data))); perr == nil && pid > 0 && internal.IsProcessRunning(pid) && pid != os.Getpid() {
			fmt.Printf("Stopping spawned proxy daemon (PID %d)...\n", pid)
			if runtime.GOOS != "windows" {
				_ = syscall.Kill(-pid, syscall.SIGTERM)
			}
		}
	}
	if err := os.RemoveAll(stateDir); err != nil {
		return fmt.Errorf("failed to remove %s: %w", stateDir, err)
	}
	return nil
}

func init() {
	uninstallCmd.Flags().BoolVar(&keepDataFlag, "keep-data", false, "Keep ~/.devmesh saved state (skips the deletion prompt)")
	rootCmd.AddCommand(uninstallCmd)
}
