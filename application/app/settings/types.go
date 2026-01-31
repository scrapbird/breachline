package settings

// PluginConfig represents a single plugin configuration
type PluginConfig struct {
	ID          string   `yaml:"id" json:"id"`                   // Unique plugin identifier (UUID from plugin.yml)
	Name        string   `yaml:"name" json:"name"`               // Display name
	Enabled     bool     `yaml:"enabled" json:"enabled"`         // Enable/disable toggle
	Path        string   `yaml:"path" json:"path"`               // Absolute path to plugin directory or executable
	Extensions  []string `yaml:"extensions" json:"extensions"`   // Cached from plugin.yml
	Description string   `yaml:"description" json:"description"` // Cached from plugin.yml
}

// Settings holds application settings that can be overridden by the user.
type Settings struct {
	// Remove omitempty so that false is serialized (we need to persist explicit overrides)
	SortByTime       bool `yaml:"sort_by_time" json:"sort_by_time"`
	SortDescending   bool `yaml:"sort_descending" json:"sort_descending"`
	EnableQueryCache bool `yaml:"enable_query_cache" json:"enable_query_cache"`
	// Cache size limit in MB for query cache (applies to all cache types)
	CacheSizeLimitMB int `yaml:"cache_size_limit_mb" json:"cache_size_limit_mb"`
	// Default timezone to assume when parsing timestamps that do not include an explicit timezone
	// Examples: "Local" (system local), "UTC", or any IANA TZ like "America/Los_Angeles"
	DefaultIngestTimezone string `yaml:"default_ingest_timezone" json:"default_ingest_timezone"`
	// Timezone to use for displaying times in the UI (histogram labels, etc.). Same semantics as above.
	DisplayTimezone string `yaml:"display_timezone" json:"display_timezone"`
	// Common time format string used to render timestamps in the UI and in copied/exported data.
	// Example: "yyyy-MM-dd HH:mm:ss" (e.g., 2024-12-31 23:59:59)
	TimestampDisplayFormat string `yaml:"timestamp_display_format" json:"timestamp_display_format"`
	// License holds the base64-encoded JWT license (not visible in settings dialog)
	License          string `yaml:"license,omitempty" json:"license,omitempty"`
	SyncSessionToken string `yaml:"sync_session_token,omitempty" json:"sync_session_token,omitempty"`
	SyncRefreshToken string `yaml:"sync_refresh_token,omitempty" json:"sync_refresh_token,omitempty"`
	// InstanceID is a unique identifier for this Breachline installation (not visible in settings dialog)
	InstanceID string `yaml:"instance_id,omitempty" json:"instance_id,omitempty"`
	// PinTimestampColumn controls whether the timestamp column is always shown as the first column
	PinTimestampColumn bool `yaml:"pin_timestamp_column" json:"pin_timestamp_column"`
	// Window size settings (not visible in settings dialog, but persisted)
	WindowWidth  int `yaml:"window_width,omitempty" json:"window_width,omitempty"`
	WindowHeight int `yaml:"window_height,omitempty" json:"window_height,omitempty"`
	// Maximum number of files when opening a directory as a virtual file
	MaxDirectoryFiles int `yaml:"max_directory_files" json:"max_directory_files"`
	// Plugin loader settings
	EnablePlugins bool           `yaml:"enable_plugins" json:"enable_plugins"`
	Plugins       []PluginConfig `yaml:"plugins,omitempty" json:"plugins,omitempty"`
}

// CacheManager interface defines methods that SettingsService needs for cache management
// This breaks the circular dependency between app and settings packages
type CacheManager interface {
	ClearAllTabCaches()
	UpdateCacheSize()
}

// defaultSettings defines the built-in defaults.
var defaultSettings = Settings{
	SortByTime:       false, // Changed from true to fix large file performance
	SortDescending:   false,
	EnableQueryCache: true,
	CacheSizeLimitMB: 100, // Default 100MB cache size
	// By default, interpret no-timezone timestamps in the system local timezone
	DefaultIngestTimezone: "Local",
	// By default, display in system local timezone
	DisplayTimezone: "Local",
	// Default display format for timestamps (common pattern, not Go layout)
	TimestampDisplayFormat: "yyyy-MM-dd HH:mm:ss",
	// Pin timestamp column is off by default
	PinTimestampColumn: false,
	// Default window size (matches main.go defaults)
	WindowWidth:  1024,
	WindowHeight: 768,
	// Default max files when opening a directory
	MaxDirectoryFiles: 500,
	// Plugin support disabled by default
	EnablePlugins: false,
	Plugins:       []PluginConfig{},
}
