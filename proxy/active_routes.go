package proxy

import (
	"fmt"
	"sync"
)

// ActiveRouteManager manages active routes in memory and synchronizes with registry.
type ActiveRouteManager struct {
	mu       sync.RWMutex
	registry *RouteRegistry
	routes   map[string]string // domain -> target (e.g. vault.localhost -> http://localhost:43127)
}

// NewActiveRouteManager creates an ActiveRouteManager.
func NewActiveRouteManager(registry *RouteRegistry) *ActiveRouteManager {
	return &ActiveRouteManager{
		registry: registry,
		routes:   make(map[string]string),
	}
}

// RegisterRoute adds a route to memory and the RouteRegistry.
func (arm *ActiveRouteManager) RegisterRoute(domain string, targetStr string) error {
	arm.mu.Lock()
	defer arm.mu.Unlock()

	if err := arm.registry.AddRoute(domain, targetStr); err != nil {
		return fmt.Errorf("failed to register route in registry: %w", err)
	}
	arm.routes[domain] = targetStr
	return nil
}

// UnregisterRoute removes a route from memory and the RouteRegistry.
func (arm *ActiveRouteManager) UnregisterRoute(domain string) {
	arm.mu.Lock()
	defer arm.mu.Unlock()

	arm.registry.RemoveRoute(domain)
	delete(arm.routes, domain)
}

// GetRoutes returns a copy of all active routes (DOMAIN -> TARGET).
func (arm *ActiveRouteManager) GetRoutes() map[string]string {
	arm.mu.RLock()
	defer arm.mu.RUnlock()

	result := make(map[string]string, len(arm.routes))
	for k, v := range arm.routes {
		result[k] = v
	}
	return result
}
