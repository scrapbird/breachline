package fileloader

import (
	"strings"
)

// excelColumnName converts a 0-based index to Excel-style column name.
// Examples: 0 -> A, 1 -> B, 25 -> Z, 26 -> AA, 27 -> AB, 701 -> ZZ, 702 -> AAA
func excelColumnName(index int) string {
	result := ""
	index++ // Convert to 1-based for the algorithm

	for index > 0 {
		index-- // Adjust for 0-based letter indexing
		result = string(rune('A'+index%26)) + result
		index /= 26
	}

	return result
}

// NormalizeHeaders replaces empty headers with Excel-style column names (A, B, ..., Z, AA, AB, ...).
// This ensures consistent column naming across all file formats and operations.
//
// This function is used by all file format readers (CSV, XLSX, JSON) to provide
// consistent header normalization throughout the application. Any changes to header
// normalization logic should be made here to ensure synchronization across all
// file reading operations.
//
// Rules:
//   - Empty or whitespace-only headers are replaced
//   - Normalized names follow Excel column naming with prefix: Unnamed_A, Unnamed_B, ..., Unnamed_Z, Unnamed_AA, ...
//   - Non-empty headers are preserved as-is
//
// Example:
//
//	Input:  ["name", "", "age", "  ", "city"]
//	Output: ["name", "Unnamed_A", "age", "Unnamed_B", "city"]
func NormalizeHeaders(header []string) []string {
	normalized := make([]string, len(header))
	emptyCount := 0

	for i, h := range header {
		// Check if header is empty or whitespace-only
		if strings.TrimSpace(h) == "" {
			// Generate Excel-style column name with Unnamed_ prefix
			normalized[i] = "Unnamed_" + excelColumnName(emptyCount)
			emptyCount++
		} else {
			normalized[i] = h
		}
	}

	return normalized
}
