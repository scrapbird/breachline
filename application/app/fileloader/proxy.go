package fileloader

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"sync"
	"time"

	"breachline/app/plugin"
	"breachline/app/timestamps"
)

// Format-agnostic dispatcher functions
// This file contains proxy functions that dispatch to the appropriate
// format-specific implementation based on detected file type.
// Compressed files (gzip, bzip2, xz) are automatically decompressed.

// Global storage for decompression warnings (per-file)
var (
	decompressionWarningsMu sync.RWMutex
	decompressionWarnings   = make(map[string]string)
)

// SetDecompressionWarning stores a decompression warning for a file path
func SetDecompressionWarning(filePath, warning string) {
	decompressionWarningsMu.Lock()
	defer decompressionWarningsMu.Unlock()
	decompressionWarnings[filePath] = warning
}

// GetDecompressionWarning retrieves and clears any decompression warning for a file
func GetDecompressionWarning(filePath string) string {
	decompressionWarningsMu.Lock()
	defer decompressionWarningsMu.Unlock()
	warning := decompressionWarnings[filePath]
	delete(decompressionWarnings, filePath)
	return warning
}

// ClearDecompressionWarning clears the decompression warning for a file
func ClearDecompressionWarning(filePath string) {
	decompressionWarningsMu.Lock()
	defer decompressionWarningsMu.Unlock()
	delete(decompressionWarnings, filePath)
}

// ReadHeader reads and returns only the header row from a file using default options.
// This function detects the file type and dispatches to the appropriate handler.
// For JSON files, jpath must be provided. For CSV/XLSX files, jpath is ignored.
// Compressed files are automatically decompressed.
// This is a convenience wrapper around ReadHeaderWithOptions.
func ReadHeader(filePath string, jpath ...string) ([]string, error) {
	jpathStr := ""
	if len(jpath) > 0 {
		jpathStr = jpath[0]
	}
	opts := DefaultFileOptions()
	opts.JPath = jpathStr
	return ReadHeaderWithOptions(filePath, opts, nil)
}

// ReadHeaderWithOptions reads and returns the header row from a file with parsing options.
// This function detects the file type and dispatches to the appropriate handler.
// For JSON files, options.JPath must be provided. For CSV/XLSX files, JPath is ignored.
// If options.NoHeaderRow is true, the first row is treated as data and synthetic headers are generated.
// ingestTz is the effective ingest timezone for JSON timestamp parsing (can be nil for default).
// Compressed files are automatically decompressed.
func ReadHeaderWithOptions(filePath string, options FileOptions, ingestTz *time.Location) ([]string, error) {
	fileType, compression := DetectFileTypeAndCompression(filePath)

	// Handle compressed files
	if compression != CompressionNone {
		result, err := DecompressFile(filePath, compression)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress file: %w", err)
		}

		// Store warning if decompression was incomplete
		if result.Warning != "" {
			SetDecompressionWarning(filePath, result.Warning)
		}

		return readHeaderFromBytes(result.Data, fileType, options, ingestTz)
	}

	// Handle uncompressed files
	switch fileType {
	case FileTypeJSON:
		if options.JPath == "" {
			return nil, fmt.Errorf("JSONPath expression is required for JSON files")
		}
		return ReadJSONHeaderWithTimezone(filePath, options.JPath, ingestTz)
	case FileTypeCSV:
		return ReadCSVHeaderWithOptions(filePath, options)
	case FileTypeXLSX:
		return ReadXLSXHeaderWithOptions(filePath, options)
	case FileTypePlugin:
		return plugin.ReadPluginHeader(context.Background(), filePath, options)
	default:
		return nil, fmt.Errorf("unknown file type for: %s", filePath)
	}
}

// readHeaderFromBytes reads header from decompressed data
func readHeaderFromBytes(data []byte, fileType FileType, options FileOptions, ingestTz *time.Location) ([]string, error) {
	switch fileType {
	case FileTypeJSON:
		if options.JPath == "" {
			return nil, fmt.Errorf("JSONPath expression is required for JSON files")
		}
		return ReadJSONHeaderFromBytes(data, options.JPath)
	case FileTypeCSV:
		return ReadCSVHeaderFromBytes(data, options)
	case FileTypeXLSX:
		return ReadXLSXHeaderFromBytes(data, options)
	default:
		return nil, fmt.Errorf("unknown file type")
	}
}

// GetRowCount returns the total number of data rows in a file using default options.
// Compressed files are automatically decompressed.
// This is a convenience wrapper around GetRowCountWithOptions.
func GetRowCount(filePath string, jpath ...string) (int, error) {
	opts := DefaultFileOptions()
	if len(jpath) > 0 {
		opts.JPath = jpath[0]
	}
	return GetRowCountWithOptions(filePath, opts)
}

// GetRowCountWithOptions returns the total number of data rows in a file with parsing options.
// This function detects the file type and dispatches to the appropriate handler.
// For JSON files, options.JPath must be provided. For CSV/XLSX files, JPath is ignored.
// If options.NoHeaderRow is true, all rows are counted (first row is data, not header).
// Compressed files are automatically decompressed.
func GetRowCountWithOptions(filePath string, options FileOptions) (int, error) {
	fileType, compression := DetectFileTypeAndCompression(filePath)

	// Handle compressed files
	if compression != CompressionNone {
		result, err := DecompressFile(filePath, compression)
		if err != nil {
			return 0, fmt.Errorf("failed to decompress file: %w", err)
		}

		// Store warning if decompression was incomplete
		if result.Warning != "" {
			SetDecompressionWarning(filePath, result.Warning)
		}

		return getRowCountFromBytes(result.Data, fileType, options)
	}

	// Handle uncompressed files
	switch fileType {
	case FileTypeJSON:
		if options.JPath == "" {
			return 0, fmt.Errorf("JSONPath expression is required for JSON files")
		}
		// Use timezone-aware function for consistent cache keys
		ingestTz := timestamps.GetIngestTimezoneWithOverride(options.IngestTimezoneOverride)
		return GetJSONRowCountWithTimezone(filePath, options.JPath, ingestTz)
	case FileTypeCSV:
		return GetCSVRowCountWithOptions(filePath, options)
	case FileTypeXLSX:
		return GetXLSXRowCountWithOptions(filePath, options)
	case FileTypePlugin:
		return plugin.GetPluginRowCount(context.Background(), filePath, options)
	default:
		return 0, fmt.Errorf("unknown file type for: %s", filePath)
	}
}

// getRowCountFromBytes gets row count from decompressed data
func getRowCountFromBytes(data []byte, fileType FileType, options FileOptions) (int, error) {
	switch fileType {
	case FileTypeJSON:
		if options.JPath == "" {
			return 0, fmt.Errorf("JSONPath expression is required for JSON files")
		}
		return GetJSONRowCountFromBytes(data, options.JPath)
	case FileTypeCSV:
		return GetCSVRowCountFromBytes(data, options)
	case FileTypeXLSX:
		return GetXLSXRowCountFromBytes(data, options)
	default:
		return 0, fmt.Errorf("unknown file type")
	}
}

// GetReader returns a reader for the specified file.
// The caller is responsible for closing the returned file handle (may be nil for compressed/XLSX files).
// This function detects the file type and dispatches to the appropriate handler.
// For JSON files, jpath must be provided. For CSV/XLSX files, jpath is ignored.
// Compressed files are automatically decompressed.
func GetReader(filePath string, options FileOptions) (*csv.Reader, *os.File, error) {
	fileType, compression := DetectFileTypeAndCompression(filePath)

	// Handle compressed files
	if compression != CompressionNone {
		result, err := DecompressFile(filePath, compression)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to decompress file: %w", err)
		}

		// Store warning if decompression was incomplete
		if result.Warning != "" {
			SetDecompressionWarning(filePath, result.Warning)
		}

		reader, err := getReaderFromBytes(result.Data, fileType, options.JPath)
		return reader, nil, err
	}

	// Handle uncompressed files
	switch fileType {
	case FileTypeJSON:
		if options.JPath == "" {
			return nil, nil, fmt.Errorf("JSONPath expression is required for JSON files")
		}
		return GetJSONReader(filePath, options.JPath)
	case FileTypeCSV:
		return GetCSVReader(filePath)
	case FileTypeXLSX:
		return GetXLSXReader(filePath)
	case FileTypePlugin:
		reader, _, err := plugin.GetPluginReader(context.Background(), filePath, options)
		if err != nil {
			return nil, nil, err
		}
		// Plugin reader reads from memory buffer, no file handle to return
		// The data is already loaded into the CSV reader
		return reader, nil, nil
	default:
		return nil, nil, fmt.Errorf("unknown file type for: %s", filePath)
	}
}

// getReaderFromBytes gets a CSV reader from decompressed data
func getReaderFromBytes(data []byte, fileType FileType, jpath string) (*csv.Reader, error) {
	switch fileType {
	case FileTypeJSON:
		if jpath == "" {
			return nil, fmt.Errorf("JSONPath expression is required for JSON files")
		}
		return GetJSONReaderFromBytes(data, jpath)
	case FileTypeCSV:
		return GetCSVReaderFromBytes(data)
	case FileTypeXLSX:
		return GetXLSXReaderFromBytes(data)
	default:
		return nil, fmt.Errorf("unknown file type")
	}
}

// ReadHeaderForPath handles both files and directories
// For directories, returns the union header across all files
func ReadHeaderForPath(path string, options FileOptions, ingestTz *time.Location) ([]string, error) {
	if IsDirectory(path) {
		info, err := DiscoverFiles(path, DirectoryDiscoveryOptions{
			Pattern: options.FilePattern, // Use file pattern for file discovery
		}, nil)
		if err != nil {
			return nil, err
		}
		return GetDirectoryHeader(info, options)
	}
	return ReadHeaderWithOptions(path, options, ingestTz)
}

// GetRowCountForPath handles both files and directories
// For directories, returns total row count across all files
func GetRowCountForPath(path string, options FileOptions) (int, error) {
	if IsDirectory(path) {
		info, err := DiscoverFiles(path, DirectoryDiscoveryOptions{
			Pattern: options.FilePattern,
		}, nil)
		if err != nil {
			return 0, err
		}
		return GetDirectoryRowCount(info, options)
	}
	return GetRowCountWithOptions(path, options)
}

// GetReaderForPath handles both files and directories
// For directories, returns a DirectoryReader that iterates through all files
// Returns a Reader interface and a Closer interface for cleanup
func GetReaderForPath(path string, options FileOptions) (interface{}, interface{}, error) {
	if IsDirectory(path) {
		info, err := DiscoverFiles(path, DirectoryDiscoveryOptions{
			Pattern: options.FilePattern,
		}, nil)
		if err != nil {
			return nil, nil, err
		}
		reader, err := NewDirectoryReader(info, options)
		if err != nil {
			return nil, nil, err
		}
		return reader, reader, nil // DirectoryReader implements both Reader and Closer
	}

	csvReader, file, err := GetReader(path, options)
	return csvReader, file, err
}
