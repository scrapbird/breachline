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

	a.tabsMu.Lock()
	a.tabs[tabID] = tab
	a.activeTabID = tabID
	a.tabsMu.Unlock()

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

	// Preload file data into cache asynchronously
	// This ensures subsequent queries and annotations can use cached data
	go a.preloadFileToCache(tab)

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
	}, nil
}

// preloadFileToCache loads file data into the main cache asynchronously
// This ensures subsequent queries and annotations use cached data instead of re-reading the file
func (a *App) preloadFileToCache(tab *FileTab) {
	if tab == nil || a.queryCache == nil {
		return
	}

	startTime := time.Now()

	// Read headers to detect the timestamp column
	// This ensures preload uses the SAME cache key as subsequent queries
	headers, err := a.readHeaderForTab(tab)
	if err != nil {
		a.Log("warn", fmt.Sprintf("[CACHE_PRELOAD_ERROR] Failed to read headers for preload: %v", err))
		return
	}

	// Detect timestamp column from headers - this is what queries will use
	timeField := ""
	timeIdx := timestamps.DetectTimestampIndex(headers)
	if timeIdx >= 0 && timeIdx < len(headers) {
		timeField = headers[timeIdx]
	}

	// Build cache key with detected timeField (matching query execution)
	// Note: Cache key includes timeField, NoHeaderRow, and IngestTimezoneOverride so different settings have different cache entries
	// IMPORTANT: Use same timezone resolution logic as query execution to avoid duplicate cache entries
	effectiveIngestTz := timestamps.GetIngestTimezoneWithOverride(tab.Options.IngestTimezoneOverride)
	tzKey := effectiveIngestTz.String()
	baseFileCacheKey := fmt.Sprintf("file:%s:time:%s:noheader:%t:tz:%s", tab.FileHash, timeField, tab.Options.NoHeaderRow, tzKey)
	if entry, found := a.queryCache.Get(baseFileCacheKey); found && entry.IsComplete {
		a.Log("debug", fmt.Sprintf("[CACHE_PRELOAD_SKIP] File already cached: %s (timeField: %s)", tab.FilePath, timeField))
		return
	}

	a.Log("debug", fmt.Sprintf("[CACHE_PRELOAD_START] Loading file to cache: %s (timeField: %s, tz: %s)", tab.FilePath, timeField, tzKey))

	// Execute empty query to load entire file into cache
	// Empty query = no filters/transformations, just loads base file data
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Convert FileTab to query package format (matching pattern from executeQueryStreamingOptimized)
	queryTab := &querypkg.FileTab{
		ID:       tab.ID,
		FilePath: tab.FilePath,
		FileHash: tab.FileHash,
		Options:  tab.Options,
	}

	// Get cache config and ingest timezone from settings (matching executeQueryStreamingOptimized)
	currentSettings := settings.GetEffectiveSettings()
	cacheConfig := querypkg.CacheConfigFromSettings(currentSettings.EnableQueryCache, currentSettings.CacheSizeLimitMB)
	ingestTimezone := timestamps.GetIngestTimezoneWithOverride(tab.Options.IngestTimezoneOverride)

	// Use wrapper function with proper timezone to ensure consistent cache keys
	// Pass detected timeField so cache key matches subsequent queries
	_, err = querypkg.ExecuteQueryInternalWithSettings(
		ctx,
		queryTab,
		"",        // empty query - loads base file data
		timeField, // use detected timestamp column for consistent cache keys
		nil,       // no workspace service needed
		a.queryCache,
		func(stage string, current, total int64, msg string) {
			// Silent progress callback for background loading
		},
		cacheConfig,
		time.Local,     // display timezone not relevant for preload
		ingestTimezone, // use effective ingest timezone for consistent cache keys
		false,          // sortByTime not needed for preload
		false,          // sortDescending not needed for preload
	)
	if err != nil {
		a.Log("warn", fmt.Sprintf("[CACHE_PRELOAD_ERROR] Failed to preload file to cache: %v", err))
		return
	}

	duration := time.Since(startTime)
	a.Log("info", fmt.Sprintf("[CACHE_PRELOAD_COMPLETE] Cached %s in %v", tab.FilePath, duration.Round(time.Millisecond)))
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

	if _, exists := a.tabs[tabID]; !exists {
		return fmt.Errorf("tab not found: %s", tabID)
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

// GetCSVRowCountForTab returns the total number of data rows for a specific tab
func (a *App) GetCSVRowCountForTab(tabID string) (int, error) {
	tab := a.GetTab(tabID)
	if tab == nil {
		return 0, fmt.Errorf("tab not found: %s", tabID)
	}
	return a.getRowCountForTab(tab)
}

// GetCSVRowCountForActiveTab returns the total number of data rows for the active tab
func (a *App) GetCSVRowCount() (int, error) {
	tab := a.GetActiveTab()
	if tab == nil {
		return 0, nil
	}
	return a.getRowCountForTab(tab)
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
	maxFiles := currentSettings.MaxDirectoryFiles
	if maxFiles <= 0 {
		maxFiles = 500
	}

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

	return &DirectoryHashCheckResult{
		HasMismatch: currentHash != storedHash,
		CurrentHash: currentHash,
		StoredHash:  storedHash,
	}, nil
}

// PreviewDirectory returns preview information about a directory without fully loading it
func (a *App) PreviewDirectory(dirPath string, pattern string, jpath string) (*fileloader.DirectoryPreviewResult, error) {
	// Get max files setting
	currentSettings := settings.GetEffectiveSettings()
	maxFiles := currentSettings.MaxDirectoryFiles
	if maxFiles <= 0 {
		maxFiles = 500
	}

	return fileloader.PreviewDirectory(dirPath, pattern, jpath, maxFiles)
}

// OpenDirectoryTabWithOptions opens a directory as a virtual file tab
func (a *App) OpenDirectoryTabWithOptions(dirPath string, opts interfaces.FileOptions) (*TabInfo, error) {
	a.Log("info", fmt.Sprintf("[OPEN_DIR_TAB] Opening directory: %s, opts=%+v", dirPath, opts))

	if dirPath == "" {
		return nil, fmt.Errorf("directory path is empty")
	}

	// Ensure IsDirectory is set
	opts.IsDirectory = true

	// Get max files setting
	currentSettings := settings.GetEffectiveSettings()
	maxFiles := currentSettings.MaxDirectoryFiles
	if maxFiles <= 0 {
		maxFiles = 500
	}

	// Discover files with progress reporting
	info, err := fileloader.DiscoverFiles(dirPath, fileloader.DirectoryDiscoveryOptions{
		Pattern:  opts.FilePattern,
		MaxFiles: maxFiles,
	}, func(progress fileloader.DiscoveryProgress) {
		// Emit progress event to frontend
		runtime.EventsEmit(a.ctx, "directory:discovery:progress", map[string]interface{}{
			"filesFound":  progress.FilesFound,
			"dirsScanned": progress.DirsScanned,
			"currentPath": progress.CurrentPath,
			"totalSize":   progress.TotalSize,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	if len(info.Files) == 0 {
		return nil, fmt.Errorf("no compatible files found in directory")
	}

	// Warn if max files limit was reached
	if len(info.Files) >= maxFiles {
		a.Log("warn", fmt.Sprintf("Directory contains more files than limit (%d). Loading first %d files.", maxFiles, maxFiles))
	}

	// Calculate directory hash for caching
	dirHash, err := fileloader.CalculateDirectoryHash(info)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate directory hash: %w", err)
	}

	// Create tab
	tabID := fmt.Sprintf("tab-%d", atomic.AddInt64(&a.nextTabID, 1))

	tab := &FileTab{
		ID:       tabID,
		FilePath: dirPath,
		FileName: fmt.Sprintf("%s/ (%d files)", filepath.Base(dirPath), len(info.Files)),
		FileHash: dirHash,
		Options:  opts,
	}
	tab.SortCond = sync.NewCond(&tab.CacheMu)

	a.tabsMu.Lock()
	a.tabs[tabID] = tab
	a.activeTabID = tabID
	a.tabsMu.Unlock()

	// Read unified headers
	headers, err := fileloader.GetDirectoryHeader(info, fileloader.FileOptions{
		JPath:               opts.JPath,
		NoHeaderRow:         opts.NoHeaderRow,
		IncludeSourceColumn: opts.IncludeSourceColumn,
	})
	if err != nil {
		a.tabsMu.Lock()
		delete(a.tabs, tabID)
		a.tabsMu.Unlock()
		return nil, fmt.Errorf("failed to read headers: %w", err)
	}

	// Detect file type from first file in directory
	detectedFileType := ""
	if len(info.Files) > 0 {
		ft := fileloader.DetectFileType(info.Files[0])
		switch ft {
		case fileloader.FileTypeJSON:
			detectedFileType = "json"
		case fileloader.FileTypeXLSX:
			detectedFileType = "xlsx"
		case fileloader.FileTypeCSV:
			detectedFileType = "csv"
		}
	}

	// Emit completion event
	runtime.EventsEmit(a.ctx, "directory:discovery:complete", map[string]interface{}{
		"filesLoaded": len(info.Files),
		"totalSize":   info.TotalSize,
	})

	return &TabInfo{
		ID:                     tabID,
		FilePath:               dirPath,
		FileName:               tab.FileName,
		FileHash:               dirHash,
		Headers:                headers,
		IngestTimezoneOverride: tab.Options.IngestTimezoneOverride,
		DetectedFileType:       detectedFileType,
	}, nil
}
