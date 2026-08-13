package internal

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const devmeshHeaderBegin = "# BEGIN DevMesh Managed Entries"
const devmeshHeaderEnd = "# END DevMesh Managed Entries"

// HostsManager manages /etc/hosts entries safely.
type HostsManager struct {
	hostsFilePath string
}

// NewHostsManager creates a new HostsManager. If hostsFilePath is empty, defaults to /etc/hosts.
func NewHostsManager(hostsFilePath string) *HostsManager {
	if hostsFilePath == "" {
		hostsFilePath = "/etc/hosts"
	}
	return &HostsManager{
		hostsFilePath: hostsFilePath,
	}
}

// AddEntry adds a domain mapping (e.g. vault.local.dev -> 127.0.0.1) into the managed section of /etc/hosts.
// If direct write fails due to permissions, it attempts to use sudo automatically.
func (hm *HostsManager) AddEntry(domain string, ip string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if ip == "" {
		ip = "127.0.0.1"
	}

	err := hm.writeEntryInternal(domain, ip)
	if err != nil && os.IsPermission(err) {
		err = hm.addEntryWithSudo(domain, ip)
	}
	return err
}

func (hm *HostsManager) writeEntryInternal(domain string, ip string) error {
	content, err := os.ReadFile(hm.hostsFilePath)
	if err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied reading %s: %w", hm.hostsFilePath, err)
		}
		// If file doesn't exist yet, we can create it or treat as empty
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to read %s: %w", hm.hostsFilePath, err)
		}
		content = []byte("")
	}

	lines := parseHostsLines(string(content))

	// Ensure DevMesh managed block exists, or find existing entries
	// We will rebuild the lines preserving non-devmesh lines and updating the devmesh managed block.
	var unrelatedLines []string
	managedEntries := make(map[string]string) // domain -> ip
	var managedOrder []string

	inManagedBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == devmeshHeaderBegin {
			inManagedBlock = true
			continue
		}
		if trimmed == devmeshHeaderEnd {
			inManagedBlock = false
			continue
		}

		if inManagedBlock {
			// Parse domain ip inside managed block
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				entryIP := parts[0]
				entryDomain := strings.ToLower(parts[1])
				if managedEntries[entryDomain] == "" {
					managedOrder = append(managedOrder, entryDomain)
				}
				managedEntries[entryDomain] = entryIP
			}
		} else {
			unrelatedLines = append(unrelatedLines, line)
		}
	}

	// Add or update the new/existing entry
	if managedEntries[domain] == "" {
		managedOrder = append(managedOrder, domain)
	}
	managedEntries[domain] = ip

	// Reconstruct file content
	var newContent bytes.Buffer
	for _, l := range unrelatedLines {
		newContent.WriteString(l);newContent.WriteString("\n")
	}

	// Append managed block
	newContent.WriteString(devmeshHeaderBegin + "\n")
	for _, d := range managedOrder {
		if targetIP, ok := managedEntries[d]; ok {
			newContent.WriteString(fmt.Sprintf("%s\t%s\n", targetIP, d))
		}
	}
	newContent.WriteString(devmeshHeaderEnd + "\n")

	// Write back to file atomically or safely
	err = os.WriteFile(hm.hostsFilePath, newContent.Bytes(), 0644)
	if err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied writing to %s (root privileges required): %w", hm.hostsFilePath, err)
		}
		return fmt.Errorf("failed to write %s: %w", hm.hostsFilePath, err)
	}

	return nil
}

func (hm *HostsManager) addEntryWithSudo(domain string, ip string) error {
	content, _ := os.ReadFile(hm.hostsFilePath)
	lines := parseHostsLines(string(content))

	var unrelatedLines []string
	managedEntries := make(map[string]string)
	var managedOrder []string

	inManagedBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == devmeshHeaderBegin {
			inManagedBlock = true
			continue
		}
		if trimmed == devmeshHeaderEnd {
			inManagedBlock = false
			continue
		}
		if inManagedBlock {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				entryIP := parts[0]
				entryDomain := strings.ToLower(parts[1])
				if managedEntries[entryDomain] == "" {
					managedOrder = append(managedOrder, entryDomain)
				}
				managedEntries[entryDomain] = entryIP
			}
		} else {
			unrelatedLines = append(unrelatedLines, line)
		}
	}

	if managedEntries[domain] == "" {
		managedOrder = append(managedOrder, domain)
	}
	managedEntries[domain] = ip

	var newContent bytes.Buffer
	for _, l := range unrelatedLines {
		newContent.WriteString(l);newContent.WriteString("\n")
	}

	newContent.WriteString(devmeshHeaderBegin + "\n")
	for _, d := range managedOrder {
		if targetIP, ok := managedEntries[d]; ok {
			newContent.WriteString(fmt.Sprintf("%s\t%s\n", targetIP, d))
		}
	}
	newContent.WriteString(devmeshHeaderEnd + "\n")

	cmd := exec.Command("sudo", "tee", hm.hostsFilePath)
	cmd.Stdin = bytes.NewReader(newContent.Bytes())
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to update %s via sudo (exit status %v): %s", hm.hostsFilePath, err, string(out))
	}
	return nil
}

// RemoveEntry removes a domain mapping from the managed section of /etc/hosts.
func (hm *HostsManager) RemoveEntry(domain string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	err := hm.removeEntryInternal(domain)
	if err != nil && os.IsPermission(err) {
		err = hm.removeEntryWithSudo(domain)
	}
	return err
}

func (hm *HostsManager) removeEntryInternal(domain string) error {
	content, err := os.ReadFile(hm.hostsFilePath)
	if err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied reading %s: %w", hm.hostsFilePath, err)
		}
		if os.IsNotExist(err) {
			return nil // Nothing to remove
		}
		return fmt.Errorf("failed to read %s: %w", hm.hostsFilePath, err)
	}

	lines := parseHostsLines(string(content))

	var unrelatedLines []string
	managedEntries := make(map[string]string)
	var managedOrder []string

	inManagedBlock := false
	hasManagedBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == devmeshHeaderBegin {
			inManagedBlock = true
			hasManagedBlock = true
			continue
		}
		if trimmed == devmeshHeaderEnd {
			inManagedBlock = false
			continue
		}

		if inManagedBlock {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				entryIP := parts[0]
				entryDomain := strings.ToLower(parts[1])
				if entryDomain != domain {
					if managedEntries[entryDomain] == "" {
						managedOrder = append(managedOrder, entryDomain)
					}
					managedEntries[entryDomain] = entryIP
				}
			}
		} else {
			unrelatedLines = append(unrelatedLines, line)
		}
	}

	if !hasManagedBlock {
		return nil
	}

	var newContent bytes.Buffer
	for _, l := range unrelatedLines {
		newContent.WriteString(l);newContent.WriteString("\n")
	}

	// If there are still managed entries, write block back
	if len(managedOrder) > 0 {
		newContent.WriteString(devmeshHeaderBegin + "\n")
		for _, d := range managedOrder {
			if targetIP, ok := managedEntries[d]; ok {
				newContent.WriteString(fmt.Sprintf("%s\t%s\n", targetIP, d))
			}
		}
		newContent.WriteString(devmeshHeaderEnd + "\n")
	}

	err = os.WriteFile(hm.hostsFilePath, newContent.Bytes(), 0644)
	if err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied writing to %s (root privileges required): %w", hm.hostsFilePath, err)
		}
		return fmt.Errorf("failed to write %s: %w", hm.hostsFilePath, err)
	}

	return nil
}

func (hm *HostsManager) removeEntryWithSudo(domain string) error {
	content, _ := os.ReadFile(hm.hostsFilePath)
	lines := parseHostsLines(string(content))

	var unrelatedLines []string
	managedEntries := make(map[string]string)
	var managedOrder []string

	inManagedBlock := false
	hasManagedBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == devmeshHeaderBegin {
			inManagedBlock = true
			hasManagedBlock = true
			continue
		}
		if trimmed == devmeshHeaderEnd {
			inManagedBlock = false
			continue
		}
		if inManagedBlock {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				entryIP := parts[0]
				entryDomain := strings.ToLower(parts[1])
				if entryDomain != domain {
					if managedEntries[entryDomain] == "" {
						managedOrder = append(managedOrder, entryDomain)
					}
					managedEntries[entryDomain] = entryIP
				}
			}
		} else {
			unrelatedLines = append(unrelatedLines, line)
		}
	}

	if !hasManagedBlock {
		return nil
	}

	var newContent bytes.Buffer
	for _, l := range unrelatedLines {
		newContent.WriteString(l);newContent.WriteString("\n")
	}

	if len(managedOrder) > 0 {
		newContent.WriteString(devmeshHeaderBegin + "\n")
		for _, d := range managedOrder {
			if targetIP, ok := managedEntries[d]; ok {
				newContent.WriteString(fmt.Sprintf("%s\t%s\n", targetIP, d))
			}
		}
		newContent.WriteString(devmeshHeaderEnd + "\n")
	}

	cmd := exec.Command("sudo", "tee", hm.hostsFilePath)
	cmd.Stdin = bytes.NewReader(newContent.Bytes())
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to update %s via sudo (exit status %v): %s", hm.hostsFilePath, err, string(out))
	}
	return nil
}

func parseHostsLines(content string) []string {
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}
