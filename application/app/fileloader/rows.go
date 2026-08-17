package fileloader

import (
	"fmt"
	"time"
)

// Rows-native file access.
//
// The generic reader path (GetReader) hands back a *csv.Reader for every format.
// For JSON and XLSX that means the parsed rows are serialized back into CSV text
// (see SliceReader) only for csv.Reader to split them apart again, so a directory
// of JSON members pays a full serialize-plus-reparse of the entire dataset on top
// of the parse it already did. The functions here return the parsed rows directly
// from the same Row-based base-data cache, skipping that round trip entirely.

// SupportsRowsNative reports whether a file's rows can be obtained directly from
// the Row-based base-data cache rather than through a csv.Reader. True for JSON
// and XLSX (compressed or not), which are whole-file parses anyway; false for CSV
// and plugin members, which stay on the streaming reader path so a single large
// member is not materialized when it does not have to be.
func SupportsRowsNative(filePath string) bool {
	fileType, _ := detectFileTypeAndCompressionCached(filePath)
	return fileType == FileTypeJSON || fileType == FileTypeXLSX
}

// DetectFileTypeForPath returns the inner file type for a path, seeing through any
// compression extension (so "events.json.gz" reports JSON, not CSV). Prefer this
// over DetectFileType, which is extension-only and reports compressed files as CSV.
func DetectFileTypeForPath(filePath string) FileType {
	fileType, _ := detectFileTypeAndCompressionCached(filePath)
	return fileType
}

// GetRowsForFile returns a file's header and all of its data rows, without the
// header row. Only valid for paths where SupportsRowsNative reports true; other
// formats must go through GetReader.
//
// The returned rows alias the base-data cache and must not be mutated by callers.
func GetRowsForFile(filePath string, options FileOptions, ingestTz *time.Location) ([]string, [][]string, error) {
	fileType, _ := detectFileTypeAndCompressionCached(filePath)

	var header []string
	var rows [][]string

	switch fileType {
	case FileTypeJSON:
		if options.JPath == "" {
			return nil, nil, fmt.Errorf("JSONPath expression is required for JSON files")
		}
		h, parsed, _, err := GetOrParseJSONAsRows(filePath, options.JPath, -1, ingestTz)
		if err != nil {
			return nil, nil, err
		}
		header = h
		rows = make([][]string, len(parsed))
		for i, r := range parsed {
			rows[i] = r.Data
		}
	case FileTypeXLSX:
		h, parsed, _, err := GetOrParseXLSXAsRows(filePath, options, -1, ingestTz)
		if err != nil {
			return nil, nil, err
		}
		header = h
		rows = make([][]string, len(parsed))
		for i, r := range parsed {
			rows[i] = r.Data
		}
	default:
		return nil, nil, fmt.Errorf("rows-native access is not supported for %s", filePath)
	}

	return header, rows, nil
}
