package app

import (
	"fmt"
	"io"
	"strings"

	"github.com/ohler55/ojg/jp"
	"github.com/ohler55/ojg/oj"
	clipboard "golang.design/x/clipboard"
)

// Maximum clipboard size in bytes (10MB) - helps avoid X11 BadLength errors on Linux
const maxClipboardSize = 10 * 1024 * 1024

// safeClipboardWrite attempts to write data to clipboard with panic recovery.
// Returns an error if the write fails or data is too large.
func safeClipboardWrite(format clipboard.Format, data []byte) (err error) {
	// Check size limit
	if len(data) > maxClipboardSize {
		return fmt.Errorf("data too large for clipboard (%d bytes, max %d bytes / %.1f MB). Try selecting fewer rows",
			len(data), maxClipboardSize, float64(maxClipboardSize)/(1024*1024))
	}

	// Use defer/recover to catch panics from clipboard operations
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("clipboard write failed: %v", r)
		}
	}()

	clipboard.Write(format, data)
	return nil
}

// applyJPathToCell applies a JPath expression to a cell value.
// If the value is valid JSON and the expression extracts data, returns the extracted value.
// Otherwise, returns the original value.
func applyJPathToCell(cellValue string, expression string) string {
	if cellValue == "" || expression == "" || expression == "$" {
		return cellValue
	}

	// Try to parse cell value as JSON
	data, err := oj.ParseString(cellValue)
	if err != nil {
		// Not valid JSON, return original
		return cellValue
	}

	// Parse the JPath expression
	x, err := jp.ParseString(expression)
	if err != nil {
		// Invalid expression, return original
		return cellValue
	}

	// Apply the expression
	results := x.Get(data)
	if len(results) == 0 {
		return ""
	}

	// Match frontend behavior with jsonpath-plus wrap: false
	// - If there's a single result, return it directly
	// - If there are multiple results, return them as a JSON array
	var result interface{}
	if len(results) == 1 {
		result = results[0]
	} else {
		// Multiple results - return as array (matching frontend behavior)
		result = results
	}

	if result == nil {
		return ""
	}

	// Convert result to string
	switch v := result.(type) {
	case string:
		return v
	case map[string]interface{}, []interface{}:
		// Object or array - JSON stringify it
		jsonBytes, err := oj.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", result)
		}
		return string(jsonBytes)
	default:
		return fmt.Sprintf("%v", result)
	}
}

// copySelectionToClipboardForTab copies selected rows from a specific tab to clipboard
func (a *App) copySelectionToClipboardForTab(tab *FileTab, req CopySelectionRequest) (*CopySelectionResult, error) {
	if a == nil {
		return nil, fmt.Errorf("app not initialised")
	}
	if tab == nil || tab.FilePath == "" {
		return &CopySelectionResult{RowsCopied: 0}, nil
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
		return nil, fmt.Errorf("clipboard not available")
	}

	// Open CSV to read the header for field index mapping
	reader, f, err := a.getReaderForTab(tab)
	if err != nil {
		return nil, err
	}
	if f != nil {
		defer f.Close()
	}
	header, err := reader.Read()
	if err != nil {
		if err == io.EOF {
			return &CopySelectionResult{RowsCopied: 0}, nil
		}
		return nil, err
	}

	effectiveQuery := strings.TrimSpace(req.Query)

	var outHeaders []string
	var fieldReorderMap []int // Maps output column index to query result column index
	var fieldReorderMapBuilt bool

	// Check if frontend explicitly specified column order via req.Fields
	// This happens when PinTimestampColumn is enabled or columns are reordered in UI
	// Frontend column order takes priority over query-based column ordering to ensure
	// copied data matches the displayed grid column order (including pinned timestamp)
	frontendColumnOrder := len(req.Fields) > 0
	if frontendColumnOrder {
		// Use req.Headers for output if available, otherwise use req.Fields
		if len(req.Headers) == len(req.Fields) {
			outHeaders = req.Headers
		} else {
			outHeaders = req.Fields
		}
		// Note: fieldReorderMap will be built lazily on first data fetch
		// using the actual query result header (not the file header)
	} else {
		// Fallback: use file header order
		outHeaders = append(outHeaders, header...)
	}

	// Helper to build fieldReorderMap from query result header
	// This maps frontend display order -> query result column indices
	buildFieldReorderMap := func(queryResultHeader []string) {
		if fieldReorderMapBuilt || !frontendColumnOrder {
			return
		}
		// Build index map from query result header
		queryIdxMap := make(map[string]int, len(queryResultHeader))
		for i, h := range queryResultHeader {
			queryIdxMap[strings.ToLower(strings.TrimSpace(h))] = i
		}
		// Map frontend fields to query result indices
		fieldReorderMap = make([]int, len(req.Fields))
		for i, field := range req.Fields {
			fieldLower := strings.ToLower(strings.TrimSpace(field))
			if idx, ok := queryIdxMap[fieldLower]; ok {
				fieldReorderMap[i] = idx
			} else {
				fieldReorderMap[i] = -1 // Field not found in query result
			}
		}
		fieldReorderMapBuilt = true
	}

	// Build column index to JPath expression map
	// This allows us to apply JPath transformations to specific columns when copying
	colJPathMap := make(map[int]string)
	if len(req.ColumnJPathExpressions) > 0 {
		for i, colName := range outHeaders {
			if expr, ok := req.ColumnJPathExpressions[colName]; ok && expr != "" && expr != "$" {
				colJPathMap[i] = expr
			}
		}
	}
	hasJPathTransforms := len(colJPathMap) > 0

	sanitize := func(s string) string {
		ss := strings.ReplaceAll(s, "\t", " ")
		ss = strings.ReplaceAll(ss, "\r", " ")
		ss = strings.ReplaceAll(ss, "\n", " ")
		return ss
	}

	var b strings.Builder
	if len(outHeaders) > 0 {
		for i, h := range outHeaders {
			if i > 0 {
				b.WriteByte('\t')
			}
			b.WriteString(sanitize(h))
		}
		b.WriteByte('\n')
	}

	appendPage := func(start, end int) (int, error) {
		if end < start {
			return 0, nil
		}
		unifiedResult, err := a.GetDataAndHistogram(tab.ID, start, end, effectiveQuery, req.TimeField, 300)
		if err != nil {
			return 0, err
		}
		// Build fieldReorderMap on first fetch using query result header
		buildFieldReorderMap(unifiedResult.Header)
		rows := unifiedResult.Rows
		for _, rec := range rows {
			// Use fieldReorderMap if frontend specified column order (e.g., PinTimestampColumn)
			if len(fieldReorderMap) > 0 {
				for i, srcIdx := range fieldReorderMap {
					if i > 0 {
						b.WriteByte('\t')
					}
					var val string
					if srcIdx >= 0 && srcIdx < len(rec) {
						val = rec[srcIdx]
					}
					// Apply JPath transformation if configured for this column
					outVal := val
					if hasJPathTransforms {
						if expr, ok := colJPathMap[i]; ok {
							outVal = applyJPathToCell(val, expr)
						}
					}
					b.WriteString(sanitize(outVal))
				}
			} else {
				for i, val := range rec {
					if i > 0 {
						b.WriteByte('\t')
					}
					// Apply JPath transformation if configured for this column
					outVal := val
					if hasJPathTransforms {
						if expr, ok := colJPathMap[i]; ok {
							outVal = applyJPathToCell(val, expr)
						}
					}
					b.WriteString(sanitize(outVal))
				}
			}
			b.WriteByte('\n')
		}
		return len(rows), nil
	}

	const chunk = 5000
	rowsCopied := 0
	if req.VirtualSelectAll {
		start := 0
		reachedEnd := false
		var total *int
		for !reachedEnd {
			end := start + chunk
			unifiedResult, err := a.GetDataAndHistogram(tab.ID, start, end, effectiveQuery, req.TimeField, 300)
			if err != nil {
				return nil, err
			}
			// Build fieldReorderMap on first fetch using query result header
			buildFieldReorderMap(unifiedResult.Header)
			if total == nil && unifiedResult.Total >= 0 {
				total = &unifiedResult.Total
			}
			rows := unifiedResult.Rows
			for _, rec := range rows {
				// Use fieldReorderMap if frontend specified column order (e.g., PinTimestampColumn)
				if len(fieldReorderMap) > 0 {
					for i, srcIdx := range fieldReorderMap {
						if i > 0 {
							b.WriteByte('\t')
						}
						var val string
						if srcIdx >= 0 && srcIdx < len(rec) {
							val = rec[srcIdx]
						}
						// Apply JPath transformation if configured for this column
						outVal := val
						if hasJPathTransforms {
							if expr, ok := colJPathMap[i]; ok {
								outVal = applyJPathToCell(val, expr)
							}
						}
						b.WriteString(sanitize(outVal))
					}
				} else {
					for i, val := range rec {
						if i > 0 {
							b.WriteByte('\t')
						}
						// Apply JPath transformation if configured for this column
						outVal := val
						if hasJPathTransforms {
							if expr, ok := colJPathMap[i]; ok {
								outVal = applyJPathToCell(val, expr)
							}
						}
						b.WriteString(sanitize(outVal))
					}
				}
				b.WriteByte('\n')
			}
			rowsCopied += len(rows)
			reachedEnd = unifiedResult.ReachedEnd
			if total != nil && rowsCopied >= *total {
				break
			}
			start = end
			if len(rows) == 0 {
				break
			}
		}
	} else if len(req.Ranges) > 0 {
		for _, rng := range req.Ranges {
			start, end := rng.Start, rng.End
			if start < 0 {
				start = 0
			}
			if end <= start {
				continue
			}
			copied, err := appendPage(start, end)
			if err != nil {
				return nil, err
			}
			rowsCopied += copied
		}
	}

	out := b.String()
	outBytes := []byte(out)

	// Use safe clipboard write with size check and panic recovery
	if err := safeClipboardWrite(clipboard.FmtText, outBytes); err != nil {
		a.Log("error", fmt.Sprintf("Clipboard write failed: %v", err))
		return nil, fmt.Errorf("failed to copy to clipboard: %v", err)
	}

	a.Log("info", fmt.Sprintf("Copied %d rows (%d bytes) to clipboard from tab %s", rowsCopied, len(outBytes), tab.ID))
	return &CopySelectionResult{RowsCopied: rowsCopied}, nil
}
