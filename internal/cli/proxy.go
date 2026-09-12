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
)

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Run the DevMesh reverse proxy in the foreground, or manage the background proxy service",
	Long: `Run the DevMesh reverse proxy server in the foreground (default 127.0.0.1:8080).

For daily use you normally don't run this directly — 'devmesh install' sets up
an always-on background proxy service on :80. Use the subcommands to manage
that background service.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		registry := proxy.NewRouteRegistry()

		// Load any active project states into the registry
		states, err := internal.ListAllProjectStates()
		if err == nil {
			for _, s := range states {
				if s.Domain != "" && s.Port > 0 {
					targetStr := fmt.Sprintf("http://localhost:%d", s.Port)
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
						targetStr := fmt.Sprintf("http://localhost:%d", cfg.Port)
						_ = registry.AddRoute(ident.Domain, targetStr)
						fmt.Printf("Loaded route from .devmesh.yaml: %s -> %s\n", ident.Domain, targetStr)
					}
				}
			}
		}

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
			fmt.Printf("Received signal %v, shutting down DevMesh proxy...\n", sig)
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
	rootCmd.AddCommand(proxyCmd)
}
