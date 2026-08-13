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
	Domain string // e.g. vault.dev
}

// ResolveIdentity resolves project identity from:
// 1. Explicit CLI flags (--name and --domain)
// 2. Configuration file (.devmesh.yaml)
// 3. Folder name -> project name -> project.dev
func ResolveIdentity(explicitName, explicitDomain, configName, configDomain string) (ProjectIdentity, error) {
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

	domain := explicitDomain
	if domain == "" {
		domain = configDomain
	}
	if domain == "" {
		// Default domain is name.dev
		domain = fmt.Sprintf("%s.dev", name)
	} else {
		// If domain is provided without TLD or as a short name (e.g. --domain api), ensure it has .dev or use as is if it has a dot.
		// Wait, prompt says: --domain api.dev or --domain api?
		// Example: --domain api.dev. If someone passes api, or api.dev. Let's support both or full domain.
		// If domain doesn't contain a dot (e.g. "api"), we could append ".dev" or keep as is. Usually domain has a dot like api.dev.
	}

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
