# File Ingestion Architecture

## Overview

The file ingestion system has been refactored to provide a centralized abstraction layer for reading data files. This architecture makes it easy to add support for new file formats in the future.

## Current Architecture

### Core Components

All file reading operations are centralized in [ingest.go](../app/ingest.go), which provides three main functions:

1. **`ReadHeader(filePath string)`** - Reads and returns only the header row from a file
2. **`GetRowCount(filePath string)`** - Returns the total number of data rows (excluding header)
3. **`GetReader(filePath string)`** - Returns a reader and file handle for streaming data

### Files Using Ingestion Functions

The following files have been refactored to use the centralized ingestion functions:

- [app_tab_helpers.go](../app/app_tab_helpers.go)
  - `readHeaderForTab()` - Uses `ReadHeader()`
  - `getCSVRowCountForTab()` - Uses `GetRowCount()`

- [app_tab_query.go](../app/app_tab_query.go)
  - `executeQueryForTab()` - Uses `GetReader()`

- [app_timestamp_column.go](../app/app_timestamp_column.go)
  - `ValidateTimestampColumn()` - Uses `GetReader()`

- [app_tab_clipboard.go](../app/app_tab_clipboard.go)
  - `copySelectionToClipboardForTab()` - Uses `GetReader()`

## Adding Support for New File Types

### Step-by-Step Guide

When you're ready to add support for a new file format (e.g., JSON, Parquet, Excel), follow these steps:

#### 1. Update File Type Detection

In [ingest.go](../app/ingest.go):
```go
const (
    FileTypeUnknown FileType = iota
    FileTypeCSV
    FileTypeJSON     // Add new type
)

func DetectFileType(filePath string) FileType {
    // Add detection logic
    if strings.HasSuffix(filePath, ".json") {
        return FileTypeJSON
    }
    // ...
}
```

#### 2. Implement Format-Specific Functions

Create new functions for the format:
```go
// ReadHeaderJSON reads headers from a JSON file
func ReadHeaderJSON(filePath string) ([]string, error) {
    // Implementation for reading JSON field names
}

// GetRowCountJSON counts records in a JSON file
func GetRowCountJSON(filePath string) (int, error) {
    // Implementation for counting JSON records
}

// GetReaderJSON returns a JSON decoder/reader
func GetReaderJSON(filePath string) (*json.Decoder, *os.File, error) {
    // Implementation for JSON streaming
}
```

#### 3. Update Main Functions to Dispatch

Modify the main functions to dispatch based on file type:
```go
func ReadHeader(filePath string) ([]string, error) {
    fileType := DetectFileType(filePath)
    switch fileType {
    case FileTypeCSV:
        return ReadHeaderCSV(filePath)
    case FileTypeJSON:
        return ReadHeaderJSON(filePath)
    default:
        return nil, fmt.Errorf("unsupported file type")
    }
}
```

#### 4. Update File Dialog Filters

In [app_tabs.go](../app/app_tabs.go), update `OpenFileWithDialogTab()`:
```go
Filters: []runtime.FileFilter{
    {DisplayName: "CSV Files", Pattern: "*.csv"},
    {DisplayName: "JSON Files", Pattern: "*.json"},
}
```

#### 5. Consider Data Type Mapping

Different formats may have different data type systems. Consider how to map:
- JSON types (string, number, boolean, object, array) → display format
- Parquet types (various numeric types, timestamps) → display format
- Excel types (text, number, date, formula) → display format

#### 6. Update Query System (if needed)

The current query system assumes tabular data. When adding new formats:
- Ensure the format can be represented as rows and columns
- Update timestamp detection if the format has native timestamp types
- Consider adding format-specific query operators

## Design Principles

1. **Single Responsibility** - Each function has a clear, focused purpose
2. **DRY (Don't Repeat Yourself)** - No duplicate file opening/reading logic
3. **Open for Extension** - Easy to add new formats without modifying existing code
4. **Consistent Interface** - All formats should provide the same basic operations
5. **Error Handling** - Clear error messages that distinguish between file format issues and I/O errors

## Future Considerations

### Potential File Formats to Support

1. **JSON/JSONL** - Popular for log files and APIs
   - Challenge: Nested structures need flattening
   - Benefit: Widely used in web services

2. **Parquet** - Efficient columnar storage
   - Challenge: Requires external library
   - Benefit: Much faster for large datasets

3. **Excel (XLSX)** - Common in business environments
   - Challenge: Complex format with multiple sheets
   - Benefit: Non-technical users prefer Excel

4. **SQLite** - Embedded database files
   - Challenge: Requires SQL query translation
   - Benefit: Efficient querying and indexing

### Performance Optimizations

When adding new formats, consider:
- **Streaming** - Avoid loading entire file into memory
- **Indexing** - Some formats support built-in indexes
- **Compression** - Handle compressed formats (gzip, etc.)
- **Schema Caching** - Cache file schemas to avoid repeated parsing

### Testing Strategy

For each new format:
1. Create sample files in `test_data/` directory
2. Add unit tests for each ingestion function
3. Test edge cases (empty files, malformed data, huge files)
4. Benchmark performance compared to CSV
