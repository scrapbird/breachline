package plugin

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"breachline/app/settings"
)

// PluginOption represents a plugin that can handle a specific file type
type PluginOption struct {
	ID             string   `json:"id"` // Plugin UUID from plugin.yml
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Path           string   `json:"path"`
	Extensions     []string `json:"extensions"`
	ExecutablePath string   `json:"executablePath"` // Full path to plugin executable
	ExecutableHash string   `json:"executableHash"` // SHA256 hash of plugin executable
}

// Logger interface for logging messages
type Logger interface {
	Log(level, message string)
}

// PluginService handles plugin management operations
type PluginService struct {
	settingsService *settings.SettingsService
	logger          Logger
}

// NewPluginService creates a new plugin service
func NewPluginService(settingsService *settings.SettingsService, logger Logger) *PluginService {
	return &PluginService{
		settingsService: settingsService,
		logger:          logger,
	}
}

// GetPlugins returns all configured plugins
func (ps *PluginService) GetPlugins() ([]settings.PluginConfig, error) {
	return ps.settingsService.GetPlugins()
}

// AddPlugin validates and adds a new plugin to the settings
func (ps *PluginService) AddPlugin(path string) (*settings.PluginConfig, error) {
	plugin, err := ps.settingsService.AddPlugin(path)
	if err != nil {
		return nil, err
	}

	// Reload plugin registry after adding
	currentSettings := settings.GetEffectiveSettings()
	if currentSettings.EnablePlugins {
		if err := InitializePluginRegistry(currentSettings.Plugins); err != nil {
			ps.log("error", fmt.Sprintf("Failed to reload plugin registry: %v", err))
		}
	}

	return plugin, nil
}

// RemovePlugin removes a plugin from the settings
func (ps *PluginService) RemovePlugin(path string) error {
	if err := ps.settingsService.RemovePlugin(path); err != nil {
		return err
	}

	// Reload plugin registry after removing
	currentSettings := settings.GetEffectiveSettings()
	if currentSettings.EnablePlugins {
		if err := InitializePluginRegistry(currentSettings.Plugins); err != nil {
			ps.log("error", fmt.Sprintf("Failed to reload plugin registry: %v", err))
		}
	}

	return nil
}

// TogglePlugin enables or disables a plugin
func (ps *PluginService) TogglePlugin(path string, enabled bool) error {
	if err := ps.settingsService.TogglePlugin(path, enabled); err != nil {
		return err
	}

	// Reload plugin registry after toggling
	currentSettings := settings.GetEffectiveSettings()
	if currentSettings.EnablePlugins {
		if err := InitializePluginRegistry(currentSettings.Plugins); err != nil {
			ps.log("error", fmt.Sprintf("Failed to reload plugin registry: %v", err))
		}
	}

	return nil
}

// ValidatePluginPath validates a plugin path and returns the manifest info
func (ps *PluginService) ValidatePluginPath(path string) (*PluginManifest, error) {
	registry := NewPluginRegistry()
	return registry.ValidatePlugin(path)
}

// GetPluginsForFile returns all plugins that can handle the given file
// Returns an empty slice if no plugins support the file type or if the file
// is handled by built-in loaders (CSV, XLSX, JSON)
func (ps *PluginService) GetPluginsForFile(filePath string) []PluginOption {
	currentSettings := settings.GetEffectiveSettings()
	if !currentSettings.EnablePlugins {
		return nil
	}

	registry := GetPluginRegistry()
	if registry == nil {
		return nil
	}

	// Get the file extension (handle compressed files too)
	ext := GetUncompressedExtension(filePath)
	if ext == "" {
		ext = filepath.Ext(filePath)
	}

	// Check if this is a built-in type (CSV, XLSX, JSON)
	extLower := strings.ToLower(ext)
	if extLower == ".csv" || extLower == ".xlsx" || extLower == ".json" {
		return nil
	}

	plugins, ok := registry.GetPluginsForExtension(ext)
	if !ok || len(plugins) == 0 {
		return nil
	}

	// Convert to PluginOption structs
	result := make([]PluginOption, len(plugins))
	for i, p := range plugins {
		// Calculate SHA256 hash of the executable
		execHash := ""
		if p.ExecPath != "" {
			if hashBytes, err := calculateFileSHA256(p.ExecPath); err == nil {
				execHash = hashBytes
			}
		}
		result[i] = PluginOption{
			ID:             p.Manifest.ID,
			Name:           p.Manifest.Name,
			Description:    p.Manifest.Description,
			Path:           p.Config.Path,
			Extensions:     p.Manifest.Extensions,
			ExecutablePath: p.ExecPath,
			ExecutableHash: execHash,
		}
	}

	return result
}

// log logs a message if logger is available
func (ps *PluginService) log(level, message string) {
	if ps.logger != nil {
		ps.logger.Log(level, message)
	}
}

// calculateFileSHA256 calculates the SHA256 hash of a file and returns it as a hex string
func calculateFileSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// PluginRequirementResult contains information about plugin requirements for a file
type PluginRequirementResult struct {
	RequiresPlugin     bool           `json:"requiresPlugin"`     // File type requires a plugin to open
	PluginsEnabled     bool           `json:"pluginsEnabled"`     // Global plugin support is enabled
	PluginAvailable    bool           `json:"pluginAvailable"`    // A suitable enabled plugin is available
	RequiredPluginID   string         `json:"requiredPluginId"`   // UUID of the required plugin (from stored options or detection)
	RequiredPluginName string         `json:"requiredPluginName"` // Name of the required plugin
	FileExtension      string         `json:"fileExtension"`      // The file extension that requires the plugin
	Plugins            []PluginOption `json:"plugins"`            // List of all available plugins for this file type
}

// CheckPluginRequirement checks if a file requires a plugin to load and returns
// detailed information about the plugin requirement status.
// The pluginId parameter is optional - if provided (e.g., from workspace file options),
// it checks specifically for that plugin's availability.
// The pluginName parameter is optional - if provided (e.g., from workspace file options),
// it will be used as the required plugin name if the plugin isn't found in settings.
func (ps *PluginService) CheckPluginRequirement(filePath string, pluginId string, pluginName string) PluginRequirementResult {
	result := PluginRequirementResult{}

	// If we have stored plugin info from workspace, pre-populate the result
	// This ensures we can show plugin details even if the plugin isn't in current settings
	if pluginId != "" {
		result.RequiredPluginID = pluginId
		result.RequiredPluginName = pluginName
	}

	// Get the file extension (handle compressed files too)
	ext := GetUncompressedExtension(filePath)
	if ext == "" {
		ext = filepath.Ext(filePath)
	}
	result.FileExtension = ext

	// Check if this is a built-in type (CSV, XLSX, JSON) - these don't require plugins
	extLower := strings.ToLower(ext)
	if extLower == ".csv" || extLower == ".xlsx" || extLower == ".json" || ext == "" {
		result.RequiresPlugin = false
		return result
	}

	// Get settings to check plugin configuration
	currentSettings := settings.GetEffectiveSettings()
	result.PluginsEnabled = currentSettings.EnablePlugins

	// Get all possible plugins for this extension
	result.Plugins = ps.GetPluginsForFile(filePath)

	// Check if any configured plugin (enabled or not) supports this extension
	var matchingPlugin *settings.PluginConfig
	var matchingEnabledPlugin *settings.PluginConfig

	// Debug: log what we're looking for
	if ps.logger != nil && pluginId != "" {
		ps.log("debug", fmt.Sprintf("[CheckPluginRequirement] Looking for plugin ID: %q, file: %s", pluginId, filePath))
		ps.log("debug", fmt.Sprintf("[CheckPluginRequirement] Available plugins: %d, plugins enabled: %t", len(currentSettings.Plugins), currentSettings.EnablePlugins))
	}

	for i := range currentSettings.Plugins {
		plugin := &currentSettings.Plugins[i]

		// Debug: log each plugin we're checking
		if ps.logger != nil && pluginId != "" {
			ps.log("debug", fmt.Sprintf("[CheckPluginRequirement] Checking plugin ID: %q, Name: %s, Enabled: %t", plugin.ID, plugin.Name, plugin.Enabled))
		}

		// If a specific pluginId is requested, only look for that plugin
		if pluginId != "" {
			if plugin.ID == pluginId {
				ps.log("debug", fmt.Sprintf("[CheckPluginRequirement] MATCH found for plugin ID: %q", pluginId))
				matchingPlugin = plugin
				if plugin.Enabled {
					matchingEnabledPlugin = plugin
				}
				break
			}
			continue
		}

		// Otherwise, check if plugin supports this extension
		for _, pluginExt := range plugin.Extensions {
			if strings.EqualFold(pluginExt, ext) {
				if matchingPlugin == nil {
					matchingPlugin = plugin
				}
				if plugin.Enabled && matchingEnabledPlugin == nil {
					matchingEnabledPlugin = plugin
				}
				break
			}
		}
	}

	// If we found a matching plugin (enabled or not), the file requires a plugin
	if matchingPlugin != nil {
		result.RequiresPlugin = true
		result.RequiredPluginID = matchingPlugin.ID
		result.RequiredPluginName = matchingPlugin.Name
		result.PluginAvailable = false

		// Check if plugin is actually available (enabled + plugins enabled globally)
		if currentSettings.EnablePlugins && matchingEnabledPlugin != nil {
			result.PluginAvailable = true
			// Use the enabled plugin's info
			result.RequiredPluginID = matchingEnabledPlugin.ID
			result.RequiredPluginName = matchingEnabledPlugin.Name
		}
	} else {
		// No plugin configured for this extension, but since it's not a built-in type,
		// it would require a plugin to open properly
		result.RequiresPlugin = true
		result.PluginAvailable = false
	}

	return result
}

// GetUncompressedExtension returns the extension of a file after stripping compression extensions
// For example: "file.csv.gz" returns ".csv"
// Returns empty string if the file has no extension after compression handling
func GetUncompressedExtension(filePath string) string {
	// Common compression extensions
	compressionExts := map[string]bool{
		".gz":  true,
		".bz2": true,
		".xz":  true,
		".zip": true,
		".zst": true,
	}

	ext := filepath.Ext(filePath)
	extLower := strings.ToLower(ext)

	// If the extension is a compression extension, get the underlying extension
	if compressionExts[extLower] {
		// Strip the compression extension and get the underlying one
		baseName := strings.TrimSuffix(filePath, ext)
		underlyingExt := filepath.Ext(baseName)
		if underlyingExt != "" {
			return underlyingExt
		}
	}

	return ext
}
