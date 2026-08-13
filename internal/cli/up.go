package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"devmesh/internal"
	"devmesh/internal/process"
	"devmesh/proxy"

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

		// Allocate port
		port, err := proxy.AllocatePort(portFlag)
		if err != nil {
			return fmt.Errorf("failed to allocate port: %w", err)
		}

		serviceName := nameFlag
		if serviceName == "" {
			serviceName = "app"
		}

		// Update hosts entry and active route registry
		hostsMgr := internal.NewHostsManager("")
		if err := hostsMgr.AddEntry(domain, "127.0.0.1"); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to update /etc/hosts for %s: %v (may require sudo privileges)\n", domain, err)
		} else {
			fmt.Printf("Updated /etc/hosts: %s -> 127.0.0.1\n", domain)
		}
		defer func() {
			_ = hostsMgr.RemoveEntry(domain)
		}()

		fmt.Printf("Starting DevMesh development service %q on domain %s (port %d) for command: %s\n", serviceName, domain, port, cmdFlag)

		mgr := process.NewManager(cmdFlag, port)
		fmt.Printf("Process assigned PID tracker. Spawning...\n")

		cwd, _ := os.Getwd()
		ctx := context.Background()
		err = mgr.RunWithCallback(ctx, nil, os.Stdout, os.Stderr, func(pid int) {
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
		})
		if err != nil {
			state := internal.ProjectState{
				Name:      ident.Name,
				Domain:    domain,
				Port:      port,
				PID:       0,
				Cmd:       cmdFlag,
				Directory: cwd,
			}
			_ = internal.SaveProjectState(state)
			return fmt.Errorf("process execution failed: %w", err)
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
	upCmd.Flags().StringVar(&domainFlag, "domain", "", "Custom domain name (e.g. \"api.dev\")")
	upCmd.Flags().IntVar(&portFlag, "port", 0, "Preferred port number (optional)")
	rootCmd.AddCommand(upCmd)
}
