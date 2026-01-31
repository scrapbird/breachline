package plugin

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	sharedtypes "github.com/scrapbird/breachline/shared/types"
)

// ReadPluginHeader reads the CSV header from a plugin-loaded file
func ReadPluginHeader(ctx context.Context, filePath string, options sharedtypes.FileOptions) ([]string, error) {
	// Get plugin for this file extension (respecting PluginID option if set)
	ext := filepath.Ext(filePath)
	plugin, ok := GetPluginForFileWithOptions(ext, options)
	if !ok {
		return nil, fmt.Errorf("no plugin registered for extension: %s", ext)
	}

	// Execute plugin to get headers
	executor := NewPluginExecutor(plugin)
	headers, err := executor.ReadHeader(ctx, filePath, options)
	if err != nil {
		return nil, fmt.Errorf("plugin header read failed: %w", err)
	}

	return headers, nil
}

// ReadPluginHeaderFromBytes is not supported for plugins
// Plugins require access to the actual file on disk
func ReadPluginHeaderFromBytes(data []byte, options sharedtypes.FileOptions) ([]string, error) {
	return nil, errors.New("reading plugin files from bytes is not supported")
}

// GetPluginRowCount returns the number of rows in a plugin-loaded file
func GetPluginRowCount(ctx context.Context, filePath string, options sharedtypes.FileOptions) (int, error) {
	// Get plugin for this file extension (respecting PluginID option if set)
	ext := filepath.Ext(filePath)
	plugin, ok := GetPluginForFileWithOptions(ext, options)
	if !ok {
		return 0, fmt.Errorf("no plugin registered for extension: %s", ext)
	}

	// Execute plugin to get row count
	executor := NewPluginExecutor(plugin)
	count, err := executor.GetRowCount(ctx, filePath, options)
	if err != nil {
		return 0, fmt.Errorf("plugin row count failed: %w", err)
	}

	return count, nil
}

// GetPluginRowCountFromBytes is not supported for plugins
// Plugins require access to the actual file on disk
func GetPluginRowCountFromBytes(data []byte, options sharedtypes.FileOptions) (int, error) {
	return 0, errors.New("reading plugin files from bytes is not supported")
}

// GetPluginReader returns a CSV reader for a plugin-loaded file
// The returned closer should be called when done with the reader
func GetPluginReader(ctx context.Context, filePath string, options sharedtypes.FileOptions) (*csv.Reader, io.ReadCloser, error) {
	// Get plugin for this file extension (respecting PluginID option if set)
	ext := filepath.Ext(filePath)
	plugin, ok := GetPluginForFileWithOptions(ext, options)
	if !ok {
		return nil, nil, fmt.Errorf("no plugin registered for extension: %s", ext)
	}

	// Execute plugin to get CSV reader
	executor := NewPluginExecutor(plugin)
	reader, closer, err := executor.GetReader(ctx, filePath, options)
	if err != nil {
		return nil, nil, fmt.Errorf("plugin reader failed: %w", err)
	}

	return reader, closer, nil
}

// GetPluginReaderFromBytes is not supported for plugins
// Plugins require access to the actual file on disk
func GetPluginReaderFromBytes(data []byte, options sharedtypes.FileOptions) (*csv.Reader, error) {
	return nil, errors.New("reading plugin files from bytes is not supported")
}

// GetPluginForFile retrieves the plugin info for a given file extension
func GetPluginForFile(ext string) (*PluginInfo, bool) {
	registry := GetPluginRegistry()
	if registry == nil {
		return nil, false
	}

	return registry.GetPluginForExtension(ext)
}

// GetPluginForFileWithOptions retrieves the plugin info, optionally using a specific plugin ID
func GetPluginForFileWithOptions(ext string, options sharedtypes.FileOptions) (*PluginInfo, bool) {
	registry := GetPluginRegistry()
	if registry == nil {
		return nil, false
	}

	// If a specific plugin ID is requested, use that (UUID-based lookup)
	if options.PluginID != "" {
		return registry.GetPluginByID(options.PluginID)
	}

	return registry.GetPluginForExtension(ext)
}
