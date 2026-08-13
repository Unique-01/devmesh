package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"devmesh/internal"
	"devmesh/proxy"

	"github.com/spf13/cobra"
)

var (
	proxyAddrFlag string
	dnsAddrFlag   string
)

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Start the DevMesh reverse proxy and DNS resolver daemon",
	Long:  `Start the DevMesh reverse proxy server (default 127.0.0.1:8080) and local DNS resolver (default 127.0.0.1:53 or fallback port) for zero-config domain resolution without requiring manual port entry or /etc/hosts updates.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		registry := proxy.NewRouteRegistry()

		// Load any active project states into the registry
		states, err := internal.ListAllProjectStates()
		if err == nil {
			for _, s := range states {
				if s.Domain != "" && s.Port > 0 {
					targetStr := fmt.Sprintf("http://127.0.0.1:%d", s.Port)
					_ = registry.AddRoute(s.Domain, targetStr)
					fmt.Printf("Loaded route from state: %s -> %s\n", s.Domain, targetStr)
				}
			}
		}

		// Also check current directory .devmesh.yaml
		if _, err := os.Stat(".devmesh.yaml"); err == nil {
			if data, err := os.ReadFile(".devmesh.yaml"); err == nil {
				if cfg, err := parseConfigYaml(data); err == nil {
					ident, _ := internal.ResolveIdentity("", "", cfg.Name, cfg.Domain)
					if cfg.Port > 0 {
						targetStr := fmt.Sprintf("http://127.0.0.1:%d", cfg.Port)
						_ = registry.AddRoute(ident.Domain, targetStr)
						fmt.Printf("Loaded route from .devmesh.yaml: %s -> %s\n", ident.Domain, targetStr)
					}
				}
			}
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start DNS server
		dnsServer := proxy.NewDNSServer(dnsAddrFlag, registry)
		go func() {
			fmt.Printf("Starting DevMesh local DNS server on %s (resolving *.local.dev / *.dev -> 127.0.0.1)\n", dnsAddrFlag)
			if err := dnsServer.Start(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "DNS server stopped or failed (note: port 53 requires root/sudo, or try --dns 127.0.0.1:5353): %v\n", err)
			}
		}()
		defer dnsServer.Close()

		// Start Reverse Proxy server
		proxyServer := proxy.NewServer(proxyAddrFlag, registry)
		fmt.Printf("Starting DevMesh reverse proxy server on %s\n", proxyAddrFlag)

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		errChan := make(chan error, 1)
		go func() {
			if err := proxyServer.Start(); err != nil {
				errChan <- err
			}
		}()

		select {
		case sig := <-sigChan:
			fmt.Printf("Received signal %v, shutting down DevMesh proxy & DNS...\n", sig)
			cancel()
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()
			_ = proxyServer.Shutdown(shutdownCtx)
		case err := <-errChan:
			return fmt.Errorf("proxy server error: %w", err)
		}

		return nil
	},
}

func init() {
	proxyCmd.Flags().StringVar(&proxyAddrFlag, "addr", "127.0.0.1:8080", "Proxy listen address")
	proxyCmd.Flags().StringVar(&dnsAddrFlag, "dns", "127.0.0.1:53", "DNS listen address (use e.g. 127.0.0.1:5353 if port 53 is restricted)")
	rootCmd.AddCommand(proxyCmd)
}
