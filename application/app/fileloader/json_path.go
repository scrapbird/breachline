package fileloader

import (
	"fmt"
	"sort"

	"github.com/ohler55/ojg/jp"
	"github.com/ohler55/ojg/oj"
)

// JSONPath processing and preview utilities
// This file contains functions for applying JSONPath expressions
// and previewing JSON data structures.

// PreviewJSONWithExpression applies a JSONPath expression to a JSON file and returns preview data.
// This function handles all the logic for generating a preview of JSON data with a JSONPath expression.
func PreviewJSONWithExpression(filePath string, expression string, maxRows int) *JSONPreviewResult {
	if filePath == "" {
		return &JSONPreviewResult{Error: "File path is empty"}
	}

	if expression == "" {
		return &JSONPreviewResult{Error: "JSONPath expression is required"}
	}

	// Parse the JSON file
	jsonData, err := parseJSONFile(filePath)
	if err != nil {
		return &JSONPreviewResult{Error: fmt.Sprintf("Failed to read or parse file: %v", err)}
	}

	// Parse the JSONPath expression
	x, err := jp.ParseString(expression)
	if err != nil {
		return &JSONPreviewResult{Error: fmt.Sprintf("Invalid JSONPath expression: %v", err)}
	}

	// Apply the expression to the data
	results := x.Get(jsonData)
	if len(results) == 0 {
		return &JSONPreviewResult{Error: "JSONPath expression returned no results"}
	}

	// Get the first result
	result := results[0]

	// Check if result is an array
	arr, ok := result.([]interface{})
	if !ok {
		// Not an array - check if it's an object and extract keys
		if objMap, isMap := result.(map[string]interface{}); isMap {
			keys := make([]string, 0, len(objMap))
			for key := range objMap {
				keys = append(keys, key)
			}
			return &JSONPreviewResult{
				Error:         "JSONPath expression must return an array. Current result is an object.",
				AvailableKeys: keys,
			}
		}
		// Some other type
		return &JSONPreviewResult{Error: fmt.Sprintf("JSONPath expression must return an array. Current result type: %T", result)}
	}

	if len(arr) == 0 {
		return &JSONPreviewResult{Error: "JSONPath expression returned empty array"}
	}

	// Apply full conversion logic
	rows, err := ApplyJSONPath(jsonData, expression)
	if err != nil {
		return &JSONPreviewResult{Error: err.Error()}
	}

	if len(rows) == 0 {
		return &JSONPreviewResult{Error: "No data returned from JSONPath expression"}
	}

	// Limit the number of rows for preview
	if maxRows <= 0 {
		maxRows = 5
	}

	headers := rows[0]
	previewRows := rows[1:]
	if len(previewRows) > maxRows {
		previewRows = previewRows[:maxRows]
	}

	return &JSONPreviewResult{
		Headers: headers,
		Rows:    previewRows,
		Error:   "",
	}
}

// valueToString converts a value to a string representation.
// If the value is a map or slice, it JSON-stringifies it.
// Otherwise, it converts it to a string using fmt.Sprintf.
func valueToString(val interface{}) string {
	if val == nil {
		return ""
	}

	// Check if the value is a map or slice (object or array in JSON)
	switch v := val.(type) {
	case map[string]interface{}:
		// It's an object - JSON stringify it
		jsonBytes, err := oj.Marshal(v)
		if err != nil {
			// Fallback to fmt.Sprintf if marshaling fails
			return fmt.Sprintf("%v", val)
		}
		return string(jsonBytes)
	case []interface{}:
		// It's an array - JSON stringify it
		jsonBytes, err := oj.Marshal(v)
		if err != nil {
			// Fallback to fmt.Sprintf if marshaling fails
			return fmt.Sprintf("%v", val)
		}
		return string(jsonBytes)
	default:
		// Primitive type - convert to string
		return fmt.Sprintf("%v", val)
	}
}

// ApplyJSONPath applies a JSONPath expression to JSON data and returns the result.
// The expression should return either:
// - An array of objects (dicts): keys from the first object become headers
// - An array of arrays (string[][]): first array is the header row
func ApplyJSONPath(data interface{}, expression string) ([][]string, error) {
	if expression == "" {
		return nil, fmt.Errorf("JSONPath expression is empty")
	}

	// Parse the JSONPath expression
	x, err := jp.ParseString(expression)
	if err != nil {
		return nil, fmt.Errorf("invalid JSONPath expression: %w", err)
	}

	// Apply the expression to the data
	results := x.Get(data)
	if len(results) == 0 {
		return nil, fmt.Errorf("JSONPath expression returned no results")
	}

	// Get the first result (should be an array)
	result := results[0]

	// Check if result is an array
	arr, ok := result.([]interface{})
	if !ok {
		return nil, fmt.Errorf("JSONPath expression must return an array")
	}

	if len(arr) == 0 {
		return nil, fmt.Errorf("JSONPath expression returned empty array")
	}

	// Check if first element is an object (dict) or array
	firstElem := arr[0]

	// Case 1: Array of objects (most common case)
	// Single-pass optimization: collect headers and build rows in one iteration
	if _, ok := firstElem.(map[string]interface{}); ok {
		headerSet := make(map[string]bool)
		headers := make([]string, 0)
		headerIndex := make(map[string]int)

		// Pre-allocate data rows slice (without header row initially)
		dataRows := make([][]string, 0, len(arr))

		for _, item := range arr {
			itemMap, ok := item.(map[string]interface{})
			if !ok {
				continue // Skip non-object items
			}

			// Check for new headers in this object
			newHeadersFound := false
			for key := range itemMap {
				if !headerSet[key] {
					headerSet[key] = true
					headerIndex[key] = len(headers)
					headers = append(headers, key)
					newHeadersFound = true
				}
			}

			// If new headers were found, expand all previous rows
			if newHeadersFound && len(dataRows) > 0 {
				for i := range dataRows {
					if len(dataRows[i]) < len(headers) {
						expanded := make([]string, len(headers))
						copy(expanded, dataRows[i])
						dataRows[i] = expanded
					}
				}
			}

			// Build current row
			row := make([]string, len(headers))
			for key, value := range itemMap {
				if idx, exists := headerIndex[key]; exists {
					if value != nil {
						row[idx] = valueToString(value)
					}
				}
			}
			dataRows = append(dataRows, row)
		}

		// Sort headers alphabetically for consistent ordering and remap rows
		sortedHeaders, sortedRows := sortHeadersAndRemapRows(headers, dataRows)

		// Normalize empty headers to unnamed_a, unnamed_b, etc.
		sortedHeaders = NormalizeHeaders(sortedHeaders)

		// Build final result with header row first
		rows := make([][]string, 0, len(dataRows)+1)
		rows = append(rows, sortedHeaders)
		rows = append(rows, sortedRows...)

		return rows, nil
	}

	// Case 2: Array of arrays
	if _, ok := firstElem.([]interface{}); ok {
		rows := make([][]string, 0, len(arr))
		for rowIdx, item := range arr {
			itemArr, ok := item.([]interface{})
			if !ok {
				continue // Skip non-array items
			}

			row := make([]string, len(itemArr))
			for i, val := range itemArr {
				if val != nil {
					row[i] = valueToString(val)
				} else {
					row[i] = ""
				}
			}

			// Normalize the header row (first row)
			if rowIdx == 0 {
				row = NormalizeHeaders(row)
			}

			rows = append(rows, row)
		}
		return rows, nil
	}

	return nil, fmt.Errorf("JSONPath expression must return an array of objects or an array of arrays")
}

// sortHeadersAndRemapRows sorts headers alphabetically and remaps all row data
// to match the new header order. This is used after single-pass header collection
// to ensure consistent output ordering.
func sortHeadersAndRemapRows(headers []string, rows [][]string) ([]string, [][]string) {
	if len(headers) == 0 {
		return headers, rows
	}

	// Create sorted copy of headers
	sortedHeaders := make([]string, len(headers))
	copy(sortedHeaders, headers)
	sort.Strings(sortedHeaders)

	// Build mapping from old index to new index
	oldToNew := make([]int, len(headers))
	newHeaderIndex := make(map[string]int)
	for i, h := range sortedHeaders {
		newHeaderIndex[h] = i
	}
	for oldIdx, h := range headers {
		oldToNew[oldIdx] = newHeaderIndex[h]
	}

	// Remap all rows
	remappedRows := make([][]string, len(rows))
	for i, row := range rows {
		newRow := make([]string, len(sortedHeaders))
		for oldIdx, value := range row {
			if oldIdx < len(oldToNew) {
				newRow[oldToNew[oldIdx]] = value
			}
		}
		remappedRows[i] = newRow
	}

	return sortedHeaders, remappedRows
}
