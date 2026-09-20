package cli

import (
	"fmt"
	"os"

	"go.yaml.in/yaml/v4"
)

func parseConfigYaml(data []byte) (Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("invalid yaml: %w", err)
	}
	return cfg, nil
}

func writeConfigYaml(configPath string, cfg Config) error {
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", configPath, err)
	}
	return nil
}

func loadProjectConfig() (Config, error) {
	configPath := ".devmesh.yaml"

	data, readErr := os.ReadFile(configPath)
	if readErr != nil {
		return Config{}, fmt.Errorf("failed to read %s: %w", configPath, readErr)
	}
	cfg, parseErr := parseConfigYaml(data)
	if parseErr != nil {
		return Config{}, fmt.Errorf("failed to parse %s: %w", configPath, parseErr)
	}
	return cfg, nil
}

