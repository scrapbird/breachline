package fileloader

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/bmatcuk/doublestar/v4"
)

// DirectoryInfo contains metadata about a discovered directory
type DirectoryInfo struct {
	RootPath   string   // Absolute path to directory
	Files      []string // List of discovered file paths (absolute)
	TotalFiles int      // Total files found
	TotalSize  int64    // Total size in bytes
}

// DirectoryDiscoveryOptions controls file discovery behavior
type DirectoryDiscoveryOptions struct {
	Pattern         string   // Glob pattern filter (e.g., "*.json.gz", "*.csv")
	ExcludePatterns []string // Patterns to exclude
	MaxFiles        int      // Maximum files to include (0 = unlimited)
	MaxDepth        int      // Maximum directory depth (0 = unlimited)
}

// DirectoryReader provides unified sequential access to all files in a directory
type DirectoryReader struct {
	info          *DirectoryInfo
	options       FileOptions
	unifiedHeader []string       // Union of all headers
	headerMap     map[string]int // Column name -> index in unified header
	currentIdx    int            // Current file index
	currentReader *csv.Reader    // Current file's CSV reader
	currentFile   *os.File       // Current file handle (for cleanup)
	currentPath   string         // Current file path (for source column)
	currentHeader []string       // Header of current file
	sourceColIdx  int            // Index of __source_file__ column (-1 if disabled)
	rootPath      string         // For relative path calculation
}

// DiscoveryProgress reports progress during directory scanning
type DiscoveryProgress struct {
	FilesFound  int
	DirsScanned int
	CurrentPath string
	TotalSize   int64
}

// DiscoveryProgressCallback is called during directory scanning
type DiscoveryProgressCallback func(progress DiscoveryProgress)

// IsDirectory checks if the path is a directory
func IsDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// DiscoverFiles recursively finds all files matching the pattern in a directory
// Returns files in discovery order (depth-first traversal)
// Pattern is required - all files should be of the same log type for consistent timestamp parsing
// Uses doublestar library for efficient pattern matching and directory traversal
func DiscoverFiles(dirPath string, options DirectoryDiscoveryOptions, progress DiscoveryProgressCallback) (*DirectoryInfo, error) {
	// Pattern is required
	if options.Pattern == "" {
		return nil, fmt.Errorf("file pattern is required (e.g., *.json.gz, *.csv)")
	}

	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %w", err)
	}

	// Use doublestar library for all pattern matching - it handles optimization automatically
	files, totalSize, err := discoverFilesWithDoublestar(absPath, options, progress)
	if err != nil {
		return nil, err
	}

	return &DirectoryInfo{
		RootPath:   absPath,
		Files:      files,
		TotalFiles: len(files),
		TotalSize:  totalSize,
	}, nil
}

// discoverFilesWithDoublestar uses doublestar library for efficient pattern matching
func discoverFilesWithDoublestar(rootPath string, options DirectoryDiscoveryOptions, progress DiscoveryProgressCallback) ([]string, int64, error) {
	var files []string
	var totalSize int64
	dirsScanned := 0

	// Create the full pattern by combining rootPath with the user pattern
	fullPattern := filepath.Join(rootPath, options.Pattern)

	// Use doublestar to find all matching files - it handles directory traversal optimization
	// For v4, we need to use the filesystem-based API
	matches, err := doublestar.FilepathGlob(fullPattern)
	if err != nil {
		return nil, 0, fmt.Errorf("pattern matching failed: %w", err)
	}

	// Process each match
	for _, match := range matches {
		// Get file info
		info, err := os.Stat(match)
		if err != nil {
			continue // Skip files we can't stat
		}

		// Skip directories
		if info.IsDir() {
			continue
		}

		// Check exclude patterns
		excluded := false
		for _, excludePattern := range options.ExcludePatterns {
			if matched, _ := filepath.Match(excludePattern, filepath.Base(match)); matched {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		files = append(files, match)
		totalSize += info.Size()

		// Report progress
		if progress != nil {
			progress(DiscoveryProgress{
				FilesFound:  len(files),
				DirsScanned: dirsScanned,
				CurrentPath: match,
				TotalSize:   totalSize,
			})
		}

		// Check max files limit
		if options.MaxFiles > 0 && len(files) >= options.MaxFiles {
			break
		}
	}

	return files, totalSize, nil
}

// GetDirectoryHeader reads headers from all files and returns unified union header
// Columns are ordered by first appearance across files
func GetDirectoryHeader(info *DirectoryInfo, options FileOptions) ([]string, error) {
	seen := make(map[string]bool)
	var unionHeader []string

	for _, filePath := range info.Files {
		header, err := readFileHeader(filePath, options)
		if err != nil {
			// Log warning, continue with other files
			continue
		}

		for _, col := range header {
			if !seen[col] {
				seen[col] = true
				unionHeader = append(unionHeader, col)
			}
		}
	}

	if len(unionHeader) == 0 {
		return nil, fmt.Errorf("no valid headers found in any files")
	}

	// Add source column if requested (at the end)
	if options.IncludeSourceColumn {
		unionHeader = append(unionHeader, "__source_file__")
	}

	return unionHeader, nil
}

// readFileHeader reads the header from a single file
func readFileHeader(filePath string, options FileOptions) ([]string, error) {
	return ReadHeaderWithOptions(filePath, options, nil)
}

// GetDirectoryRowCount returns total row count across all files
func GetDirectoryRowCount(info *DirectoryInfo, options FileOptions) (int, error) {
	totalCount := 0

	for _, filePath := range info.Files {
		count, err := GetRowCountWithOptions(filePath, options)
		if err != nil {
			// Skip files that can't be read
			continue
		}
		totalCount += count
	}

	return totalCount, nil
}

// NewDirectoryReader creates a reader that iterates through all files
func NewDirectoryReader(info *DirectoryInfo, options FileOptions) (*DirectoryReader, error) {
	if info == nil || len(info.Files) == 0 {
		return nil, fmt.Errorf("no files to read")
	}

	// Build union header
	header, err := GetDirectoryHeader(info, options)
	if err != nil {
		return nil, fmt.Errorf("failed to build union header: %w", err)
	}

	// Build header map for fast column lookup
	headerMap := make(map[string]int)
	for i, col := range header {
		headerMap[col] = i
	}

	// Find source column index
	sourceColIdx := -1
	if options.IncludeSourceColumn {
		sourceColIdx = headerMap["__source_file__"]
	}

	return &DirectoryReader{
		info:          info,
		options:       options,
		unifiedHeader: header,
		headerMap:     headerMap,
		currentIdx:    0,
		sourceColIdx:  sourceColIdx,
		rootPath:      info.RootPath,
	}, nil
}

// Read returns the next row from the directory (unified schema)
// Returns io.EOF when all files are exhausted
func (dr *DirectoryReader) Read() ([]string, error) {
	for {
		// If no current reader, open next file
		if dr.currentReader == nil {
			if dr.currentIdx >= len(dr.info.Files) {
				return nil, io.EOF
			}

			filePath := dr.info.Files[dr.currentIdx]
			dr.currentIdx++

			reader, file, err := GetReader(filePath, dr.options)
			if err != nil {
				// Log warning, try next file
				continue
			}

			dr.currentReader = reader
			dr.currentFile = file
			dr.currentPath = filePath

			// Read and cache header for current file
			dr.currentHeader, err = readFileHeader(filePath, dr.options)
			if err != nil {
				dr.closeCurrentFile()
				continue
			}

			// Skip header row if present (we read it separately for mapping)
			if !dr.options.NoHeaderRow {
				_, err := reader.Read()
				if err != nil {
					dr.closeCurrentFile()
					continue
				}
			}
		}

		// Read next row from current file
		row, err := dr.currentReader.Read()
		if err == io.EOF {
			dr.closeCurrentFile()
			continue // Move to next file
		}
		if err != nil {
			dr.closeCurrentFile()
			continue // Skip problematic rows
		}

		// Map row to unified schema
		unifiedRow := dr.mapToUnifiedSchema(row)

		return unifiedRow, nil
	}
}

// mapToUnifiedSchema maps a row from the current file to the unified schema
func (dr *DirectoryReader) mapToUnifiedSchema(row []string) []string {
	unified := make([]string, len(dr.unifiedHeader))

	// Map each column from current file to unified position
	for i, val := range row {
		if i < len(dr.currentHeader) {
			colName := dr.currentHeader[i]
			if unifiedIdx, ok := dr.headerMap[colName]; ok {
				unified[unifiedIdx] = val
			}
		}
	}

	// Add source file column if enabled
	if dr.sourceColIdx >= 0 {
		relPath, err := filepath.Rel(dr.rootPath, dr.currentPath)
		if err != nil {
			relPath = dr.currentPath // Fallback to absolute path
		}
		unified[dr.sourceColIdx] = relPath
	}

	return unified
}

// Header returns the unified header
func (dr *DirectoryReader) Header() []string {
	return dr.unifiedHeader
}

// Close releases all resources
func (dr *DirectoryReader) Close() error {
	dr.closeCurrentFile()
	return nil
}

// closeCurrentFile closes the current file and resets the reader
func (dr *DirectoryReader) closeCurrentFile() {
	if dr.currentFile != nil {
		dr.currentFile.Close()
		dr.currentFile = nil
	}
	dr.currentReader = nil
	dr.currentPath = ""
	dr.currentHeader = nil
}

// CalculateDirectoryHash generates a hash for the directory contents.
// Algorithm:
// 1. Calculate SHA256 hash for each file's contents
// 2. Sort file paths alphabetically
// 3. For each file, append: file hash + relative path (to ensure directory structure is part of hash)
// 4. Hash the concatenated result
// This allows using existing annotation storage logic without special handling.
func CalculateDirectoryHash(info *DirectoryInfo) (string, error) {
	if info == nil || len(info.Files) == 0 {
		return "", fmt.Errorf("no files in directory info")
	}

	// Sort files for consistent hashing
	sortedFiles := make([]string, len(info.Files))
	copy(sortedFiles, info.Files)
	sort.Strings(sortedFiles)

	// Collect file hashes and relative paths
	var combinedData []byte
	for _, filePath := range sortedFiles {
		fileHash, err := calculateFileContentHash(filePath)
		if err != nil {
			// Skip files that can't be read, but continue with others
			continue
		}

		// Calculate relative path from directory root
		relPath, err := filepath.Rel(info.RootPath, filePath)
		if err != nil {
			// If we can't get relative path, skip this file
			continue
		}

		// Append file hash and relative path to combined data
		// This ensures directory structure is part of the hash
		combinedData = append(combinedData, fileHash...)
		combinedData = append(combinedData, []byte(relPath)...)
	}

	if len(combinedData) == 0 {
		return "", fmt.Errorf("failed to hash any files in directory")
	}

	// Hash the combined result
	finalHash := sha256.Sum256(combinedData)
	return hex.EncodeToString(finalHash[:]), nil
}

// calculateFileContentHash computes SHA256 hash of a file's contents
func calculateFileContentHash(filePath string) ([]byte, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

// DirectoryPreviewResult contains preview information for a directory
type DirectoryPreviewResult struct {
	Files      []string `json:"files"`      // Sample of discovered files (relative paths)
	Headers    []string `json:"headers"`    // Unified header columns
	TotalFiles int      `json:"totalFiles"` // Total number of files found
	TotalSize  int64    `json:"totalSize"`  // Total size in bytes
}

// PreviewDirectory returns preview information about a directory without fully loading it
func PreviewDirectory(dirPath string, pattern string, jpath string, maxFiles int) (*DirectoryPreviewResult, error) {
	// Discover files
	info, err := DiscoverFiles(dirPath, DirectoryDiscoveryOptions{
		Pattern:  pattern,
		MaxFiles: maxFiles,
	}, nil)
	if err != nil {
		return nil, err
	}

	if len(info.Files) == 0 {
		return nil, fmt.Errorf("no compatible files found in directory")
	}

	// Get unified header
	headers, err := GetDirectoryHeader(info, FileOptions{
		JPath:               jpath,
		IncludeSourceColumn: false, // Don't include in preview
	})
	if err != nil {
		return nil, err
	}

	// Convert file paths to relative paths for display
	relativeFiles := make([]string, len(info.Files))
	for i, f := range info.Files {
		rel, err := filepath.Rel(info.RootPath, f)
		if err != nil {
			rel = f
		}
		relativeFiles[i] = rel
	}

	return &DirectoryPreviewResult{
		Files:      relativeFiles,
		Headers:    headers,
		TotalFiles: info.TotalFiles,
		TotalSize:  info.TotalSize,
	}, nil
}
