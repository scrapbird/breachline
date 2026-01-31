package query

import (
	"breachline/app/cache"
	"breachline/app/fileloader"
	"breachline/app/timestamps"
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/ohler55/ojg/jp"
	"github.com/ohler55/ojg/oj"
)

// Debug logging for query troubleshooting (always enabled for now)

// QueryExecutor provides the new query execution interface
type QueryExecutor struct {
	cache          *cache.Cache
	progress       ProgressCallback
	cacheConfig    CacheConfig
	timezone       *time.Location // User's display timezone for time filters
	ingestTimezone *time.Location // Timezone for parsing timestamps without timezone info
	sortByTime     bool           // Auto-sort by timestamp when enabled
	sortDescending bool           // Sort order for auto-sort
}

// normalizeHeaderName converts empty headers to "unnamed_a", "unnamed_b", etc.
// and returns the normalized name in lowercase for case-insensitive matching
func normalizeHeaderName(header []string, index int) string {
	h := strings.ToLower(strings.TrimSpace(header[index]))
	if h == "" {
		// Count how many empty headers come before this one
		emptyCount := 0
		for i := 0; i <= index; i++ {
			if strings.TrimSpace(header[i]) == "" {
				emptyCount++
			}
		}
		return fmt.Sprintf("unnamed_%c", 'a'+emptyCount-1)
	}
	return h
}

// parseJPathFieldCondition parses a field name that may contain a JPath expression.
// Returns columnName, jpathExpr, hasJPath.
// Example: "requestParameters{$.durationSeconds}" -> "requestParameters", "$.durationSeconds", true
// Example: "username" -> "username", "", false
func parseJPathFieldCondition(fieldName string) (string, string, bool) {
	// Look for the pattern: columnName{jpathExpr}
	openBrace := strings.Index(fieldName, "{")
	if openBrace == -1 {
		return fieldName, "", false
	}

	// Find the matching closing brace
	closeBrace := strings.LastIndex(fieldName, "}")
	if closeBrace == -1 || closeBrace <= openBrace {
		return fieldName, "", false
	}

	columnName := strings.TrimSpace(fieldName[:openBrace])
	jpathExpr := strings.TrimSpace(fieldName[openBrace+1 : closeBrace])

	if columnName == "" || jpathExpr == "" {
		return fieldName, "", false
	}

	return columnName, jpathExpr, true
}

// evaluateJPath parses JSON from columnValue and applies jpathExpr.
// Returns the extracted value as a string and true on success.
// Returns empty string and false on failure (invalid JSON, invalid JPath, no results).
func evaluateJPath(columnValue string, jpathExpr string) (string, bool) {
	if columnValue == "" || jpathExpr == "" {
		return "", false
	}

	// Parse JSON from column value
	data, err := oj.ParseString(columnValue)
	if err != nil {
		return "", false
	}

	// Parse JPath expression
	path, err := jp.ParseString(jpathExpr)
	if err != nil {
		// Invalid JPath syntax - this should be caught earlier for error reporting
		return "", false
	}

	// Apply JPath
	results := path.Get(data)
	if len(results) == 0 {
		return "", false
	}

	// Convert result to string
	result := results[0]
	switch v := result.(type) {
	case string:
		return v, true
	case float64:
		// Handle integers stored as float64
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v)), true
		}
		return fmt.Sprintf("%v", v), true
	case int64:
		return fmt.Sprintf("%d", v), true
	case int:
		return fmt.Sprintf("%d", v), true
	case bool:
		return fmt.Sprintf("%t", v), true
	case nil:
		return "", true
	case map[string]interface{}, []interface{}:
		// Complex types - JSON stringify
		jsonBytes, err := oj.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v), true
		}
		return string(jsonBytes), true
	default:
		return fmt.Sprintf("%v", v), true
	}
}

// NewQueryExecutor creates a new query executor
func NewQueryExecutor(c *cache.Cache, progress ProgressCallback, cacheConfig CacheConfig) *QueryExecutor {
	return &QueryExecutor{
		cache:          c,
		progress:       progress,
		cacheConfig:    cacheConfig,
		timezone:       time.Local,                            // Default to local timezone
		ingestTimezone: timestamps.GetDefaultIngestTimezone(), // Default ingest timezone from settings
		sortByTime:     false,
		sortDescending: false,
	}
}

// NewQueryExecutorWithTimezone creates a new query executor with a specific timezone
func NewQueryExecutorWithTimezone(c *cache.Cache, progress ProgressCallback, cacheConfig CacheConfig, timezone *time.Location) *QueryExecutor {
	return &QueryExecutor{
		cache:          c,
		progress:       progress,
		cacheConfig:    cacheConfig,
		timezone:       timezone,
		ingestTimezone: timestamps.GetDefaultIngestTimezone(), // Default ingest timezone from settings
		sortByTime:     false,
		sortDescending: false,
	}
}

// NewQueryExecutorWithIngestTimezone creates a new query executor with both display and ingest timezones
func NewQueryExecutorWithIngestTimezone(c *cache.Cache, progress ProgressCallback, cacheConfig CacheConfig, displayTimezone *time.Location, ingestTimezone *time.Location) *QueryExecutor {
	return &QueryExecutor{
		cache:          c,
		progress:       progress,
		cacheConfig:    cacheConfig,
		timezone:       displayTimezone,
		ingestTimezone: ingestTimezone,
		sortByTime:     false,
		sortDescending: false,
	}
}

// NewQueryExecutorWithSettings creates a new query executor with all settings
func NewQueryExecutorWithSettings(c *cache.Cache, progress ProgressCallback, cacheConfig CacheConfig, timezone *time.Location, ingestTimezone *time.Location, sortByTime bool, sortDescending bool) *QueryExecutor {
	if ingestTimezone == nil {
		ingestTimezone = timestamps.GetDefaultIngestTimezone()
	}
	return &QueryExecutor{
		cache:          c,
		progress:       progress,
		cacheConfig:    cacheConfig,
		timezone:       timezone,
		ingestTimezone: ingestTimezone,
		sortByTime:     sortByTime,
		sortDescending: sortDescending,
	}
}

// QueryExecutionResult contains the result of query execution with header tracking
type QueryExecutionResult struct {
	OriginalHeader []string // Original file header (all columns)
	Header         []string // Result header (may be modified by queries)
	DisplayColumns []int    // Mapping from result columns to original columns
	Rows           [][]string
	// OPTIMIZATION: Preserve StageResult for histogram generation
	// This contains pre-parsed Row objects with timestamps and TimestampStats
	StageResult *StageResult // Full result with pre-parsed timestamps (nil if not available)
	// PipelineCacheKey is the cache key built from parsed pipeline stages
	// Used for histogram caching to ensure consistent keys regardless of query whitespace
	PipelineCacheKey string
}

// ExecuteQuery executes a query using the new pipeline architecture
func (qe *QueryExecutor) ExecuteQuery(ctx context.Context, tab *FileTab, query string, timeField string, workspaceService interface{}) (*QueryExecutionResult, error) {
	if tab == nil || tab.FilePath == "" {
		return &QueryExecutionResult{
			OriginalHeader: []string{},
			Header:         []string{},
			DisplayColumns: []int{},
			Rows:           [][]string{},
		}, nil
	}

	// Parse query into pipeline stages FIRST (before reading file)
	// This allows us to check cache before expensive file read
	pipeline, err := qe.buildPipelineFromQuery(ctx, tab, query, timeField, workspaceService)
	if err != nil {
		return nil, fmt.Errorf("failed to build pipeline: %w", err)
	}

	// Build cache key from pipeline stages - always compute this for histogram caching
	// This ensures consistent cache keys regardless of query whitespace
	// BuildCacheKeyFull includes timeField, NoHeaderRow, and IngestTimezoneOverride so different settings use different cache entries
	pipelineCacheKey := BuildCacheKeyFull(tab.FileHash, pipeline.GetStages(), timeField, tab.Options.NoHeaderRow, tab.Options.IngestTimezoneOverride)

	// OPTIMIZATION: Check cache BEFORE reading file
	if qe.cache != nil && qe.cacheConfig.EnablePipelineCache {
		fmt.Printf("[CACHE_KEY_DEBUG] Query: %q -> Cache key: %s\n", query, pipelineCacheKey)
		if entry, found := qe.cache.Get(pipelineCacheKey); found && entry.IsComplete {
			fmt.Printf("[CACHE_KEY_DEBUG] CACHE HIT for key: %s\n", pipelineCacheKey)
			// Cache hit! Return cached result without reading file
			// IMPORTANT: Reassign DisplayIndex because cached Row objects may have stale values from previous queries
			for i, row := range entry.Rows {
				row.DisplayIndex = i
			}
			// OPTIMIZATION: Preserve StageResult for histogram generation
			cachedStageResult := &StageResult{
				OriginalHeader: entry.OriginalHeader,
				Header:         entry.Header,
				DisplayColumns: entry.DisplayColumns,
				Rows:           entry.Rows,
				TimestampStats: entry.TimestampStats, // Preserved from cache
			}
			return &QueryExecutionResult{
				OriginalHeader:   entry.OriginalHeader,
				Header:           entry.Header,
				DisplayColumns:   entry.DisplayColumns,
				Rows:             RowsToStrings(entry.Rows),
				StageResult:      cachedStageResult, // Preserve for histogram
				PipelineCacheKey: pipelineCacheKey,  // Include for histogram cache lookup
			}, nil
		}
	}

	// Cache miss for full pipeline - check if we have base file data cached
	var inputResult *StageResult

	// Include timeField, NoHeaderRow, and EFFECTIVE IngestTimezone in cache key so changing these settings invalidates cache
	// This ensures timestamps are re-parsed from the correct column when user changes it
	// And ensures files opened with different header modes or timezone settings don't share cached data
	// Use effective timezone (per-file override or global setting) to ensure cache invalidation on global changes
	effectiveIngestTz := timestamps.GetIngestTimezoneWithOverride(tab.Options.IngestTimezoneOverride)
	tzKey := effectiveIngestTz.String()
	baseFileCacheKey := fmt.Sprintf("file:%s:time:%s:noheader:%t:tz:%s", tab.FileHash, timeField, tab.Options.NoHeaderRow, tzKey)
	if qe.cache != nil && qe.cacheConfig.EnablePipelineCache {
		if entry, found := qe.cache.Get(baseFileCacheKey); found && entry.IsComplete {
			// Base file cached! Use it instead of reading from disk
			inputResult = &StageResult{
				OriginalHeader: entry.OriginalHeader,
				Header:         entry.Header,
				DisplayColumns: entry.DisplayColumns,
				Rows:           entry.Rows,
				TimestampStats: entry.TimestampStats, // Preserve timestamp stats for histogram
			}
		}
	}

	// If base file not cached, read from disk
	if inputResult == nil {
		reader := fileloader.NewFileReader(tab, qe.progress, ctx)
		defer reader.Close()

		// Resolve timestamp column index from user-specified timeField (or auto-detect if empty)
		timeIdx := qe.detectTimestampIndex(reader, timeField)

		// Read all data from file with correct timestamp column
		needsSorting := qe.needsSort(query, timeField)
		if needsSorting {
			// Use sorted file reader
			desc := qe.isDescendingSort(query)
			inputResult, err = reader.ReadRowsWithSort(timeIdx, desc)
		} else {
			// Use regular file reader with specified timestamp column
			inputResult, err = reader.ReadRowsWithTimeIdx(timeIdx)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}

		// Cache the base file data for future queries
		if qe.cache != nil && qe.cacheConfig.EnablePipelineCache && len(inputResult.Rows) > 0 {
			// Store base file data in cache with pre-parsed timestamps and timestamp stats
			// For JSON files: rows are shared pointers to baseDataStorage entries (sharedFromBaseData=true)
			// For CSV/XLSX files: this IS the authoritative data copy (sharedFromBaseData=false)
			// Using false for CSV/XLSX ensures accurate cache size accounting
			fileType := fileloader.DetectFileType(tab.FilePath)
			sharedFromBaseData := (fileType == fileloader.FileTypeJSON)
			qe.cache.StoreWithMetadata(baseFileCacheKey, inputResult.OriginalHeader, inputResult.Header, inputResult.DisplayColumns, inputResult.Rows, inputResult.TimestampStats, sharedFromBaseData)
		}
	}

	// Execute pipeline
	result, err := pipeline.Execute(inputResult)
	if err != nil {
		return nil, fmt.Errorf("pipeline execution failed: %w", err)
	}

	// Convert Row objects to raw string data for QueryExecutionResult
	// OPTIMIZATION: Preserve StageResult for histogram generation
	stageResult := &StageResult{
		OriginalHeader: result.OriginalHeader,
		Header:         result.Header,
		DisplayColumns: result.DisplayColumns,
		Rows:           result.Rows,
		TimestampStats: result.TimestampStats,
	}
	return &QueryExecutionResult{
		OriginalHeader:   result.OriginalHeader,
		Header:           result.Header,
		DisplayColumns:   result.DisplayColumns,
		Rows:             RowsToStrings(result.Rows),
		StageResult:      stageResult,      // Preserve for histogram with pre-parsed timestamps
		PipelineCacheKey: pipelineCacheKey, // Include for histogram cache lookup
	}, nil
}

// buildPipelineFromQuery parses a query string and builds a pipeline
func (qe *QueryExecutor) buildPipelineFromQuery(ctx context.Context, tab *FileTab, query string, timeField string, workspaceService interface{}) (*QueryPipeline, error) {
	fmt.Printf("[PIPELINE_DEBUG] Building pipeline for query: %q\n", query)
	builder := NewPipelineBuilder(ctx, tab.FileHash, timeField, tab.Options.NoHeaderRow, tab.Options.IngestTimezoneOverride, qe.cache, qe.progress, qe.cacheConfig)

	// Get header for column parsing (needed even for empty queries to detect timestamp column)
	reader := fileloader.NewFileReader(tab, nil, ctx)
	header, err := reader.Header()
	reader.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	// Helper to resolve timestamp column - use user-specified timeField or auto-detect
	resolveTimestampColumn := func() (string, int) {
		if timeField != "" {
			// Use user-specified timestamp column
			timeFieldLower := strings.ToLower(strings.TrimSpace(timeField))
			for i, h := range header {
				if strings.ToLower(strings.TrimSpace(h)) == timeFieldLower {
					return h, i
				}
			}
		}
		// Fallback to auto-detection
		timestampIdx := timestamps.DetectTimestampIndex(header)
		if timestampIdx >= 0 && timestampIdx < len(header) {
			return header[timestampIdx], timestampIdx
		}
		return "", -1
	}

	// Handle empty query case
	if query == "" {
		// Add implicit time sort if sortByTime is enabled (even for empty queries)
		if qe.sortByTime {
			timestampColumnName, timestampIdx := resolveTimestampColumn()
			if timestampIdx >= 0 {
				builder.AddTimeSort(timestampColumnName, qe.sortDescending)
			}
		}
		return builder.Build(), nil
	}

	// Parse pipeline stages
	stages := qe.splitPipesTopLevel(query)
	var unknownOperations []string

	for _, stage := range stages {
		stage = strings.TrimSpace(stage)
		if stage == "" {
			continue
		}

		tokens := qe.splitRespectingQuotes(stage)
		if len(tokens) == 0 {
			continue
		}

		head := strings.ToLower(tokens[0])

		switch head {
		case "columns":
			if columnNames := qe.parseColumnsStage(stage, header); len(columnNames) > 0 {
				builder.AddColumns(columnNames)
			}
		case "sort":
			columnNames, descending := qe.parseSortStage(stage, header)
			fmt.Printf("[SORT_DEBUG] Parsed stage %q: columns=%v, descending=%v\n", stage, columnNames, descending)
			if len(columnNames) > 0 {
				// Check if we're sorting by a timestamp column
				if len(columnNames) == 1 && qe.isTimestampColumn(columnNames[0], header) {
					// Use time-based sorting for timestamp columns
					desc := len(descending) > 0 && descending[0]
					fmt.Printf("[SORT_DEBUG] Using time sort for %q, desc=%v\n", columnNames[0], desc)
					builder.AddTimeSort(columnNames[0], desc)
				} else {
					// Use regular column-based sorting
					fmt.Printf("[SORT_DEBUG] Using regular sort for columns=%v, descending=%v\n", columnNames, descending)
					builder.AddSort(columnNames, descending)
				}
			}
		case "dedup":
			keyColumnNames := qe.parseDedupStage(stage, header)
			// Always add dedup stage, even with empty keyColumnNames (means dedup all)
			builder.AddDedup(keyColumnNames)
		case "limit":
			if count := qe.parseLimitStage(stage); count > 0 {
				builder.AddLimit(count)
			}
		case "strip":
			builder.AddStrip()
		case "filter":
			if filterQuery := qe.parseFilterStage(stage); filterQuery != "" {
				matcher := qe.buildMatcherFromQuery(filterQuery, header, timeField, -1, nil)
				// Include display timezone in cache key to ensure cache invalidation when timezone changes
				displayTzStr := qe.getDisplayTimezone().String()
				builder.AddFilter(matcher, -1, nil, workspaceService, tab.FileHash, tab.Options, stage, displayTzStr)
			}
		case "annotated":
			// Handle annotated operator as its own stage
			builder.AddAnnotated(workspaceService, tab.FileHash, tab.Options, false)
		case "not":
			// Check if this is "NOT annotated" - if so, use annotated stage with negation
			tokens := qe.splitRespectingQuotes(stage)
			if len(tokens) >= 2 && strings.EqualFold(tokens[1], "annotated") {
				// This is "NOT annotated" - use annotated stage with negation
				builder.AddAnnotated(workspaceService, tab.FileHash, tab.Options, true)
			} else {
				// Handle other NOT operations as filter stages (fallback to old behavior)
				matcher := qe.buildMatcherFromQuery(stage, header, timeField, -1, nil)
				// Include display timezone in cache key to ensure cache invalidation when timezone changes
				displayTzStr := qe.getDisplayTimezone().String()
				builder.AddFilter(matcher, -1, nil, workspaceService, tab.FileHash, tab.Options, stage, displayTzStr)
			}
		case "before", "after":
			// Handle time filter operations as implicit filter stages
			// When someone types "after 2024-01-01", treat it as "filter after 2024-01-01"
			// The buildMatcherFromQuery function will extract and process the time filters
			matcher := qe.buildMatcherFromQuery(stage, header, timeField, -1, nil)
			// Include display timezone in cache key to ensure cache invalidation when timezone changes
			displayTzStr := qe.getDisplayTimezone().String()
			builder.AddFilter(matcher, -1, nil, workspaceService, tab.FileHash, tab.Options, stage, displayTzStr)
		default:
			// Unknown operation - collect for error reporting
			unknownOperations = append(unknownOperations, head)
		}
	}

	// Return error if unknown operations were found
	if len(unknownOperations) > 0 {
		return nil, fmt.Errorf("unknown operation(s): %s", strings.Join(unknownOperations, ", "))
	}

	// Add implicit time sort if sortByTime is enabled and no explicit sort was added
	hasSortInQuery := qe.hasSortStage(query)
	fmt.Printf("[IMPLICIT_SORT_DEBUG] sortByTime=%t, hasSortStage=%t, query=%q\n",
		qe.sortByTime, hasSortInQuery, query)
	if qe.sortByTime && !hasSortInQuery {
		// Use user-specified timestamp column or auto-detect
		timestampColumnName, timestampIdx := resolveTimestampColumn()
		if timestampIdx >= 0 {
			fmt.Printf("[IMPLICIT_SORT_DEBUG] Adding implicit time sort on column: %s (from timeField=%q)\n", timestampColumnName, timeField)
			builder.AddTimeSort(timestampColumnName, qe.sortDescending)
		}
	} else {
		fmt.Printf("[IMPLICIT_SORT_DEBUG] NOT adding implicit sort\n")
	}

	return builder.Build(), nil
}

// hasSortStage checks if the query contains an explicit sort stage
func (qe *QueryExecutor) hasSortStage(query string) bool {
	if query == "" {
		return false
	}

	// Parse pipeline stages to check for sort stages
	stages := qe.splitPipesTopLevel(query)

	for _, stage := range stages {
		stage = strings.TrimSpace(stage)
		tokens := qe.splitRespectingQuotes(stage)
		if len(tokens) == 0 {
			continue
		}

		head := strings.ToLower(tokens[0])
		if head == "sort" {
			return true
		}
	}

	return false
}

// needsSort determines if the query requires sorting by checking for actual sort stages
func (qe *QueryExecutor) needsSort(query string, timeField string) bool {
	if query == "" {
		return false
	}

	// Parse pipeline stages to check for actual sort stages
	stages := qe.splitPipesTopLevel(query)

	for _, stage := range stages {
		stage = strings.TrimSpace(stage)
		if stage == "" {
			continue
		}

		tokens := qe.splitRespectingQuotes(stage)
		if len(tokens) == 0 {
			continue
		}

		head := strings.ToLower(tokens[0])
		if head == "sort" {
			return true
		}
	}

	// Don't sort just because timeField exists - only sort if explicitly requested
	return false
}

// detectTimestampIndex detects the timestamp column index
func (qe *QueryExecutor) detectTimestampIndex(reader FileReader, timeField string) int {
	header, err := reader.Header()
	if err != nil {
		return -1
	}

	if timeField != "" {
		timeFieldLower := strings.ToLower(strings.TrimSpace(timeField))
		for i, h := range header {
			if strings.ToLower(strings.TrimSpace(h)) == timeFieldLower {
				return i
			}
		}
	}

	// Auto-detect timestamp column (simplified)
	for i, h := range header {
		headerLower := strings.ToLower(h)
		if strings.Contains(headerLower, "time") || strings.Contains(headerLower, "date") {
			return i
		}
	}

	return -1
}

// resolveTimestampIndex resolves the timestamp column index from header using user-specified timeField
// Falls back to auto-detection if timeField is empty
func (qe *QueryExecutor) resolveTimestampIndex(header []string, timeField string) int {
	if timeField != "" {
		// Use user-specified timestamp column
		timeFieldLower := strings.ToLower(strings.TrimSpace(timeField))
		for i, h := range header {
			if strings.ToLower(strings.TrimSpace(h)) == timeFieldLower {
				return i
			}
		}
	}

	// Fallback to auto-detection using timestamps package
	return timestamps.DetectTimestampIndex(header)
}

// isDescendingSort determines if sort should be descending by parsing sort stages
func (qe *QueryExecutor) isDescendingSort(query string) bool {
	if query == "" {
		return false
	}

	// Parse pipeline stages to check for descending in actual sort stages
	stages := qe.splitPipesTopLevel(query)

	for _, stage := range stages {
		stage = strings.TrimSpace(stage)
		if stage == "" {
			continue
		}

		tokens := qe.splitRespectingQuotes(stage)
		if len(tokens) == 0 {
			continue
		}

		head := strings.ToLower(tokens[0])
		if head == "sort" {
			// Check for desc/asc direction in the sort stage
			for _, token := range tokens {
				if strings.EqualFold(token, "desc") || strings.EqualFold(token, "descending") {
					return true
				} else if strings.EqualFold(token, "asc") || strings.EqualFold(token, "ascending") {
					return false
				}
			}
			// Default to ascending if no direction specified
			return false
		}
	}

	// No sort stage found, default to ascending
	return false
}

// Query parsing functions (imported from existing util.go and search.go logic)

func (qe *QueryExecutor) splitPipesTopLevel(s string) []string {
	var out []string
	var cur strings.Builder
	inQuote := rune(0)
	for _, r := range s {
		if r == '"' || r == '\'' {
			if inQuote == 0 {
				inQuote = r
			} else if inQuote == r {
				inQuote = 0
			}
			cur.WriteRune(r)
			continue
		}
		if inQuote == 0 && r == '|' {
			out = append(out, strings.TrimSpace(cur.String()))
			cur.Reset()
			continue
		}
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		out = append(out, strings.TrimSpace(cur.String()))
	}
	return out
}

func (qe *QueryExecutor) splitRespectingQuotes(s string) []string {
	var out []string
	var cur strings.Builder
	inQuote := rune(0)
	for _, r := range s {
		if r == '"' || r == '\'' {
			if inQuote == 0 {
				inQuote = r
			} else if inQuote == r {
				inQuote = 0
			}
			cur.WriteRune(r)
			continue
		}
		if inQuote == 0 && (r == ' ' || r == '\t' || r == '\n' || r == '|') {
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
			continue
		}
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// splitByCommaRespectingQuotes splits a string by commas, but respects quoted strings
func (qe *QueryExecutor) splitByCommaRespectingQuotes(s string) []string {
	var out []string
	var cur strings.Builder
	inQuote := rune(0)
	for _, r := range s {
		if r == '"' || r == '\'' {
			if inQuote == 0 {
				inQuote = r
			} else if inQuote == r {
				inQuote = 0
			}
			cur.WriteRune(r)
			continue
		}
		if inQuote == 0 && r == ',' {
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
			continue
		}
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func (qe *QueryExecutor) parseColumnsStage(stage string, header []string) []string {
	tokens := qe.splitRespectingQuotes(stage)
	if len(tokens) == 0 || !strings.EqualFold(tokens[0], "columns") {
		return []string{}
	}

	var columnNames []string
	if len(tokens) > 1 {
		// Collect the rest of tokens as the columns spec string
		spec := strings.TrimSpace(strings.Join(tokens[1:], " "))
		// Split by comma
		parts := strings.Split(spec, ",")
		for _, p := range parts {
			name := strings.TrimSpace(qe.unquoteIfQuoted(p))
			if name == "" {
				continue
			}
			// Store the original column name (will be resolved at execution time)
			columnNames = append(columnNames, name)
		}
	}

	return columnNames
}

func (qe *QueryExecutor) parseSortStage(stage string, header []string) ([]string, []bool) {
	tokens := qe.splitRespectingQuotes(stage)
	if len(tokens) == 0 || !strings.EqualFold(tokens[0], "sort") {
		return []string{}, []bool{}
	}

	if len(tokens) < 2 {
		return []string{}, []bool{}
	}

	// Collect all tokens after "sort" (including direction keywords)
	// Direction keywords will be processed per-column in the suffix matching logic below
	var columnTokens []string
	for i := 1; i < len(tokens); i++ {
		columnTokens = append(columnTokens, tokens[i])
	}

	if len(columnTokens) == 0 {
		return []string{}, []bool{}
	}

	// Join all column tokens and split by comma to support multiple columns
	// This handles cases like: sort "event source", "event time"
	columnsStr := strings.Join(columnTokens, " ")
	columnParts := qe.splitByCommaRespectingQuotes(columnsStr)

	var columnNames []string
	var descendingFlags []bool

	for _, part := range columnParts {
		// Remove direction keywords from this column part and determine direction
		partTrimmed := strings.TrimSpace(part)
		columnDescending := false // Default to ascending for this column

		// Check for direction keyword and strip it (case-insensitive)
		partLower := strings.ToLower(partTrimmed)
		if strings.HasSuffix(partLower, " descending") {
			columnDescending = true
			partTrimmed = strings.TrimSpace(partTrimmed[:len(partTrimmed)-len(" descending")])
		} else if strings.HasSuffix(partLower, " desc") {
			columnDescending = true
			partTrimmed = strings.TrimSpace(partTrimmed[:len(partTrimmed)-len(" desc")])
		} else if strings.HasSuffix(partLower, " ascending") {
			columnDescending = false
			partTrimmed = strings.TrimSpace(partTrimmed[:len(partTrimmed)-len(" ascending")])
		} else if strings.HasSuffix(partLower, " asc") {
			columnDescending = false
			partTrimmed = strings.TrimSpace(partTrimmed[:len(partTrimmed)-len(" asc")])
		}

		// Parse column spec (may include JPath expression)
		columnSpec := strings.Trim(strings.TrimSpace(partTrimmed), `"`)

		// Check if this column has a JPath expression (e.g., "columnName{$.path}")
		// Extract base column name for header validation
		baseColumnName := columnSpec
		if openBrace := strings.Index(columnSpec, "{"); openBrace != -1 {
			baseColumnName = columnSpec[:openBrace]
		}
		baseColumnNameLower := strings.ToLower(strings.TrimSpace(baseColumnName))

		// Verify base column exists in header (but return the name, not index)
		found := false
		for i := range header {
			if normalizeHeaderName(header, i) == baseColumnNameLower {
				found = true
				break
			}
		}

		if found {
			// Keep the full column spec (with JPath if present), but lowercase the base column name
			// Preserve JPath case since JSON keys can be case-sensitive
			var finalColumnName string
			if openBrace := strings.Index(columnSpec, "{"); openBrace != -1 {
				finalColumnName = baseColumnNameLower + columnSpec[openBrace:]
			} else {
				finalColumnName = baseColumnNameLower
			}
			columnNames = append(columnNames, finalColumnName)
			descendingFlags = append(descendingFlags, columnDescending)
		}
	}

	if len(columnNames) == 0 {
		return []string{}, []bool{}
	}

	return columnNames, descendingFlags
}

func (qe *QueryExecutor) parseDedupStage(stage string, header []string) []string {
	tokens := qe.splitRespectingQuotes(stage)
	if len(tokens) == 0 || !strings.EqualFold(tokens[0], "dedup") {
		return []string{}
	}

	// If no column specified, dedup on all columns
	if len(tokens) == 1 {
		return []string{} // Empty means all columns
	}

	// Parse specific column names (keep them normalized for resolution at execution time)
	var columnNames []string
	for i := 1; i < len(tokens); i++ {
		columnSpec := strings.Trim(tokens[i], `"`)

		// Check if this column has a JPath expression (e.g., "columnName{$.path}")
		// Extract base column name for header validation
		baseColumnName := columnSpec
		if openBrace := strings.Index(columnSpec, "{"); openBrace != -1 {
			baseColumnName = columnSpec[:openBrace]
		}
		baseColumnNameLower := strings.ToLower(strings.TrimSpace(baseColumnName))

		// Verify base column exists in header
		found := false
		for j := range header {
			if normalizeHeaderName(header, j) == baseColumnNameLower {
				found = true
				break
			}
		}
		if found {
			// Keep the full column spec (with JPath if present), but lowercase the base column name
			// Preserve JPath case since JSON keys can be case-sensitive
			if openBrace := strings.Index(columnSpec, "{"); openBrace != -1 {
				columnNames = append(columnNames, baseColumnNameLower+columnSpec[openBrace:])
			} else {
				columnNames = append(columnNames, baseColumnNameLower)
			}
		}
	}

	return columnNames
}

func (qe *QueryExecutor) parseLimitStage(stage string) int {
	tokens := qe.splitRespectingQuotes(stage)
	if len(tokens) < 2 || !strings.EqualFold(tokens[0], "limit") {
		return 0
	}

	if count, err := fmt.Sscanf(tokens[1], "%d", new(int)); err == nil && count == 1 {
		var result int
		fmt.Sscanf(tokens[1], "%d", &result)
		return result
	}

	return 0
}

func (qe *QueryExecutor) parseFilterStage(stage string) string {
	tokens := qe.splitRespectingQuotes(stage)
	if len(tokens) < 2 || !strings.EqualFold(tokens[0], "filter") {
		return ""
	}

	// Join all tokens after "filter" to form the filter query
	filterQuery := strings.TrimSpace(strings.Join(tokens[1:], " "))
	return filterQuery
}

func (qe *QueryExecutor) buildMatcherFromQuery(query string, header []string, timeField string, displayIdx int, formatFunc func(string) (string, bool)) func(*Row) bool {
	if query == "" {
		return func(_ *Row) bool { return true }
	}

	// Extract time filters and clean the query
	afterMs, beforeMs, cleanedQuery := qe.ExtractTimeFilters(query)

	// Resolve timestamp index from user-specified timeField (or auto-detect if empty)
	timeIdx := qe.resolveTimestampIndex(header, timeField)

	// If only time filters (no other query), return time-only matcher
	if cleanedQuery == "" || cleanedQuery == "*" {
		return func(row *Row) bool {
			if (afterMs != nil || beforeMs != nil) && timeIdx >= 0 {
				// Use pre-parsed timestamp from Row
				if !row.HasTime {
					return false
				}
				tsMs := row.Timestamp
				if afterMs != nil && tsMs < *afterMs {
					return false
				}
				if beforeMs != nil && tsMs > *beforeMs {
					return false
				}
			}
			return true
		}
	}

	// Use cleaned query for the rest of processing
	query = cleanedQuery

	// Check if the query contains boolean operators (AND, OR, NOT, parentheses)
	// If so, parse it as a boolean expression
	if ContainsBooleanOperators(query) {
		ast, ok := ParseFilterExpression(query)
		if ok && ast != nil {
			// Create a condition evaluator that handles individual conditions
			evalCondition := qe.createConditionEvaluator(header)

			// Return a matcher that evaluates the boolean expression
			return func(row *Row) bool {
				// Apply time filters first using pre-parsed timestamp
				if (afterMs != nil || beforeMs != nil) && timeIdx >= 0 {
					if !row.HasTime {
						return false
					}
					tsMs := row.Timestamp
					if afterMs != nil && tsMs < *afterMs {
						return false
					}
					if beforeMs != nil && tsMs > *beforeMs {
						return false
					}
				}
				// Evaluate the boolean expression
				return ast.Eval(row.Data, evalCondition)
			}
		}
	}

	// Note: Annotated operator handling has been moved to its own stage type.
	// This prevents "filter annotated" usage and forces users to use "annotated" as a standalone operation.

	// Parse field-specific queries like "username=scrappy" or "event name"=Describe
	// Also supports JPath expressions like "requestParameters{$.durationSeconds}=3600"
	if strings.Contains(query, "=") && !strings.Contains(query, "!=") {
		parts := strings.SplitN(query, "=", 2)
		if len(parts) == 2 {
			fieldName := strings.TrimSpace(parts[0])
			targetValue := strings.TrimSpace(parts[1])

			// Remove quotes from field name if present
			if len(fieldName) >= 2 && ((fieldName[0] == '"' && fieldName[len(fieldName)-1] == '"') || (fieldName[0] == '\'' && fieldName[len(fieldName)-1] == '\'')) {
				fieldName = fieldName[1 : len(fieldName)-1]
			}

			// Check for JPath expression in field name
			columnName, jpathExpr, hasJPath := parseJPathFieldCondition(fieldName)
			if hasJPath {
				columnName = strings.ToLower(strings.TrimSpace(columnName))
			} else {
				columnName = strings.ToLower(strings.TrimSpace(fieldName))
			}

			// Remove quotes from target value if present
			if len(targetValue) >= 2 && ((targetValue[0] == '"' && targetValue[len(targetValue)-1] == '"') || (targetValue[0] == '\'' && targetValue[len(targetValue)-1] == '\'')) {
				targetValue = targetValue[1 : len(targetValue)-1]
			}

			// Find the column index for this field
			// Headers are already normalized at ingestion
			fieldIndex := -1
			for i, h := range header {
				if strings.EqualFold(h, columnName) {
					fieldIndex = i
					break
				}
			}

			if fieldIndex >= 0 {
				// Check if this is filtering on the timestamp column
				isTimestampColumn := fieldIndex == timeIdx && !hasJPath

				// Check if targetValue ends with * for prefix matching
				if strings.HasSuffix(targetValue, "*") {
					prefixValue := targetValue[:len(targetValue)-1] // Remove the *

					// For timestamp column, prepare timestamp matching info
					var tsMatchInfo *timestamps.TimestampMatchInfo
					if isTimestampColumn {
						tsMatchInfo = timestamps.PrepareTimestampMatch(targetValue, qe.timezone)
					}

					return func(row *Row) bool {
						// Apply time filters first using pre-parsed timestamp
						if (afterMs != nil || beforeMs != nil) && timeIdx >= 0 {
							if !row.HasTime {
								return false
							}
							tsMs := row.Timestamp
							if afterMs != nil && tsMs < *afterMs {
								return false
							}
							if beforeMs != nil && tsMs > *beforeMs {
								return false
							}
						}

						// For timestamp column, use timestamp-aware matching
						if isTimestampColumn && tsMatchInfo != nil {
							return timestamps.MatchTimestamp(row.Timestamp, row.HasTime, qe.timezone, "", tsMatchInfo)
						}

						// Apply field match
						if fieldIndex < len(row.Data) {
							actualValue := strings.TrimSpace(row.Data[fieldIndex])

							// If JPath expression, evaluate it first
							if hasJPath {
								extractedValue, ok := evaluateJPath(actualValue, jpathExpr)
								if !ok {
									return false
								}
								actualValue = extractedValue
							}

							// If just "*" (empty prefix), match any non-empty value
							if prefixValue == "" {
								return actualValue != ""
							}
							matches := strings.HasPrefix(strings.ToLower(actualValue), strings.ToLower(prefixValue))
							return matches
						}
						return false
					}
				} else {
					// For timestamp column, prepare timestamp matching info
					var tsMatchInfo *timestamps.TimestampMatchInfo
					if isTimestampColumn {
						tsMatchInfo = timestamps.PrepareTimestampMatch(targetValue, qe.timezone)
					}

					return func(row *Row) bool {
						// Apply time filters first using pre-parsed timestamp
						if (afterMs != nil || beforeMs != nil) && timeIdx >= 0 {
							if !row.HasTime {
								return false
							}
							tsMs := row.Timestamp
							if afterMs != nil && tsMs < *afterMs {
								return false
							}
							if beforeMs != nil && tsMs > *beforeMs {
								return false
							}
						}

						// For timestamp column, use timestamp-aware matching
						if isTimestampColumn && tsMatchInfo != nil {
							return timestamps.MatchTimestamp(row.Timestamp, row.HasTime, qe.timezone, "", tsMatchInfo)
						}

						// Apply field match
						if fieldIndex < len(row.Data) {
							actualValue := strings.TrimSpace(row.Data[fieldIndex])

							// If JPath expression, evaluate it first
							if hasJPath {
								extractedValue, ok := evaluateJPath(actualValue, jpathExpr)
								if !ok {
									return false
								}
								actualValue = extractedValue
							}

							matches := strings.EqualFold(actualValue, targetValue)
							return matches
						}
						return false
					}
				}
			} else {
				return func(row *Row) bool { return false }
			}
		}
	}

	// Parse field-specific NOT EQUAL queries like "username!=scrappy" or "event name"!=Describe
	// Also supports JPath expressions like "requestParameters{$.durationSeconds}!=3600"
	if strings.Contains(query, "!=") {
		parts := strings.SplitN(query, "!=", 2)
		if len(parts) == 2 {
			fieldName := strings.TrimSpace(parts[0])
			targetValue := strings.TrimSpace(parts[1])

			// Remove quotes from field name if present
			if len(fieldName) >= 2 && ((fieldName[0] == '"' && fieldName[len(fieldName)-1] == '"') || (fieldName[0] == '\'' && fieldName[len(fieldName)-1] == '\'')) {
				fieldName = fieldName[1 : len(fieldName)-1]
			}

			// Check for JPath expression in field name
			columnName, jpathExpr, hasJPath := parseJPathFieldCondition(fieldName)
			if hasJPath {
				columnName = strings.ToLower(strings.TrimSpace(columnName))
			} else {
				columnName = strings.ToLower(strings.TrimSpace(fieldName))
			}

			// Remove quotes from target value if present
			if len(targetValue) >= 2 && ((targetValue[0] == '"' && targetValue[len(targetValue)-1] == '"') || (targetValue[0] == '\'' && targetValue[len(targetValue)-1] == '\'')) {
				targetValue = targetValue[1 : len(targetValue)-1]
			}

			// Find the column index for this field
			fieldIndex := -1
			for i := range header {
				if normalizeHeaderName(header, i) == columnName {
					fieldIndex = i
					break
				}
			}

			if fieldIndex >= 0 {
				// Check if targetValue ends with * for prefix matching
				if strings.HasSuffix(targetValue, "*") {
					prefixValue := targetValue[:len(targetValue)-1] // Remove the *
					return func(row *Row) bool {
						// Apply time filters first using pre-parsed timestamp
						if (afterMs != nil || beforeMs != nil) && timeIdx >= 0 {
							if !row.HasTime {
								return false
							}
							tsMs := row.Timestamp
							if afterMs != nil && tsMs < *afterMs {
								return false
							}
							if beforeMs != nil && tsMs > *beforeMs {
								return false
							}
						}
						// Apply field match
						if fieldIndex < len(row.Data) {
							actualValue := strings.TrimSpace(row.Data[fieldIndex])

							// If JPath expression, evaluate it first
							if hasJPath {
								extractedValue, ok := evaluateJPath(actualValue, jpathExpr)
								if !ok {
									// JPath evaluation failed - treat as not matching (so != returns true)
									return true
								}
								actualValue = extractedValue
							}

							return !strings.HasPrefix(strings.ToLower(actualValue), strings.ToLower(prefixValue))
						}
						return true
					}
				} else {
					return func(row *Row) bool {
						// Apply time filters first using pre-parsed timestamp
						if (afterMs != nil || beforeMs != nil) && timeIdx >= 0 {
							if !row.HasTime {
								return false
							}
							tsMs := row.Timestamp
							if afterMs != nil && tsMs < *afterMs {
								return false
							}
							if beforeMs != nil && tsMs > *beforeMs {
								return false
							}
						}
						// Apply field match
						if fieldIndex < len(row.Data) {
							actualValue := strings.TrimSpace(row.Data[fieldIndex])

							// If JPath expression, evaluate it first
							if hasJPath {
								extractedValue, ok := evaluateJPath(actualValue, jpathExpr)
								if !ok {
									// JPath evaluation failed - treat as not matching (so != returns true)
									return true
								}
								actualValue = extractedValue
							}

							return !strings.EqualFold(actualValue, targetValue)
						}
						return true
					}
				}
			}
		}
	}

	// Parse field-specific CONTAINS queries like "username~scrappy" or "event name"~Describe
	// Also supports JPath expressions like "requestParameters{$.roleArn}~admin"
	if strings.Contains(query, "~") && !strings.Contains(query, "!~") {
		parts := strings.SplitN(query, "~", 2)
		if len(parts) == 2 {
			fieldName := strings.TrimSpace(parts[0])
			targetValue := strings.TrimSpace(parts[1])

			// Remove quotes from field name if present
			if len(fieldName) >= 2 && ((fieldName[0] == '"' && fieldName[len(fieldName)-1] == '"') || (fieldName[0] == '\'' && fieldName[len(fieldName)-1] == '\'')) {
				fieldName = fieldName[1 : len(fieldName)-1]
			}

			// Check for JPath expression in field name
			columnName, jpathExpr, hasJPath := parseJPathFieldCondition(fieldName)
			if hasJPath {
				columnName = strings.ToLower(strings.TrimSpace(columnName))
			} else {
				columnName = strings.ToLower(strings.TrimSpace(fieldName))
			}

			// Remove quotes from target value if present
			if len(targetValue) >= 2 && ((targetValue[0] == '"' && targetValue[len(targetValue)-1] == '"') || (targetValue[0] == '\'' && targetValue[len(targetValue)-1] == '\'')) {
				targetValue = targetValue[1 : len(targetValue)-1]
			}
			targetValueLower := strings.ToLower(targetValue)

			// Find the column index for this field
			fieldIndex := -1
			for i, h := range header {
				if strings.EqualFold(h, columnName) {
					fieldIndex = i
					break
				}
			}

			if fieldIndex >= 0 {
				return func(row *Row) bool {
					// Apply time filters first using pre-parsed timestamp
					if (afterMs != nil || beforeMs != nil) && timeIdx >= 0 {
						if !row.HasTime {
							return false
						}
						tsMs := row.Timestamp
						if afterMs != nil && tsMs < *afterMs {
							return false
						}
						if beforeMs != nil && tsMs > *beforeMs {
							return false
						}
					}
					// Apply field contains match
					if fieldIndex < len(row.Data) {
						actualValue := strings.TrimSpace(row.Data[fieldIndex])

						// If JPath expression, evaluate it first
						if hasJPath {
							extractedValue, ok := evaluateJPath(actualValue, jpathExpr)
							if !ok {
								return false
							}
							actualValue = extractedValue
						}

						return strings.Contains(strings.ToLower(actualValue), targetValueLower)
					}
					return false
				}
			} else {
				return func(row *Row) bool { return false }
			}
		}
	}

	// Parse field-specific NOT CONTAINS queries like "username!~scrappy" or "event name"!~Describe
	// Also supports JPath expressions like "requestParameters{$.roleArn}!~admin"
	if strings.Contains(query, "!~") {
		parts := strings.SplitN(query, "!~", 2)
		if len(parts) == 2 {
			fieldName := strings.TrimSpace(parts[0])
			targetValue := strings.TrimSpace(parts[1])

			// Remove quotes from field name if present
			if len(fieldName) >= 2 && ((fieldName[0] == '"' && fieldName[len(fieldName)-1] == '"') || (fieldName[0] == '\'' && fieldName[len(fieldName)-1] == '\'')) {
				fieldName = fieldName[1 : len(fieldName)-1]
			}

			// Check for JPath expression in field name
			columnName, jpathExpr, hasJPath := parseJPathFieldCondition(fieldName)
			if hasJPath {
				columnName = strings.ToLower(strings.TrimSpace(columnName))
			} else {
				columnName = strings.ToLower(strings.TrimSpace(fieldName))
			}

			// Remove quotes from target value if present
			if len(targetValue) >= 2 && ((targetValue[0] == '"' && targetValue[len(targetValue)-1] == '"') || (targetValue[0] == '\'' && targetValue[len(targetValue)-1] == '\'')) {
				targetValue = targetValue[1 : len(targetValue)-1]
			}
			targetValueLower := strings.ToLower(targetValue)

			// Find the column index for this field
			fieldIndex := -1
			for i, h := range header {
				if strings.EqualFold(h, columnName) {
					fieldIndex = i
					break
				}
			}

			if fieldIndex >= 0 {
				return func(row *Row) bool {
					// Apply time filters first using pre-parsed timestamp
					if (afterMs != nil || beforeMs != nil) && timeIdx >= 0 {
						if !row.HasTime {
							return false
						}
						tsMs := row.Timestamp
						if afterMs != nil && tsMs < *afterMs {
							return false
						}
						if beforeMs != nil && tsMs > *beforeMs {
							return false
						}
					}
					// Apply field NOT contains match
					if fieldIndex < len(row.Data) {
						actualValue := strings.TrimSpace(row.Data[fieldIndex])

						// If JPath expression, evaluate it first
						if hasJPath {
							extractedValue, ok := evaluateJPath(actualValue, jpathExpr)
							if !ok {
								// JPath evaluation failed - treat as not containing (so !~ returns true)
								return true
							}
							actualValue = extractedValue
						}

						return !strings.Contains(strings.ToLower(actualValue), targetValueLower)
					}
					return true
				}
			} else {
				// Field not found, so it doesn't contain the value
				return func(row *Row) bool { return true }
			}
		}
	}

	// Fallback to simple contains-based matching for other queries
	// Unquote the query if it's quoted for literal text search
	unquotedQuery := qe.unquoteIfQuoted(query)
	queryLower := strings.ToLower(unquotedQuery)

	// Check if query ends with * for prefix matching
	if strings.HasSuffix(queryLower, "*") {
		prefixValue := queryLower[:len(queryLower)-1] // Remove the *
		return func(row *Row) bool {
			// Apply time filters first using pre-parsed timestamp
			if (afterMs != nil || beforeMs != nil) && timeIdx >= 0 {
				if !row.HasTime {
					return false
				}
				tsMs := row.Timestamp
				if afterMs != nil && tsMs < *afterMs {
					return false
				}
				if beforeMs != nil && tsMs > *beforeMs {
					return false
				}
			}
			// Check if any field starts with the prefix
			for _, field := range row.Data {
				if strings.HasPrefix(strings.ToLower(field), prefixValue) {
					return true
				}
			}
			return false
		}
	} else {
		return func(row *Row) bool {
			// Apply time filters first using pre-parsed timestamp
			if (afterMs != nil || beforeMs != nil) && timeIdx >= 0 {
				if !row.HasTime {
					return false
				}
				tsMs := row.Timestamp
				if afterMs != nil && tsMs < *afterMs {
					return false
				}
				if beforeMs != nil && tsMs > *beforeMs {
					return false
				}
			}
			// Check if any field contains the query string
			for _, field := range row.Data {
				if strings.Contains(strings.ToLower(field), queryLower) {
					return true
				}
			}
			return false
		}
	}
}

// createConditionEvaluator creates a function that evaluates individual conditions against a row.
// This is used by the boolean expression parser to evaluate each literal condition.
// Supports JPath expressions like "columnName{$.path}=value" for filtering on JSON column content.
func (qe *QueryExecutor) createConditionEvaluator(header []string) func(condition string, row []string) bool {
	return func(condition string, row []string) bool {
		condition = strings.TrimSpace(condition)
		if condition == "" {
			return true
		}

		// Handle field=value conditions (including JPath expressions)
		if strings.Contains(condition, "=") && !strings.Contains(condition, "!=") {
			parts := strings.SplitN(condition, "=", 2)
			if len(parts) == 2 {
				fieldName := strings.TrimSpace(parts[0])
				targetValue := strings.TrimSpace(parts[1])

				// Remove quotes from field name if present
				if len(fieldName) >= 2 && ((fieldName[0] == '"' && fieldName[len(fieldName)-1] == '"') || (fieldName[0] == '\'' && fieldName[len(fieldName)-1] == '\'')) {
					fieldName = fieldName[1 : len(fieldName)-1]
				}

				// Check for JPath expression in field name
				columnName, jpathExpr, hasJPath := parseJPathFieldCondition(fieldName)
				if hasJPath {
					columnName = strings.ToLower(strings.TrimSpace(columnName))
				} else {
					columnName = strings.ToLower(strings.TrimSpace(fieldName))
				}

				// Remove quotes from target value if present
				if len(targetValue) >= 2 && ((targetValue[0] == '"' && targetValue[len(targetValue)-1] == '"') || (targetValue[0] == '\'' && targetValue[len(targetValue)-1] == '\'')) {
					targetValue = targetValue[1 : len(targetValue)-1]
				}

				// Find the column index for this field
				fieldIndex := -1
				for i, h := range header {
					if strings.EqualFold(h, columnName) {
						fieldIndex = i
						break
					}
				}

				if fieldIndex >= 0 && fieldIndex < len(row) {
					actualValue := strings.TrimSpace(row[fieldIndex])

					// If JPath expression, evaluate it first
					if hasJPath {
						extractedValue, ok := evaluateJPath(actualValue, jpathExpr)
						if !ok {
							return false
						}
						actualValue = extractedValue
					}

					// Check for prefix matching (ending with *)
					if strings.HasSuffix(targetValue, "*") {
						prefixValue := targetValue[:len(targetValue)-1]
						// If just "*" (empty prefix), match any non-empty value
						if prefixValue == "" {
							return actualValue != ""
						}
						return strings.HasPrefix(strings.ToLower(actualValue), strings.ToLower(prefixValue))
					}
					return strings.EqualFold(actualValue, targetValue)
				}
				return false
			}
		}

		// Handle field!=value conditions (including JPath expressions)
		if strings.Contains(condition, "!=") {
			parts := strings.SplitN(condition, "!=", 2)
			if len(parts) == 2 {
				fieldName := strings.TrimSpace(parts[0])
				targetValue := strings.TrimSpace(parts[1])

				// Remove quotes from field name if present
				if len(fieldName) >= 2 && ((fieldName[0] == '"' && fieldName[len(fieldName)-1] == '"') || (fieldName[0] == '\'' && fieldName[len(fieldName)-1] == '\'')) {
					fieldName = fieldName[1 : len(fieldName)-1]
				}

				// Check for JPath expression in field name
				columnName, jpathExpr, hasJPath := parseJPathFieldCondition(fieldName)
				if hasJPath {
					columnName = strings.ToLower(strings.TrimSpace(columnName))
				} else {
					columnName = strings.ToLower(strings.TrimSpace(fieldName))
				}

				// Remove quotes from target value if present
				if len(targetValue) >= 2 && ((targetValue[0] == '"' && targetValue[len(targetValue)-1] == '"') || (targetValue[0] == '\'' && targetValue[len(targetValue)-1] == '\'')) {
					targetValue = targetValue[1 : len(targetValue)-1]
				}

				// Find the column index for this field
				fieldIndex := -1
				for i := range header {
					if normalizeHeaderName(header, i) == columnName {
						fieldIndex = i
						break
					}
				}

				if fieldIndex >= 0 && fieldIndex < len(row) {
					actualValue := strings.TrimSpace(row[fieldIndex])

					// If JPath expression, evaluate it first
					if hasJPath {
						extractedValue, ok := evaluateJPath(actualValue, jpathExpr)
						if !ok {
							// JPath evaluation failed - treat as not matching (so != returns true)
							return true
						}
						actualValue = extractedValue
					}

					// Check for prefix matching (ending with *)
					if strings.HasSuffix(targetValue, "*") {
						prefixValue := targetValue[:len(targetValue)-1]
						return !strings.HasPrefix(strings.ToLower(actualValue), strings.ToLower(prefixValue))
					}
					return !strings.EqualFold(actualValue, targetValue)
				}
				return true // Field not found, so it's not equal
			}
		}

		// Handle field~value conditions (contains, including JPath expressions)
		if strings.Contains(condition, "~") && !strings.Contains(condition, "!~") {
			parts := strings.SplitN(condition, "~", 2)
			if len(parts) == 2 {
				fieldName := strings.TrimSpace(parts[0])
				targetValue := strings.TrimSpace(parts[1])

				// Remove quotes from field name if present
				if len(fieldName) >= 2 && ((fieldName[0] == '"' && fieldName[len(fieldName)-1] == '"') || (fieldName[0] == '\'' && fieldName[len(fieldName)-1] == '\'')) {
					fieldName = fieldName[1 : len(fieldName)-1]
				}

				// Check for JPath expression in field name
				columnName, jpathExpr, hasJPath := parseJPathFieldCondition(fieldName)
				if hasJPath {
					columnName = strings.ToLower(strings.TrimSpace(columnName))
				} else {
					columnName = strings.ToLower(strings.TrimSpace(fieldName))
				}

				// Remove quotes from target value if present
				if len(targetValue) >= 2 && ((targetValue[0] == '"' && targetValue[len(targetValue)-1] == '"') || (targetValue[0] == '\'' && targetValue[len(targetValue)-1] == '\'')) {
					targetValue = targetValue[1 : len(targetValue)-1]
				}
				targetValueLower := strings.ToLower(targetValue)

				// Find the column index for this field
				fieldIndex := -1
				for i, h := range header {
					if strings.EqualFold(h, columnName) {
						fieldIndex = i
						break
					}
				}

				if fieldIndex >= 0 && fieldIndex < len(row) {
					actualValue := strings.TrimSpace(row[fieldIndex])

					// If JPath expression, evaluate it first
					if hasJPath {
						extractedValue, ok := evaluateJPath(actualValue, jpathExpr)
						if !ok {
							return false
						}
						actualValue = extractedValue
					}

					return strings.Contains(strings.ToLower(actualValue), targetValueLower)
				}
				return false
			}
		}

		// Handle field!~value conditions (not contains, including JPath expressions)
		if strings.Contains(condition, "!~") {
			parts := strings.SplitN(condition, "!~", 2)
			if len(parts) == 2 {
				fieldName := strings.TrimSpace(parts[0])
				targetValue := strings.TrimSpace(parts[1])

				// Remove quotes from field name if present
				if len(fieldName) >= 2 && ((fieldName[0] == '"' && fieldName[len(fieldName)-1] == '"') || (fieldName[0] == '\'' && fieldName[len(fieldName)-1] == '\'')) {
					fieldName = fieldName[1 : len(fieldName)-1]
				}

				// Check for JPath expression in field name
				columnName, jpathExpr, hasJPath := parseJPathFieldCondition(fieldName)
				if hasJPath {
					columnName = strings.ToLower(strings.TrimSpace(columnName))
				} else {
					columnName = strings.ToLower(strings.TrimSpace(fieldName))
				}

				// Remove quotes from target value if present
				if len(targetValue) >= 2 && ((targetValue[0] == '"' && targetValue[len(targetValue)-1] == '"') || (targetValue[0] == '\'' && targetValue[len(targetValue)-1] == '\'')) {
					targetValue = targetValue[1 : len(targetValue)-1]
				}
				targetValueLower := strings.ToLower(targetValue)

				// Find the column index for this field
				fieldIndex := -1
				for i, h := range header {
					if strings.EqualFold(h, columnName) {
						fieldIndex = i
						break
					}
				}

				if fieldIndex >= 0 && fieldIndex < len(row) {
					actualValue := strings.TrimSpace(row[fieldIndex])

					// If JPath expression, evaluate it first
					if hasJPath {
						extractedValue, ok := evaluateJPath(actualValue, jpathExpr)
						if !ok {
							// JPath evaluation failed - treat as not containing (so !~ returns true)
							return true
						}
						actualValue = extractedValue
					}

					return !strings.Contains(strings.ToLower(actualValue), targetValueLower)
				}
				return true // Field not found, so it doesn't contain the value
			}
		}

		// Simple contains-based matching (search any field for the term)
		unquotedCondition := qe.unquoteIfQuoted(condition)
		conditionLower := strings.ToLower(unquotedCondition)

		// Check for prefix matching (ending with *)
		if strings.HasSuffix(conditionLower, "*") {
			prefixValue := conditionLower[:len(conditionLower)-1]
			for _, field := range row {
				fieldValue := strings.TrimSpace(field)
				// If just "*" (empty prefix), match any non-empty value
				if prefixValue == "" {
					if fieldValue != "" {
						return true
					}
				} else if strings.HasPrefix(strings.ToLower(fieldValue), prefixValue) {
					return true
				}
			}
			return false
		}

		// Standard contains matching
		for _, field := range row {
			if strings.Contains(strings.ToLower(field), conditionLower) {
				return true
			}
		}
		return false
	}
}

func (qe *QueryExecutor) unquoteIfQuoted(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// isQuoted checks if a string is wrapped in quotes
func isQuoted(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		return (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')
	}
	return false
}

// ExtractTimeFilters scans the query for special time filters: "after VALUE" and/or "before VALUE".
// Returns pointers to epoch ms values if present and the cleaned query with those tokens removed.
func (qe *QueryExecutor) ExtractTimeFilters(query string) (after *int64, before *int64, cleaned string) {
	toks := qe.splitRespectingQuotesForTimeFilters(query)
	if len(toks) == 0 {
		return nil, nil, strings.TrimSpace(query)
	}
	var out []string
	now := time.Now()
	// Use DisplayTimezone for timezone-less absolute timestamps in filters
	// This ensures users can enter filter times in their preferred display timezone
	loc := qe.getDisplayTimezone()
	i := 0
	for i < len(toks) {
		t := toks[i]
		tl := strings.ToLower(strings.TrimSpace(t))
		if tl == "after" && i+1 < len(toks) {
			// Try to parse the time value, potentially consuming multiple tokens
			consumed, ms := qe.tryParseTimeValue(toks, i+1, now, loc)
			if consumed > 0 {
				after = &ms
				i += 1 + consumed // Skip "after" + consumed tokens
				continue
			}
		} else if tl == "before" && i+1 < len(toks) {
			// Try to parse the time value, potentially consuming multiple tokens
			consumed, ms := qe.tryParseTimeValue(toks, i+1, now, loc)
			if consumed > 0 {
				before = &ms
				i += 1 + consumed // Skip "before" + consumed tokens
				continue
			}
		}
		out = append(out, t)
		i++
	}
	cleaned = strings.TrimSpace(strings.Join(out, " "))
	return
}

// tryParseTimeValue attempts to parse a time value starting at the given index,
// potentially consuming multiple tokens to handle datetime strings with spaces.
// Returns the number of tokens consumed and the parsed timestamp in milliseconds.
func (qe *QueryExecutor) tryParseTimeValue(toks []string, startIdx int, now time.Time, loc *time.Location) (consumed int, ms int64) {
	if startIdx >= len(toks) {
		return 0, 0
	}

	// First, try parsing just the first token (handles quoted strings and single tokens)
	val := strings.TrimSpace(qe.unquoteIfQuoted(toks[startIdx]))
	if ms, ok := timestamps.ParseFlexibleTime(val, now, loc); ok {
		// If it's a quoted string, we're done
		if isQuoted(toks[startIdx]) {
			return 1, ms
		}

		// If it's not quoted, check if we can parse more tokens to get a better match
		// Try combining with the next token (for "2024-01-01 12:00:00" format)
		if startIdx+1 < len(toks) {
			combinedVal := val + " " + strings.TrimSpace(toks[startIdx+1])
			if combinedMs, ok := timestamps.ParseFlexibleTime(combinedVal, now, loc); ok {
				// The combined value parsed successfully, use it instead
				return 2, combinedMs
			}
		}

		// Single token parsing worked, use it
		return 1, ms
	}

	// Single token didn't work, try combining with next token
	if startIdx+1 < len(toks) {
		combinedVal := val + " " + strings.TrimSpace(toks[startIdx+1])
		if ms, ok := timestamps.ParseFlexibleTime(combinedVal, now, loc); ok {
			return 2, ms
		}
	}

	// Nothing worked
	return 0, 0
}

// splitRespectingQuotesForTimeFilters splits by whitespace outside quotes
func (qe *QueryExecutor) splitRespectingQuotesForTimeFilters(s string) []string {
	var out []string
	var cur strings.Builder
	inQuote := rune(0)
	for _, r := range s {
		if r == '"' || r == '\'' {
			if inQuote == 0 {
				inQuote = r
			} else if inQuote == r {
				inQuote = 0
			}
			cur.WriteRune(r)
			continue
		}
		if inQuote == 0 && (unicode.IsSpace(r) || r == '|') {
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
			continue
		}
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// getDisplayTimezone returns the user's display timezone setting
func (qe *QueryExecutor) getDisplayTimezone() *time.Location {
	if qe.timezone != nil {
		return qe.timezone
	}
	return time.Local // Fallback to local timezone
}

// isTimestampColumn checks if a column name corresponds to the detected timestamp column
func (qe *QueryExecutor) isTimestampColumn(columnName string, header []string) bool {
	// Detect the timestamp column index
	timestampIdx := timestamps.DetectTimestampIndex(header)
	if timestampIdx < 0 || timestampIdx >= len(header) {
		return false
	}

	// Normalize the detected timestamp column name
	detectedTimestampName := normalizeHeaderName(header, timestampIdx)

	// Normalize the input column name
	normalizedColumnName := strings.ToLower(strings.Trim(strings.TrimSpace(columnName), `"`))

	// Check if they match
	return detectedTimestampName == normalizedColumnName
}

// ExecuteQueryInternalWithConfig executes a query with custom cache configuration
func ExecuteQueryInternalWithConfig(ctx context.Context, tab *FileTab, query string, timeField string, workspaceService interface{}, c *cache.Cache, progress ProgressCallback, cacheConfig CacheConfig) (*QueryExecutionResult, error) {
	executor := NewQueryExecutor(c, progress, cacheConfig)
	result, err := executor.ExecuteQuery(ctx, tab, query, timeField, workspaceService)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	return result, nil
}

// ExecuteQueryInternalWithConfigAndTimezone executes a query with custom cache configuration and timezone
func ExecuteQueryInternalWithConfigAndTimezone(ctx context.Context, tab *FileTab, query string, timeField string, workspaceService interface{}, c *cache.Cache, progress ProgressCallback, cacheConfig CacheConfig, timezone *time.Location) (*QueryExecutionResult, error) {
	executor := NewQueryExecutorWithTimezone(c, progress, cacheConfig, timezone)
	result, err := executor.ExecuteQuery(ctx, tab, query, timeField, workspaceService)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	return result, nil
}

// ExecuteQueryInternalWithSettings executes a query with all settings including sortByTime
func ExecuteQueryInternalWithSettings(ctx context.Context, tab *FileTab, query string, timeField string, workspaceService interface{}, c *cache.Cache, progress ProgressCallback, cacheConfig CacheConfig, timezone *time.Location, ingestTimezone *time.Location, sortByTime bool, sortDescending bool) (*QueryExecutionResult, error) {
	executor := NewQueryExecutorWithSettings(c, progress, cacheConfig, timezone, ingestTimezone, sortByTime, sortDescending)
	result, err := executor.ExecuteQuery(ctx, tab, query, timeField, workspaceService)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	return result, nil
}
