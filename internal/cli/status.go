package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Unique-01/devmesh/internal"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current devmesh status (proxy, routes, projects)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if addr, err := findProxyDaemon(); err == nil {
			fmt.Println("Proxy: running")
			fmt.Printf("Address: %s\n", addr)
			fmt.Printf("Routes: %d active\n", fetchRouteCount(addr))

		} else {
			fmt.Println("Proxy: stopped")
			fmt.Println("Address: —")
			fmt.Println("Routes: 0 active")
		}

		count := 0
		if states, err := internal.ListAllProjectStates(); err == nil {
			count = len(states)
		}
		fmt.Printf("Projects: %d\n", count)
		return nil
	},
}

func fetchRouteCount(addr string) int {
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get("http://" + addr + "/_devmesh/routes")
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	var routes map[string]string
	if json.NewDecoder(resp.Body).Decode(&routes) != nil {
		return 0
	}
	return len(routes)
}

func init() {
	rootCmd.AddCommand(statusCmd)
	proxyCmd.AddCommand(statusCmd)
}
