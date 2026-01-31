package app

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"breachline/app/cache"
	"breachline/app/fileloader"
	"breachline/app/histogram"
	"breachline/app/interfaces"
	"breachline/app/plugin"
	"breachline/app/query"
	"breachline/app/settings"
	"breachline/app/timestamps"

	"github.com/minio/highwayhash"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	clipboard "golang.design/x/clipboard"
)

// App struct
type App struct {
	ctx context.Context

	// Multi-tab support
	tabsMu      sync.RWMutex
	tabs        map[string]*FileTab // keyed by tab ID
	activeTabID string
	nextTabID   int64

	// clipboard init
	clipOnce sync.Once
	clipOK   bool

	// workspace service for annotations
	workspaceService *WorkspaceManager

	// persistent query cache
	queryCache *cache.Cache

	// plugin service for plugin management
	pluginService *plugin.PluginService

	// locate files cancellation support
	locateCancelFunc context.CancelFunc
	locateCancelMu   sync.Mutex
}

// NewApp creates a new App application struct
func NewApp() *App {
	// Load settings to get cache size
	currentSettings := settings.GetEffectiveSettings()
	cacheSizeBytes := int64(currentSettings.CacheSizeLimitMB) * 1024 * 1024

	app := &App{
		tabs:       make(map[string]*FileTab),
		queryCache: cache.NewCache(cacheSizeBytes), // Settings-based cache size
	}

	// Set the logger for the cache after app is created
	// Note: The logger will be available after Startup() is called and ctx is set

	return app
}

// NewWorkspaceService creates a new workspace manager
func NewWorkspaceService() *WorkspaceManager {
	return NewWorkspaceManager()
}

// NewSyncService creates a new sync service
func NewSyncService() interface{} {
	// Import is in sync package, return as interface to avoid circular dependency
	return nil // Will be properly implemented
}

// GetActiveTab returns the currently active file tab (nil if none)
func (a *App) GetActiveTab() *FileTab {
	a.tabsMu.RLock()
	defer a.tabsMu.RUnlock()
	if a.activeTabID == "" {
		return nil
	}
	return a.tabs[a.activeTabID]
}

// GetQueryCache returns the query cache for cache invalidation registration
func (a *App) GetQueryCache() interface{} {
	return a.queryCache
}

// CacheStatsResponse contains cache statistics for the frontend
type CacheStatsResponse struct {
	TotalSize    int64   `json:"totalSize"`
	MaxSize      int64   `json:"maxSize"`
	UsagePercent float64 `json:"usagePercent"`
	EntryCount   int     `json:"entryCount"`
}

// GetCacheStats returns the current cache statistics for the frontend
func (a *App) GetCacheStats() CacheStatsResponse {
	if a.queryCache == nil {
		return CacheStatsResponse{}
	}
	stats := a.queryCache.GetCacheStats()
	return CacheStatsResponse{
		TotalSize:    stats.TotalSize,
		MaxSize:      stats.MaxSize,
		UsagePercent: stats.UsagePercent,
		EntryCount:   stats.TotalEntries,
	}
}

// GetTab returns a specific tab by ID
func (a *App) GetTab(tabID string) *FileTab {
	a.tabsMu.RLock()
	defer a.tabsMu.RUnlock()
	return a.tabs[tabID]
}

// SetWorkspaceService sets the workspace service for the app
func (a *App) SetWorkspaceService(ws *WorkspaceManager) {
	a.workspaceService = ws
}

// RangeSpec represents an inclusive range of zero-based row indexes in the filtered view
type RangeSpec struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// CopySelectionRequest carries the selection and projection information for backend clipboard copy
type CopySelectionRequest struct {
	Ranges                 []RangeSpec       `json:"ranges"`
	VirtualSelectAll       bool              `json:"virtualSelectAll"`
	Fields                 []string          `json:"fields"`
	Headers                []string          `json:"headers"`
	Query                  string            `json:"query"`
	TimeField              string            `json:"timeField"`
	ColumnJPathExpressions map[string]string `json:"columnJPathExpressions"` // Column name -> JPath expression for extracting displayed values
}

// CopySelectionResult reports the number of data rows copied
type CopySelectionResult struct {
	RowsCopied int `json:"rowsCopied"`
}

// AnnotationSelectionRequest carries the selection information for annotation operations
type AnnotationSelectionRequest struct {
	Ranges           []RangeSpec `json:"ranges"`
	VirtualSelectAll bool        `json:"virtualSelectAll"`
	Query            string      `json:"query"`
	TimeField        string      `json:"timeField"`
	Note             string      `json:"note"`
	Color            string      `json:"color"`
}

// AnnotationSelectionResult reports the number of rows annotated
type AnnotationSelectionResult struct {
	RowsAnnotated int `json:"rowsAnnotated"`
}

// CopySelectionToClipboard delegates to tab-based implementation
func (a *App) CopySelectionToClipboard(req CopySelectionRequest) (*CopySelectionResult, error) {
	tab := a.GetActiveTab()
	if tab == nil {
		return &CopySelectionResult{RowsCopied: 0}, nil
	}
	return a.copySelectionToClipboardForTab(tab, req)
}

// CopyPNGFromDataURL decodes a data URL (PNG) and puts the image on the system clipboard.
// Returns true on success. This uses golang.design/x/clipboard under the hood.
func (a *App) CopyPNGFromDataURL(dataURL string) (bool, error) {
	if a == nil {
		return false, fmt.Errorf("app not initialised")
	}
	// Lazy init clipboard
	a.clipOnce.Do(func() {
		if err := clipboard.Init(); err == nil {
			a.clipOK = true
		} else {
			a.clipOK = false
			if a.ctx != nil {
				a.Log("error", fmt.Sprintf("Clipboard init failed: %v", err))
			}
		}
	})
	if !a.clipOK {
		return false, fmt.Errorf("clipboard not available")
	}
	if strings.TrimSpace(dataURL) == "" {
		return false, fmt.Errorf("empty data URL")
	}
	comma := strings.Index(dataURL, ",")
	if comma < 0 {
		return false, fmt.Errorf("invalid data URL")
	}
	b64 := strings.TrimSpace(dataURL[comma+1:])
	imgBytes, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return false, fmt.Errorf("decode image: %w", err)
	}
	// Write PNG bytes to clipboard using safe write with panic recovery
	if err := safeClipboardWrite(clipboard.FmtImage, imgBytes); err != nil {
		a.Log("error", fmt.Sprintf("Clipboard write failed: %v", err))
		return false, fmt.Errorf("failed to copy image to clipboard: %v", err)
	}
	a.Log("info", "Histogram screenshot copied to clipboard (image)")
	return true, nil
}

// SavePNGFromDataURL opens a Save File dialog and saves a PNG decoded from a
// provided data URL (e.g. "data:image/png;base64,...."). Returns true on
// success. The defaultName (without path) is used to prefill the filename.
func (a *App) SavePNGFromDataURL(dataURL string, defaultName string) (bool, error) {
	if a == nil || a.ctx == nil {
		return false, fmt.Errorf("app context not initialised")
	}
	if strings.TrimSpace(dataURL) == "" {
		return false, fmt.Errorf("empty data URL")
	}
	// Extract the base64 payload
	comma := strings.Index(dataURL, ",")
	if comma < 0 {
		return false, fmt.Errorf("invalid data URL: no comma separator")
	}
	payload := dataURL[comma+1:]
	// Some data URLs may be URL-safe base64; replace spaces if any
	payload = strings.TrimSpace(payload)
	imgBytes, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return false, fmt.Errorf("failed to decode image base64: %w", err)
	}

	// Open Save File dialog
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save Histogram Screenshot",
		DefaultFilename: strings.TrimSpace(defaultName),
		Filters:         []runtime.FileFilter{{DisplayName: "PNG Image", Pattern: "*.png"}},
	})
	if err != nil {
		return false, err
	}
	if path == "" {
		// user cancelled
		return false, nil
	}
	// Ensure .png extension
	if !strings.HasSuffix(strings.ToLower(path), ".png") {
		path = path + ".png"
	}
	if err := os.WriteFile(path, imgBytes, 0o644); err != nil {
		return false, err
	}
	a.Log("info", fmt.Sprintf("Saved histogram screenshot to %s", filepath.Base(path)))
	return true, nil
}

// log emits a structured log event to the frontend console window
func (a *App) Log(level, message string) {
	if a == nil || a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "log", map[string]any{
		"level":   level,
		"message": message,
	})
}

// ternary returns a if cond is true, otherwise b
func ternary[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}

// ClearAllTabCaches clears all cached sort and query data for all tabs.
// This should be called when global settings change (e.g., sort settings)
// to ensure subsequent queries use fresh data with the new settings.
func (a *App) ClearAllTabCaches() {
	a.tabsMu.RLock()
	tabs := make([]*FileTab, 0, len(a.tabs))
	for _, tab := range a.tabs {
		tabs = append(tabs, tab)
	}
	a.tabsMu.RUnlock()

	// Clear caches for each tab
	for _, tab := range tabs {
		tab.CacheMu.Lock()
		// Clear sorted rows cache
		tab.SortedRows = nil
		tab.SortedHeader = nil
		tab.SortedForFile = ""
		tab.SortedTimeField = ""
		// Clear query cache
		tab.QueryCache = nil
		tab.QueryCacheOrder = nil
		// Broadcast to wake any waiting goroutines
		if tab.SortCond != nil {
			tab.SortCond.Broadcast()
		}
		tab.CacheMu.Unlock()

		// Cancel any in-progress sort operations
		tab.SortMu.Lock()
		if tab.SortCancel != nil {
			tab.SortCancel()
			tab.SortCancel = nil
		}
		tab.SortActive = 0
		tab.SortingForFile = ""
		tab.SortingTimeField = ""
		tab.SortMu.Unlock()
	}
}

// JSONPreviewRequest contains the data needed to preview JSON with a JSONPath expression
type JSONPreviewRequest struct {
	FilePath   string `json:"filePath"`
	Expression string `json:"expression"`
	MaxRows    int    `json:"maxRows"`
}

// JSONPreviewResponse contains the preview data
type JSONPreviewResponse struct {
	Headers       []string   `json:"headers"`
	Rows          [][]string `json:"rows"`
	Error         string     `json:"error"`
	AvailableKeys []string   `json:"availableKeys"` // Keys available at current path (when result is not an array)
}

// PreviewJSONWithExpression applies a JSONPath expression to a JSON file and returns preview data
func (a *App) PreviewJSONWithExpression(req JSONPreviewRequest) *JSONPreviewResponse {
	// Delegate to the fileloader package function
	result := fileloader.PreviewJSONWithExpression(req.FilePath, req.Expression, req.MaxRows)

	// Convert the result to the response type
	return &JSONPreviewResponse{
		Headers:       result.Headers,
		Rows:          result.Rows,
		Error:         result.Error,
		AvailableKeys: result.AvailableKeys,
	}
}

// SetTabJPath sets the JSONPath ingest expression for a tab
func (a *App) SetTabJPath(tabID string, expression string) error {
	a.tabsMu.Lock()
	defer a.tabsMu.Unlock()

	tab, exists := a.tabs[tabID]
	if !exists {
		return fmt.Errorf("tab not found: %s", tabID)
	}

	tab.Options.JPath = expression
	return nil
}

// Startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	// Set the logger for the cache now that we have a context
	if a.queryCache != nil {
		a.queryCache.SetLogger(a)

		// Set the cache for JSON file parsing so parsed JSON data is cached
		fileloader.SetJSONCache(a.queryCache)
	}

	// Initialize plugin service
	a.pluginService = plugin.NewPluginService(settings.NewSettingsService(), a)

	// Initialize plugin registry if plugins are enabled
	currentSettings := settings.GetEffectiveSettings()
	if currentSettings.EnablePlugins {
		if err := plugin.InitializePluginRegistry(currentSettings.Plugins); err != nil {
			a.Log("error", fmt.Sprintf("Failed to initialize plugin registry: %v", err))
		} else {
			a.Log("info", fmt.Sprintf("Initialized plugin registry with %d plugins", len(currentSettings.Plugins)))
		}
	}
}

// Ctx returns the app context
func (a *App) Ctx() context.Context {
	return a.ctx
}

// Note: DetectFileType has been moved to fileloader package.
// Callers should use fileloader.DetectFileType() directly.

// DetectTimestampIndex detects the timestamp column index
func (a *App) DetectTimestampIndex(header []string) int {
	return timestamps.DetectTimestampIndex(header)
}

// GetEffectiveSettings returns the effective settings
func (a *App) GetEffectiveSettings() *interfaces.Settings {
	currentSettings := settings.GetEffectiveSettings()
	return &interfaces.Settings{
		DisplayTimezone:        currentSettings.DisplayTimezone,
		DefaultIngestTimezone:  currentSettings.DefaultIngestTimezone,
		TimestampDisplayFormat: currentSettings.TimestampDisplayFormat,
		SortByTime:             currentSettings.SortByTime,
		SortDescending:         currentSettings.SortDescending,
		EnableQueryCache:       currentSettings.EnableQueryCache,
		License:                currentSettings.License,
		EnablePlugins:          currentSettings.EnablePlugins,
		Plugins:                convertPluginConfigs(currentSettings.Plugins),
	}
}

// convertPluginConfigs converts settings.PluginConfig to interfaces.PluginConfig
func convertPluginConfigs(configs []settings.PluginConfig) []interfaces.PluginConfig {
	result := make([]interfaces.PluginConfig, len(configs))
	for i, c := range configs {
		result[i] = interfaces.PluginConfig{
			Name:        c.Name,
			Enabled:     c.Enabled,
			Path:        c.Path,
			Extensions:  c.Extensions,
			Description: c.Description,
		}
	}
	return result
}

// SaveWindowSize saves the current window dimensions to the settings file
func (a *App) SaveWindowSize(width, height int) error {
	// Validate minimum window size
	if width < 400 || height < 300 {
		return fmt.Errorf("window size too small: minimum 400x300, got %dx%d", width, height)
	}

	// Get current settings
	currentSettings := settings.GetEffectiveSettings()

	// Update window size
	currentSettings.WindowWidth = width
	currentSettings.WindowHeight = height

	// Save settings using the settings service
	settingsService := settings.NewSettingsService()
	return settingsService.SaveSettings(currentSettings)
}

// GetSavedWindowSize returns the saved window dimensions from settings
func (a *App) GetSavedWindowSize() (width, height int, err error) {
	currentSettings := settings.GetEffectiveSettings()

	// Return saved dimensions or defaults if not set
	width = currentSettings.WindowWidth
	height = currentSettings.WindowHeight

	// Ensure minimum size constraints
	if width < 400 {
		width = 1024 // default
	}
	if height < 300 {
		height = 768 // default
	}

	return width, height, nil
}

// ParseTimestampMillis parses a timestamp string to milliseconds
func (a *App) ParseTimestampMillis(s string, fallbackLoc interface{}) (int64, bool) {
	var loc *time.Location
	if fallbackLoc != nil {
		if l, ok := fallbackLoc.(*time.Location); ok {
			loc = l
		}
	}
	ms, ok := timestamps.ParseTimestampMillis(s, loc)
	return ms, ok
}

// ParseTimestampMillisWithCache parses a timestamp string using cached parser if available
func (a *App) ParseTimestampMillisWithCache(s string, fallbackLoc interface{}, tab *interfaces.FileTab) (int64, bool) {
	if tab == nil {
		// Fallback to non-cached version
		return a.ParseTimestampMillis(s, fallbackLoc)
	}

	var loc *time.Location
	if fallbackLoc != nil {
		if l, ok := fallbackLoc.(*time.Location); ok {
			loc = l
		}
	}

	// Get current timezone setting for cache invalidation
	currentSettings := a.GetEffectiveSettings()
	currentTZ := currentSettings.DefaultIngestTimezone

	// Check if we have a cached parser
	tab.TimestampParserMu.RLock()
	cachedParser := tab.TimestampParser
	cachedTZ := tab.TimestampParserTZ
	tab.TimestampParserMu.RUnlock()

	// Check if cached parser is valid (same timezone setting)
	if cachedParser != nil && cachedTZ == currentTZ {
		if parserInfo, ok := cachedParser.(*timestamps.TimestampParserInfo); ok {
			if ms, success := parserInfo.Parse(s); success {
				return ms, true
			} else {
				// Cached parser failed - log error and fall back to detection
				a.Log("error", fmt.Sprintf("Cached timestamp parser failed for input '%s' in file %s", s, tab.FileName))
				// Clear the cached parser since it's failing
				tab.TimestampParserMu.Lock()
				tab.TimestampParser = nil
				tab.TimestampParserTZ = ""
				tab.TimestampParserMu.Unlock()
				return 0, false
			}
		}
	}

	// No cached parser or cache invalid - detect and cache new parser
	parserInfo, ms, ok := timestamps.DetectAndCacheTimestampParser(s, loc)
	if ok && parserInfo != nil {
		// Cache the successful parser
		tab.TimestampParserMu.Lock()
		tab.TimestampParser = parserInfo
		tab.TimestampParserTZ = currentTZ
		tab.TimestampParserMu.Unlock()

		a.Log("debug", fmt.Sprintf("Cached timestamp parser '%s' for file %s", parserInfo.FormatName, tab.FileName))
		return ms, true
	}

	// No parser found
	if s != "" {
		a.Log("error", fmt.Sprintf("Failed to parse timestamp '%s' in file %s", s, tab.FileName))
	}
	return 0, false
}

// ClearTimestampParserCaches clears all cached timestamp parsers and query caches when timezone settings change
func (a *App) ClearTimestampParserCaches() {
	a.tabsMu.RLock()
	defer a.tabsMu.RUnlock()

	for _, tab := range a.tabs {
		if tab != nil {
			tab.TimestampParserMu.Lock()
			tab.TimestampParser = nil
			tab.TimestampParserTZ = ""
			tab.TimestampParserMu.Unlock()
		}
	}

	// Also clear the query cache since cached Row.Timestamp values depend on ingest timezone
	// and cache keys now include the timezone, so old entries would be orphaned anyway
	if a.queryCache != nil {
		a.queryCache.Clear()
		a.Log("debug", "Cleared query cache due to timezone setting change")
	}

	a.Log("debug", "Cleared all timestamp parser caches due to timezone setting change")
}

// SaveSettings saves application settings and clears parser caches if timezone changed
func (a *App) SaveSettings(newSettings *interfaces.Settings) error {
	// Get current settings to detect timezone changes
	oldSettings := a.GetEffectiveSettings()

	// Convert from interface Settings to settings.Settings
	settingsToSave := settings.Settings{
		SortByTime:             newSettings.SortByTime,
		SortDescending:         newSettings.SortDescending,
		EnableQueryCache:       newSettings.EnableQueryCache,
		DisplayTimezone:        newSettings.DisplayTimezone,
		DefaultIngestTimezone:  newSettings.DefaultIngestTimezone,
		TimestampDisplayFormat: newSettings.TimestampDisplayFormat,
		License:                newSettings.License,
	}

	// Check if timezone settings changed
	timezoneChanged := oldSettings.DefaultIngestTimezone != newSettings.DefaultIngestTimezone ||
		oldSettings.DisplayTimezone != newSettings.DisplayTimezone

	// Save settings using the settings service
	settingsService := settings.NewSettingsService()
	err := settingsService.SaveSettings(settingsToSave)

	// Clear parser caches if timezone changed
	if err == nil && timezoneChanged {
		a.ClearTimestampParserCaches()
	}

	return err
}

// GetPlugins returns all configured plugins
func (a *App) GetPlugins() ([]settings.PluginConfig, error) {
	return a.pluginService.GetPlugins()
}

// AddPlugin validates and adds a new plugin to the settings
func (a *App) AddPlugin(path string) (*settings.PluginConfig, error) {
	return a.pluginService.AddPlugin(path)
}

// RemovePlugin removes a plugin from the settings
func (a *App) RemovePlugin(path string) error {
	return a.pluginService.RemovePlugin(path)
}

// TogglePlugin enables or disables a plugin
func (a *App) TogglePlugin(path string, enabled bool) error {
	return a.pluginService.TogglePlugin(path, enabled)
}

// ValidatePluginPath validates a plugin path and returns the manifest info
func (a *App) ValidatePluginPath(path string) (*plugin.PluginManifest, error) {
	return a.pluginService.ValidatePluginPath(path)
}

// OpenPluginDialog opens a file dialog for selecting plugin manifest files
// Returns the selected plugin.yml file path
func (a *App) OpenPluginDialog() (string, error) {
	filePath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Plugin Manifest",
		Filters: []runtime.FileFilter{
			{DisplayName: "Plugin Manifest Files", Pattern: "*.yml;*.yaml"},
			{DisplayName: "All Files", Pattern: "*"},
		},
	})
	if err != nil {
		return "", err
	}
	return filePath, nil
}

// GetPluginsForFile returns all plugins that can handle the given file
// Returns an empty slice if no plugins support the file type or if the file
// is handled by built-in loaders (CSV, XLSX, JSON)
func (a *App) GetPluginsForFile(filePath string) []plugin.PluginOption {
	return a.pluginService.GetPluginsForFile(filePath)
}

// CheckPluginRequirement checks if a file requires a plugin to load and returns
// detailed information about the plugin requirement status.
// The pluginId parameter is optional - if provided (e.g., from workspace file options),
// it checks specifically for that plugin's availability.
// The pluginName parameter is optional - if provided (e.g., from workspace file options),
// it will be used as the required plugin name if the plugin isn't found in settings.
func (a *App) CheckPluginRequirement(filePath string, pluginId string, pluginName string) plugin.PluginRequirementResult {
	return a.pluginService.CheckPluginRequirement(filePath, pluginId, pluginName)
}

// CheckDirectoryPluginRequirements checks if any files in a directory require plugins
// and returns the requirement result for the first matching file that requires a plugin.
// This is used to show warnings before opening a directory.
func (a *App) CheckDirectoryPluginRequirements(dirPath string, filePattern string) (plugin.PluginRequirementResult, error) {
	// If no pattern specified, default to all files
	if filePattern == "" {
		filePattern = "*"
	}

	// Get max files setting to limit scan
	currentSettings := settings.GetEffectiveSettings()
	maxFiles := currentSettings.MaxDirectoryFiles
	if maxFiles <= 0 {
		maxFiles = 500
	}

	// Discover files in the directory
	// We use the same discovery logic as when actually opening the directory
	info, err := fileloader.DiscoverFiles(dirPath, fileloader.DirectoryDiscoveryOptions{
		Pattern:  filePattern,
		MaxFiles: maxFiles,
	}, nil)

	if err != nil {
		return plugin.PluginRequirementResult{}, fmt.Errorf("failed to discover directory files: %w", err)
	}

	a.Log("debug", fmt.Sprintf("[CheckDirectoryPluginRequirements] Discovered %d files in %s with pattern %s", len(info.Files), dirPath, filePattern))

	var firstAvailableResult *plugin.PluginRequirementResult

	// Check each discovered file for plugin requirements
	for _, file := range info.Files {
		// Check if this file requires a plugin
		// We pass empty strings for pluginId and pluginName as we don't know them yet
		result := a.pluginService.CheckPluginRequirement(file, "", "")

		if result.RequiresPlugin {
			a.Log("debug", fmt.Sprintf("[CheckDirectoryPluginRequirements] File %s requires plugin. Available: %t, Enabled: %t", file, result.PluginAvailable, result.PluginsEnabled))

			// If the plugin is NOT available or NOT enabled, this is a critical requirement we must report
			if !result.PluginsEnabled || !result.PluginAvailable {
				return result, nil
			}

			// If we found a file that requires a plugin, and it IS available,
			// keep track of it but continue scanning for other files that might have UNMET requirements.
			if firstAvailableResult == nil {
				firstAvailableResult = &result
			}
		}
	}

	// If we found any files that require plugins (even if available), return the first one.
	// This allows the frontend to show the Plugin Selection or Warning dialog for the directory.
	if firstAvailableResult != nil {
		return *firstAvailableResult, nil
	}

	// If we get here, either no files require plugins, or all files that require plugins
	// have a valid, enabled plugin available.
	// Return a "success" result (RequiresPlugin=false)
	return plugin.PluginRequirementResult{RequiresPlugin: false}, nil
}

// ReadJSONHeader reads JSON header using JSONPath
func (a *App) ReadJSONHeader(filePath, jpath string) ([]string, error) {
	return fileloader.ReadJSONHeader(filePath, jpath)
}

// GetJSONReader gets a JSON reader using JSONPath
func (a *App) GetJSONReader(filePath, jpath string) (interfaces.Reader, interfaces.Closer, error) {
	reader, closer, err := fileloader.GetJSONReader(filePath, jpath)
	return reader, closer, err
}

// ReadHeader reads file header
func (a *App) ReadHeader(filePath string) ([]string, error) {
	return fileloader.ReadHeader(filePath)
}

// GetReader gets a file reader
func (a *App) GetReader(filePath string) (interfaces.Reader, interfaces.Closer, error) {
	reader, closer, err := fileloader.GetReader(filePath, fileloader.DefaultFileOptions())
	return reader, closer, err
}

// IsLicensed checks if the app is licensed
func (a *App) IsLicensed() bool {
	return IsLicensed()
}

// FileHashKey is the hardcoded key used for file hashing
// This ensures consistent file hashes regardless of whether a workspace is open
// Column hashes for annotations still use the workspace-specific hash key
var FileHashKey = []byte("breachline hash key\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00")

// CalculateFileHash calculates a HighwayHash of the file content using the hardcoded FileHashKey
// This ensures files always have the same hash regardless of workspace context
func CalculateFileHash(filePath string) (string, error) {
	return CalculateFileHashWithKey(filePath, FileHashKey)
}

// CalculateFileHashWithKey calculates a HighwayHash of the file content using the provided key
func CalculateFileHashWithKey(filePath string, hashKey []byte) (string, error) {
	if len(hashKey) != 32 {
		return "", fmt.Errorf("hash key must be exactly 32 bytes, got %d", len(hashKey))
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash, err := highwayhash.New(hashKey)
	if err != nil {
		return "", fmt.Errorf("failed to create hash: %w", err)
	}

	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// Workspace methods for frontend compatibility
func (a *App) OpenWorkspace() error {
	if a.workspaceService == nil {
		return fmt.Errorf("workspace service not initialized")
	}
	return a.workspaceService.ChooseAndOpenWorkspace()
}

func (a *App) CloseWorkspace() error {
	if a.workspaceService == nil {
		return fmt.Errorf("workspace service not initialized")
	}
	return a.workspaceService.CloseWorkspace()
}

func (a *App) IsWorkspaceOpen() bool {
	if a.workspaceService == nil {
		return false
	}
	return a.workspaceService.IsWorkspaceOpen()
}

func (a *App) GetWorkspaceFiles() ([]*interfaces.WorkspaceFile, error) {
	if a.workspaceService == nil {
		return nil, fmt.Errorf("workspace service not initialized")
	}
	return a.workspaceService.GetWorkspaceFiles()
}

func (a *App) GetWorkspacePath() string {
	if a.workspaceService == nil {
		return ""
	}
	return a.workspaceService.GetWorkspaceIdentifier()
}

func (a *App) GetWorkspaceName() string {
	if a.workspaceService == nil {
		return ""
	}
	return a.workspaceService.GetWorkspaceName()
}

func (a *App) IsRemoteWorkspace() bool {
	if a.workspaceService == nil {
		return false
	}
	return a.workspaceService.IsRemoteWorkspace()
}

// AddFileToWorkspace adds a file to the workspace with file options
func (a *App) AddFileToWorkspace(filePath string, opts interfaces.FileOptions) error {
	a.Log("info", fmt.Sprintf("[ADD_FILE] AddFileToWorkspace called for: %s directory=%v", filePath, fileloader.IsDirectory(filePath)))
	if a.workspaceService == nil {
		return fmt.Errorf("workspace service not initialized")
	}

	// Check if path is a directory
	if fileloader.IsDirectory(filePath) {
		// Ensure IsDirectory option is set
		opts.IsDirectory = true

		// Get max files setting
		currentSettings := settings.GetEffectiveSettings()
		maxFiles := currentSettings.MaxDirectoryFiles
		if maxFiles <= 0 {
			maxFiles = 500
		}

		// Use wildcard pattern if none specified
		pattern := opts.FilePattern
		if pattern == "" {
			pattern = "*"
			// Update options with pattern so it matches what's used for hash
			opts.FilePattern = pattern
		}

		// Discover files in the directory
		info, err := fileloader.DiscoverFiles(filePath, fileloader.DirectoryDiscoveryOptions{
			Pattern:  pattern,
			MaxFiles: maxFiles,
		}, nil)
		if err != nil {
			return fmt.Errorf("failed to discover directory files: %w", err)
		}

		// FileHash computed from directory content
		// We use nil for progress callback here as this is a synchronous add
		fileHash, err := fileloader.CalculateDirectoryHash(info)
		if err != nil {
			return fmt.Errorf("failed to calculate directory hash: %w", err)
		}

		return a.workspaceService.AddFileToWorkspace(fileHash, opts, filePath, "")
	}

	// Calculate file hash using hardcoded key for consistent hashing
	fileHash, err := CalculateFileHash(filePath)
	if err != nil {
		return fmt.Errorf("failed to calculate file hash: %w", err)
	}

	return a.workspaceService.AddFileToWorkspace(fileHash, opts, filePath, "")
}

func (a *App) UpdateFileDescription(fileHash string, opts interfaces.FileOptions, description string) error {
	if a.workspaceService == nil {
		return fmt.Errorf("workspace service not initialized")
	}
	return a.workspaceService.UpdateFileDescription(fileHash, opts, description)
}

func (a *App) RemoveFileFromWorkspace(filePath string, opts interfaces.FileOptions) error {
	if a.workspaceService == nil {
		return fmt.Errorf("workspace service not initialized")
	}

	// Calculate file hash using hardcoded key for consistent hashing
	fileHash, err := CalculateFileHash(filePath)
	if err != nil {
		return fmt.Errorf("failed to calculate file hash: %w", err)
	}

	return a.workspaceService.RemoveFileFromWorkspace(fileHash, opts)
}

// RemoveFileFromWorkspaceByHash removes a file from workspace using the file hash directly
func (a *App) RemoveFileFromWorkspaceByHash(fileHash string, opts interfaces.FileOptions) error {
	if a.workspaceService == nil {
		return fmt.Errorf("workspace service not initialized")
	}
	return a.workspaceService.RemoveFileFromWorkspace(fileHash, opts)
}

// AddFileToWorkspaceByHash adds a file to workspace using a pre-calculated hash
// This is used for directories where the hash is computed from directory contents
func (a *App) AddFileToWorkspaceByHash(fileHash string, opts interfaces.FileOptions, filePath string) error {
	if a.workspaceService == nil {
		return fmt.Errorf("workspace service not initialized")
	}
	return a.workspaceService.AddFileToWorkspace(fileHash, opts, filePath, "")
}

func (a *App) AddAnnotations(filePath string, opts interfaces.FileOptions, rowIndices []int, timeField string, note string, color string, query string) error {
	if a.workspaceService == nil {
		return fmt.Errorf("workspace service not initialized")
	}

	// Calculate file hash using hardcoded key for consistent hashing
	fileHash, err := CalculateFileHash(filePath)
	if err != nil {
		return fmt.Errorf("failed to calculate file hash: %w", err)
	}

	return a.workspaceService.AddAnnotations(fileHash, opts, rowIndices, timeField, note, color, query)
}

func (a *App) AddAnnotationsByHash(fileHash string, opts interfaces.FileOptions, rowIndices []int, timeField string, note string, color string, query string) error {
	if a.workspaceService == nil {
		return fmt.Errorf("workspace service not initialized")
	}
	return a.workspaceService.AddAnnotations(fileHash, opts, rowIndices, timeField, note, color, query)
}

// AddAnnotationsToSelection adds annotations to a selection that supports virtual select all
func (a *App) AddAnnotationsToSelection(req AnnotationSelectionRequest) (*AnnotationSelectionResult, error) {
	tab := a.GetActiveTab()
	if tab == nil {
		return &AnnotationSelectionResult{RowsAnnotated: 0}, nil
	}
	return a.addAnnotationsToSelectionForTab(tab, req)
}

// DeleteAnnotationsFromSelection deletes annotations from a selection that supports virtual select all
func (a *App) DeleteAnnotationsFromSelection(req AnnotationSelectionRequest) (*AnnotationSelectionResult, error) {
	tab := a.GetActiveTab()
	if tab == nil {
		return &AnnotationSelectionResult{RowsAnnotated: 0}, nil
	}
	return a.deleteAnnotationsFromSelectionForTab(tab, req)
}

// GetRowAnnotations retrieves annotations for the specified display row indices
// It maps display indices to actual file row indices for correct annotation lookup
func (a *App) GetRowAnnotations(fileHash string, opts interfaces.FileOptions, rowIndices []int, query string, timeField string) (map[int]*interfaces.AnnotationResult, error) {
	if a.workspaceService == nil {
		return nil, fmt.Errorf("workspace service not initialized")
	}

	a.Log("debug", fmt.Sprintf("[GetRowAnnotations] Called with %d row indices", len(rowIndices)))

	// Get active tab for query execution
	tab := a.GetActiveTab()
	if tab == nil {
		return nil, fmt.Errorf("no active tab")
	}

	// Execute query ONCE to get raw, unformatted row data
	// This follows the same pattern as AddAnnotationsWithRows - execute query in app layer,
	// pass raw results to workspace service method to avoid redundant query execution
	result, err := a.ExecuteQueryForTabWithMetadata(tab, query, timeField)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	// CRITICAL: Extract actual file row indices from StageResult.Rows
	// rowIndices contains DISPLAY indices (0, 1, 2...) which may differ from
	// actual file row indices when data is sorted/filtered
	actualRowIndices := make([]int, len(rowIndices))
	rows := make([][]string, len(rowIndices))

	for i, displayIndex := range rowIndices {
		if displayIndex >= 0 && displayIndex < len(result.Rows) {
			rows[i] = result.Rows[displayIndex]
			// Get actual file row index from StageResult
			if result.StageResult != nil && displayIndex < len(result.StageResult.Rows) {
				actualRowIndices[i] = result.StageResult.Rows[displayIndex].RowIndex
				a.Log("debug", fmt.Sprintf("[GetRowAnnotations] Display index %d -> actual row index %d", displayIndex, actualRowIndices[i]))
			} else {
				actualRowIndices[i] = displayIndex // Fallback
			}
		} else {
			rows[i] = []string{} // Empty row for invalid indices
			actualRowIndices[i] = displayIndex
		}
	}

	// Call optimized method with actual file row indices
	// GetRowAnnotationsWithRows expects rowIndices to be actual file row indices
	annotationResults, err := a.workspaceService.GetRowAnnotationsWithRows(fileHash, opts, actualRowIndices, rows, result.OriginalHeader)
	if err != nil {
		return nil, err
	}

	// Map results back to display indices for frontend
	displayResults := make(map[int]*interfaces.AnnotationResult)
	for i, displayIndex := range rowIndices {
		if result, ok := annotationResults[actualRowIndices[i]]; ok {
			displayResults[displayIndex] = result
		} else {
			displayResults[displayIndex] = &interfaces.AnnotationResult{Note: "", Color: ""}
		}
	}

	return displayResults, nil
}

func (a *App) DeleteAnnotations(filePath string, opts interfaces.FileOptions, rowIndices []int, timeField string, query string) error {
	if a.workspaceService == nil {
		return fmt.Errorf("workspace service not initialized")
	}

	// Calculate file hash using hardcoded key for consistent hashing
	fileHash, err := CalculateFileHash(filePath)
	if err != nil {
		return fmt.Errorf("failed to calculate file hash: %w", err)
	}

	return a.workspaceService.DeleteRowAnnotations(fileHash, opts, rowIndices, timeField, query)
}

func (a *App) DeleteAnnotationsByHash(fileHash string, opts interfaces.FileOptions, rowIndices []int, timeField string, query string) error {
	a.Log("debug", fmt.Sprintf("DeleteAnnotationsByHash called with fileHash=%s, opts=%+v, rowIndices=%v, timeField=%s, query=%s", fileHash, opts, rowIndices, timeField, query))

	if a.workspaceService == nil {
		a.Log("error", "DeleteAnnotationsByHash: workspace service not initialized")
		return fmt.Errorf("workspace service not initialized")
	}

	a.Log("debug", "DeleteAnnotationsByHash: calling workspaceService.DeleteRowAnnotations")
	return a.workspaceService.DeleteRowAnnotations(fileHash, opts, rowIndices, timeField, query)
}

func (a *App) ExportWorkspaceTimeline() error {
	if a.workspaceService == nil {
		return fmt.Errorf("workspace service not initialized")
	}
	return a.workspaceService.ExportWorkspaceTimeline()
}

// GetFileAnnotations returns all annotations for a file with display index mapping
// Used by the annotation panel to show all annotations in the current file
func (a *App) GetFileAnnotations(fileHash string, opts interfaces.FileOptions) ([]*interfaces.FileAnnotationInfo, error) {
	if a.workspaceService == nil {
		return nil, fmt.Errorf("workspace service not initialized")
	}

	annotations, err := a.workspaceService.GetFileAnnotations(fileHash, opts)
	if err != nil {
		return nil, err
	}

	// Get active tab to map original indices to display indices
	tab := a.GetActiveTab()
	if tab == nil {
		// No active tab, return annotations without display index mapping
		return annotations, nil
	}

	// Try to get the query-specific index mapping from the tab
	// This reflects the current query results (filtered/sorted)
	tab.QueryIndexMapMu.RLock()
	indexMap := tab.QueryIndexMap
	tab.QueryIndexMapMu.RUnlock()

	if indexMap != nil {
		// Map original row indices to display indices
		for _, annot := range annotations {
			if displayIdx, ok := indexMap[annot.OriginalRowIndex]; ok {
				annot.DisplayRowIndex = displayIdx
			}
			// Otherwise DisplayRowIndex stays at -1 (not visible in current view)
		}
	}

	return annotations, nil
}

// OpenMultipleFilesDialog opens a file dialog allowing multiple file selection
func (a *App) OpenMultipleFilesDialog() ([]string, error) {
	filePaths, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Files to Locate",
		Filters: []runtime.FileFilter{
			{DisplayName: "All Files", Pattern: "*.*"},
		},
	})
	if err != nil {
		return nil, err
	}
	return filePaths, nil
}

// OpenDirectoryDialog opens a directory selection dialog
func (a *App) OpenDirectoryDialog() (string, error) {
	dirPath, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Directory to Scan",
	})
	if err != nil {
		return "", err
	}
	return dirPath, nil
}

// GetInstanceID returns a unique identifier for this application instance
func (a *App) GetInstanceID() (string, error) {
	// Use the persistent instance ID from settings
	currentSettings := settings.GetEffectiveSettings()
	if currentSettings.InstanceID == "" {
		return "", fmt.Errorf("instance ID not found in settings")
	}
	return currentSettings.InstanceID, nil
}

// CancelLocateFiles cancels the currently running locate files operation
func (a *App) CancelLocateFiles() {
	a.locateCancelMu.Lock()
	defer a.locateCancelMu.Unlock()
	if a.locateCancelFunc != nil {
		a.Log("info", "[LOCATE_FILES] Cancellation requested by user")
		a.locateCancelFunc()
	}
}

// LocateWorkspaceFilesResult represents the result of locating workspace files
type LocateWorkspaceFilesResult struct {
	MatchedCount int                   `json:"matchedCount"`
	MatchedFiles []MatchedFileLocation `json:"matchedFiles"`
}

// MatchedFileLocation represents a file that was matched and needs location storage
type MatchedFileLocation struct {
	FilePath    string `json:"filePath"`
	FileHash    string `json:"fileHash"`
	WorkspaceID string `json:"workspaceId"`
	InstanceID  string `json:"instanceId"`
}

// FileSelection represents a selected file or directory from the frontend
type FileSelection struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	IsDirectory bool   `json:"isDirectory"`
}

// LocateWorkspaceFiles processes selected files/directories and matches them with workspace files
func (a *App) LocateWorkspaceFiles(selections []FileSelection) (*LocateWorkspaceFilesResult, error) {
	if a.workspaceService == nil {
		return nil, fmt.Errorf("workspace service not initialized")
	}

	// Check if we have a remote workspace open
	isRemote := a.IsRemoteWorkspace()
	if !isRemote {
		return nil, fmt.Errorf("file location is only available for remote workspaces")
	}

	// Get workspace ID and instance ID
	workspaceID := a.GetWorkspacePath()

	instanceID, err := a.GetInstanceID()
	if err != nil {
		return nil, fmt.Errorf("failed to get instance ID: %w", err)
	}

	// Get workspace files to match against
	workspaceFiles, err := a.GetWorkspaceFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace files: %w", err)
	}

	// Separate workspace files into regular files and directory files
	workspaceFileHashes := make(map[string]bool)
	workspaceDirectoryFiles := make([]interfaces.WorkspaceFile, 0)

	for _, wf := range workspaceFiles {
		if wf.FileHash == "" {
			continue
		}

		if wf.Options.IsDirectory {
			// This is a directory file - store it for directory matching
			workspaceDirectoryFiles = append(workspaceDirectoryFiles, *wf)
			a.Log("debug", fmt.Sprintf("[LOCATE_FILES] Workspace directory file: %s (pattern: %s, hash: %s)", wf.FilePath, wf.Options.FilePattern, wf.FileHash))
		} else {
			// Regular file - add to hash map for quick lookup
			workspaceFileHashes[wf.FileHash] = true
		}
	}

	a.Log("debug", fmt.Sprintf("[LOCATE_FILES] Found %d regular files and %d directory files in workspace", len(workspaceFileHashes), len(workspaceDirectoryFiles)))

	// Create cancellable context for this operation
	ctx, cancel := context.WithCancel(context.Background())

	// Store cancel function so it can be called from CancelLocateFiles
	a.locateCancelMu.Lock()
	a.locateCancelFunc = cancel
	a.locateCancelMu.Unlock()

	// Clean up cancel function and emit completion event when done
	defer func() {
		a.locateCancelMu.Lock()
		a.locateCancelFunc = nil
		a.locateCancelMu.Unlock()

		// Check if context was cancelled
		if ctx.Err() == context.Canceled {
			a.Log("info", "[LOCATE_FILES] Operation cancelled by user")
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "locate:cancelled")
			}
		} else {
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "locate:complete")
			}
		}
	}()

	a.Log("debug", fmt.Sprintf("[LOCATE_FILES] Starting locate operation with %d regular file(s) and %d directory file(s) to match against", len(workspaceFileHashes), len(workspaceDirectoryFiles)))

	var matchedFiles []MatchedFileLocation

	// Process each selection
	for _, selection := range selections {
		// Check if cancelled
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if selection.IsDirectory {
			a.Log("debug", fmt.Sprintf("[LOCATE_FILES] Processing directory: %s", selection.Path))
			// Scan directory for files and directories
			dirMatches, err := a.scanDirectoryForMatches(ctx, selection.Path, workspaceFileHashes, workspaceDirectoryFiles, workspaceID, instanceID)
			if err != nil {
				if err == context.Canceled {
					return nil, err
				}
				return nil, fmt.Errorf("failed to scan directory %s: %w", selection.Path, err)
			}
			a.Log("debug", fmt.Sprintf("[LOCATE_FILES] Directory scan completed: %d matches found", len(dirMatches)))
			matchedFiles = append(matchedFiles, dirMatches...)
		} else {
			a.Log("debug", fmt.Sprintf("[LOCATE_FILES] Processing single file: %s", selection.Path))
			// Process single file
			matched, err := a.processFileForMatch(selection.Path, workspaceFileHashes, workspaceID, instanceID)
			if err != nil {
				return nil, fmt.Errorf("failed to process file %s: %w", selection.Path, err)
			}
			if matched != nil {
				a.Log("debug", fmt.Sprintf("[LOCATE_FILES] File matched: %s", selection.Path))
				matchedFiles = append(matchedFiles, *matched)

				// Emit progress for single file match
				if a.ctx != nil {
					runtime.EventsEmit(a.ctx, "locate:progress", map[string]interface{}{
						"filePath":     selection.Path,
						"filesScanned": 1,
						"matchCount":   len(matchedFiles),
					})
				}
			} else {
				a.Log("debug", fmt.Sprintf("[LOCATE_FILES] File did not match workspace: %s", selection.Path))

				// Emit progress for single file no match
				if a.ctx != nil {
					runtime.EventsEmit(a.ctx, "locate:progress", map[string]interface{}{
						"filePath":     selection.Path,
						"filesScanned": 1,
						"matchCount":   len(matchedFiles),
					})
				}
			}
		}
	}

	a.Log("info", fmt.Sprintf("[LOCATE_FILES] Locate operation completed: %d total matches found", len(matchedFiles)))

	return &LocateWorkspaceFilesResult{
		MatchedCount: len(matchedFiles),
		MatchedFiles: matchedFiles,
	}, nil
}

// scanDirectoryForMatches recursively scans a directory for files and directories that match workspace files
func (a *App) scanDirectoryForMatches(ctx context.Context, dirPath string, workspaceFileHashes map[string]bool, workspaceDirectoryFiles []interfaces.WorkspaceFile, workspaceID, instanceID string) ([]MatchedFileLocation, error) {
	var matchedFiles []MatchedFileLocation
	var filesScanned int
	var directoriesScanned int
	var filesSkippedDueToErrors int

	// Track directories we've already checked to avoid duplicate checks
	checkedDirectories := make(map[string]bool)

	a.Log("debug", fmt.Sprintf("[LOCATE_FILES] Starting directory scan: %s", dirPath))

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		// Check if cancelled
		select {
		case <-ctx.Done():
			a.Log("debug", "[LOCATE_FILES] Scan cancelled during directory walk")
			return context.Canceled
		default:
		}

		if err != nil {
			filesSkippedDueToErrors++
			a.Log("debug", fmt.Sprintf("[LOCATE_FILES] Skipping due to access error: %s (error: %v)", path, err))
			return nil // Skip files/dirs with errors
		}

		if info.IsDir() {
			a.Log("debug", fmt.Sprintf("[LOCATE_FILES] Entering directory: %s", path))

			// Check if we've already processed this directory
			if checkedDirectories[path] {
				return nil
			}
			checkedDirectories[path] = true
			directoriesScanned++

			// Try to match this directory against workspace directory files
			for _, wsDirFile := range workspaceDirectoryFiles {
				// Check if cancelled
				select {
				case <-ctx.Done():
					return context.Canceled
				default:
				}

				dirMatched, err := a.processDirectoryForMatch(path, wsDirFile, workspaceID, instanceID)
				if err != nil {
					a.Log("debug", fmt.Sprintf("[LOCATE_FILES] Error checking directory %s against pattern %s: %v", path, wsDirFile.Options.FilePattern, err))
					continue // Try next pattern
				}

				if dirMatched != nil {
					a.Log("info", fmt.Sprintf("[LOCATE_FILES] *** DIRECTORY MATCH FOUND *** %s (pattern: %s, hash: %s)", path, wsDirFile.Options.FilePattern, dirMatched.FileHash))
					matchedFiles = append(matchedFiles, *dirMatched)

					// Emit progress event for directory match
					if a.ctx != nil {
						runtime.EventsEmit(a.ctx, "locate:progress", map[string]interface{}{
							"filePath":     path,
							"filesScanned": filesScanned,
							"matchCount":   len(matchedFiles),
						})
					}
					// Don't break - a directory might match multiple patterns
				}
			}

			return nil // Continue walking
		}

		// This is a regular file
		filesScanned++

		matched, err := a.processFileForMatch(path, workspaceFileHashes, workspaceID, instanceID)
		if err != nil {
			filesSkippedDueToErrors++
			a.Log("debug", fmt.Sprintf("[LOCATE_FILES] Error processing file %s: %v", path, err))

			// Emit progress even for errors
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "locate:progress", map[string]interface{}{
					"filePath":     path,
					"filesScanned": filesScanned,
					"matchCount":   len(matchedFiles),
				})
			}

			return nil // Skip files with errors, don't fail the whole operation
		}

		if matched != nil {
			matchedFiles = append(matchedFiles, *matched)
		}

		// Emit progress event for this file
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "locate:progress", map[string]interface{}{
				"filePath":     path,
				"filesScanned": filesScanned,
				"matchCount":   len(matchedFiles),
			})
		}

		return nil
	})

	a.Log("info", fmt.Sprintf("[LOCATE_FILES] Directory scan summary: scanned %d files and %d directories, found %d matches, skipped %d items due to errors", filesScanned, directoriesScanned, len(matchedFiles), filesSkippedDueToErrors))

	return matchedFiles, err
}

// processDirectoryForMatch checks if a directory matches a workspace directory file
// by discovering files with the pattern and calculating the directory hash
func (a *App) processDirectoryForMatch(dirPath string, wsDirFile interfaces.WorkspaceFile, workspaceID, instanceID string) (*MatchedFileLocation, error) {
	// Discover files in this directory using the workspace's file pattern
	info, err := fileloader.DiscoverFiles(dirPath, fileloader.DirectoryDiscoveryOptions{
		Pattern: wsDirFile.Options.FilePattern,
	}, nil)

	if err != nil {
		// If we can't discover files, it's not a match
		return nil, fmt.Errorf("failed to discover files in directory: %w", err)
	}

	// If no files match the pattern, it's not a match
	if len(info.Files) == 0 {
		return nil, nil
	}

	// Calculate the directory hash using the same algorithm as workspace
	dirHash, err := fileloader.CalculateDirectoryHash(info)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate directory hash: %w", err)
	}

	a.Log("debug", fmt.Sprintf("[LOCATE_FILES] Directory %s hash: %s (pattern: %s, %d files)", dirPath, dirHash, wsDirFile.Options.FilePattern, len(info.Files)))

	// Check if this directory hash matches the workspace directory file
	if dirHash != wsDirFile.FileHash {
		return nil, nil // No match
	}

	// Directory matches! Return the match info
	a.Log("info", fmt.Sprintf("[LOCATE_FILES] *** DIRECTORY MATCH CONFIRMED *** %s matches workspace directory %s (hash: %s)", dirPath, wsDirFile.FilePath, dirHash))
	return &MatchedFileLocation{
		FilePath:    dirPath,
		FileHash:    dirHash,
		WorkspaceID: workspaceID,
		InstanceID:  instanceID,
	}, nil
}

// processFileForMatch checks if a single file matches any workspace file and returns match info if it does
func (a *App) processFileForMatch(filePath string, workspaceFileHashes map[string]bool, workspaceID, instanceID string) (*MatchedFileLocation, error) {
	// Calculate file hash using hardcoded key for consistent matching
	fileHash, err := CalculateFileHash(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate file hash: %w", err)
	}

	// Log the file being processed with its hash
	a.Log("debug", fmt.Sprintf("[LOCATE_FILES] Processing: %s (hash: %s)", filePath, fileHash))

	// Check if this file hash matches any workspace file
	if !workspaceFileHashes[fileHash] {
		return nil, nil // No match
	}

	// File matches! Return the match info
	a.Log("info", fmt.Sprintf("[LOCATE_FILES] *** MATCH FOUND *** %s (hash: %s)", filePath, fileHash))
	return &MatchedFileLocation{
		FilePath:    filePath,
		FileHash:    fileHash,
		WorkspaceID: workspaceID,
		InstanceID:  instanceID,
	}, nil
}

// CreateLocalWorkspace opens a save dialog and creates a new local workspace file
func (a *App) CreateLocalWorkspace() error {
	if a.ctx == nil {
		return fmt.Errorf("app context not initialized")
	}

	// Open save file dialog with .breachline extension
	filePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Create Local Workspace",
		DefaultFilename: "workspace.breachline",
		Filters: []runtime.FileFilter{
			{DisplayName: "BreachLine Workspace", Pattern: "*.breachline"},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to open save dialog: %w", err)
	}
	if filePath == "" {
		return nil // User cancelled
	}

	// Create the workspace using the workspace service
	if a.workspaceService == nil {
		return fmt.Errorf("workspace service not initialized")
	}

	err = a.workspaceService.CreateWorkspace(filePath)
	if err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	// Open the newly created workspace
	err = a.workspaceService.OpenWorkspace(filePath)
	if err != nil {
		return fmt.Errorf("failed to open created workspace: %w", err)
	}

	a.Log("info", fmt.Sprintf("Created and opened local workspace: %s", filePath))
	return nil
}

// CreateRemoteWorkspaceRequest represents the request for creating a remote workspace
type CreateRemoteWorkspaceRequest struct {
	Name string `json:"name"`
}

// CreateRemoteWorkspace creates a new remote workspace via the sync API
func (a *App) CreateRemoteWorkspace(name string) error {
	if name == "" {
		return fmt.Errorf("workspace name is required")
	}

	if a.workspaceService == nil {
		return fmt.Errorf("workspace service not initialized")
	}

	// Create and open the remote workspace using the workspace service
	err := a.workspaceService.CreateAndOpenRemoteWorkspace(name)
	if err != nil {
		return fmt.Errorf("failed to create and open remote workspace: %w", err)
	}

	a.Log("info", fmt.Sprintf("Created and opened remote workspace: %s", name))
	return nil
}

// RefreshFileLocations refreshes file locations from the sync API (remote workspaces only)
func (a *App) RefreshFileLocations() error {
	if a.workspaceService == nil {
		return fmt.Errorf("workspace service not initialized")
	}
	return a.workspaceService.RefreshFileLocations()
}

// UpdateCacheSize updates the query cache size limit based on current settings
func (a *App) UpdateCacheSize() {
	if a.queryCache == nil {
		return
	}

	currentSettings := settings.GetEffectiveSettings()
	newSizeBytes := int64(currentSettings.CacheSizeLimitMB) * 1024 * 1024
	a.queryCache.UpdateMaxSize(newSizeBytes)

	a.Log("debug", fmt.Sprintf("Updated cache size limit to %d MB (%d bytes)", currentSettings.CacheSizeLimitMB, newSizeBytes))
}

// StoreHistogramInQueryCache stores histogram data in the query cache using the proper StoreWithHistogram method
// This method has access to the query package and can handle the type conversion properly
func (a *App) StoreHistogramInQueryCache(cacheKey string, originalHeader []string, header []string, displayColumns []int, rows [][]string, histogramResp *histogram.HistogramResponse, timeField string, bucketSeconds int) {
	if a.queryCache == nil || histogramResp == nil {
		return
	}

	// Convert histogram.HistogramBucket to cache.HistogramBucket
	queryHistogramBuckets := make([]cache.HistogramBucket, len(histogramResp.Buckets))
	for i, bucket := range histogramResp.Buckets {
		queryHistogramBuckets[i] = cache.HistogramBucket{
			Start: bucket.Start,
			Count: bucket.Count,
		}
	}

	// Detect timestamp field and convert rows to Row objects
	// Note: Using nil for ingestTimezone will use default from settings
	// This is acceptable for cache storage since the query already used the correct timezone
	timeFieldIdx := timestamps.DetectTimestampIndex(originalHeader)
	rowObjects := query.StringsToRows(rows, timeFieldIdx, nil)

	// Use the proper StoreWithHistogram method
	a.queryCache.StoreWithHistogram(cacheKey, originalHeader, header, displayColumns, rowObjects,
		queryHistogramBuckets, histogramResp.MinTs, histogramResp.MaxTs, timeField, bucketSeconds)

	a.Log("debug", fmt.Sprintf("[HISTOGRAM_CACHE_STORE] Successfully stored histogram with %d buckets using StoreWithHistogram", len(histogramResp.Buckets)))
}
