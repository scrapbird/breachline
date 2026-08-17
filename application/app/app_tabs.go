package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"breachline/app/cache"
	"breachline/app/fileloader"
	"breachline/app/histogram"
	"breachline/app/interfaces"
	"breachline/app/plugin"
	querypkg "breachline/app/query"
	"breachline/app/settings"
	"breachline/app/timestamps"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Tab management methods for App

// updateIndexMap rebuilds the original-to-display index mapping for a tab
// This is called after query execution to enable fast lookups in FindDisplayIndexForOriginalRow
func (a *App) updateIndexMap(tab *FileTab, stageResult *querypkg.StageResult, timeField string) {
	if stageResult == nil || len(stageResult.Rows) == 0 {
		return
	}

	// Get current sort settings
	currentSettings := settings.GetEffectiveSettings()

	tab.IndexMapMu.Lock()
	defer tab.IndexMapMu.Unlock()

	// Build the map from all rows
	tab.OriginalToDisplayMap = make(map[int]int, len(stageResult.Rows))
	for _, row := range stageResult.Rows {
		tab.OriginalToDisplayMap[row.RowIndex] = row.DisplayIndex
	}

	// Track what sort config this map was built for
	tab.IndexMapSortedByTime = currentSettings.SortByTime
	tab.IndexMapSortedDesc = currentSettings.SortDescending
	tab.IndexMapSortedTimeField = timeField

	a.Log("debug", fmt.Sprintf("[INDEX_MAP] Built index map with %d entries (sortByTime=%t, desc=%t, timeField=%s)",
		len(tab.OriginalToDisplayMap), currentSettings.SortByTime, currentSettings.SortDescending, timeField))
}

// updateQueryIndexMap rebuilds the query-specific index mapping for a tab
// This is called after every query to enable accurate display indices for the annotation panel
func (a *App) updateQueryIndexMap(tab *FileTab, stageResult *querypkg.StageResult) {
	if stageResult == nil {
		return
	}

	tab.QueryIndexMapMu.Lock()
	defer tab.QueryIndexMapMu.Unlock()

	// Build the map from current query results
	tab.QueryIndexMap = make(map[int]int, len(stageResult.Rows))
	for _, row := range stageResult.Rows {
		tab.QueryIndexMap[row.RowIndex] = row.DisplayIndex
	}

	a.Log("debug", fmt.Sprintf("[QUERY_INDEX_MAP] Built query index map with %d entries", len(tab.QueryIndexMap)))
}

// isIndexMapValid checks if the cached index map is still valid for current settings
func (a *App) isIndexMapValid(tab *FileTab, timeField string) bool {
	currentSettings := settings.GetEffectiveSettings()

	tab.IndexMapMu.RLock()
	defer tab.IndexMapMu.RUnlock()

	if tab.OriginalToDisplayMap == nil {
		return false
	}

	return tab.IndexMapSortedByTime == currentSettings.SortByTime &&
		tab.IndexMapSortedDesc == currentSettings.SortDescending &&
		tab.IndexMapSortedTimeField == timeField
}

// OpenFileTab opens a file from a given path and creates a new tab
// Supports CSV, XLSX, and JSON file formats
func (a *App) OpenFileTab(filePath string) (*TabInfo, error) {
	return a.OpenFileTabWithOptions(filePath, interfaces.FileOptions{})
}

// OpenFileTabWithOptions opens a file with parsing options
// Uses interfaces.FileOptions which contains JPath, NoHeaderRow, and IngestTimezoneOverride
func (a *App) OpenFileTabWithOptions(filePath string, opts interfaces.FileOptions) (*TabInfo, error) {
	// Debug: log the received options
	a.Log("info", fmt.Sprintf("[OPEN_TAB] OpenFileTabWithOptions called: filePath=%s, opts=%+v", filePath, opts))

	if filePath == "" {
		return nil, fmt.Errorf("file path is empty")
	}

	// Check if the path is a directory - redirect to OpenDirectoryTabWithOptions
	if opts.IsDirectory || fileloader.IsDirectory(filePath) {
		opts.IsDirectory = true
		return a.OpenDirectoryTabWithOptions(filePath, opts)
	}

	// Note: We intentionally do NOT check if file is already open here.
	// The frontend handles duplicate detection by checking filepath+options combination.
	// This allows the same file to be open in multiple tabs with different options.

	// Create new tab
	tabID := fmt.Sprintf("tab-%d", atomic.AddInt64(&a.nextTabID, 1))

	// File hashing now uses a hardcoded key for consistent hashes regardless of workspace context
	tab := NewFileTab(tabID, filePath)

	// Set all options from the provided FileOptions
	tab.Options = opts

	// Canonicalize the plugin identity. When a file is handled by a plugin (by
	// extension) but the caller did not pin a specific plugin, the file is still
	// loaded through that plugin - yet the tab's options would record no plugin,
	// so its identity (dedup key, workspace entry, annotation lookup) diverges
	// from an otherwise-identical tab opened with the plugin named explicitly.
	// Record the plugin actually used so every open of the same file resolves to
	// the same identity.
	if tab.Options.PluginID == "" && fileloader.DetectFileType(filePath) == fileloader.FileTypePlugin {
		if info, ok := plugin.GetPluginForFileWithOptions(filepath.Ext(filePath), tab.Options); ok {
			tab.Options.PluginID = info.Manifest.ID
			tab.Options.PluginName = info.Manifest.Name
		}
	}

	a.tabsMu.Lock()
	a.tabs[tabID] = tab
	a.activeTabID = tabID
	a.tabsMu.Unlock()

	// Report the phases of this open to the frontend. For JSON and XLSX the header
	// read below parses the entire file, which for a large one is the whole wait,
	// so without this the UI can only spin. The sink is registered per path and torn
	// down here, so member files of a directory load never report through it.
	fileloader.SetFileProgressCallback(filePath, func(progress fileloader.LoadProgress) {
		a.emitLoadProgress(loadKindFile, progress)
	})
	defer func() {
		fileloader.ClearFileProgressCallback(filePath)
		a.emitDirectoryOpenDone()
	}()

	// Read headers for the tab
	// This works for CSV, XLSX, and JSON files (if jpath is provided)
	// The readHeaderForTab function uses tab.NoHeaderRow to determine how to parse headers
	headers, err := a.readHeaderForTab(tab)
	if err != nil {
		// Clean up tab on error
		a.tabsMu.Lock()
		delete(a.tabs, tabID)
		a.tabsMu.Unlock()
		return nil, fmt.Errorf("failed to read headers: %w", err)
	}

	// Capture the header on the tab so the first-load reader can reuse it instead
	// of re-reading it from disk. Same value already placed in TabInfo.Headers.
	tab.Headers = headers

	// Note: we no longer preload the file into the cache here. The grid's first
	// query populates the base-file cache on its own; the previous async preload
	// duplicated that work and could race the first query into reading the whole
	// file twice (there is no in-flight de-duplication on base-file loads).

	// Check for any decompression warning from compressed file loading
	decompressionWarning := fileloader.GetDecompressionWarning(filePath)

	return &TabInfo{
		ID:                     tabID,
		FilePath:               filePath,
		FileName:               tab.FileName,
		FileHash:               tab.FileHash,
		Headers:                headers,
		IngestTimezoneOverride: tab.Options.IngestTimezoneOverride,
		DecompressionWarning:   decompressionWarning,
		PluginID:               tab.Options.PluginID,
		PluginName:             tab.Options.PluginName,
	}, nil
}

// OpenFileDialog opens a file dialog and returns the selected file path
// This allows the frontend to decide how to handle the file (e.g., show ingest dialog for JSON)
// Supports compressed files (.gz, .bz2, .xz) containing CSV, XLSX, or JSON data.
// Also includes extensions from enabled plugins.
func (a *App) OpenFileDialog() (string, error) {
	// Start with base supported extensions
	patterns := []string{
		"*.csv", "*.xlsx", "*.json",
		"*.csv.gz", "*.json.gz", "*.xlsx.gz",
		"*.csv.bz2", "*.json.bz2", "*.xlsx.bz2",
		"*.csv.xz", "*.json.xz", "*.xlsx.xz",
		"*.gz", "*.bz2", "*.xz",
	}

	// Add plugin extensions if plugins are enabled
	currentSettings := settings.GetEffectiveSettings()
	if currentSettings.EnablePlugins {
		registry := plugin.GetPluginRegistry()
		if registry != nil {
			pluginExts := registry.GetSupportedExtensions()
			for _, ext := range pluginExts {
				// Add the base extension pattern (e.g., "*.parquet")
				patterns = append(patterns, "*"+ext)
				// Also add compressed variants
				patterns = append(patterns, "*"+ext+".gz")
				patterns = append(patterns, "*"+ext+".bz2")
				patterns = append(patterns, "*"+ext+".xz")
			}
		}
	}

	// Join all patterns with semicolons
	allPatterns := strings.Join(patterns, ";")

	filePath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Open Data File",
		Filters: []runtime.FileFilter{
			{DisplayName: "All Supported Files", Pattern: allPatterns},
		},
	})
	if err != nil {
		return "", err
	}
	return filePath, nil
}

// IsPathDirectory checks if the given path is a directory
func (a *App) IsPathDirectory(path string) bool {
	return fileloader.IsDirectory(path)
}

// OpenFileWithDialogTab opens a file dialog and creates a new tab for the selected file
// Supports CSV, XLSX, and JSON file formats
func (a *App) OpenFileWithDialogTab() (*TabInfo, error) {
	filePath, err := a.OpenFileDialog()
	if err != nil || filePath == "" {
		return nil, err
	}

	// Use OpenFileTab to handle the actual file opening logic
	return a.OpenFileTab(filePath)
}

// GetTabs returns all open tabs
func (a *App) GetTabs() []TabInfo {
	a.tabsMu.RLock()
	defer a.tabsMu.RUnlock()

	tabs := make([]TabInfo, 0, len(a.tabs))
	for _, tab := range a.tabs {
		tabs = append(tabs, TabInfo{
			ID:                     tab.ID,
			FileName:               tab.FileName,
			FilePath:               tab.FilePath,
			FileHash:               tab.FileHash,
			IngestTimezoneOverride: tab.Options.IngestTimezoneOverride,
			Truncated:              tab.Truncated,
			FilesLoaded:            tab.FilesLoaded,
		})
	}
	return tabs
}

// SetActiveTab sets the active tab by ID
func (a *App) SetActiveTab(tabID string) error {
	a.tabsMu.Lock()
	defer a.tabsMu.Unlock()

	if _, exists := a.tabs[tabID]; !exists {
		return fmt.Errorf("tab not found: %s", tabID)
	}
	a.activeTabID = tabID
	return nil
}

// CloseTab closes a tab by ID
func (a *App) CloseTab(tabID string) error {
	a.tabsMu.Lock()
	defer a.tabsMu.Unlock()

	tab, exists := a.tabs[tabID]
	if !exists {
		return fmt.Errorf("tab not found: %s", tabID)
	}

	// Drop the cached directory snapshot so reopening this directory re-reads disk
	// rather than reusing the file list and schema captured at the previous open.
	if tab.Options.IsDirectory && tab.FilePath != "" {
		stillOpen := false
		for id, other := range a.tabs {
			if id != tabID && other.Options.IsDirectory && other.FilePath == tab.FilePath {
				stillOpen = true
				break
			}
		}
		if !stillOpen {
			fileloader.ClearDirectorySnapshotsFor(tab.FilePath)
		}
	}

	delete(a.tabs, tabID)

	// If closing the active tab, switch to another tab if available
	if a.activeTabID == tabID {
		a.activeTabID = ""
		// Set first available tab as active
		for id := range a.tabs {
			a.activeTabID = id
			break
		}
	}

	return nil
}

// GetActiveTabID returns the currently active tab ID
func (a *App) GetActiveTabID() string {
	a.tabsMu.RLock()
	defer a.tabsMu.RUnlock()
	return a.activeTabID
}

// OpenFileHeadersWithDialog opens a file dialog and returns the CSV headers for a new tab
func (a *App) OpenFileHeadersWithDialog() ([]string, error) {
	_, err := a.OpenFileWithDialogTab()
	if err != nil {
		return nil, err
	}

	tab := a.GetActiveTab()
	if tab == nil {
		return nil, fmt.Errorf("no active tab")
	}

	return a.readHeaderForTab(tab)
}

// ExecuteQueryForTab executes a query for a specific tab and returns all results
// Returns display header but FULL rows (all columns preserved)
// IMPORTANT: Rows contain ALL columns for annotation hash calculation
func (a *App) ExecuteQueryForTab(tab *FileTab, query string, timeField string) ([]string, [][]string, error) {
	result, err := a.executeQueryInternal(tab, query, timeField)
	if err != nil {
		return nil, nil, err
	}
	// Return display header but FULL rows
	// Annotations will use result.OriginalHeader for hash calculation
	return result.Header, result.Rows, nil
}

// ExecuteQueryForTabWithMetadata executes a query and returns complete QueryResult with metadata
// Use this for annotation operations that need original header and display columns
func (a *App) ExecuteQueryForTabWithMetadata(tab *FileTab, query string, timeField string) (*interfaces.QueryExecutionResult, error) {
	result, err := a.executeQueryInternal(tab, query, timeField)
	if err != nil {
		return nil, err
	}
	// Convert to interfaces type and preserve StageResult for histogram optimization
	return &interfaces.QueryExecutionResult{
		OriginalHeader:   result.OriginalHeader,
		Header:           result.Header,
		DisplayColumns:   result.DisplayColumns,
		Rows:             result.Rows,
		Total:            int64(len(result.Rows)),
		Cached:           false,                   // Not tracked in query.QueryExecutionResult
		StageResult:      result.StageResult,      // Preserve for histogram optimization
		PipelineCacheKey: result.PipelineCacheKey, // Preserve for histogram cache lookup
	}, nil
}

// GetDataAndHistogram returns paginated grid data immediately and generates histogram asynchronously
// Query results are returned instantly, histogram is generated in background and emitted via event
func (a *App) GetDataAndHistogram(tabID string, startRow int, endRow int, query string, timeField string, bucketSeconds int) (*DataAndHistogramResponse, error) {
	// Normalize query string by trimming leading/trailing whitespace
	// This ensures consistent cache keys for both pipeline and histogram caches
	query = strings.TrimSpace(query)

	tab := a.GetTab(tabID)
	if tab == nil {
		return nil, fmt.Errorf("tab not found: %s", tabID)
	}

	a.Log("debug", fmt.Sprintf("[ASYNC_QUERY] Starting async query for tab %s with query: %s", tabID, query))

	// Increment histogram version for this query BEFORE executing query
	// This ensures the loading spinner shows immediately
	tab.HistogramMu.Lock()
	tab.HistogramVersion++
	currentVersion := tab.HistogramVersion
	histogramVersion := fmt.Sprintf("%s:%d", tabID, currentVersion)
	tab.HistogramMu.Unlock()

	// Execute query to get data (this is fast - no histogram generation)
	result, err := a.ExecuteQueryForTabWithMetadata(tab, query, timeField)
	if err != nil {
		return nil, err
	}

	header := result.Header
	allRows := result.Rows
	originalHeader := result.OriginalHeader
	displayColumns := result.DisplayColumns

	// Update index map for empty queries (unfiltered view) to enable fast lookups
	// This is used by FindDisplayIndexForOriginalRow to avoid re-running queries
	if query == "" && result.StageResult != nil {
		a.updateIndexMap(tab, result.StageResult, timeField)
	}

	// Always update the query index map for annotation panel
	// This tracks display indices for the current query results
	if result.StageResult != nil {
		a.updateQueryIndexMap(tab, result.StageResult)
	}

	a.Log("debug", fmt.Sprintf("[ASYNC_QUERY] Generated histogram version: %s", histogramVersion))

	// Check histogram cache BEFORE spawning async generation
	var cachedHistogram *histogram.HistogramResponse
	var histogramCached bool

	// Use the pipeline cache key from query execution result
	// This ensures consistent cache keys regardless of query whitespace
	pipelineCacheKey := result.PipelineCacheKey

	if a.queryCache != nil && pipelineCacheKey != "" {
		// Check pipeline cache for existing histogram using the key from query execution
		if cached, ok := a.queryCache.Get(pipelineCacheKey); ok && cached.HasHistogram {
			if cached.HistogramTimeField == timeField {
				a.Log("debug", fmt.Sprintf("[HISTOGRAM_CACHE_HIT] Using cached histogram from pipeline cache: %s", pipelineCacheKey))
				appHistogramBuckets := make([]histogram.HistogramBucket, len(cached.HistogramBuckets))
				for i, bucket := range cached.HistogramBuckets {
					appHistogramBuckets[i] = histogram.HistogramBucket{
						Start: bucket.Start,
						Count: bucket.Count,
					}
				}
				cachedHistogram = &histogram.HistogramResponse{
					Buckets: appHistogramBuckets,
					MinTs:   cached.HistogramMinTs,
					MaxTs:   cached.HistogramMaxTs,
				}
				histogramCached = true
			}
		}
	}

	// If histogram not cached, spawn async generation
	if cachedHistogram == nil {
		a.Log("debug", fmt.Sprintf("[ASYNC_HISTOGRAM] Spawning async histogram generation for version %s", histogramVersion))
		// Get display timezone for consistent filter boundary parsing
		// This must match the timezone used during query execution
		currentSettings := settings.GetEffectiveSettings()
		displayTimezone := timestamps.GetLocationForTZ(currentSettings.DisplayTimezone)
		// Pass StageResult, pipeline cache key, and display timezone for optimized histogram generation
		go a.generateHistogramAsync(tabID, histogramVersion, query, timeField, result.StageResult, bucketSeconds, pipelineCacheKey, displayTimezone)
		// Set empty histogram for immediate return
		cachedHistogram = &histogram.HistogramResponse{
			Buckets: []histogram.HistogramBucket{},
			MinTs:   0,
			MaxTs:   0,
		}
		histogramCached = false
	}

	// Apply pagination for grid data
	total := len(allRows)
	if startRow >= total {
		return &DataAndHistogramResponse{
			OriginalHeader:   originalHeader,
			Header:           header,
			DisplayColumns:   displayColumns,
			Rows:             [][]string{},
			OriginalIndices:  []int{},
			DisplayIndices:   []int{},
			ReachedEnd:       true,
			Total:            total,
			Annotations:      []bool{},
			AnnotationColors: []string{},
			HistogramBuckets: cachedHistogram.Buckets,
			MinTs:            cachedHistogram.MinTs,
			MaxTs:            cachedHistogram.MaxTs,
			HistogramVersion: histogramVersion,
			HistogramCached:  histogramCached,
		}, nil
	}

	endIdx := endRow
	if endIdx > total {
		endIdx = total
	}

	// Get full rows for this page
	fullRows := allRows[startRow:endIdx]
	reachedEnd := endIdx >= total

	// Apply display column filtering to show only selected columns
	var displayRows [][]string
	if len(displayColumns) == 0 {
		// No filtering - show all columns
		displayRows = fullRows
	} else {
		// Filter to show only display columns
		displayRows = make([][]string, len(fullRows))
		for i, fullRow := range fullRows {
			displayRow := make([]string, len(displayColumns))
			for j, colIdx := range displayColumns {
				if colIdx >= 0 && colIdx < len(fullRow) {
					displayRow[j] = fullRow[colIdx]
				}
			}
			displayRows[i] = displayRow
		}
	}

	// Check annotations for paginated rows if workspace is open
	// IMPORTANT: Use Row objects from StageResult which have RowIndex populated
	annotations := make([]bool, len(fullRows))
	annotationColors := make([]string, len(fullRows))

	if a.workspaceService != nil && a.workspaceService.IsWorkspaceOpen() && tab.FileHash != "" {
		a.Log("debug", fmt.Sprintf("[UNIFIED_QUERY] Checking annotations for tab %s: fileHash=%s, opts=%+v",
			tab.ID, tab.FileHash, tab.Options))

		// Early exit if no annotations exist for this file
		if !a.workspaceService.HasAnnotationsForFile(tab.FileHash, tab.Options) {
			a.Log("debug", fmt.Sprintf("[UNIFIED_QUERY] No annotations for file with opts=%+v, skipping annotation check", tab.Options))
		} else {
			// Get workspace hash key (not used for row-index lookups, but kept for interface compatibility)
			hashKey := a.workspaceService.GetHashKey()
			if hashKey == nil {
				a.Log("warn", "[UNIFIED_QUERY] No workspace hash key available for annotation matching")
			} else {
				// Use StageResult.Rows which have RowIndex properly populated
				// This is critical for row-index-based annotation matching
				if result.StageResult != nil && len(result.StageResult.Rows) > 0 {
					// Apply the same pagination to StageResult.Rows
					stageRows := result.StageResult.Rows[startRow:endIdx]
					annotations, annotationColors = a.workspaceService.IsRowAnnotatedBatchWithColors(tab.FileHash, tab.Options, stageRows, hashKey)
				} else {
					a.Log("warn", "[UNIFIED_QUERY] StageResult.Rows not available for annotation matching, falling back to creating Row objects without RowIndex")
					// Fallback: create Row objects without RowIndex (won't work well with row-index-based annotations)
					interfaceRows := make([]*interfaces.Row, len(fullRows))
					for i, row := range fullRows {
						interfaceRows[i] = &interfaces.Row{DisplayIndex: -1, Data: row}
					}
					annotations, annotationColors = a.workspaceService.IsRowAnnotatedBatchWithColors(tab.FileHash, tab.Options, interfaceRows, hashKey)
				}
			}
		}
	}

	// Apply timestamp formatting to display rows
	if a.shouldFormatTimestamps(header, timeField) {
		a.Log("debug", fmt.Sprintf("[TIMESTAMP_FORMAT_UNIFIED] Applying timestamp formatting to %d rows", len(displayRows)))
		displayRows = a.formatTimestampsInRows(displayRows, header, timeField, tab.Options.IngestTimezoneOverride)
	}

	// Extract row indices from StageResult.Rows (paginated slice)
	// StageResult must be available - if not, this is a bug that needs to be fixed
	if result.StageResult == nil || len(result.StageResult.Rows) == 0 {
		return nil, fmt.Errorf("internal error: StageResult not populated by query execution (query: %q, rows returned: %d)", query, len(allRows))
	}

	originalIndices := make([]int, len(fullRows))
	displayIndices := make([]int, len(fullRows))
	stageRows := result.StageResult.Rows[startRow:endIdx]
	for i, row := range stageRows {
		originalIndices[i] = row.RowIndex
		displayIndices[i] = row.DisplayIndex
	}

	a.Log("debug", fmt.Sprintf("[ASYNC_QUERY] Completed async query - returning %d rows and %d histogram buckets (cached: %v)", len(displayRows), len(cachedHistogram.Buckets), histogramCached))

	return &DataAndHistogramResponse{
		OriginalHeader:   originalHeader,
		Header:           header,
		DisplayColumns:   displayColumns,
		Rows:             displayRows, // Return display-filtered and timestamp-formatted rows
		OriginalIndices:  originalIndices,
		DisplayIndices:   displayIndices,
		ReachedEnd:       reachedEnd,
		Total:            total,
		Annotations:      annotations,
		AnnotationColors: annotationColors,
		HistogramBuckets: cachedHistogram.Buckets,
		MinTs:            cachedHistogram.MinTs,
		MaxTs:            cachedHistogram.MaxTs,
		HistogramVersion: histogramVersion,
		HistogramCached:  histogramCached,
	}, nil
}

// FindDisplayIndexForOriginalRow finds the display position of a row given its original file position
// Uses cached index map when available for O(1) lookup, falls back to query execution if cache is invalid
func (a *App) FindDisplayIndexForOriginalRow(tabID string, originalFileIndex int, timeField string) (int, error) {
	tab := a.GetTab(tabID)
	if tab == nil {
		return -1, fmt.Errorf("tab not found: %s", tabID)
	}

	a.Log("debug", fmt.Sprintf("[FIND_DISPLAY_INDEX] Looking for original index %d", originalFileIndex))

	// Try to use cached index map first (O(1) lookup)
	if a.isIndexMapValid(tab, timeField) {
		tab.IndexMapMu.RLock()
		if displayIndex, ok := tab.OriginalToDisplayMap[originalFileIndex]; ok {
			tab.IndexMapMu.RUnlock()
			a.Log("debug", fmt.Sprintf("[FIND_DISPLAY_INDEX] Cache hit: original index %d -> display index %d", originalFileIndex, displayIndex))
			return displayIndex, nil
		}
		tab.IndexMapMu.RUnlock()
		a.Log("debug", fmt.Sprintf("[FIND_DISPLAY_INDEX] Original index %d not in cached map", originalFileIndex))
		return -1, fmt.Errorf("original index %d not found in cached map", originalFileIndex)
	}

	a.Log("debug", "[FIND_DISPLAY_INDEX] Cache miss or invalid, executing query")

	// Cache invalid or not available - execute query to rebuild map
	result, err := a.ExecuteQueryForTabWithMetadata(tab, "", timeField)
	if err != nil {
		return -1, fmt.Errorf("failed to execute query: %w", err)
	}

	// StageResult should have all rows with their RowIndex and DisplayIndex populated
	if result.StageResult == nil || len(result.StageResult.Rows) == 0 {
		return -1, fmt.Errorf("no rows in result")
	}

	// Update the index map for future lookups
	a.updateIndexMap(tab, result.StageResult, timeField)

	// Now look up in the freshly populated map
	tab.IndexMapMu.RLock()
	if displayIndex, ok := tab.OriginalToDisplayMap[originalFileIndex]; ok {
		tab.IndexMapMu.RUnlock()
		a.Log("debug", fmt.Sprintf("[FIND_DISPLAY_INDEX] Found original index %d at display position %d (after rebuild)", originalFileIndex, displayIndex))
		return displayIndex, nil
	}
	tab.IndexMapMu.RUnlock()

	return -1, fmt.Errorf("original index %d not found in %d rows", originalFileIndex, len(result.StageResult.Rows))
}

// generateHistogramAsync generates a histogram asynchronously and emits an event when complete
// This runs in a goroutine and does not block the query response
// OPTIMIZED: Now accepts StageResult with pre-parsed timestamps instead of raw rows
// displayTimezone is used for parsing time filter boundaries (after/before) consistently with query execution
func (a *App) generateHistogramAsync(tabID string, version string, query string, timeField string, stageResultInterface interface{}, bucketSeconds int, pipelineCacheKey string, displayTimezone *time.Location) {
	// Get tab
	tab := a.GetTab(tabID)
	if tab == nil {
		a.Log("warn", fmt.Sprintf("[ASYNC_HISTOGRAM] Tab not found: %s", tabID))
		return
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Store cancel function
	tab.HistogramMu.Lock()
	if tab.HistogramCancel != nil {
		tab.HistogramCancel() // Cancel previous histogram
	}
	tab.HistogramCancel = cancel
	tab.HistogramMu.Unlock()

	// Clean up on exit
	defer func() {
		tab.HistogramMu.Lock()
		tab.HistogramCancel = nil
		tab.HistogramMu.Unlock()
	}()

	a.Log("debug", fmt.Sprintf("[ASYNC_HISTOGRAM] Starting OPTIMIZED histogram generation for version %s", version))

	// Convert interface{} to *query.StageResult
	var stageResult *querypkg.StageResult
	if stageResultInterface != nil {
		var ok bool
		stageResult, ok = stageResultInterface.(*querypkg.StageResult)
		if !ok {
			a.Log("error", fmt.Sprintf("[ASYNC_HISTOGRAM] Invalid StageResult type for version %s", version))
			a.emitHistogramError(tabID, version, "invalid StageResult type")
			return
		}
	}

	if stageResult == nil {
		a.Log("error", fmt.Sprintf("[ASYNC_HISTOGRAM] Nil StageResult for version %s", version))
		a.emitHistogramError(tabID, version, "nil StageResult")
		return
	}

	// Generate histogram using OPTIMIZED BuildFromStageResult
	// This uses pre-parsed timestamps and pre-calculated min/max stats
	// IMPORTANT: Use the same displayTimezone as query execution for consistent filter boundary parsing
	qe := querypkg.NewQueryExecutorWithTimezone(nil, nil, querypkg.DefaultCacheConfig(), displayTimezone)

	// Convert query.StageResult to histogram.StageResult
	// Map Row and TimestampStats to histogram package types
	histogramRows := make([]*histogram.Row, len(stageResult.Rows))
	for i, qRow := range stageResult.Rows {
		histogramRows[i] = &histogram.Row{
			Data:      qRow.Data,
			Timestamp: qRow.Timestamp,
			HasTime:   qRow.HasTime,
		}
	}

	var histogramStats *histogram.TimestampStats
	if stageResult.TimestampStats != nil {
		histogramStats = &histogram.TimestampStats{
			TimeFieldIdx: stageResult.TimestampStats.TimeFieldIdx,
			MinTimestamp: stageResult.TimestampStats.MinTimestamp,
			MaxTimestamp: stageResult.TimestampStats.MaxTimestamp,
			ValidCount:   stageResult.TimestampStats.ValidCount,
		}
	}

	histogramStageResult := &histogram.StageResult{
		OriginalHeader: stageResult.OriginalHeader,
		Header:         stageResult.Header,
		DisplayColumns: stageResult.DisplayColumns,
		Rows:           histogramRows,
		TimestampStats: histogramStats,
	}

	histogramResult, err := histogram.BuildFromStageResult(
		ctx,
		histogramStageResult,
		query,
		0,  // Let BuildFromStageResult calculate optimal bucket size (same as BuildFromRows)
		qe, // Implements TimeFilterExtractor
	)

	// Handle cancellation
	if err == context.Canceled {
		a.Log("debug", fmt.Sprintf("[ASYNC_HISTOGRAM] Cancelled for version %s", version))
		return
	}

	// Handle errors
	if err != nil {
		a.Log("error", fmt.Sprintf("[ASYNC_HISTOGRAM] Generation failed for version %s: %v", version, err))
		a.emitHistogramError(tabID, version, err.Error())
		return
	}

	// Cache the histogram using the pipeline cache key
	if a.queryCache != nil && histogramResult != nil && len(histogramResult.Buckets) > 0 && pipelineCacheKey != "" {
		// Calculate bucket size from histogram result
		var calculatedBucketSeconds int
		if len(histogramResult.Buckets) > 1 {
			calculatedBucketSeconds = int((histogramResult.Buckets[1].Start - histogramResult.Buckets[0].Start) / 1000)
		} else {
			calculatedBucketSeconds = 300 // Default 5 minutes
		}

		// Convert histogram buckets to cache format
		queryHistogramBuckets := make([]cache.HistogramBucket, len(histogramResult.Buckets))
		for i, bucket := range histogramResult.Buckets {
			queryHistogramBuckets[i] = cache.HistogramBucket{
				Start: bucket.Start,
				Count: bucket.Count,
			}
		}

		// Add histogram to existing pipeline cache entry using the consistent cache key
		if a.queryCache.AddHistogramToEntry(pipelineCacheKey, queryHistogramBuckets, histogramResult.MinTs, histogramResult.MaxTs, timeField, calculatedBucketSeconds) {
			a.Log("debug", fmt.Sprintf("[ASYNC_HISTOGRAM] Cached histogram in pipeline cache for version %s, key: %s", version, pipelineCacheKey))
		} else {
			a.Log("debug", fmt.Sprintf("[ASYNC_HISTOGRAM] Could not add histogram to pipeline cache entry: %s", pipelineCacheKey))
		}
	}

	// Emit success event
	event := &histogram.HistogramReadyEvent{
		TabID:   tabID,
		Version: version,
		Buckets: histogramResult.Buckets,
		MinTs:   histogramResult.MinTs,
		MaxTs:   histogramResult.MaxTs,
	}
	a.emitHistogramReady(event)
	a.Log("debug", fmt.Sprintf("[ASYNC_HISTOGRAM] Completed and emitted event for version %s with %d buckets", version, len(histogramResult.Buckets)))
}

// createTimestampFormatter creates a timestamp formatting function using user settings
// ingestTimezone is the timezone used for parsing timestamps without timezone info
func (a *App) createTimestampFormatter(ingestTimezone *time.Location) func(string) string {
	// Get effective settings for timestamp formatting
	effective := a.GetEffectiveSettings()
	tzName := strings.TrimSpace(effective.DisplayTimezone)

	var displayLoc *time.Location
	switch strings.ToUpper(tzName) {
	case "", "LOCAL":
		displayLoc = time.Local
	case "UTC":
		displayLoc = time.UTC
	default:
		if l, err := time.LoadLocation(tzName); err == nil {
			displayLoc = l
		} else {
			displayLoc = time.Local
		}
	}

	return func(s string) string {
		if ms, ok := timestamps.ParseTimestampMillis(s, ingestTimezone); ok {
			t := time.UnixMilli(ms).In(displayLoc)

			// Convert pattern to Go layout
			toGoLayout := func(p string) string {
				p = strings.TrimSpace(p)
				if p == "" {
					return "2006-01-02 15:04:05"
				}
				r := strings.NewReplacer(
					"yyyy", "2006",
					"yy", "06",
					"MM", "01",
					"dd", "02",
					"HH", "15",
					"mm", "04",
					"ss", "05",
					"SSS", "000",
					"zzz", "MST",
				)
				return r.Replace(p)
			}
			pattern := strings.TrimSpace(effective.TimestampDisplayFormat)
			if pattern == "" {
				pattern = "yyyy-MM-dd HH:mm:ss"
			}
			layout := toGoLayout(pattern)
			return t.Format(layout)
		} else {
			return s // Return original if we can't parse
		}
	}
}

// formatTimestampsInRows applies timestamp formatting to the specified timestamp column
// timeField is the user-selected timestamp column name; if empty, falls back to auto-detection
// ingestTzOverride is the per-file timezone override (empty string for default)
func (a *App) formatTimestampsInRows(rows [][]string, header []string, timeField string, ingestTzOverride string) [][]string {
	if len(rows) == 0 || len(header) == 0 {
		return rows
	}

	// Find the timestamp column index
	// If timeField is specified, use it; otherwise fall back to auto-detection
	timestampIdx := -1
	if timeField != "" {
		// Find the column by name (case-insensitive)
		lowerTimeField := strings.ToLower(strings.TrimSpace(timeField))
		for i, h := range header {
			if strings.ToLower(strings.TrimSpace(h)) == lowerTimeField {
				timestampIdx = i
				break
			}
		}
	}
	// Fall back to auto-detection if timeField not found or not specified
	if timestampIdx < 0 {
		timestampIdx = timestamps.DetectTimestampIndex(header)
	}
	if timestampIdx < 0 || timestampIdx >= len(header) {
		return rows // No valid timestamp column found
	}

	// Create formatter with the correct ingest timezone
	ingestTimezone := timestamps.GetIngestTimezoneWithOverride(ingestTzOverride)
	formatTimestamp := a.createTimestampFormatter(ingestTimezone)

	// Format timestamps in each row (only the detected timestamp column)
	formattedRows := make([][]string, len(rows))
	for i, row := range rows {
		formattedRow := make([]string, len(row))
		copy(formattedRow, row)

		// Format only the detected timestamp column
		if timestampIdx < len(formattedRow) {
			formattedRow[timestampIdx] = formatTimestamp(formattedRow[timestampIdx])
		}

		formattedRows[i] = formattedRow
	}

	return formattedRows
}

// shouldFormatTimestamps determines if timestamp formatting should be applied
// timeField is the user-selected timestamp column name; if empty, falls back to auto-detection
func (a *App) shouldFormatTimestamps(header []string, timeField string) bool {
	// Find the timestamp column index
	timestampIdx := -1
	if timeField != "" {
		// Find the column by name (case-insensitive)
		lowerTimeField := strings.ToLower(strings.TrimSpace(timeField))
		for i, h := range header {
			if strings.ToLower(strings.TrimSpace(h)) == lowerTimeField {
				timestampIdx = i
				break
			}
		}
	}
	// Fall back to auto-detection if timeField not found or not specified
	if timestampIdx < 0 {
		timestampIdx = timestamps.DetectTimestampIndex(header)
	}
	return timestampIdx >= 0 && timestampIdx < len(header)
}

// DirectoryHashCheckResult contains the result of comparing directory hashes
type DirectoryHashCheckResult struct {
	HasMismatch bool   `json:"hasMismatch"`
	CurrentHash string `json:"currentHash"`
	StoredHash  string `json:"storedHash"`
}

// CheckDirectoryHashMismatch checks if a directory's current hash differs from the stored hash
// This is used to warn users when opening a directory from workspace if files have changed
func (a *App) CheckDirectoryHashMismatch(dirPath string, filePattern string, storedHash string) (*DirectoryHashCheckResult, error) {
	a.Log("info", fmt.Sprintf("[HASH_CHECK] CheckDirectoryHashMismatch called: dir=%s, pattern=%s, stored=%s", dirPath, filePattern, storedHash))

	if dirPath == "" {
		return nil, fmt.Errorf("directory path is empty")
	}

	if storedHash == "" {
		// No stored hash to compare against - no mismatch
		return &DirectoryHashCheckResult{
			HasMismatch: false,
			CurrentHash: "",
			StoredHash:  "",
		}, nil
	}

	// Get max files setting
	currentSettings := settings.GetEffectiveSettings()
	maxFiles := currentSettings.DirectoryFileLimit()

	// Use wildcard pattern if none specified, to avoid "pattern required" error
	pattern := filePattern
	if pattern == "" {
		pattern = "*"
	}

	// Discover files in the directory
	info, err := fileloader.DiscoverFiles(dirPath, fileloader.DirectoryDiscoveryOptions{
		Pattern:  pattern,
		MaxFiles: maxFiles,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	if len(info.Files) == 0 {
		// No files found - this is a mismatch since the stored hash implies there were files
		return &DirectoryHashCheckResult{
			HasMismatch: true,
			CurrentHash: "",
			StoredHash:  storedHash,
		}, nil
	}

	// Calculate current directory hash
	currentHash, err := fileloader.CalculateDirectoryHash(info)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate directory hash: %w", err)
	}

	if currentHash == storedHash {
		return &DirectoryHashCheckResult{
			HasMismatch: false,
			CurrentHash: currentHash,
			StoredHash:  storedHash,
		}, nil
	}

	// The stored hash may predate metadata hashing. Before reporting a mismatch,
	// check it against the legacy content hash: an unchanged directory recorded by
	// an older version must not look modified just because the algorithm changed.
	if legacyHash, legacyErr := fileloader.CalculateDirectoryContentHash(context.Background(), info, nil); legacyErr == nil && legacyHash == storedHash {
		a.Log("info", "[HASH_CHECK] Directory matches its stored content hash (recorded before metadata hashing); not a mismatch")
		return &DirectoryHashCheckResult{
			HasMismatch: false,
			CurrentHash: storedHash,
			StoredHash:  storedHash,
		}, nil
	}

	return &DirectoryHashCheckResult{
		HasMismatch: true,
		CurrentHash: currentHash,
		StoredHash:  storedHash,
	}, nil
}

// PreviewDirectory returns preview information about a directory without fully loading it
func (a *App) PreviewDirectory(dirPath string, pattern string, jpath string) (*fileloader.DirectoryPreviewResult, error) {
	// Get max files setting
	currentSettings := settings.GetEffectiveSettings()
	maxFiles := currentSettings.DirectoryFileLimit()

	// The preview scans and samples the directory, which on a large archive is not
	// instant, so it runs under the same cancellable context and progress reporting
	// as an open and its snapshot is reused when the user goes on to open.
	ctx, finish := a.beginDirectoryOpen()
	defer finish()

	return fileloader.PreviewDirectoryContext(ctx, dirPath, pattern, jpath, maxFiles, a.emitDirectoryOpenProgress)
}

// CancelDirectoryOpen cancels an in-progress directory open. Scanning, hashing and
// schema resolution over a large archive can run for minutes, so the user must be
// able to abandon it without killing the app.
func (a *App) CancelDirectoryOpen() {
	a.dirOpenCancelMu.Lock()
	cancel := a.dirOpenCancelFunc
	a.dirOpenCancelMu.Unlock()

	if cancel != nil {
		a.Log("info", "[OPEN_DIR_TAB] Cancelling directory open")
		cancel()
	}
}

// beginDirectoryOpen installs a fresh cancellable context for a directory open,
// cancelling any open still running, and returns it with its cleanup function.
func (a *App) beginDirectoryOpen() (context.Context, func()) {
	a.dirOpenCancelMu.Lock()
	if a.dirOpenCancelFunc != nil {
		a.dirOpenCancelFunc()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.dirOpenCancelFunc = cancel
	a.dirOpenGeneration++
	generation := a.dirOpenGeneration
	a.dirOpenCancelMu.Unlock()

	return ctx, func() {
		a.dirOpenCancelMu.Lock()
		// Only surrender the slot if a later open has not already claimed it.
		// Clearing unconditionally would drop a newer open's cancel function when
		// this one finished after that one started, leaving it uncancellable.
		if a.dirOpenGeneration == generation {
			a.dirOpenCancelFunc = nil
		}
		a.dirOpenCancelMu.Unlock()
		cancel()

		// Always close out the progress sequence. Emitting this from the deferred
		// cleanup rather than the success path is what guarantees a cancelled or
		// failed open still tells the frontend it is over.
		a.emitDirectoryOpenDone()
	}
}

// Load lifecycle events. Opening a directory and opening a single large file are
// the same thing from the user's side - a wait with phases - so both report on one
// channel and drive one progress dialog, distinguished by the kind field.
const (
	loadProgressEvent = "load:progress"
	loadDoneEvent     = "load:done"

	loadKindDirectory = "directory"
	loadKindFile      = "file"
)

// directoryEventObserver is nil in normal operation. Tests set it to record the
// load lifecycle events sent to the frontend, so the invariant that every progress
// sequence ends in a done event is directly assertable; a stuck progress dialog is
// otherwise only visible by driving the GUI.
var directoryEventObserver func(name string, payload map[string]interface{})

// emitDirectoryEvent sends a load lifecycle event to the frontend.
func (a *App) emitDirectoryEvent(name string, payload map[string]interface{}) {
	if observer := directoryEventObserver; observer != nil {
		observer(name, payload)
	}
	// The loader calls this from worker goroutines and can be driven before Startup
	// has supplied a context (tests, headless use), where EventsEmit would panic.
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, name, payload)
}

// emitDirectoryOpenProgress forwards loader phase progress to the frontend so the
// open reports what it is doing instead of stalling silently.
func (a *App) emitDirectoryOpenProgress(progress fileloader.LoadProgress) {
	a.emitLoadProgress(loadKindDirectory, progress)
}

// emitLoadProgress reports one phase of a load in progress.
func (a *App) emitLoadProgress(kind string, progress fileloader.LoadProgress) {
	a.emitDirectoryEvent(loadProgressEvent, map[string]interface{}{
		"kind":    kind,
		"phase":   progress.Phase,
		"current": progress.Current,
		"total":   progress.Total,
		"message": progress.Message,
	})
}

// emitDirectoryOpenDone tells the frontend that a directory progress sequence has
// finished. Every path that can emit progress must end by calling this, including
// the query that loads the rows: the rows are read during the first query after an
// open, so a sequence that only terminated at the end of the open itself left the
// progress dialog on screen for the whole load and never took it down.
func (a *App) emitDirectoryOpenDone() {
	a.emitDirectoryEvent(loadDoneEvent, map[string]interface{}{})
}

// resolveDirectoryHash returns the hash identifying this directory, preferring one
// already recorded for it in the open workspace.
//
// Directory identity is derived from file metadata rather than file contents. A
// workspace written before that change recorded a content hash, and annotations are
// keyed by hash, so adopting the stored value when it still describes this
// directory is what keeps those annotations attached to their rows.
func (a *App) resolveDirectoryHash(ctx context.Context, info *fileloader.DirectoryInfo, opts interfaces.FileOptions) (string, error) {
	dirHash, err := fileloader.CalculateDirectoryHash(info)
	if err != nil {
		return "", err
	}

	if a.workspaceService == nil || !a.workspaceService.IsWorkspaceOpen() {
		return dirHash, nil
	}
	if file, lookupErr := a.workspaceService.GetWorkspaceFile(dirHash, opts); lookupErr == nil && file != nil {
		return dirHash, nil
	}

	legacyHash, legacyErr := fileloader.CalculateDirectoryContentHash(ctx, info, a.emitDirectoryOpenProgress)
	if legacyErr != nil {
		return dirHash, nil
	}
	if file, lookupErr := a.workspaceService.GetWorkspaceFile(legacyHash, opts); lookupErr == nil && file != nil {
		a.Log("info", "[OPEN_DIR_TAB] Using the content hash this directory was recorded under in the workspace so existing annotations stay attached")
		return legacyHash, nil
	}

	return dirHash, nil
}

// OpenDirectoryTabWithOptions opens a directory as a virtual file tab
func (a *App) OpenDirectoryTabWithOptions(dirPath string, opts interfaces.FileOptions) (*TabInfo, error) {
	a.Log("info", fmt.Sprintf("[OPEN_DIR_TAB] Opening directory: %s, opts=%+v", dirPath, opts))

	if dirPath == "" {
		return nil, fmt.Errorf("directory path is empty")
	}

	// Ensure IsDirectory is set
	opts.IsDirectory = true

	ctx, finish := a.beginDirectoryOpen()
	defer finish()

	// Get max files setting
	currentSettings := settings.GetEffectiveSettings()
	maxFiles := currentSettings.DirectoryFileLimit()

	// A directory reopened after being closed must see what is on disk now, not a
	// snapshot left over from a previous open.
	fileloader.ClearDirectorySnapshotsFor(dirPath)

	// Discover files and resolve the schema once. Every later consumer (the query
	// pipeline's header lookup, timestamp column detection, the row load) reuses
	// this same snapshot rather than rescanning and re-parsing every member file.
	loadOptions := fileloader.FileOptions{
		JPath:                  opts.JPath,
		NoHeaderRow:            opts.NoHeaderRow,
		IncludeSourceColumn:    opts.IncludeSourceColumn,
		IngestTimezoneOverride: opts.IngestTimezoneOverride,
		FilePattern:            opts.FilePattern,
	}

	info, err := fileloader.GetDirectorySnapshot(ctx, dirPath, loadOptions, maxFiles, a.emitDirectoryOpenProgress)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("directory open cancelled")
		}
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	if len(info.Files) == 0 {
		return nil, fmt.Errorf("no compatible files found in directory")
	}

	// Warn if the directory held more matching files than the limit allowed, so
	// the dataset loaded is only a subset. This is surfaced to the user via the
	// Truncated flag on TabInfo below.
	if info.Truncated {
		a.Log("warn", fmt.Sprintf("Directory contains more files than the limit (%d). Loaded the first %d files; the rest were skipped.", maxFiles, len(info.Files)))
	}

	// Calculate directory hash for caching
	dirHash, err := a.resolveDirectoryHash(ctx, info, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate directory hash: %w", err)
	}

	// Read unified headers from the snapshot (no further file reads).
	headers, err := fileloader.GetDirectoryHeader(info, loadOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to read headers: %w", err)
	}

	if ctx.Err() != nil {
		return nil, fmt.Errorf("directory open cancelled")
	}

	// Create tab
	tabID := fmt.Sprintf("tab-%d", atomic.AddInt64(&a.nextTabID, 1))

	tab := &FileTab{
		ID:          tabID,
		FilePath:    dirPath,
		FileName:    fmt.Sprintf("%s/ (%d files)", filepath.Base(dirPath), len(info.Files)),
		FileHash:    dirHash,
		Options:     opts,
		Truncated:   info.Truncated,
		FilesLoaded: len(info.Files),
	}
	tab.SortCond = sync.NewCond(&tab.CacheMu)

	// Capture the union header on the tab. The first-load reader seeds from this,
	// so the query pipeline does not re-resolve the schema of every member file.
	tab.Headers = headers

	a.tabsMu.Lock()
	a.tabs[tabID] = tab
	a.activeTabID = tabID
	a.tabsMu.Unlock()

	// Detect file type from first file in directory. Detection sees through any
	// compression extension, so a directory of .json.gz reports as JSON.
	detectedFileType := ""
	if len(info.Files) > 0 {
		ft := fileloader.DetectFileTypeForPath(info.Files[0])
		switch ft {
		case fileloader.FileTypeJSON:
			detectedFileType = "json"
		case fileloader.FileTypeXLSX:
			detectedFileType = "xlsx"
		case fileloader.FileTypeCSV:
			detectedFileType = "csv"
		}
	}

	// Project what loading this directory will cost in memory. The whole dataset is
	// materialized in RAM, and for a compressed archive the on-disk size understates
	// that by an order of magnitude, so the user is told before committing to a load
	// that cannot finish.
	estimate := fileloader.EstimateDirectoryMemory(info)
	if estimate.Warning != "" {
		a.Log("warn", fmt.Sprintf("[OPEN_DIR_TAB] %s", estimate.Warning))
	}

	// Emit completion event. The matching directory:open:done comes from the
	// deferred cleanup installed by beginDirectoryOpen, so it fires on every exit
	// path rather than only this one.
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "directory:discovery:complete", map[string]interface{}{
			"filesLoaded": len(info.Files),
			"totalSize":   info.TotalSize,
		})
	}

	return &TabInfo{
		ID:                        tabID,
		FilePath:                  dirPath,
		FileName:                  tab.FileName,
		FileHash:                  dirHash,
		Headers:                   headers,
		IngestTimezoneOverride:    tab.Options.IngestTimezoneOverride,
		DetectedFileType:          detectedFileType,
		Truncated:                 info.Truncated,
		FilesLoaded:               len(info.Files),
		SampledSchema:             info.SampledSchema,
		SchemaSampled:             info.SchemaSampled,
		EstimatedUncompressedSize: estimate.EstimatedBytes,
		MemoryWarning:             estimate.Warning,
	}, nil
}
