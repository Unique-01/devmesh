package cli

import (
	"strings"
	"testing"
)

func TestBuildUnitContent(t *testing.T) {
	unit := buildUnitContent("/usr/local/bin/devmesh", "unic", "/home/unic")

	checks := []string{
		"User=unic",
		"Environment=HOME=/home/unic",
		"ExecStart=/usr/local/bin/devmesh proxy --addr 127.0.0.1:80",
		"AmbientCapabilities=CAP_NET_BIND_SERVICE",
		"CapabilityBoundingSet=CAP_NET_BIND_SERVICE",
		"NoNewPrivileges=true",
		"Restart=on-failure",
		"WantedBy=multi-user.target",
	}
	for _, c := range checks {
		if !strings.Contains(unit, c) {
			t.Errorf("expected unit to contain %q, unit:\n%s", c, unit)
		}
	}
	if strings.Contains(unit, "User=root") {
		t.Errorf("service must not run as root, unit:\n%s", unit)
	}
}
