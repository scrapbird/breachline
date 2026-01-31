package fileloader

import (
	"encoding/csv"
	"fmt"
	"os"
	"sync"
	"time"

	"breachline/app/cache"
	"breachline/app/interfaces"
	"breachline/app/timestamps"

	"github.com/ohler55/ojg/oj"
)

// JSONCache interface for caching parsed JSON data using Row-based storage
// This enables efficient sharing of Row pointers between base data and query caches
type JSONCache interface {
	GetBaseData(key string, filePath string) (cache.BaseDataEntry, bool)
	StoreBaseData(key string, filePath string, header []string, rows []*interfaces.Row, timestampStats *interfaces.TimestampStats)
	GetHeader(key string, filePath string) ([]string, bool)
	StoreHeader(key string, filePath string, header []string)
}

// Global cache reference for JSON parsing
var (
	jsonCacheMu sync.RWMutex
	jsonCache   JSONCache
)

// SetJSONCache sets the cache to use for JSON file parsing.
// This should be called during app initialization with the main cache.
func SetJSONCache(c JSONCache) {
	jsonCacheMu.Lock()
	defer jsonCacheMu.Unlock()
	jsonCache = c
}

// getJSONCache returns the current cache (thread-safe)
func getJSONCache() JSONCache {
	jsonCacheMu.RLock()
	defer jsonCacheMu.RUnlock()
	return jsonCache
}

// JSON file reading and ingestion functions
// This file contains all JSON-specific operations for reading headers,
// counting rows, and creating readers for JSON files.

// parseJSONFile reads a JSON file and parses it using oj.Parse.
// This helper function centralizes JSON file reading to avoid duplication.
// It supports both standard JSON and JSON streaming format.
// JSON streaming format allows multiple JSON objects/arrays in a file, separated by whitespace.
// Objects can span multiple lines or appear multiple times on the same line.
// This function automatically handles compressed files (.gz, .bz2, .xz).
func parseJSONFile(filePath string) (interface{}, error) {
	if filePath == "" {
		return nil, fmt.Errorf("file path is empty")
	}

	// Detect if the file is compressed
	_, compression := DetectFileTypeAndCompression(filePath)

	var data []byte
	var err error

	// Handle compressed files
	if compression != CompressionNone {
		result, decompressErr := DecompressFile(filePath, compression)
		if decompressErr != nil {
			return nil, fmt.Errorf("failed to decompress file: %w", decompressErr)
		}
		data = result.Data
	} else {
		// Read uncompressed file directly
		data, err = os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}
	}

	return parseJSONData(data)
}

// parseJSONData parses JSON data from bytes.
// It supports both standard JSON and JSON streaming format.
// This is the core parsing function used by both file-based and in-memory loading.
func parseJSONData(data []byte) (interface{}, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("data is empty")
	}

	// First try to parse as standard JSON
	jsonData, err := oj.Parse(data)
	if err == nil {
		return jsonData, nil
	}

	// If standard JSON parsing fails, try JSON streaming format
	// Extract individual JSON objects/arrays from the stream
	objects, streamErr := parseJSONStream(data)
	if streamErr != nil {
		// If streaming also fails, return the original error
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	if len(objects) == 0 {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Return the collection of objects as an array
	return objects, nil
}

// parseJSONStream extracts multiple JSON values from a byte stream.
// It handles objects and arrays that may span multiple lines or appear multiple times per line.
func parseJSONStream(data []byte) ([]interface{}, error) {
	var objects []interface{}
	str := string(data)
	pos := 0

	for pos < len(str) {
		// Skip whitespace
		for pos < len(str) && (str[pos] == ' ' || str[pos] == '\t' || str[pos] == '\n' || str[pos] == '\r') {
			pos++
		}

		if pos >= len(str) {
			break
		}

		// Find the start of a JSON value
		if str[pos] != '{' && str[pos] != '[' {
			return nil, fmt.Errorf("expected { or [ at position %d", pos)
		}

		// Extract one complete JSON value (object or array)
		start := pos
		end, err := findJSONValueEnd(str, pos)
		if err != nil {
			return nil, err
		}

		// Parse the extracted JSON value
		jsonStr := str[start:end]
		obj, err := oj.ParseString(jsonStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse JSON at position %d: %w", start, err)
		}
		objects = append(objects, obj)
		pos = end
	}

	return objects, nil
}

// findJSONValueEnd finds the end position of a JSON value (object or array) starting at pos.
// It properly handles nested objects/arrays and strings with escape sequences.
func findJSONValueEnd(str string, pos int) (int, error) {
	if pos >= len(str) {
		return 0, fmt.Errorf("unexpected end of input")
	}

	var stack []rune
	inString := false
	escaped := false

	for i := pos; i < len(str); i++ {
		ch := rune(str[i])

		if escaped {
			escaped = false
			continue
		}

		if inString {
			if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inString = false
			}
			continue
		}

		switch ch {
		case '"':
			inString = true
		case '{', '[':
			stack = append(stack, ch)
		case '}':
			if len(stack) == 0 || stack[len(stack)-1] != '{' {
				return 0, fmt.Errorf("unmatched } at position %d", i)
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return i + 1, nil
			}
		case ']':
			if len(stack) == 0 || stack[len(stack)-1] != '[' {
				return 0, fmt.Errorf("unmatched ] at position %d", i)
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return i + 1, nil
			}
		}
	}

	if len(stack) > 0 {
		return 0, fmt.Errorf("unclosed JSON value")
	}

	return len(str), nil
}

// ReadJSONHeader reads a JSON file and extracts headers using a JSONPath expression.
// If expression is empty, it returns an error.
// Uses Row-based caching to avoid re-parsing large JSON files.
// Note: This function uses nil for timezone, which defaults to "default" in cache keys.
// Use ReadJSONHeaderWithTimezone for consistent cache keys with query execution.
func ReadJSONHeader(filePath string, expression string) ([]string, error) {
	header, _, _, err := GetOrParseJSONAsRows(filePath, expression, -1, nil) // -1 for auto-detect, nil for default timezone
	if err != nil {
		return nil, err
	}
	return header, nil
}

// ReadJSONHeaderWithTimezone reads a JSON file and extracts headers using a JSONPath expression.
// The ingestTz parameter ensures consistent cache keys with query execution.
// Uses Row-based caching to avoid re-parsing large JSON files.
func ReadJSONHeaderWithTimezone(filePath string, expression string, ingestTz *time.Location) ([]string, error) {
	header, _, _, err := GetOrParseJSONAsRows(filePath, expression, -1, ingestTz)
	if err != nil {
		return nil, err
	}
	return header, nil
}

// GetJSONRowCount returns the number of data rows from a JSON file using a JSONPath expression.
// The count excludes the header row.
// Uses Row-based caching to avoid re-parsing large JSON files.
func GetJSONRowCount(filePath string, expression string) (int, error) {
	_, rows, _, err := GetOrParseJSONAsRows(filePath, expression, -1, nil) // -1 for auto-detect, nil for default timezone
	if err != nil {
		return 0, err
	}
	return len(rows), nil
}

// GetJSONRowCountWithTimezone returns the number of data rows from a JSON file using a JSONPath expression.
// The ingestTz parameter ensures consistent cache keys with query execution.
// Uses Row-based caching to avoid re-parsing large JSON files.
func GetJSONRowCountWithTimezone(filePath string, expression string, ingestTz *time.Location) (int, error) {
	_, rows, _, err := GetOrParseJSONAsRows(filePath, expression, -1, ingestTz)
	if err != nil {
		return 0, err
	}
	return len(rows), nil
}

// GetJSONReader returns a CSV reader that reads JSON data converted to CSV format.
// The JSONPath expression is used to extract the data from the JSON file.
// Uses Row-based caching to avoid re-parsing large JSON files.
func GetJSONReader(filePath string, expression string) (*csv.Reader, *os.File, error) {
	header, rows, _, err := GetOrParseJSONAsRows(filePath, expression, -1, nil) // -1 for auto-detect, nil for default timezone
	if err != nil {
		return nil, nil, err
	}

	// Convert Row objects back to [][]string for CSV reader compatibility
	// Include header as first row
	stringRows := make([][]string, len(rows)+1)
	stringRows[0] = header
	for i, row := range rows {
		stringRows[i+1] = row.Data
	}

	// Use SliceReader to convert rows to CSV format on-the-fly
	sliceReader := NewSliceReader(stringRows)
	reader := csv.NewReader(sliceReader)
	// Allow variable number of fields per record to handle corrupted files
	reader.FieldsPerRecord = -1
	return reader, nil, nil
}

// buildBaseDataCacheKey creates a cache key for base file data (Row-based)
// Includes timeIdx and effective ingest timezone so changing either invalidates the cache
func buildBaseDataCacheKey(filePath, jpath string, timeIdx int, ingestTz *time.Location) string {
	// Use provided timezone for cache key (includes per-file overrides)
	// IMPORTANT: If nil, use the default ingest timezone from settings to ensure consistent keys
	effectiveTz := ingestTz
	if effectiveTz == nil {
		effectiveTz = timestamps.GetDefaultIngestTimezone()
	}
	tzKey := effectiveTz.String()
	return fmt.Sprintf("basedata:%s::%s::time:%d::tz:%s", filePath, jpath, timeIdx, tzKey)
}

// buildHeaderCacheKey creates a cache key for header-only data (timeIdx-independent)
func buildHeaderCacheKey(filePath, jpath string) string {
	return fmt.Sprintf("header:%s::%s", filePath, jpath)
}

// GetOrParseJSONAsRows retrieves cached base file data or parses the file, converts to Row objects,
// and caches the result. This is the preferred method for JSON loading as it enables efficient
// sharing of Row pointers between the base data cache and query result caches.
//
// timeIdx: the index of the timestamp column to use for parsing. Use -1 for auto-detection.
// ingestTz: the effective ingest timezone for parsing timestamps (includes per-file override)
// Returns: header, rows with pre-parsed timestamps, timestamp stats, error
func GetOrParseJSONAsRows(filePath string, expression string, timeIdx int, ingestTz *time.Location) ([]string, []*interfaces.Row, *interfaces.TimestampStats, error) {
	if expression == "" {
		return nil, nil, nil, fmt.Errorf("JSONPath expression is required for JSON files")
	}

	cache := getJSONCache()

	// If we have a specific timeIdx (not auto-detect), check cache first
	if timeIdx >= 0 {
		cacheKey := buildBaseDataCacheKey(filePath, expression, timeIdx, ingestTz)
		if cache != nil {
			if entry, found := cache.GetBaseData(cacheKey, filePath); found {
				// Cache hit - return Row pointers directly (no copying!)
				return entry.GetHeader(), entry.GetRows(), entry.GetTimestampStats(), nil
			}
		}
	}

	// OPTIMIZATION: For auto-detect case (timeIdx=-1), check header cache first
	// This avoids parsing the entire file just to get the header for timestamp column detection
	if timeIdx < 0 && cache != nil {
		headerCacheKey := buildHeaderCacheKey(filePath, expression)
		if cachedHeader, found := cache.GetHeader(headerCacheKey, filePath); found {
			// We have the header cached - use it to compute effectiveTimeIdx
			effectiveTimeIdx := timestamps.DetectTimestampIndex(cachedHeader)
			// Now check base data cache with the computed timeIdx
			cacheKey := buildBaseDataCacheKey(filePath, expression, effectiveTimeIdx, ingestTz)
			if entry, found := cache.GetBaseData(cacheKey, filePath); found {
				// Cache hit - return Row pointers directly (no copying!)
				return entry.GetHeader(), entry.GetRows(), entry.GetTimestampStats(), nil
			}
		}
	}

	// Cache miss - parse the file
	jsonData, err := parseJSONFile(filePath)
	if err != nil {
		return nil, nil, nil, err
	}

	stringRows, err := ApplyJSONPath(jsonData, expression)
	if err != nil {
		return nil, nil, nil, err
	}

	if len(stringRows) == 0 {
		return nil, nil, nil, fmt.Errorf("no rows returned from JSONPath expression")
	}

	// Extract header (first row)
	header := stringRows[0]

	// Store header in cache for future auto-detect calls
	if cache != nil {
		headerCacheKey := buildHeaderCacheKey(filePath, expression)
		cache.StoreHeader(headerCacheKey, filePath, header)
	}

	// Resolve timeIdx: use provided value or auto-detect
	effectiveTimeIdx := timeIdx
	if effectiveTimeIdx < 0 {
		effectiveTimeIdx = timestamps.DetectTimestampIndex(header)
	}

	// Check if we already have base data cached with the resolved timeIdx
	// (This handles the case where header was cached but base data wasn't)
	if cache != nil {
		cacheKey := buildBaseDataCacheKey(filePath, expression, effectiveTimeIdx, ingestTz)
		if entry, found := cache.GetBaseData(cacheKey, filePath); found {
			// Cache hit - return Row pointers directly (no copying!)
			return entry.GetHeader(), entry.GetRows(), entry.GetTimestampStats(), nil
		}
	}

	// Cache miss - build rows from already-parsed data
	timeFieldIdx := effectiveTimeIdx

	// Convert string rows to Row objects with pre-parsed timestamps (ONCE)
	rows := make([]*interfaces.Row, 0, len(stringRows)-1)
	var timestampStats *interfaces.TimestampStats

	if timeFieldIdx >= 0 {
		timestampStats = &interfaces.TimestampStats{
			TimeFieldIdx: timeFieldIdx,
			MinTimestamp: 0,
			MaxTimestamp: 0,
			ValidCount:   0,
		}
	}

	for i := 1; i < len(stringRows); i++ {
		row := &interfaces.Row{
			RowIndex:     i - 1, // 0-based index (header is row 0, so first data row is index 0)
			DisplayIndex: -1,    // Will be assigned after query pipeline completes
			Data:         stringRows[i],
		}

		// Parse timestamp if time field exists
		if timeFieldIdx >= 0 && timeFieldIdx < len(stringRows[i]) {
			if ms, ok := timestamps.ParseTimestampMillis(stringRows[i][timeFieldIdx], ingestTz); ok {
				row.Timestamp = ms
				row.HasTime = true

				// Track timestamp stats
				if timestampStats != nil {
					if timestampStats.ValidCount == 0 || ms < timestampStats.MinTimestamp {
						timestampStats.MinTimestamp = ms
					}
					if ms > timestampStats.MaxTimestamp {
						timestampStats.MaxTimestamp = ms
					}
					timestampStats.ValidCount++
				}
			}
		}

		rows = append(rows, row)
	}

	// Store in cache with the resolved timeIdx
	if cache != nil {
		cacheKey := buildBaseDataCacheKey(filePath, expression, effectiveTimeIdx, ingestTz)
		cache.StoreBaseData(cacheKey, filePath, header, rows, timestampStats)
	}

	return header, rows, timestampStats, nil
}

// ========== FromBytes variants for decompressed data ==========

// ReadJSONHeaderFromBytes reads JSON data in memory and extracts headers using a JSONPath expression.
// If expression is empty, it returns an error.
func ReadJSONHeaderFromBytes(data []byte, expression string) ([]string, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("data is empty")
	}
	if expression == "" {
		return nil, fmt.Errorf("JSONPath expression is required for JSON data")
	}

	jsonData, err := parseJSONData(data)
	if err != nil {
		return nil, err
	}

	stringRows, err := ApplyJSONPath(jsonData, expression)
	if err != nil {
		return nil, err
	}

	if len(stringRows) == 0 {
		return nil, fmt.Errorf("no rows returned from JSONPath expression")
	}

	// First row is the header
	return stringRows[0], nil
}

// GetJSONRowCountFromBytes returns the number of data rows from JSON data in memory.
func GetJSONRowCountFromBytes(data []byte, expression string) (int, error) {
	if len(data) == 0 {
		return 0, fmt.Errorf("data is empty")
	}
	if expression == "" {
		return 0, fmt.Errorf("JSONPath expression is required for JSON data")
	}

	jsonData, err := parseJSONData(data)
	if err != nil {
		return 0, err
	}

	stringRows, err := ApplyJSONPath(jsonData, expression)
	if err != nil {
		return 0, err
	}

	if len(stringRows) == 0 {
		return 0, nil
	}

	// Subtract 1 for the header row
	return len(stringRows) - 1, nil
}

// GetJSONReaderFromBytes returns a CSV reader that reads JSON data from memory converted to CSV format.
func GetJSONReaderFromBytes(data []byte, expression string) (*csv.Reader, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("data is empty")
	}
	if expression == "" {
		return nil, fmt.Errorf("JSONPath expression is required for JSON data")
	}

	jsonData, err := parseJSONData(data)
	if err != nil {
		return nil, err
	}

	stringRows, err := ApplyJSONPath(jsonData, expression)
	if err != nil {
		return nil, err
	}

	if len(stringRows) == 0 {
		return nil, fmt.Errorf("no rows returned from JSONPath expression")
	}

	// Use SliceReader to convert rows to CSV format on-the-fly
	sliceReader := NewSliceReader(stringRows)
	reader := csv.NewReader(sliceReader)
	reader.FieldsPerRecord = -1
	return reader, nil
}

// GetOrParseJSONAsRowsFromBytes retrieves base file data from bytes, converts to Row objects.
// This is the in-memory equivalent of GetOrParseJSONAsRows.
// Note: This function does not use caching since the source is in-memory data.
func GetOrParseJSONAsRowsFromBytes(data []byte, expression string, timeIdx int, ingestTz *time.Location) ([]string, []*interfaces.Row, *interfaces.TimestampStats, error) {
	if len(data) == 0 {
		return nil, nil, nil, fmt.Errorf("data is empty")
	}
	if expression == "" {
		return nil, nil, nil, fmt.Errorf("JSONPath expression is required for JSON data")
	}

	jsonData, err := parseJSONData(data)
	if err != nil {
		return nil, nil, nil, err
	}

	stringRows, err := ApplyJSONPath(jsonData, expression)
	if err != nil {
		return nil, nil, nil, err
	}

	if len(stringRows) == 0 {
		return nil, nil, nil, fmt.Errorf("no rows returned from JSONPath expression")
	}

	// Extract header (first row)
	header := stringRows[0]

	// Resolve timeIdx: use provided value or auto-detect
	effectiveTimeIdx := timeIdx
	if effectiveTimeIdx < 0 {
		effectiveTimeIdx = timestamps.DetectTimestampIndex(header)
	}

	timeFieldIdx := effectiveTimeIdx

	// Convert string rows to Row objects with pre-parsed timestamps
	rows := make([]*interfaces.Row, 0, len(stringRows)-1)
	var timestampStats *interfaces.TimestampStats

	if timeFieldIdx >= 0 {
		timestampStats = &interfaces.TimestampStats{
			TimeFieldIdx: timeFieldIdx,
			MinTimestamp: 0,
			MaxTimestamp: 0,
			ValidCount:   0,
		}
	}

	for i := 1; i < len(stringRows); i++ {
		row := &interfaces.Row{
			RowIndex:     i - 1,
			DisplayIndex: -1,
			Data:         stringRows[i],
		}

		// Parse timestamp if time field exists
		if timeFieldIdx >= 0 && timeFieldIdx < len(stringRows[i]) {
			if ms, ok := timestamps.ParseTimestampMillis(stringRows[i][timeFieldIdx], ingestTz); ok {
				row.Timestamp = ms
				row.HasTime = true

				// Track timestamp stats
				if timestampStats != nil {
					if timestampStats.ValidCount == 0 || ms < timestampStats.MinTimestamp {
						timestampStats.MinTimestamp = ms
					}
					if ms > timestampStats.MaxTimestamp {
						timestampStats.MaxTimestamp = ms
					}
					timestampStats.ValidCount++
				}
			}
		}

		rows = append(rows, row)
	}

	return header, rows, timestampStats, nil
}
