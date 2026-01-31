package settings

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// SettingsService manages reading/writing settings from disk.
type SettingsService struct {
	ctx          context.Context
	cacheManager CacheManager
}

func NewSettingsService() *SettingsService {
	return &SettingsService{}
}

// SetCacheManager allows the main function to inject the cache manager
func (s *SettingsService) SetCacheManager(cm CacheManager) {
	s.cacheManager = cm
}

// Startup receives the Wails context
func (s *SettingsService) Startup(ctx context.Context) {
	s.ctx = ctx
}

// GetSettings returns the effective settings (defaults overlaid with file overrides if any).
func (s *SettingsService) GetSettings() (Settings, error) {
	// Start with defaults and overlay any on-disk overrides
	settings := defaultSettings
	path, err := settingsFilePath()
	if err != nil {
		return settings, err
	}
	// If file doesn't exist, return defaults
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return settings, nil
		}
		return settings, err
	}
	// Read and unmarshal
	b, err := os.ReadFile(path)
	if err != nil {
		return settings, err
	}
	// Unmarshal into a generic map to detect key presence
	var m map[string]any
	if err := yaml.Unmarshal(b, &m); err != nil {
		return settings, err
	}
	if v, ok := m["sort_by_time"]; ok {
		if vb, okb := v.(bool); okb {
			settings.SortByTime = vb
		}
	}
	if v, ok := m["sort_descending"]; ok {
		if vb, okb := v.(bool); okb {
			settings.SortDescending = vb
		}
	}
	if v, ok := m["enable_query_cache"]; ok {
		if vb, okb := v.(bool); okb {
			settings.EnableQueryCache = vb
		}
	}
	if v, ok := m["cache_size_limit_mb"]; ok {
		if vi, oki := v.(int); oki {
			settings.CacheSizeLimitMB = vi
		}
	}
	if v, ok := m["default_ingest_timezone"]; ok {
		if vs, oks := v.(string); oks {
			settings.DefaultIngestTimezone = vs
		}
	}
	if v, ok := m["display_timezone"]; ok {
		if vs, oks := v.(string); oks {
			settings.DisplayTimezone = vs
		}
	}
	if v, ok := m["timestamp_display_format"]; ok {
		if vs, oks := v.(string); oks {
			settings.TimestampDisplayFormat = vs
		}
	}
	if v, ok := m["license"]; ok {
		if vs, oks := v.(string); oks {
			settings.License = vs
		}
	}
	if v, ok := m["sync_session_token"]; ok {
		if vs, oks := v.(string); oks {
			settings.SyncSessionToken = vs
		}
	}
	if v, ok := m["sync_refresh_token"]; ok {
		if vs, oks := v.(string); oks {
			settings.SyncRefreshToken = vs
		}
	}
	if v, ok := m["instance_id"]; ok {
		if vs, oks := v.(string); oks {
			settings.InstanceID = vs
		}
	}
	if v, ok := m["pin_timestamp_column"]; ok {
		if vb, okb := v.(bool); okb {
			settings.PinTimestampColumn = vb
		}
	}
	if v, ok := m["window_width"]; ok {
		if vi, oki := v.(int); oki && vi >= 400 {
			settings.WindowWidth = vi
		}
	}
	if v, ok := m["window_height"]; ok {
		if vi, oki := v.(int); oki && vi >= 300 {
			settings.WindowHeight = vi
		}
	}
	if v, ok := m["max_directory_files"]; ok {
		if vi, oki := v.(int); oki && vi >= 10 {
			settings.MaxDirectoryFiles = vi
		}
	}
	if v, ok := m["enable_plugins"]; ok {
		if vb, okb := v.(bool); okb {
			settings.EnablePlugins = vb
		}
	}
	if v, ok := m["plugins"]; ok {
		// Parse plugins array
		if pluginsArray, ok := v.([]interface{}); ok {
			plugins := make([]PluginConfig, 0, len(pluginsArray))
			for _, p := range pluginsArray {
				if pluginMap, ok := p.(map[string]interface{}); ok {
					plugin := PluginConfig{}
					if id, ok := pluginMap["id"].(string); ok {
						plugin.ID = id
					}
					if name, ok := pluginMap["name"].(string); ok {
						plugin.Name = name
					}
					if enabled, ok := pluginMap["enabled"].(bool); ok {
						plugin.Enabled = enabled
					}
					if path, ok := pluginMap["path"].(string); ok {
						plugin.Path = path
					}
					if description, ok := pluginMap["description"].(string); ok {
						plugin.Description = description
					}
					if exts, ok := pluginMap["extensions"].([]interface{}); ok {
						plugin.Extensions = make([]string, 0, len(exts))
						for _, ext := range exts {
							if extStr, ok := ext.(string); ok {
								plugin.Extensions = append(plugin.Extensions, extStr)
							}
						}
					}
					plugins = append(plugins, plugin)
				}
			}
			settings.Plugins = plugins
		}
	}
	return settings, nil
}

// SaveSettings saves only the values that differ from defaults into YAML in the binary directory.
func (s *SettingsService) SaveSettings(in Settings) error {
	// Get current settings to detect changes
	old := GetEffectiveSettings()
	sortChanged := old.SortByTime != in.SortByTime || old.SortDescending != in.SortDescending
	cacheSizeChanged := old.CacheSizeLimitMB != in.CacheSizeLimitMB

	// Build a minimal map containing only non-default values to avoid zero-value serialization pitfalls
	data := make(map[string]any)
	if in.SortByTime != defaultSettings.SortByTime {
		data["sort_by_time"] = in.SortByTime
	}
	if in.SortDescending != defaultSettings.SortDescending {
		data["sort_descending"] = in.SortDescending
	}
	if in.EnableQueryCache != defaultSettings.EnableQueryCache {
		data["enable_query_cache"] = in.EnableQueryCache
	}
	if in.CacheSizeLimitMB != defaultSettings.CacheSizeLimitMB {
		data["cache_size_limit_mb"] = in.CacheSizeLimitMB
	}
	if strings.TrimSpace(in.DefaultIngestTimezone) != strings.TrimSpace(defaultSettings.DefaultIngestTimezone) {
		data["default_ingest_timezone"] = strings.TrimSpace(in.DefaultIngestTimezone)
	}
	if strings.TrimSpace(in.DisplayTimezone) != strings.TrimSpace(defaultSettings.DisplayTimezone) {
		data["display_timezone"] = strings.TrimSpace(in.DisplayTimezone)
	}
	if strings.TrimSpace(in.TimestampDisplayFormat) != strings.TrimSpace(defaultSettings.TimestampDisplayFormat) {
		data["timestamp_display_format"] = strings.TrimSpace(in.TimestampDisplayFormat)
	}
	// Preserve existing license from file (not visible in settings dialog, but must persist)
	// Use incoming license if provided, otherwise use the existing one from old settings
	licenseToSave := strings.TrimSpace(in.License)
	if licenseToSave == "" {
		licenseToSave = strings.TrimSpace(old.License)
	}
	if licenseToSave != "" {
		data["license"] = licenseToSave
	}

	// Preserve sync tokens (not visible in settings dialog, but must persist)
	// Use incoming tokens if provided, otherwise use the existing ones from old settings
	syncSessionToken := strings.TrimSpace(in.SyncSessionToken)
	if syncSessionToken == "" {
		syncSessionToken = strings.TrimSpace(old.SyncSessionToken)
	}
	if syncSessionToken != "" {
		data["sync_session_token"] = syncSessionToken
	}

	syncRefreshToken := strings.TrimSpace(in.SyncRefreshToken)
	if syncRefreshToken == "" {
		syncRefreshToken = strings.TrimSpace(old.SyncRefreshToken)
	}
	if syncRefreshToken != "" {
		data["sync_refresh_token"] = syncRefreshToken
	}

	// Preserve instance ID (not visible in settings dialog, but must persist)
	// Use incoming instance ID if provided, otherwise use the existing one from old settings
	instanceID := strings.TrimSpace(in.InstanceID)
	if instanceID == "" {
		instanceID = strings.TrimSpace(old.InstanceID)
	}
	if instanceID != "" {
		data["instance_id"] = instanceID
	}

	// Save pin timestamp column setting if different from default
	if in.PinTimestampColumn != defaultSettings.PinTimestampColumn {
		data["pin_timestamp_column"] = in.PinTimestampColumn
	}

	// Preserve window size (not visible in settings dialog, but must persist)
	// Use incoming window size if provided, otherwise use the existing one from old settings
	windowWidth := in.WindowWidth
	if windowWidth == 0 {
		windowWidth = old.WindowWidth
	}
	if windowWidth != defaultSettings.WindowWidth && windowWidth >= 400 {
		data["window_width"] = windowWidth
	}

	windowHeight := in.WindowHeight
	if windowHeight == 0 {
		windowHeight = old.WindowHeight
	}
	if windowHeight != defaultSettings.WindowHeight && windowHeight >= 300 {
		data["window_height"] = windowHeight
	}

	// Save max directory files setting if different from default
	maxDirFiles := in.MaxDirectoryFiles
	if maxDirFiles == 0 {
		maxDirFiles = old.MaxDirectoryFiles
	}
	if maxDirFiles != defaultSettings.MaxDirectoryFiles && maxDirFiles >= 10 {
		data["max_directory_files"] = maxDirFiles
	}

	// Save plugin settings if different from default
	if in.EnablePlugins != defaultSettings.EnablePlugins {
		data["enable_plugins"] = in.EnablePlugins
	}
	// Always save plugins array if it's not empty, preserving from old settings if not provided
	pluginsToSave := in.Plugins
	if pluginsToSave == nil && old.Plugins != nil {
		pluginsToSave = old.Plugins
	}
	if len(pluginsToSave) > 0 {
		data["plugins"] = pluginsToSave
	}

	path, err := settingsFilePath()
	if err != nil {
		return err
	}

	if len(data) == 0 {
		// If there is an existing file, remove it to reflect defaults-only state
		if _, statErr := os.Stat(path); statErr == nil {
			_ = os.Remove(path)
		}
		// Clear caches if sort settings changed
		if sortChanged && s.cacheManager != nil {
			s.cacheManager.ClearAllTabCaches()
		}
		return nil
	}
	b, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return err
	}

	// Clear all tab caches if sort settings changed
	// This ensures subsequent queries use fresh data with new sort settings
	if sortChanged && s.cacheManager != nil {
		s.cacheManager.ClearAllTabCaches()
	}

	// Update cache size if cache size setting changed
	if cacheSizeChanged && s.cacheManager != nil {
		s.cacheManager.UpdateCacheSize()
	}

	return nil
}

// ClearSyncTokens removes the sync session and refresh tokens from the settings file
func (s *SettingsService) ClearSyncTokens() error {
	// Get current settings
	settings, err := s.GetSettings()
	if err != nil {
		return err
	}

	// Build a map with all current settings except sync tokens
	data := make(map[string]any)

	// Add non-default settings (excluding sync tokens)
	if settings.SortByTime != defaultSettings.SortByTime {
		data["sort_by_time"] = settings.SortByTime
	}
	if settings.SortDescending != defaultSettings.SortDescending {
		data["sort_descending"] = settings.SortDescending
	}
	if settings.EnableQueryCache != defaultSettings.EnableQueryCache {
		data["enable_query_cache"] = settings.EnableQueryCache
	}
	if settings.CacheSizeLimitMB != defaultSettings.CacheSizeLimitMB {
		data["cache_size_limit_mb"] = settings.CacheSizeLimitMB
	}
	if strings.TrimSpace(settings.DefaultIngestTimezone) != strings.TrimSpace(defaultSettings.DefaultIngestTimezone) {
		data["default_ingest_timezone"] = strings.TrimSpace(settings.DefaultIngestTimezone)
	}
	if strings.TrimSpace(settings.DisplayTimezone) != strings.TrimSpace(defaultSettings.DisplayTimezone) {
		data["display_timezone"] = strings.TrimSpace(settings.DisplayTimezone)
	}
	if strings.TrimSpace(settings.TimestampDisplayFormat) != strings.TrimSpace(defaultSettings.TimestampDisplayFormat) {
		data["timestamp_display_format"] = strings.TrimSpace(settings.TimestampDisplayFormat)
	}

	// Preserve pin timestamp column setting
	if settings.PinTimestampColumn != defaultSettings.PinTimestampColumn {
		data["pin_timestamp_column"] = settings.PinTimestampColumn
	}

	// Preserve license but exclude sync tokens
	licenseToSave := strings.TrimSpace(settings.License)
	if licenseToSave != "" {
		data["license"] = licenseToSave
	}

	// Preserve instance ID (must not be cleared during logout)
	instanceID := strings.TrimSpace(settings.InstanceID)
	if instanceID != "" {
		data["instance_id"] = instanceID
	}

	// Preserve max directory files setting
	if settings.MaxDirectoryFiles != defaultSettings.MaxDirectoryFiles && settings.MaxDirectoryFiles >= 10 {
		data["max_directory_files"] = settings.MaxDirectoryFiles
	}

	// Preserve plugin settings
	if settings.EnablePlugins != defaultSettings.EnablePlugins {
		data["enable_plugins"] = settings.EnablePlugins
	}
	if len(settings.Plugins) > 0 {
		data["plugins"] = settings.Plugins
	}

	// Note: Intentionally NOT adding sync tokens - this clears them

	path, err := settingsFilePath()
	if err != nil {
		return err
	}

	if len(data) == 0 {
		// If there is an existing file, remove it to reflect defaults-only state
		if _, statErr := os.Stat(path); statErr == nil {
			_ = os.Remove(path)
		}
		return nil
	}

	b, err := yaml.Marshal(data)
	if err != nil {
		return err
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, b, 0o644)
}

// EnsureInstanceID generates and saves a unique instance ID if one doesn't exist
func (s *SettingsService) EnsureInstanceID() error {
	settings, err := s.GetSettings()
	if err != nil {
		return err
	}

	// If instance ID already exists, nothing to do
	if strings.TrimSpace(settings.InstanceID) != "" {
		return nil
	}

	// Generate new UUID for this instance
	newInstanceID := uuid.New().String()
	settings.InstanceID = newInstanceID

	// Save the settings with the new instance ID
	return s.SaveSettings(settings)
}

// GetPlugins returns all configured plugins
func (s *SettingsService) GetPlugins() ([]PluginConfig, error) {
	settings, err := s.GetSettings()
	if err != nil {
		return nil, err
	}
	return settings.Plugins, nil
}

// AddPlugin validates and adds a new plugin to the settings
// Returns the newly created PluginConfig
func (s *SettingsService) AddPlugin(path string) (*PluginConfig, error) {
	// Import fileloader here to avoid circular dependency
	// We'll use reflection or direct call
	// For now, we'll do basic validation and let the registry handle detailed validation

	if path == "" {
		return nil, errors.New("plugin path cannot be empty")
	}

	// Check if path exists
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("plugin path does not exist")
		}
		return nil, err
	}

	// Get current settings
	settings, err := s.GetSettings()
	if err != nil {
		return nil, err
	}

	// Check if plugin already exists
	for _, plugin := range settings.Plugins {
		if plugin.Path == path {
			return nil, errors.New("plugin already exists at this path")
		}
	}

	// Read plugin manifest to get metadata
	// We need to determine if path is a directory or executable
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	var pluginDir string
	if info.IsDir() {
		pluginDir = path
	} else {
		pluginDir = filepath.Dir(path)
	}

	manifestPath := filepath.Join(pluginDir, "plugin.yml")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("plugin.yml not found in plugin directory")
		}
		return nil, err
	}

	// Parse manifest
	var manifest struct {
		ID          string   `yaml:"id"`
		Name        string   `yaml:"name"`
		Version     string   `yaml:"version"`
		Description string   `yaml:"description"`
		Extensions  []string `yaml:"extensions"`
	}
	if err := yaml.Unmarshal(manifestData, &manifest); err != nil {
		return nil, errors.New("invalid plugin.yml format")
	}

	// Validate required ID field
	if manifest.ID == "" {
		return nil, errors.New("plugin.yml missing required field: id (UUID)")
	}

	// Check if plugin with same ID already exists
	for _, plugin := range settings.Plugins {
		if plugin.ID == manifest.ID {
			return nil, fmt.Errorf("plugin with ID %s already exists", manifest.ID)
		}
	}

	// Create plugin config
	newPlugin := PluginConfig{
		ID:          manifest.ID,
		Name:        manifest.Name,
		Enabled:     true, // Enable by default
		Path:        path,
		Extensions:  manifest.Extensions,
		Description: manifest.Description,
	}

	// Add to settings
	settings.Plugins = append(settings.Plugins, newPlugin)

	// Save settings
	if err := s.SaveSettings(settings); err != nil {
		return nil, err
	}

	return &newPlugin, nil
}

// RemovePlugin removes a plugin from the settings
func (s *SettingsService) RemovePlugin(path string) error {
	settings, err := s.GetSettings()
	if err != nil {
		return err
	}

	// Find and remove plugin
	found := false
	newPlugins := make([]PluginConfig, 0, len(settings.Plugins))
	for _, plugin := range settings.Plugins {
		if plugin.Path != path {
			newPlugins = append(newPlugins, plugin)
		} else {
			found = true
		}
	}

	if !found {
		return errors.New("plugin not found")
	}

	settings.Plugins = newPlugins

	// Save settings
	return s.SaveSettings(settings)
}

// TogglePlugin enables or disables a plugin
func (s *SettingsService) TogglePlugin(path string, enabled bool) error {
	settings, err := s.GetSettings()
	if err != nil {
		return err
	}

	// Find and update plugin
	found := false
	for i := range settings.Plugins {
		if settings.Plugins[i].Path == path {
			settings.Plugins[i].Enabled = enabled
			found = true
			break
		}
	}

	if !found {
		return errors.New("plugin not found")
	}

	// Save settings
	return s.SaveSettings(settings)
}
