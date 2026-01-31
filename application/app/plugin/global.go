package plugin

import (
	"sync"

	"breachline/app/settings"
)

var (
	globalPluginRegistry *PluginRegistry
	pluginRegistryMu     sync.RWMutex
)

// SetPluginRegistry sets the global plugin registry
func SetPluginRegistry(registry *PluginRegistry) {
	pluginRegistryMu.Lock()
	defer pluginRegistryMu.Unlock()
	globalPluginRegistry = registry
}

// GetPluginRegistry returns the global plugin registry
// Returns nil if plugins are not initialized
func GetPluginRegistry() *PluginRegistry {
	pluginRegistryMu.RLock()
	defer pluginRegistryMu.RUnlock()
	return globalPluginRegistry
}

// InitializePluginRegistry creates and initializes the global plugin registry from settings
// This should be called when the application starts or when plugin settings change
func InitializePluginRegistry(configs []settings.PluginConfig) error {
	registry := NewPluginRegistry()

	if err := registry.LoadFromSettings(configs); err != nil {
		return err
	}

	SetPluginRegistry(registry)
	return nil
}

// ClearPluginRegistry clears the global plugin registry
// This is useful for testing or when disabling plugins
func ClearPluginRegistry() {
	SetPluginRegistry(nil)
}
