// Package types provides shared type definitions used across the BreachLine application
// and its supporting infrastructure (sync-api, etc.)
package types

// FileOptions contains all options that define a virtual file variant.
// Two files with the same hash but different options are considered different virtual files.
//
// This is the canonical definition used by both the application and the sync-api.
// JSON/YAML tags use camelCase for frontend compatibility.
// DynamoDB tags use snake_case for database storage.
type FileOptions struct {
	// Common file options
	JPath                  string `json:"jpath,omitempty" dynamodbav:"jpath,omitempty" yaml:"jpath,omitempty"`
	NoHeaderRow            bool   `json:"noHeaderRow,omitempty" dynamodbav:"no_header_row,omitempty" yaml:"noHeaderRow,omitempty"`
	IngestTimezoneOverride string `json:"ingestTimezoneOverride,omitempty" dynamodbav:"ingest_timezone_override,omitempty" yaml:"ingestTimezoneOverride,omitempty"`

	// Plugin options - PluginID is the UUID from plugin.yml that uniquely identifies the plugin
	PluginID   string `json:"pluginId,omitempty" dynamodbav:"plugin_id,omitempty" yaml:"pluginId,omitempty"`
	PluginName string `json:"pluginName,omitempty" dynamodbav:"plugin_name,omitempty" yaml:"pluginName,omitempty"` // Display name for UI

	// Directory loading options
	IsDirectory         bool   `json:"isDirectory,omitempty" dynamodbav:"is_directory,omitempty" yaml:"isDirectory,omitempty"`
	FilePattern         string `json:"filePattern,omitempty" dynamodbav:"file_pattern,omitempty" yaml:"filePattern,omitempty"`
	IncludeSourceColumn bool   `json:"includeSourceColumn,omitempty" dynamodbav:"include_source_column,omitempty" yaml:"includeSourceColumn,omitempty"`
}

// Key returns a unique string key for this options combination.
// Used for composite keys and map lookups.
func (fo FileOptions) Key() string {
	noHeaderStr := "false"
	if fo.NoHeaderRow {
		noHeaderStr = "true"
	}
	tzStr := fo.IngestTimezoneOverride
	if tzStr == "" {
		tzStr = "default"
	}
	// Include plugin ID in key (UUID-based identification)
	pluginStr := fo.PluginID
	if pluginStr == "" {
		pluginStr = "default"
	}
	// Include directory options in key
	dirStr := "file"
	if fo.IsDirectory {
		dirStr = "dir"
		if fo.FilePattern != "" {
			dirStr += ":" + fo.FilePattern
		}
		if fo.IncludeSourceColumn {
			dirStr += ":src"
		}
	}
	return fo.JPath + "::" + noHeaderStr + "::" + tzStr + "::" + pluginStr + "::" + dirStr
}

// IsEmpty returns true if all options are at default values.
func (fo FileOptions) IsEmpty() bool {
	return fo.JPath == "" && !fo.NoHeaderRow && fo.IngestTimezoneOverride == "" &&
		fo.PluginID == "" && !fo.IsDirectory && fo.FilePattern == "" && !fo.IncludeSourceColumn
}

// Equals returns true if two FileOptions are equivalent.
func (fo FileOptions) Equals(other FileOptions) bool {
	return fo.Key() == other.Key()
}

// DefaultFileOptions returns the default file options.
func DefaultFileOptions() FileOptions {
	return FileOptions{
		NoHeaderRow: false,
	}
}
