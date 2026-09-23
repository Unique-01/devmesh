package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/Unique-01/devmesh/internal"
	"github.com/Unique-01/devmesh/internal/process"

	"github.com/spf13/cobra"
)

var (
	cmdFlag  string
	nameFlag string
	portFlag int
)

// Config represents the .devmesh.yaml configuration file format.
type Config struct {
	Name string `yaml:"name" json:"name"`
	Cmd  string `yaml:"cmd" json:"cmd"`
	Port int    `yaml:"port,omitempty" json:"port,omitempty"`
}

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start development command with automatic PORT assignment and proxy routing",
	Long:  `Start development command with automatic PORT assignment (injected as PORT environment variable) and proxy routing.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := ".devmesh.yaml"

		// 1. If --cmd is provided, create/update .devmesh.yaml automatically.
		cfg, err := loadProjectConfig()

		var configName string
		configName = cfg.Name
		if portFlag == 0 && cfg.Port > 0 {
			portFlag = cfg.Port
		}

		ident, err := internal.ResolveIdentity(nameFlag, configName)
		if err != nil {
			return fmt.Errorf("failed to resolve identity: %w", err)
		}
		nameFlag = ident.Name
		domain := ident.Domain

		state, err := internal.LoadProjectState(nameFlag)
		if err == nil {
			if state.PID > 0 && internal.IsProcessRunning(state.PID) {
				return fmt.Errorf(
					"project %q is already running (PID %d). Use 'devmesh stop %s' or 'devmesh restart %s'",
					nameFlag, state.PID, nameFlag, nameFlag,
				)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to load project state: %w", err)
		}
		
		if cmdFlag != "" {
			cfg := Config{
				Name: nameFlag,
				Cmd:  cmdFlag,
				Port: portFlag,
			}

			if err := writeConfigYaml(configPath, cfg); err != nil {
				return err
			}
			fmt.Printf("Created %s configuration for project %q\n", configPath, cfg.Name)
		} else {
			// 2. If no --cmd, check if .devmesh.yaml exists to load from.
			if _, err := os.Stat(configPath); err == nil {
				data, err := os.ReadFile(configPath)
				if err != nil {
					return fmt.Errorf("failed to read %s: %w", configPath, err)
				}
				cfg, err := parseConfigYaml(data)
				if err != nil {
					return fmt.Errorf("failed to parse %s: %w", configPath, err)
				}
				cmdFlag = cfg.Cmd
				fmt.Printf("Loaded configuration from %s (name: %s, cmd: %s)\n", configPath, nameFlag, cmdFlag)
			} else if os.IsNotExist(err) {
				return fmt.Errorf("required flag \"cmd\" not set and no %s found (e.g. devmesh up --cmd \"pnpm dev\" --name vault)", configPath)
			} else {
				return fmt.Errorf("failed to check %s: %w", configPath, err)
			}
		}

		if cmdFlag == "" {
			return fmt.Errorf("required flag \"cmd\" not set (e.g. devmesh up --cmd \"pnpm dev\")")
		}

		// Port strategy:
		//   - Explicit --port (or config port): use it; only if it is already
		//     taken by another program, fall back to a random unprivileged port.
		//   - Otherwise (port 0): do NOT inject PORT. The app runs on the port
		//     it intends to use (e.g. Vite on 5173) and DevMesh detects the
		//     bound port after startup to wire the proxy route. If the app
		//     crashes because its intended port is busy, it is restarted once
		//     with an available port injected via PORT.
		port := 0
		if portFlag > 0 {
			p, allocErr := process.AllocatePort(portFlag)
			if allocErr != nil {
				return fmt.Errorf("failed to allocate port: %w", allocErr)
			}
			port = p
		}

		// Ensure Proxy Daemon is running
		proxyAddr, err := ensureProxyDaemon()
		if err != nil {
			return fmt.Errorf("failed to ensure proxy daemon: %w", err)
		}
		fmt.Printf("Proxy daemon running on %s\n", proxyAddr)

		registerRoute := func() {
			if port <= 0 {
				return
			}
			if rErr := internal.RegisterRoute(proxyAddr, domain, fmt.Sprintf("http://localhost:%d", port)); rErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to register route %s -> localhost:%d: %v\n", domain, port, rErr)
			} else {
				fmt.Printf("Registered route: %s -> http://localhost:%d\n", domain, port)
			}
		}
		defer func() {
			_ = internal.DeregisterRoute(proxyAddr, domain)
		}()

		serviceName := nameFlag
		portLabel := "auto-detect"
		if port > 0 {
			portLabel = strconv.Itoa(port)
		}
		fmt.Printf("Starting DevMesh development service %q on domain %s (port: %s) for command: %s\n", serviceName, domain, portLabel, cmdFlag)

		cwd, _ := os.Getwd()
		ctx := context.Background()

		var runErr error
		saveProjectState := func(currentPort, pid int) {
			state := internal.ProjectState{
				Name:      ident.Name,
				Domain:    domain,
				Port:      currentPort,
				PID:       pid,
				Cmd:       cmdFlag,
				Directory: cwd,
			}
			_ = internal.SaveProjectState(state)
		}
		for attempt := 1; ; attempt++ {
			mgr := process.NewManager(cmdFlag, port)

			onStart := func(pid int) {
				saveProjectState(port, pid)
				fmt.Printf("Project %q state saved (PID: %d)\n", ident.Name, pid)
			}

			onPortDetected := func(detectedPort int) {
				port = detectedPort
				registerRoute()
				saveProjectState(port, mgr.PID())
				fmt.Printf("Detected service port %d. Route active: %s -> http://localhost:%d\n", port, domain, port)
			}

			runErr = mgr.RunWithCallback(ctx, nil, os.Stdout, os.Stderr, onStart, onPortDetected)

			// The app's intended port was taken by another program and the app
			// crashed on it: retry ONCE with an available port (re-checking the
			// preferred port first, then a random unprivileged one) injected
			// via PORT. Apps that fall back on their own (e.g. Vite) never hit
			// this path — their final port is picked up by detection above.
			if errors.Is(runErr, process.ErrAddrInUse) && attempt == 1 {
				newPort, allocErr := process.AllocatePort(port)
				if allocErr != nil {
					break
				}
				if port > 0 {
					fmt.Printf("Intended port %d is already in use by another program. Restarting on port %d...\n", port, newPort)
				} else {
					fmt.Printf("App failed to bind its intended port (already in use). Restarting on port %d...\n", newPort)
				}
				port = newPort
				registerRoute()
				continue
			}
			break
		}

		if runErr != nil {
			saveProjectState(port, 0)
			return fmt.Errorf("process execution failed: %w", runErr)
		}

		saveProjectState(port, 0)
		return nil
	},
}

func init() {
	upCmd.Flags().StringVar(&cmdFlag, "cmd", "", "Development command to run (e.g. \"pnpm dev\")")
	upCmd.Flags().StringVar(&nameFlag, "name", "", "Service name (e.g. \"vault\")")
	upCmd.Flags().IntVar(&portFlag, "port", 0, "Preferred port number (optional)")
	rootCmd.AddCommand(upCmd)
}
