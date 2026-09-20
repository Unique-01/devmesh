package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProjectIdentity defines the name and domain for a project.
type ProjectIdentity struct {
	Name   string // e.g. vault
	Domain string // e.g. vault.localhost
}

// ResolveIdentity resolves project identity from:
// 1. Explicit CLI flags (--name)
// 2. Configuration file (.devmesh.yaml)
// 3. Folder name -> project name -> project.localhost
func ResolveIdentity(explicitName, configName string) (ProjectIdentity, error) {
	name := explicitName
	if name == "" {
		name = configName
	}
	if name == "" {
		cwd, err := os.Getwd()
		if err == nil {
			base := filepath.Base(cwd)
			if base != "" && base != "." && base != "/" {
				name = base
			}
		}
	}
	if name == "" {
		name = "app"
	}

	// Sanitize name for domain if needed (lowercase, replace spaces/underscores with dashes)
	name = sanitizeProjectName(name)
	// Default domain is name.localhost
	domain := fmt.Sprintf("%s.localhost", name)

	return ProjectIdentity{
		Name:   name,
		Domain: domain,
	}, nil
}

func sanitizeProjectName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	// Replace spaces and underscores with hyphens
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")
	return name
}
