package query

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"breachline/app/interfaces"
	"breachline/app/timestamps"

	"github.com/ohler55/ojg/jp"
	"github.com/ohler55/ojg/oj"
)

// resolvedColumn holds information about a column with optional JPath expression
type resolvedColumn struct {
	index     int    // Column index in the original header
	jpathExpr string // JPath expression to apply, empty if none
}

// parseColumnJPath parses a column name that may contain a JPath expression.
// Returns columnName, jpathExpr, hasJPath.
// Example: "requestParameters{$.durationSeconds}" -> "requestParameters", "$.durationSeconds", true
func parseColumnJPath(colName string) (string, string, bool) {
	openBrace := strings.Index(colName, "{")
	if openBrace == -1 {
		return colName, "", false
	}

	closeBrace := strings.LastIndex(colName, "}")
	if closeBrace == -1 || closeBrace <= openBrace {
		return colName, "", false
	}

	columnName := strings.TrimSpace(colName[:openBrace])
	jpathExpr := strings.TrimSpace(colName[openBrace+1 : closeBrace])

	if columnName == "" || jpathExpr == "" {
		return colName, "", false
	}

	return columnName, jpathExpr, true
}

// evaluateColumnJPath extracts a value from JSON content using a JPath expression
func evaluateColumnJPath(jsonValue string, jpathExpr string) (string, bool) {
	if jsonValue == "" || jpathExpr == "" {
		return "", false
	}

	// Parse JSON from column value
	data, err := oj.ParseString(jsonValue)
	if err != nil {
		return "", false
	}

	// Parse JPath expression
	path, err := jp.ParseString(jpathExpr)
	if err != nil {
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
		jsonBytes, err := oj.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v), true
		}
		return string(jsonBytes), true
	default:
		return fmt.Sprintf("%v", v), true
	}
}

// FilterStage filters rows based on a matcher function
type FilterStage struct {
	matcher         func(*Row) bool
	displayIdx      int
	formatFunc      func(string) (string, bool)
	workspace       interface{} // WorkspaceManager - using interface{} to avoid import cycle
	fileHash        string
	opts            interfaces.FileOptions
	name            string
	filterQuery     string // Store the original filter query for cache key generation
	displayTimezone string // Display timezone used for parsing time filters (for cache key)
}

// NewFilterStage creates a new filter stage
func NewFilterStage(matcher func(*Row) bool, displayIdx int, formatFunc func(string) (string, bool), workspace interface{}, fileHash string, opts interfaces.FileOptions, filterQuery string, displayTimezone string) *FilterStage {
	return &FilterStage{
		matcher:         matcher,
		displayIdx:      displayIdx,
		formatFunc:      formatFunc,
		workspace:       workspace,
		fileHash:        fileHash,
		opts:            opts,
		name:            "filter",
		filterQuery:     filterQuery,
		displayTimezone: displayTimezone,
	}
}

// Execute processes the input data and returns filtered rows with timestamp stats
func (f *FilterStage) Execute(input *StageResult) (*StageResult, error) {
	// Get headers and detect timestamp field
	header := input.Header
	originalHeader := input.OriginalHeader
	if len(originalHeader) == 0 {
		originalHeader = header
	}
	timeFieldIdx := timestamps.DetectTimestampIndex(originalHeader)

	// Work directly with input rows (already pre-parsed)
	rows := input.Rows

	// Initialize timestamp tracking if time field exists
	var timestampStats *TimestampStats
	if timeFieldIdx >= 0 {
		timestampStats = &TimestampStats{
			TimeFieldIdx: timeFieldIdx,
			MinTimestamp: 0,
			MaxTimestamp: 0,
			ValidCount:   0,
		}
	}

	// Process filtering synchronously using pre-parsed Row objects
	var filteredRows []*Row
	for _, row := range rows {
		// Match against row (includes pre-parsed timestamp for timestamp column filtering)
		if f.matcher(row) {
			filteredRows = append(filteredRows, row)

			// Track timestamps using pre-parsed values (no parsing needed!)
			if timestampStats != nil && row.HasTime {
				if timestampStats.ValidCount == 0 || row.Timestamp < timestampStats.MinTimestamp {
					timestampStats.MinTimestamp = row.Timestamp
				}
				if row.Timestamp > timestampStats.MaxTimestamp {
					timestampStats.MaxTimestamp = row.Timestamp
				}
				timestampStats.ValidCount++
			}
		}
	}

	// Return StageResult with timestamp stats
	return &StageResult{
		OriginalHeader: originalHeader,
		Header:         header,
		DisplayColumns: input.DisplayColumns,
		Rows:           filteredRows,
		TimestampStats: timestampStats,
	}, nil
}

// CanCache returns true if this stage can be cached
func (f *FilterStage) CanCache() bool {
	return true
}

// CacheKey returns a unique key for caching
// Includes display timezone to ensure cache invalidation when timezone changes
func (f *FilterStage) CacheKey() string {
	return fmt.Sprintf("filter:%s:%s:%s:dtz:%s", f.fileHash, f.opts.Key(), f.filterQuery, f.displayTimezone)
}

// Name returns the stage name
func (f *FilterStage) Name() string {
	return f.name
}

// EstimateOutputSize estimates output size (filters typically reduce size)
func (f *FilterStage) EstimateOutputSize() float64 {
	return 0.5 // Assume 50% of rows pass filter on average
}

// SortStage sorts rows by specified columns
type SortStage struct {
	columnNames []string // Column names to sort by (resolved at execution time)
	descending  []bool
	timeColumn  string // Time column name for time-based sorting, empty for regular sort
	useExtSort  bool
	name        string
}

// NewSortStage creates a new sort stage
// columnNames: list of column names to sort by
// descending: corresponding sort directions for each column
func NewSortStage(columnNames []string, descending []bool) *SortStage {
	return &SortStage{
		columnNames: columnNames,
		descending:  descending,
		timeColumn:  "",
		useExtSort:  true,
		name:        "sort",
	}
}

// NewTimeSortStage creates a sort stage for timestamp sorting
func NewTimeSortStage(timeColumnName string, desc bool) *SortStage {
	return &SortStage{
		columnNames: []string{timeColumnName},
		descending:  []bool{desc},
		timeColumn:  timeColumnName,
		useExtSort:  true,
		name:        "time_sort",
	}
}

// Execute processes the input data and returns sorted rows with timestamp stats
func (s *SortStage) Execute(input *StageResult) (*StageResult, error) {
	// Get headers and detect timestamp field
	header := input.Header
	originalHeader := input.OriginalHeader
	if len(originalHeader) == 0 {
		originalHeader = header
	}
	displayColumns := input.DisplayColumns
	timeFieldIdx := timestamps.DetectTimestampIndex(originalHeader)

	// CRITICAL: Make a copy of the rows slice to avoid mutating cached data
	// The input rows may come from the cache, and sorting in-place would corrupt
	// cached entries with different sort orders
	rows := make([]*Row, len(input.Rows))
	copy(rows, input.Rows)

	// Resolve column names to indices (with JPath support) based on input header
	resolvedColumns := s.resolveColumnsWithJPath(input)

	// Calculate timestamp stats from all rows using pre-parsed timestamps
	var timestampStats *TimestampStats
	if timeFieldIdx >= 0 {
		timestampStats = &TimestampStats{
			TimeFieldIdx: timeFieldIdx,
			MinTimestamp: 0,
			MaxTimestamp: 0,
			ValidCount:   0,
		}
		for _, row := range rows {
			if row.HasTime {
				if timestampStats.ValidCount == 0 || row.Timestamp < timestampStats.MinTimestamp {
					timestampStats.MinTimestamp = row.Timestamp
				}
				if row.Timestamp > timestampStats.MaxTimestamp {
					timestampStats.MaxTimestamp = row.Timestamp
				}
				timestampStats.ValidCount++
			}
		}
	}

	// Sort the rows
	if s.timeColumn != "" {
		// Use time-based sorting with pre-parsed timestamps
		// We don't need the column index since timestamps are pre-parsed in Row objects
		desc := len(s.descending) > 0 && s.descending[0]
		fmt.Printf("[SORT_EXECUTE_DEBUG] s.descending=%v, desc=%v, timeColumn=%q\n", s.descending, desc, s.timeColumn)
		s.sortRowsByTime(rows, -1, desc) // timeIdx not used in optimized version
	} else {
		// Use column-based sorting with JPath support (pass timeFieldIdx for timestamp handling)
		s.sortRowsByResolvedColumns(rows, resolvedColumns, timeFieldIdx)
	}

	return &StageResult{
		OriginalHeader: originalHeader,
		Header:         header,
		DisplayColumns: displayColumns,
		Rows:           rows,
		TimestampStats: timestampStats,
	}, nil
}

// resolveColumnsWithJPath resolves column names to indices with JPath support
// This method handles display column mapping correctly when sort follows column operations
// and supports JPath expressions like "requestParameters{$.durationSeconds}"
func (s *SortStage) resolveColumnsWithJPath(input *StageResult) []resolvedColumn {
	header := input.Header
	originalHeader := input.OriginalHeader
	displayColumns := input.DisplayColumns

	var resolved []resolvedColumn
	for _, colName := range s.columnNames {
		// Check if this column has a JPath expression
		actualColName, jpathExpr, hasJPath := parseColumnJPath(colName)
		colNameLower := strings.ToLower(strings.TrimSpace(actualColName))
		foundIdx := -1

		// First, try to find the column in the original header
		var originalIndex = -1
		for i := range originalHeader {
			headerNormalized := normalizeHeaderNameDirect(originalHeader, i)
			if headerNormalized == colNameLower {
				originalIndex = i
				foundIdx = originalIndex
				break
			}
		}

		// If found in original header, use that index directly
		if originalIndex >= 0 {
			resolved = append(resolved, resolvedColumn{index: originalIndex, jpathExpr: jpathExpr})
		} else {
			// Fallback: try to find in current display header and map through displayColumns
			for i := range header {
				headerNormalized := normalizeHeaderNameDirect(header, i)
				if headerNormalized == colNameLower {
					if len(displayColumns) > 0 && i < len(displayColumns) {
						// Map through display columns to get original index
						foundIdx = displayColumns[i]
						resolved = append(resolved, resolvedColumn{index: displayColumns[i], jpathExpr: jpathExpr})
					} else {
						// No display mapping, use current index
						foundIdx = i
						resolved = append(resolved, resolvedColumn{index: i, jpathExpr: jpathExpr})
					}
					break
				}
			}
		}

		// Debug logging
		if foundIdx < 0 {
			fmt.Printf("[SORT_DEBUG] Column '%s' NOT FOUND in headers\n", colName)
			fmt.Printf("[SORT_DEBUG] Original header: %v\n", originalHeader)
			fmt.Printf("[SORT_DEBUG] Current header: %v\n", header)
		} else {
			if hasJPath {
				fmt.Printf("[SORT_DEBUG] Column '%s' with JPath '%s' resolved to index %d\n",
					actualColName, jpathExpr, foundIdx)
			} else {
				fmt.Printf("[SORT_DEBUG] Column '%s' resolved to index %d (header[%d]='%s')\n",
					colName, foundIdx, foundIdx, originalHeader[foundIdx])
			}
		}
	}

	fmt.Printf("[SORT_DEBUG] Final resolved columns: %v for columns: %v\n", resolved, s.columnNames)
	return resolved
}

// sortRowsByTime sorts rows by timestamp using pre-parsed timestamps
func (s *SortStage) sortRowsByTime(rows []*Row, timeIdx int, desc bool) {
	if len(rows) == 0 {
		return
	}

	// Debug: show first few timestamps before sort
	if len(rows) > 3 {
		fmt.Printf("[SORT_BEFORE] First 3 timestamps: %d, %d, %d (desc=%v)\n",
			rows[0].Timestamp, rows[1].Timestamp, rows[2].Timestamp, desc)
	}

	// Optimized O(n log n) sort using Go's sort.Slice
	// Note: timeIdx parameter is unused - we use pre-parsed Row.Timestamp values
	sort.Slice(rows, func(i, j int) bool {
		iHasTime := rows[i].HasTime
		jHasTime := rows[j].HasTime

		// Handle missing timestamps - missing values go to end regardless of sort direction
		if !iHasTime && !jHasTime {
			return false // Both missing, maintain relative order
		}
		if !iHasTime {
			return false // i missing, j has time - i goes to end
		}
		if !jHasTime {
			return true // i has time, j missing - j goes to end
		}

		// Both have timestamps - compare based on sort direction
		if desc {
			return rows[i].Timestamp > rows[j].Timestamp // Descending: newer first
		}
		return rows[i].Timestamp < rows[j].Timestamp // Ascending: older first
	})

	// Debug: show first few timestamps after sort
	if len(rows) > 3 {
		fmt.Printf("[SORT_AFTER] First 3 timestamps: %d, %d, %d (desc=%v)\n",
			rows[0].Timestamp, rows[1].Timestamp, rows[2].Timestamp, desc)
	}
}

// sortRowsByResolvedColumns sorts rows by multiple columns with JPath support
func (s *SortStage) sortRowsByResolvedColumns(rows []*Row, columns []resolvedColumn, timeFieldIdx int) {
	if len(columns) == 0 || len(rows) == 0 {
		return
	}

	// Ensure descending slice matches columns length
	descending := s.descending
	if len(descending) < len(columns) {
		for len(descending) < len(columns) {
			descending = append(descending, false)
		}
	}

	// Optimized O(n log n) multi-column sort using Go's sort.Slice
	sort.Slice(rows, func(i, j int) bool {
		// Compare each column in order until difference found
		for k, col := range columns {
			colIdx := col.index
			jpathExpr := col.jpathExpr

			if colIdx < 0 {
				continue
			}
			desc := descending[k]

			// Check if this column is the timestamp column (only when no JPath expression)
			isTimestampColumn := (timeFieldIdx >= 0 && colIdx == timeFieldIdx && jpathExpr == "")

			if isTimestampColumn {
				// Use pre-parsed timestamps for correct comparison
				iHasTime := rows[i].HasTime
				jHasTime := rows[j].HasTime

				// Handle missing timestamps - missing values go to end
				if !iHasTime && !jHasTime {
					continue // Both missing, check next column
				}
				if !iHasTime {
					return false // i missing, goes to end
				}
				if !jHasTime {
					return true // j missing, goes to end
				}

				// Compare timestamps
				var cmp int
				if rows[i].Timestamp < rows[j].Timestamp {
					cmp = -1
				} else if rows[i].Timestamp > rows[j].Timestamp {
					cmp = 1
				} else {
					cmp = 0
				}

				// Apply direction and return result if values differ
				if cmp != 0 {
					if desc {
						return cmp > 0 // Descending: newer first
					}
					return cmp < 0 // Ascending: older first
				}
				// Timestamps equal, continue to next column
			} else {
				// Regular column (with optional JPath) - use string/numeric comparison
				var aVal, bVal string
				var aEmpty, bEmpty bool

				// Get values from row data
				if colIdx >= len(rows[i].Data) || strings.TrimSpace(rows[i].Data[colIdx]) == "" {
					aEmpty = true
				} else {
					aVal = rows[i].Data[colIdx]
					// If JPath expression, evaluate it
					if jpathExpr != "" {
						extractedVal, ok := evaluateColumnJPath(aVal, jpathExpr)
						if ok {
							aVal = extractedVal
							aEmpty = (strings.TrimSpace(aVal) == "")
						} else {
							aEmpty = true // JPath evaluation failed, treat as empty
						}
					}
				}

				if colIdx >= len(rows[j].Data) || strings.TrimSpace(rows[j].Data[colIdx]) == "" {
					bEmpty = true
				} else {
					bVal = rows[j].Data[colIdx]
					// If JPath expression, evaluate it
					if jpathExpr != "" {
						extractedVal, ok := evaluateColumnJPath(bVal, jpathExpr)
						if ok {
							bVal = extractedVal
							bEmpty = (strings.TrimSpace(bVal) == "")
						} else {
							bEmpty = true // JPath evaluation failed, treat as empty
						}
					}
				}

				// Empty value handling - empty values go to end regardless of sort direction
				if aEmpty && bEmpty {
					continue // Both empty, check next column
				}
				if aEmpty {
					return false // i empty, goes to end
				}
				if bEmpty {
					return true // j empty, goes to end
				}

				// Try numeric comparison
				aNum, aNumOk := s.parseNumeric(aVal)
				bNum, bNumOk := s.parseNumeric(bVal)

				var cmp int
				if aNumOk && bNumOk {
					// Numeric comparison
					if aNum < bNum {
						cmp = -1
					} else if aNum > bNum {
						cmp = 1
					} else {
						cmp = 0
					}
				} else {
					// String comparison (case-insensitive)
					aLower := strings.ToLower(aVal)
					bLower := strings.ToLower(bVal)
					if aLower < bLower {
						cmp = -1
					} else if aLower > bLower {
						cmp = 1
					} else {
						cmp = 0
					}
				}

				// Apply direction and return result if values differ
				if cmp != 0 {
					if desc {
						return cmp > 0 // Descending: larger values first
					}
					return cmp < 0 // Ascending: smaller values first
				}
				// Values equal, continue to next column
			}
		}
		return false // All columns equal, maintain relative order
	})
}

// parseNumeric attempts to parse a string as a float64
func (s *SortStage) parseNumeric(s_val string) (float64, bool) {
	s_val = strings.TrimSpace(s_val)
	if s_val == "" {
		return 0, false
	}
	// Try to parse as float
	if val, err := strconv.ParseFloat(s_val, 64); err == nil {
		return val, true
	}
	return 0, false
}

// normalizeHeaderNameDirect converts empty headers to "unnamed_a", "unnamed_b", etc.
// and returns the normalized name in lowercase for case-insensitive matching
func normalizeHeaderNameDirect(header []string, index int) string {
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

// CanCache returns true if this stage can be cached
func (s *SortStage) CanCache() bool {
	return true
}

// CacheKey returns a unique key for caching
func (s *SortStage) CacheKey() string {
	colStr := strings.Join(s.columnNames, ",")
	descStr := strings.Join(boolSliceToStringSlice(s.descending), ",")
	key := fmt.Sprintf("sort:cols=%s:desc=%s:time=%s", colStr, descStr, s.timeColumn)
	fmt.Printf("[SORT_CACHE_KEY_DEBUG] SortStage.CacheKey: columns=%v, descending=%v -> key=%s\n", s.columnNames, s.descending, key)
	return key
}

// Name returns the stage name
func (s *SortStage) Name() string {
	return s.name
}

// EstimateOutputSize estimates output size (sorting doesn't change row count)
func (s *SortStage) EstimateOutputSize() float64 {
	return 1.0 // Same number of rows
}

// DedupStage removes duplicate rows based on key columns
type DedupStage struct {
	keyColumnNames []string // Column names to deduplicate on (resolved at execution time)
	name           string
}

// NewDedupStage creates a new deduplication stage
// keyColumnNames: list of column names to deduplicate on, empty means all columns
func NewDedupStage(keyColumnNames []string) *DedupStage {
	return &DedupStage{
		keyColumnNames: keyColumnNames,
		name:           "dedup",
	}
}

// Execute processes the input data and returns deduplicated rows with timestamp stats
func (d *DedupStage) Execute(input *StageResult) (*StageResult, error) {
	// Get headers and detect timestamp field
	header := input.Header
	originalHeader := input.OriginalHeader
	if len(originalHeader) == 0 {
		originalHeader = header
	}
	displayColumns := input.DisplayColumns
	timeFieldIdx := timestamps.DetectTimestampIndex(originalHeader)

	// Work directly with input rows (already pre-parsed)
	rows := input.Rows

	// Resolve column names to indices (with JPath support) based on input header
	resolvedColumns := d.resolveColumnsWithJPath(input)

	// Initialize timestamp tracking
	var timestampStats *TimestampStats
	if timeFieldIdx >= 0 {
		timestampStats = &TimestampStats{
			TimeFieldIdx: timeFieldIdx,
			MinTimestamp: 0,
			MaxTimestamp: 0,
			ValidCount:   0,
		}
	}

	// Process deduplication using pre-parsed Row objects
	seen := make(map[string]bool)
	var deduplicatedRows []*Row

	for _, row := range rows {
		// Create deduplication key from row data (with JPath support)
		key := d.createKeyWithResolvedColumns(row.Data, resolvedColumns)
		if !seen[key] {
			seen[key] = true
			deduplicatedRows = append(deduplicatedRows, row)

			// Track timestamps using pre-parsed values (no parsing needed!)
			if timestampStats != nil && row.HasTime {
				if timestampStats.ValidCount == 0 || row.Timestamp < timestampStats.MinTimestamp {
					timestampStats.MinTimestamp = row.Timestamp
				}
				if row.Timestamp > timestampStats.MaxTimestamp {
					timestampStats.MaxTimestamp = row.Timestamp
				}
				timestampStats.ValidCount++
			}
		}
	}

	// Return StageResult with timestamp stats
	return &StageResult{
		OriginalHeader: originalHeader,
		Header:         header,
		DisplayColumns: displayColumns,
		Rows:           deduplicatedRows,
		TimestampStats: timestampStats,
	}, nil
}

// resolveColumnsWithJPath resolves column names to indices with JPath support
// This method handles display column mapping correctly when dedup follows column operations
// and supports JPath expressions like "requestParameters{$.durationSeconds}"
func (d *DedupStage) resolveColumnsWithJPath(input *StageResult) []resolvedColumn {
	header := input.Header
	originalHeader := input.OriginalHeader
	displayColumns := input.DisplayColumns

	// If no specific columns specified, use all displayed columns (no JPath support for all-columns dedup)
	if len(d.keyColumnNames) == 0 {
		if len(displayColumns) == 0 {
			// No display filtering, use all columns
			resolved := make([]resolvedColumn, len(header))
			for i := range header {
				resolved[i] = resolvedColumn{index: i, jpathExpr: ""}
			}
			return resolved
		} else {
			// Use all displayed columns (map through display columns to get original indices)
			resolved := make([]resolvedColumn, len(displayColumns))
			for i, idx := range displayColumns {
				resolved[i] = resolvedColumn{index: idx, jpathExpr: ""}
			}
			return resolved
		}
	}

	// Resolve each column name to its index in the ORIGINAL header (row data)
	var resolved []resolvedColumn
	for _, colName := range d.keyColumnNames {
		// Check if this column has a JPath expression
		actualColName, jpathExpr, _ := parseColumnJPath(colName)
		colNameLower := strings.ToLower(strings.TrimSpace(actualColName))

		// First, try to find the column in the original header
		var originalIndex = -1
		for i := range originalHeader {
			headerNormalized := normalizeHeaderNameDirect(originalHeader, i)
			if headerNormalized == colNameLower {
				originalIndex = i
				break
			}
		}

		// If found in original header, use that index directly
		if originalIndex >= 0 {
			resolved = append(resolved, resolvedColumn{index: originalIndex, jpathExpr: jpathExpr})
		} else {
			// Fallback: try to find in current display header and map through displayColumns
			for i := range header {
				headerNormalized := normalizeHeaderNameDirect(header, i)
				if headerNormalized == colNameLower {
					if len(displayColumns) > 0 && i < len(displayColumns) {
						// Map through display columns to get original index
						resolved = append(resolved, resolvedColumn{index: displayColumns[i], jpathExpr: jpathExpr})
					} else {
						// No display mapping, use current index
						resolved = append(resolved, resolvedColumn{index: i, jpathExpr: jpathExpr})
					}
					break
				}
			}
		}
	}

	return resolved
}

// createKeyWithResolvedColumns creates a unique key from the specified columns with JPath support
func (d *DedupStage) createKeyWithResolvedColumns(row []string, columns []resolvedColumn) string {
	var keyParts []string

	for _, col := range columns {
		colIdx := col.index
		jpathExpr := col.jpathExpr

		if colIdx >= 0 && colIdx < len(row) {
			value := row[colIdx]

			// If JPath expression, evaluate it
			if jpathExpr != "" {
				extractedVal, ok := evaluateColumnJPath(value, jpathExpr)
				if ok {
					value = extractedVal
				} else {
					value = "" // JPath evaluation failed, use empty string
				}
			}

			// Make dedup case-insensitive (consistent with filter operations)
			value = strings.ToLower(strings.TrimSpace(value))

			keyParts = append(keyParts, value)
		}
	}

	return strings.Join(keyParts, "\x00") // Use null separator
}

// CanCache returns true if this stage can be cached
func (d *DedupStage) CanCache() bool {
	return true
}

// CacheKey returns a unique key for caching
func (d *DedupStage) CacheKey() string {
	colStr := strings.Join(d.keyColumnNames, ",")
	return fmt.Sprintf("dedup:cols=%s", colStr)
}

// Name returns the stage name
func (d *DedupStage) Name() string {
	return d.name
}

// EstimateOutputSize estimates output size (dedup typically reduces size)
func (d *DedupStage) EstimateOutputSize() float64 {
	return 0.8 // Assume 80% of rows remain after dedup
}

// LimitStage limits the number of output rows
type LimitStage struct {
	count int
	name  string
}

// NewLimitStage creates a new limit stage
func NewLimitStage(count int) *LimitStage {
	return &LimitStage{
		count: count,
		name:  "limit",
	}
}

// Execute processes the input data and returns limited rows with timestamp stats
func (l *LimitStage) Execute(input *StageResult) (*StageResult, error) {
	// Get headers and detect timestamp field
	header := input.Header
	originalHeader := input.OriginalHeader
	if len(originalHeader) == 0 {
		originalHeader = header
	}
	displayColumns := input.DisplayColumns
	timeFieldIdx := timestamps.DetectTimestampIndex(originalHeader)

	// Work directly with input rows (already pre-parsed)
	rows := input.Rows

	// Apply limit to materialized data
	var limitedRows []*Row
	if l.count > 0 && l.count < len(rows) {
		limitedRows = rows[:l.count]
	} else {
		limitedRows = rows
	}

	// Calculate timestamp stats for limited rows using pre-parsed timestamps
	var timestampStats *TimestampStats
	if timeFieldIdx >= 0 {
		timestampStats = &TimestampStats{
			TimeFieldIdx: timeFieldIdx,
			MinTimestamp: 0,
			MaxTimestamp: 0,
			ValidCount:   0,
		}
		// Track timestamps for the limited rows
		for _, row := range limitedRows {
			if row.HasTime {
				if timestampStats.ValidCount == 0 || row.Timestamp < timestampStats.MinTimestamp {
					timestampStats.MinTimestamp = row.Timestamp
				}
				if row.Timestamp > timestampStats.MaxTimestamp {
					timestampStats.MaxTimestamp = row.Timestamp
				}
				timestampStats.ValidCount++
			}
		}
	}

	// Return StageResult with timestamp stats
	return &StageResult{
		OriginalHeader: originalHeader,
		Header:         header,
		DisplayColumns: displayColumns,
		Rows:           limitedRows,
		TimestampStats: timestampStats,
	}, nil
}

// CanCache returns true if this stage can be cached
func (l *LimitStage) CanCache() bool {
	return true
}

// CacheKey returns a unique key for caching
func (l *LimitStage) CacheKey() string {
	return fmt.Sprintf("limit:%d", l.count)
}

// Name returns the stage name
func (l *LimitStage) Name() string {
	return l.name
}

// EstimateOutputSize estimates output size based on limit
func (l *LimitStage) EstimateOutputSize() float64 {
	// This is tricky - we don't know input size, so return -1 for unknown
	return -1
}

// StripStage removes empty columns from display (without removing data)
type StripStage struct {
	name string
}

// NewStripStage creates a new strip stage
func NewStripStage() *StripStage {
	return &StripStage{
		name: "strip",
	}
}

// Execute processes the input data and strips empty columns from display with timestamp stats
func (s *StripStage) Execute(input *StageResult) (*StageResult, error) {
	// Get headers and detect timestamp field
	inputHeader := input.Header
	originalHeader := input.OriginalHeader
	if len(originalHeader) == 0 {
		originalHeader = inputHeader
	}
	inputDisplayColumns := input.DisplayColumns
	timeFieldIdx := timestamps.DetectTimestampIndex(originalHeader)

	// Work directly with input rows (already pre-parsed)
	rows := input.Rows

	// Calculate timestamp stats using pre-parsed timestamps
	var timestampStats *TimestampStats
	if timeFieldIdx >= 0 {
		timestampStats = &TimestampStats{
			TimeFieldIdx: timeFieldIdx,
			MinTimestamp: 0,
			MaxTimestamp: 0,
			ValidCount:   0,
		}
		for _, row := range rows {
			if row.HasTime {
				if timestampStats.ValidCount == 0 || row.Timestamp < timestampStats.MinTimestamp {
					timestampStats.MinTimestamp = row.Timestamp
				}
				if row.Timestamp > timestampStats.MaxTimestamp {
					timestampStats.MaxTimestamp = row.Timestamp
				}
				timestampStats.ValidCount++
			}
		}
	}

	// Identify which DISPLAYED columns are empty
	emptyDisplayColumns := s.identifyEmptyDisplayColumns(rows, inputDisplayColumns)

	if len(emptyDisplayColumns) == 0 {
		// No empty columns, pass through unchanged
		return &StageResult{
			OriginalHeader: originalHeader,
			Header:         inputHeader,
			DisplayColumns: inputDisplayColumns,
			Rows:           rows,
			TimestampStats: timestampStats,
		}, nil
	}

	// Build new display header and display columns (excluding empty ones)
	var newDisplayHeader []string
	var newDisplayColumns []int

	if len(inputDisplayColumns) == 0 {
		// Input shows all columns
		for i, headerName := range inputHeader {
			if !emptyDisplayColumns[i] {
				newDisplayHeader = append(newDisplayHeader, headerName)
				newDisplayColumns = append(newDisplayColumns, i)
			}
		}
	} else {
		// Input has filtered columns already
		for i, originalIdx := range inputDisplayColumns {
			if !emptyDisplayColumns[i] {
				newDisplayHeader = append(newDisplayHeader, inputHeader[i])
				newDisplayColumns = append(newDisplayColumns, originalIdx)
			}
		}
	}

	// Handle edge case: all columns stripped
	if len(newDisplayColumns) == 0 {
		// Return empty stream but preserve original header
		return &StageResult{
			OriginalHeader: originalHeader,
			Header:         []string{},
			DisplayColumns: []int{},
			Rows:           rows,
			TimestampStats: timestampStats,
		}, nil
	}

	// Return result with updated display columns (rows unchanged!)
	return &StageResult{
		OriginalHeader: originalHeader,
		Header:         newDisplayHeader,
		DisplayColumns: newDisplayColumns,
		Rows:           rows, // Full rows preserved
		TimestampStats: timestampStats,
	}, nil
}

// identifyEmptyDisplayColumns finds which displayed columns are empty
func (s *StripStage) identifyEmptyDisplayColumns(rows []*Row, displayColumns []int) map[int]bool {
	emptyColumns := make(map[int]bool)

	if len(rows) == 0 {
		return emptyColumns
	}

	// Determine which columns to check
	var columnsToCheck []int
	if len(displayColumns) == 0 {
		// Check all columns in the row
		firstRow := rows[0].Data
		for i := range firstRow {
			columnsToCheck = append(columnsToCheck, i)
		}
	} else {
		// Check only displayed columns
		columnsToCheck = displayColumns
	}

	// Initialize all as empty
	for i := range columnsToCheck {
		emptyColumns[i] = true
	}

	// Check each row
	for _, row := range rows {
		for i, colIdx := range columnsToCheck {
			if colIdx >= 0 && colIdx < len(row.Data) {
				if strings.TrimSpace(row.Data[colIdx]) != "" {
					// Found non-empty value
					delete(emptyColumns, i)
				}
			}
		}

		// Early exit if no columns are empty
		if len(emptyColumns) == 0 {
			break
		}
	}

	return emptyColumns
}

// CanCache returns true if this stage can be cached
func (s *StripStage) CanCache() bool {
	return true
}

// CacheKey returns a unique key for caching
func (s *StripStage) CacheKey() string {
	return "strip"
}

// Name returns the stage name
func (s *StripStage) Name() string {
	return s.name
}

// EstimateOutputSize estimates output size (strip doesn't change row count)
func (s *StripStage) EstimateOutputSize() float64 {
	return 1.0 // Same number of rows, potentially fewer columns
}

// ColumnsStage selects specific columns to display (without removing data)
type ColumnsStage struct {
	columnNames []string // Column names to display (resolved at execution time)
	name        string
}

// NewColumnsStage creates a new columns stage
func NewColumnsStage(columnNames []string) *ColumnsStage {
	return &ColumnsStage{
		columnNames: columnNames,
		name:        "columns",
	}
}

// Execute processes the input data and updates display columns with timestamp stats
func (c *ColumnsStage) Execute(input *StageResult) (*StageResult, error) {
	// Get headers
	inputHeader := input.Header
	originalHeader := input.OriginalHeader
	if len(originalHeader) == 0 {
		// First stage - input header IS the original header
		originalHeader = inputHeader
	}
	timeFieldIdx := timestamps.DetectTimestampIndex(originalHeader)

	// Work directly with input rows (already pre-parsed)
	rows := input.Rows

	// Calculate timestamp stats using pre-parsed timestamps
	var timestampStats *TimestampStats
	if timeFieldIdx >= 0 {
		timestampStats = &TimestampStats{
			TimeFieldIdx: timeFieldIdx,
			MinTimestamp: 0,
			MaxTimestamp: 0,
			ValidCount:   0,
		}
		for _, row := range rows {
			if row.HasTime {
				if timestampStats.ValidCount == 0 || row.Timestamp < timestampStats.MinTimestamp {
					timestampStats.MinTimestamp = row.Timestamp
				}
				if row.Timestamp > timestampStats.MaxTimestamp {
					timestampStats.MaxTimestamp = row.Timestamp
				}
				timestampStats.ValidCount++
			}
		}
	}

	// Resolve column names to indices against the INPUT header
	// Build header lookup (case-insensitive)
	idxMap := make(map[string]int, len(inputHeader))
	for i := range inputHeader {
		key := normalizeHeaderNameDirect(inputHeader, i)
		idxMap[key] = i
	}

	// Find indices for our column names in the input header
	var indices []int
	for _, colName := range c.columnNames {
		if idx, ok := idxMap[strings.ToLower(strings.TrimSpace(colName))]; ok {
			indices = append(indices, idx)
		}
	}

	// Build new display header based on resolved indices
	var displayHeader []string
	for _, idx := range indices {
		if idx >= 0 && idx < len(inputHeader) {
			displayHeader = append(displayHeader, inputHeader[idx])
		}
	}

	// Calculate new display columns relative to ORIGINAL header
	// If input already has display columns, we need to map through them
	inputDisplayColumns := input.DisplayColumns
	var newDisplayColumns []int

	if len(inputDisplayColumns) == 0 {
		// Input shows all columns, so our indices map directly to original
		newDisplayColumns = indices
	} else {
		// Input has filtered display, map our indices through input's display columns
		for _, idx := range indices {
			if idx >= 0 && idx < len(inputDisplayColumns) {
				originalIdx := inputDisplayColumns[idx]
				newDisplayColumns = append(newDisplayColumns, originalIdx)
			}
		}
	}

	// Return result with updated metadata - no data processing needed!
	return &StageResult{
		OriginalHeader: originalHeader,
		Header:         displayHeader,
		DisplayColumns: newDisplayColumns,
		Rows:           rows,
		TimestampStats: timestampStats,
	}, nil
}

// CanCache returns true if this stage can be cached
func (c *ColumnsStage) CanCache() bool {
	return true
}

// CacheKey returns a unique key for caching
func (c *ColumnsStage) CacheKey() string {
	colStr := strings.Join(c.columnNames, ",")
	return fmt.Sprintf("columns:%s", colStr)
}

// Name returns the stage name
func (c *ColumnsStage) Name() string {
	return c.name
}

// EstimateOutputSize estimates output size (columns doesn't change row count)
func (c *ColumnsStage) EstimateOutputSize() float64 {
	return 1.0 // Same number of rows
}

// Utility functions

// boolSliceToStringSlice converts []bool to []string
func boolSliceToStringSlice(bools []bool) []string {
	strs := make([]string, len(bools))
	for i, v := range bools {
		if v {
			strs[i] = "true"
		} else {
			strs[i] = "false"
		}
	}
	return strs
}

// AnnotatedStage filters rows based on annotation status
type AnnotatedStage struct {
	workspace interface{} // WorkspaceManager - using interface{} to avoid import cycle
	fileHash  string
	opts      interfaces.FileOptions
	negated   bool // true for "NOT annotated"
	name      string
}

// NewAnnotatedStage creates a new annotated stage
func NewAnnotatedStage(workspace interface{}, fileHash string, opts interfaces.FileOptions, negated bool) *AnnotatedStage {
	name := "annotated"
	if negated {
		name = "not_annotated"
	}
	return &AnnotatedStage{
		workspace: workspace,
		fileHash:  fileHash,
		opts:      opts,
		negated:   negated,
		name:      name,
	}
}

// Execute processes the input data and returns rows filtered by annotation status with timestamp stats
// OPTIMIZED: Uses batch processing with early-exit for files without annotations
func (a *AnnotatedStage) Execute(input *StageResult) (*StageResult, error) {
	// Get headers and detect timestamp field
	header := input.Header
	originalHeader := input.OriginalHeader
	if len(originalHeader) == 0 {
		originalHeader = header
	}
	displayColumns := input.DisplayColumns
	timeFieldIdx := timestamps.DetectTimestampIndex(originalHeader)

	// Work directly with input rows (already pre-parsed)
	rows := input.Rows

	// Try to get batch annotation interface
	type batchAnnotationChecker interface {
		HasAnnotationsForFile(fileHash string, opts interfaces.FileOptions) bool
		IsRowAnnotatedBatch(fileHash string, opts interfaces.FileOptions, rows []*interfaces.Row, hashKey []byte) []bool
		IsRowAnnotatedBatchWithInfo(fileHash string, opts interfaces.FileOptions, rows []*interfaces.Row, hashKey []byte) []*interfaces.RowAnnotationInfo
		GetHashKey() []byte
	}

	// OPTIMIZATION 4: Early exit if no annotations exist for this file
	if a.workspace != nil && a.fileHash != "" {
		if ws, ok := a.workspace.(batchAnnotationChecker); ok {
			if !ws.HasAnnotationsForFile(a.fileHash, a.opts) {
				// No annotations exist for this file
				if a.negated {
					// "not annotated" - return all rows (all are not annotated)
					return input, nil
				}
				// "annotated" - return empty (none are annotated)
				return &StageResult{
					OriginalHeader: originalHeader,
					Header:         header,
					DisplayColumns: displayColumns,
					Rows:           []*Row{},
					TimestampStats: nil,
				}, nil
			}

			// OPTIMIZATION 2: Use batch processing with parallelization
			hashKey := ws.GetHashKey()
			if hashKey != nil {
				// Convert to interfaces.Row slice for the batch call
				interfaceRows := make([]*interfaces.Row, len(rows))
				for i, row := range rows {
					interfaceRows[i] = &interfaces.Row{
						RowIndex:     row.RowIndex,     // Preserve original row index for annotation matching
						DisplayIndex: row.DisplayIndex, // Preserve display index
						Data:         row.Data,
						Timestamp:    row.Timestamp,
						HasTime:      row.HasTime,
						FirstColHash: row.FirstColHash,
						Annotation:   row.Annotation, // Preserve any existing annotation
					}
				}

				// Batch check all rows in parallel and get full annotation info
				// This populates Row.Annotation for caching
				annotationInfos := ws.IsRowAnnotatedBatchWithInfo(a.fileHash, a.opts, interfaceRows, hashKey)

				// Build boolean results and sync annotation info back to rows
				annotationResults := make([]bool, len(rows))
				for i, info := range annotationInfos {
					annotationResults[i] = info != nil
					if info != nil {
						// Copy annotation info back to the original row for caching
						rows[i].Annotation = info
					}
				}

				// Filter rows based on results
				return a.filterRowsWithResults(rows, annotationResults, header, originalHeader, displayColumns, timeFieldIdx)
			}
		}
	}

	// Fallback to sequential processing if batch interface not available
	return a.executeSequential(input, header, originalHeader, displayColumns, timeFieldIdx)
}

// filterRowsWithResults filters rows based on pre-computed annotation results
func (a *AnnotatedStage) filterRowsWithResults(rows []*Row, annotationResults []bool, header, originalHeader []string, displayColumns []int, timeFieldIdx int) (*StageResult, error) {
	// Initialize timestamp tracking
	var timestampStats *TimestampStats
	if timeFieldIdx >= 0 {
		timestampStats = &TimestampStats{
			TimeFieldIdx: timeFieldIdx,
			MinTimestamp: 0,
			MaxTimestamp: 0,
			ValidCount:   0,
		}
	}

	var filteredRows []*Row
	for i, row := range rows {
		matched := annotationResults[i]

		// Apply negation if needed
		if a.negated {
			matched = !matched
		}

		if matched {
			filteredRows = append(filteredRows, row)

			// Track timestamps using pre-parsed values
			if timestampStats != nil && row.HasTime {
				if timestampStats.ValidCount == 0 || row.Timestamp < timestampStats.MinTimestamp {
					timestampStats.MinTimestamp = row.Timestamp
				}
				if row.Timestamp > timestampStats.MaxTimestamp {
					timestampStats.MaxTimestamp = row.Timestamp
				}
				timestampStats.ValidCount++
			}
		}
	}

	return &StageResult{
		OriginalHeader: originalHeader,
		Header:         header,
		DisplayColumns: displayColumns,
		Rows:           filteredRows,
		TimestampStats: timestampStats,
	}, nil
}

// executeSequential is the fallback sequential processing method
func (a *AnnotatedStage) executeSequential(input *StageResult, header, originalHeader []string, displayColumns []int, timeFieldIdx int) (*StageResult, error) {
	rows := input.Rows

	// Initialize timestamp tracking
	var timestampStats *TimestampStats
	if timeFieldIdx >= 0 {
		timestampStats = &TimestampStats{
			TimeFieldIdx: timeFieldIdx,
			MinTimestamp: 0,
			MaxTimestamp: 0,
			ValidCount:   0,
		}
	}

	// Process filtering synchronously using pre-parsed Row objects
	var filteredRows []*Row
	for _, row := range rows {
		// Check annotation status using row index
		matched := false
		if a.workspace != nil && a.fileHash != "" {
			// Try to cast workspace to the interface we need
			if ws, ok := a.workspace.(interface {
				IsRowAnnotatedByIndex(fileHash string, opts interfaces.FileOptions, rowIndex int) (bool, *interfaces.AnnotationResult)
			}); ok {
				found, result := ws.IsRowAnnotatedByIndex(a.fileHash, a.opts, row.RowIndex)
				matched = found
				// Populate Row.Annotation for caching if annotated
				if found && result != nil {
					row.Annotation = &interfaces.RowAnnotationInfo{
						ID:    result.ID,
						Color: result.Color,
						Note:  result.Note,
					}
				}
			}
		}

		// Apply negation if needed
		if a.negated {
			matched = !matched
		}

		if matched {
			filteredRows = append(filteredRows, row)

			// Track timestamps using pre-parsed values (no parsing needed!)
			if timestampStats != nil && row.HasTime {
				if timestampStats.ValidCount == 0 || row.Timestamp < timestampStats.MinTimestamp {
					timestampStats.MinTimestamp = row.Timestamp
				}
				if row.Timestamp > timestampStats.MaxTimestamp {
					timestampStats.MaxTimestamp = row.Timestamp
				}
				timestampStats.ValidCount++
			}
		}
	}

	// Return StageResult with timestamp stats
	return &StageResult{
		OriginalHeader: originalHeader,
		Header:         header,
		DisplayColumns: displayColumns,
		Rows:           filteredRows,
		TimestampStats: timestampStats,
	}, nil
}

// CanCache returns true if this stage can be cached
func (a *AnnotatedStage) CanCache() bool {
	return true
}

// CacheKey returns a unique key for caching
func (a *AnnotatedStage) CacheKey() string {
	negatedStr := ""
	if a.negated {
		negatedStr = "_negated"
	}
	return fmt.Sprintf("annotated_v4%s:%s:%s", negatedStr, a.fileHash, a.opts.Key())
}

// Name returns the stage name
func (a *AnnotatedStage) Name() string {
	return a.name
}

// EstimateOutputSize estimates output size (annotation filters typically reduce size significantly)
func (a *AnnotatedStage) EstimateOutputSize() float64 {
	if a.negated {
		return 0.9 // Assume 90% of rows are not annotated
	}
	return 0.1 // Assume 10% of rows are annotated
}
