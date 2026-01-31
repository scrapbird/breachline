package settings

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// GetEffectiveSettings returns the effective settings (defaults overlaid with file overrides if any).
// If anything goes wrong, it returns defaults.
func GetEffectiveSettings() Settings {
	settings := defaultSettings
	path, err := settingsFilePath()
	if err != nil {
		return settings
	}
	if _, err := os.Stat(path); err != nil {
		// no file or other stat error -> return defaults
		return settings
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return settings
	}
	var m map[string]any
	if err := yaml.Unmarshal(b, &m); err != nil {
		return settings
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
	return settings
}

func settingsFilePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(exe)
	return filepath.Join(dir, "breachline.yml"), nil
}
