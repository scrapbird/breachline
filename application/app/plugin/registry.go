package plugin

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"breachline/app/settings"

	"gopkg.in/yaml.v3"
)

// PluginInfo contains all information about a loaded plugin
type PluginInfo struct {
	Config   settings.PluginConfig
	Manifest PluginManifest
	ExecPath string // Resolved absolute path to executable
}

// PluginRegistry manages the collection of available plugins
type PluginRegistry struct {
	plugins map[string][]*PluginInfo // extension → plugins (lowercase extensions, multiple plugins can support same extension)
	mu      sync.RWMutex
}

// NewPluginRegistry creates a new plugin registry
func NewPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		plugins: make(map[string][]*PluginInfo),
	}
}

// LoadFromSettings loads plugins from the settings configuration
// Only enabled plugins are loaded into the registry
func (r *PluginRegistry) LoadFromSettings(configs []settings.PluginConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Clear existing plugins
	r.plugins = make(map[string][]*PluginInfo)

	var loadErrors []string

	for _, config := range configs {
		// Skip disabled plugins
		if !config.Enabled {
			continue
		}

		// Validate and load plugin
		manifest, execPath, err := r.validatePluginConfig(config)
		if err != nil {
			loadErrors = append(loadErrors, fmt.Sprintf("plugin %s: %v", config.Name, err))
			continue
		}

		// Create plugin info
		pluginInfo := &PluginInfo{
			Config:   config,
			Manifest: *manifest,
			ExecPath: execPath,
		}

		// Register extensions - append to list, allowing multiple plugins per extension
		for _, ext := range manifest.Extensions {
			extLower := strings.ToLower(ext)
			r.plugins[extLower] = append(r.plugins[extLower], pluginInfo)
		}
	}

	// Return combined errors if any
	if len(loadErrors) > 0 {
		return fmt.Errorf("plugin loading errors:\n  - %s", strings.Join(loadErrors, "\n  - "))
	}

	return nil
}

// validatePluginConfig validates a plugin configuration and returns the manifest and executable path
func (r *PluginRegistry) validatePluginConfig(config settings.PluginConfig) (*PluginManifest, string, error) {
	// Check if path exists
	if config.Path == "" {
		return nil, "", errors.New("plugin path is empty")
	}

	// Check if path is a file or directory
	info, err := os.Stat(config.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", fmt.Errorf("path does not exist: %s", config.Path)
		}
		return nil, "", fmt.Errorf("cannot access path: %v", err)
	}

	var pluginDir string
	var execPath string

	if info.IsDir() {
		// Path is a directory - look for plugin.yml
		pluginDir = config.Path
	} else {
		// Path is a file - could be either plugin.yml or the executable
		pluginDir = filepath.Dir(config.Path)

		// Check if this is a manifest file
		fileName := filepath.Base(config.Path)
		if fileName == "plugin.yml" || fileName == "plugin.yaml" {
			// Path is the manifest file itself, executable will be resolved later
			execPath = ""
		} else {
			// Path is the executable
			execPath = config.Path
		}
	}

	// Read and parse plugin.yml
	manifestPath := filepath.Join(pluginDir, "plugin.yml")
	manifest, err := r.readManifest(manifestPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read manifest: %v", err)
	}

	// Resolve executable path if not already set
	if execPath == "" {
		execPath, err = r.resolveExecutable(pluginDir, manifest.Executable)
		if err != nil {
			return nil, "", fmt.Errorf("failed to resolve executable: %v", err)
		}
	}

	// Validate executable exists and is executable
	if err := r.validateExecutable(execPath); err != nil {
		return nil, "", err
	}

	return manifest, execPath, nil
}

// readManifest reads and parses a plugin.yml manifest file
func (r *PluginRegistry) readManifest(manifestPath string) (*PluginManifest, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("plugin.yml not found at %s", manifestPath)
		}
		return nil, fmt.Errorf("cannot read manifest: %v", err)
	}

	var manifest PluginManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("invalid YAML: %v", err)
	}

	// Validate required fields
	if err := validateManifest(&manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

// resolveExecutable resolves the executable path relative to the plugin directory
func (r *PluginRegistry) resolveExecutable(pluginDir string, executable string) (string, error) {
	// If executable is already absolute, use it as-is
	if filepath.IsAbs(executable) {
		return executable, nil
	}

	// Otherwise, resolve relative to plugin directory
	execPath := filepath.Join(pluginDir, executable)

	// Clean the path
	execPath = filepath.Clean(execPath)

	return execPath, nil
}

// validateExecutable checks if the executable exists and has proper permissions
func (r *PluginRegistry) validateExecutable(execPath string) error {
	info, err := os.Stat(execPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("executable not found: %s", execPath)
		}
		return fmt.Errorf("cannot access executable: %v", err)
	}

	if info.IsDir() {
		return fmt.Errorf("executable is a directory: %s", execPath)
	}

	// On Unix-like systems, check execute permission
	// On Windows, this check is less relevant but doesn't hurt
	mode := info.Mode()
	if mode&0111 == 0 {
		// No execute permission set
		return fmt.Errorf("executable does not have execute permission: %s", execPath)
	}

	return nil
}

// GetPluginForExtension returns the first plugin registered for the given file extension
// Extension comparison is case-insensitive
// For backwards compatibility, returns the first plugin if multiple are registered
func (r *PluginRegistry) GetPluginForExtension(ext string) (*PluginInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	extLower := strings.ToLower(ext)
	plugins, ok := r.plugins[extLower]
	if !ok || len(plugins) == 0 {
		return nil, false
	}
	return plugins[0], true
}

// GetPluginsForExtension returns all plugins registered for the given file extension
// Extension comparison is case-insensitive
func (r *PluginRegistry) GetPluginsForExtension(ext string) ([]*PluginInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	extLower := strings.ToLower(ext)
	plugins, ok := r.plugins[extLower]
	if !ok || len(plugins) == 0 {
		return nil, false
	}
	return plugins, true
}

// GetPluginByPath returns a plugin by its configuration path
// This is used when the user explicitly selects which plugin to use
// Deprecated: Use GetPluginByID instead for consistent identification
func (r *PluginRegistry) GetPluginByPath(path string) (*PluginInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Search through all plugins to find the one with matching path
	for _, plugins := range r.plugins {
		for _, plugin := range plugins {
			if plugin.Config.Path == path {
				return plugin, true
			}
		}
	}
	return nil, false
}

// GetPluginByID returns a plugin by its unique UUID identifier
// This is the preferred method for plugin lookup as UUIDs remain stable
// even if the plugin is moved to a different location
func (r *PluginRegistry) GetPluginByID(id string) (*PluginInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Search through all plugins to find the one with matching ID
	for _, plugins := range r.plugins {
		for _, plugin := range plugins {
			if plugin.Manifest.ID == id {
				return plugin, true
			}
		}
	}
	return nil, false
}

// ListPlugins returns all registered plugins
func (r *PluginRegistry) ListPlugins() []*PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Build unique list of plugins (same plugin can be registered for multiple extensions)
	// Use manifest ID for deduplication as it's the canonical identifier
	seen := make(map[string]bool)
	var result []*PluginInfo

	for _, plugins := range r.plugins {
		for _, plugin := range plugins {
			if !seen[plugin.Manifest.ID] {
				seen[plugin.Manifest.ID] = true
				result = append(result, plugin)
			}
		}
	}

	return result
}

// ValidatePlugin validates a plugin at the given path without loading it into the registry
// This is used by the UI to validate plugins before adding them
// The path can be either a plugin directory or an executable file
func (r *PluginRegistry) ValidatePlugin(path string) (*PluginManifest, error) {
	manifest, _, err := r.validatePluginConfig(settings.PluginConfig{
		Path: path,
	})
	return manifest, err
}

// GetSupportedExtensions returns a list of all file extensions supported by registered plugins
// Extensions are returned in lowercase and include the leading dot
func (r *PluginRegistry) GetSupportedExtensions() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Use a map to deduplicate extensions
	extensionSet := make(map[string]bool)

	for ext := range r.plugins {
		extensionSet[ext] = true
	}

	// Convert map keys to slice
	extensions := make([]string, 0, len(extensionSet))
	for ext := range extensionSet {
		extensions = append(extensions, ext)
	}

	return extensions
}
