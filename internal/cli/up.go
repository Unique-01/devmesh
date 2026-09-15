package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"devmesh/internal"
	"devmesh/internal/process"

	"github.com/spf13/cobra"
)

var (
	cmdFlag    string
	nameFlag   string
	domainFlag string
	portFlag   int
)

// Config represents the .devmesh.yaml configuration file format.
type Config struct {
	Name   string `yaml:"name" json:"name"`
	Domain string `yaml:"domain" json:"domain"`
	Cmd    string `yaml:"cmd" json:"cmd"`
	Port   int    `yaml:"port,omitempty" json:"port,omitempty"`
}

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start development command with automatic PORT assignment and proxy routing",
	Long:  `Start development command with automatic PORT assignment (injected as PORT environment variable) and proxy routing.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if os.Getuid() == 0 {
			return fmt.Errorf("command does not support running as root — try again without sudo")
		}
		configPath := ".devmesh.yaml"

		// 1. If --cmd is provided, create/update .devmesh.yaml automatically.
		var configName, configDomain string
		if _, err := os.Stat(configPath); err == nil {
			if data, err := os.ReadFile(configPath); err == nil {
				if cfg, err := parseConfigYaml(data); err == nil {
					configName = cfg.Name
					configDomain = cfg.Domain
					if portFlag == 0 && cfg.Port > 0 {
						portFlag = cfg.Port
					}
				}
			}
		}

		ident, err := internal.ResolveIdentity(nameFlag, domainFlag, configName, configDomain)
		if err != nil {
			return fmt.Errorf("failed to resolve identity: %w", err)
		}
		nameFlag = ident.Name
		domain := ident.Domain

		if cmdFlag != "" {
			cfg := Config{
				Name:   nameFlag,
				Domain: domain,
				Cmd:    cmdFlag,
				Port:   portFlag,
			}

			yamlContent := fmt.Sprintf("name: %s\ndomain: %s\ncmd: %q\n", cfg.Name, cfg.Domain, cfg.Cmd)
			if cfg.Port > 0 {
				yamlContent += fmt.Sprintf("port: %d\n", cfg.Port)
			}

			if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", configPath, err)
			}
			fmt.Printf("Created %s configuration for project %q (domain: %s)\n", configPath, cfg.Name, cfg.Domain)
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
				fmt.Printf("Loaded configuration from %s (name: %s, domain: %s, cmd: %s)\n", configPath, nameFlag, domain, cmdFlag)
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
				// Nothing to route to yet; the route is registered as soon as
				// the app's real port is detected. Until then the proxy
				// answers 502 for this domain instead of silently forwarding
				// to an unrelated port.
				return
			}
			// Forward via "localhost", not a hardcoded IP: the OS may map
			// localhost to ::1 and/or 127.0.0.1, and apps (e.g. Node 17+
			// tools like Vite) bind whichever the resolver gives them. The
			// Go dialer tries every resolved address, covering both families.
			if rErr := internal.RegisterRoute(proxyAddr, domain, fmt.Sprintf("http://localhost:%d", port)); rErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to register route %s -> localhost:%d: %v\n", domain, port, rErr)
			} else {
				fmt.Printf("Registered route: %s -> http://localhost:%d\n", domain, port)
			}
		}
		registerRoute()

		defer func() {
			_ = internal.DeregisterRoute(proxyAddr, domain)
		}()

		serviceName := nameFlag
		if serviceName == "" {
			serviceName = "app"
		}

		portLabel := "auto-detect"
		if port > 0 {
			portLabel = strconv.Itoa(port)
		}
		fmt.Printf("Starting DevMesh development service %q on domain %s (port: %s) for command: %s\n", serviceName, domain, portLabel, cmdFlag)

		cwd, _ := os.Getwd()
		ctx := context.Background()

		var runErr error
		for attempt := 1; ; attempt++ {
			mgr := process.NewManager(cmdFlag, port)

			onStart := func(pid int) {
				state := internal.ProjectState{
					Name:      ident.Name,
					Domain:    domain,
					Port:      port,
					PID:       pid,
					Cmd:       cmdFlag,
					Directory: cwd,
				}
				_ = internal.SaveProjectState(state)
				fmt.Printf("Project %q state saved (PID: %d)\n", ident.Name, pid)
			}

			onPortDetected := func(detectedPort int) {
				port = detectedPort
				registerRoute()
				state := internal.ProjectState{
					Name:      ident.Name,
					Domain:    domain,
					Port:      port,
					PID:       mgr.PID(),
					Cmd:       cmdFlag,
					Directory: cwd,
				}
				_ = internal.SaveProjectState(state)
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
			state := internal.ProjectState{
				Name:      ident.Name,
				Domain:    domain,
				Port:      port,
				PID:       0,
				Cmd:       cmdFlag,
				Directory: cwd,
			}
			_ = internal.SaveProjectState(state)
			return fmt.Errorf("process execution failed: %w", runErr)
		}

		state := internal.ProjectState{
			Name:      ident.Name,
			Domain:    domain,
			Port:      port,
			PID:       0,
			Cmd:       cmdFlag,
			Directory: cwd,
		}
		_ = internal.SaveProjectState(state)

		return nil
	},
}

func parseConfigYaml(data []byte) (Config, error) {
	var cfg Config
	lines := string(data)
	scanner := bufio.NewScanner(strings.NewReader(lines))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		// Remove quotes if present
		val = strings.Trim(val, "\"'")

		switch key {
		case "name":
			cfg.Name = val
		case "domain":
			cfg.Domain = val
		case "cmd":
			cfg.Cmd = val
		case "port":
			if p, err := strconv.Atoi(val); err == nil {
				cfg.Port = p
			}
		}
	}
	return cfg, nil
}

func init() {
	upCmd.Flags().StringVar(&cmdFlag, "cmd", "", "Development command to run (e.g. \"pnpm dev\")")
	upCmd.Flags().StringVar(&nameFlag, "name", "", "Service name (e.g. \"vault\")")
	upCmd.Flags().StringVar(&domainFlag, "domain", "", "Custom domain name (e.g. \"api.localhost\")")
	upCmd.Flags().IntVar(&portFlag, "port", 0, "Preferred port number (optional)")
	rootCmd.AddCommand(upCmd)
}
