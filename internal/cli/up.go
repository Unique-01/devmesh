package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"devmesh/internal/process"
	"devmesh/proxy"

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
		if cmdFlag != "" {
			if nameFlag == "" {
				// If name flag is not provided, check if .devmesh.yaml exists to resolve name and other flags, otherwise use folder root name.
				if _, err := os.Stat(configPath); err == nil {
					data, err := os.ReadFile(configPath)
					if err == nil {
						if cfg, err := parseConfigYaml(data); err == nil {
							if cfg.Name != "" {
								nameFlag = cfg.Name
							}
							if portFlag == 0 && cfg.Port > 0 {
								portFlag = cfg.Port
							}
						}
					}
				}
				if nameFlag == "" {
					cwd, err := os.Getwd()
					if err == nil {
						nameFlag = filepath.Base(cwd)
					}
					if nameFlag == "" || nameFlag == "." || nameFlag == "/" {
						nameFlag = "app"
					}
				}
			}

			cfg := Config{
				Name: nameFlag,
				Cmd:  cmdFlag,
				Port: portFlag,
			}

			yamlContent := fmt.Sprintf("name: %s\ncmd: %q\n", cfg.Name, cfg.Cmd)
			if cfg.Port > 0 {
				yamlContent += fmt.Sprintf("port: %d\n", cfg.Port)
			}

			if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", configPath, err)
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
				nameFlag = cfg.Name
				if portFlag == 0 && cfg.Port > 0 {
					portFlag = cfg.Port
				}
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

		// Allocate port
		port, err := proxy.AllocatePort(portFlag)
		if err != nil {
			return fmt.Errorf("failed to allocate port: %w", err)
		}

		serviceName := nameFlag
		if serviceName == "" {
			serviceName = "app"
		}

		fmt.Printf("Starting DevMesh development service %q on port %d for command: %s\n", serviceName, port, cmdFlag)

		mgr := process.NewManager(cmdFlag, port)
		fmt.Printf("Process assigned PID tracker. Spawning...\n")

		ctx := context.Background()
		if err := mgr.Run(ctx, nil, os.Stdout, os.Stderr); err != nil {
			return fmt.Errorf("process execution failed: %w", err)
		}

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
	upCmd.Flags().IntVar(&portFlag, "port", 0, "Preferred port number (optional)")
	rootCmd.AddCommand(upCmd)
}
