package fileloader

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"breachline/app/plugin"
)

// detectionCacheEntry holds a memoized detection result along with the file
// stat guard (mtime + size) that validates it. If the file changes on disk,
// the guard no longer matches and detection is re-run.
type detectionCacheEntry struct {
	fileType    FileType
	compression CompressionType
	modTimeUnix int64
	size        int64
}

// Global memoization cache for file type + compression detection (per absolute path).
// Detection is otherwise recomputed on every dispatcher call, and for files without a
// compression extension each recompute opens the file to peek at magic bytes. Caching the
// result (guarded by mtime + size) elides the redundant peeks while still running the full
// detection, including the magic-byte peek, on a cache miss.
var (
	detectionCacheMu sync.RWMutex
	detectionCache   = make(map[string]detectionCacheEntry)
)

// fileParseObserver is nil in normal operation. Formats that are parsed whole
// (JSON, XLSX) report each parse through it, so a test can assert that a directory
// load parses each member once rather than once per caller that asks for a header.
// That property is the entire reason for the snapshot cache and the Row-based
// base-data caches, and it is invisible in output, so nothing else would catch its
// regression.
//
// It may be called from worker goroutines; implementations must be safe for
// concurrent use.
var fileParseObserver func(filePath string)

// setFileParseObserver installs the observer and returns a function restoring the
// previous one.
func setFileParseObserver(f func(filePath string)) func() {
	previous := fileParseObserver
	fileParseObserver = f
	return func() { fileParseObserver = previous }
}

// notifyFileParse reports that a file is about to be read and parsed in full.
func notifyFileParse(filePath string) {
	if observer := fileParseObserver; observer != nil {
		observer(filePath)
	}
}

// detectFileTypeAndCompressionCached returns the detected (FileType, CompressionType)
// pair for a file, using a memoization cache keyed by absolute path and guarded by the
// file's mtime + size. On a cache miss (or a stat mismatch indicating the file changed)
// it runs the full DetectFileTypeAndCompression logic - including the magic-byte peek -
// and stores the result. If the file cannot be stat'd, it falls back to the uncached
// detection so behavior is never worse than before.
func detectFileTypeAndCompressionCached(filePath string) (FileType, CompressionType) {
	if filePath == "" {
		return FileTypeUnknown, CompressionNone
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		// Can't build a stable key; fall back to uncached detection.
		return DetectFileTypeAndCompression(filePath)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		// Can't validate the guard; fall back to uncached detection.
		return DetectFileTypeAndCompression(filePath)
	}
	modTimeUnix := info.ModTime().UnixNano()
	size := info.Size()

	// Fast path: read lock and check for a valid cached entry.
	detectionCacheMu.RLock()
	entry, found := detectionCache[absPath]
	detectionCacheMu.RUnlock()
	if found && entry.modTimeUnix == modTimeUnix && entry.size == size {
		return entry.fileType, entry.compression
	}

	// Miss (or stale) - run the full detection, including the magic-byte peek.
	fileType, compression := DetectFileTypeAndCompression(filePath)

	detectionCacheMu.Lock()
	detectionCache[absPath] = detectionCacheEntry{
		fileType:    fileType,
		compression: compression,
		modTimeUnix: modTimeUnix,
		size:        size,
	}
	detectionCacheMu.Unlock()

	return fileType, compression
}

// ClearDetectionCache removes the memoized detection result for a file path.
// Mirrors ClearDecompressionWarning; call this on tab close or file re-open so an
// edited file re-detects even within the mtime guard window. Keys are absolute paths.
func ClearDetectionCache(filePath string) {
	if filePath == "" {
		return
	}
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}
	detectionCacheMu.Lock()
	defer detectionCacheMu.Unlock()
	delete(detectionCache, absPath)
}

// compressionExtensions maps compression extensions to their CompressionType
var compressionExtensions = map[string]CompressionType{
	".gz":  CompressionGzip,
	".bz2": CompressionBzip2,
	".xz":  CompressionXZ,
}

// DetectFileType determines the file type based on the file extension.
// This function can be extended in the future to support more sophisticated
// detection methods (e.g., magic number detection).
//
// Supported file types:
//   - CSV (.csv)
//   - XLSX (.xlsx)
//   - JSON (.json)
//
// Returns FileTypeCSV as default for backwards compatibility.
// Note: This function does NOT handle compressed files. Use DetectFileTypeAndCompression instead.
func DetectFileType(filePath string) FileType {
	if filePath == "" {
		return FileTypeUnknown
	}

	// Simple extension-based detection for now
	// Future: Could use magic number detection for more reliability
	lower := strings.ToLower(filePath)

	if strings.HasSuffix(lower, ".csv") {
		return FileTypeCSV
	}

	if strings.HasSuffix(lower, ".xlsx") {
		return FileTypeXLSX
	}

	if strings.HasSuffix(lower, ".json") {
		return FileTypeJSON
	}

	// Check plugin registry for custom file types
	ext := filepath.Ext(filePath)
	if registry := plugin.GetPluginRegistry(); registry != nil {
		if _, ok := registry.GetPluginForExtension(ext); ok {
			return FileTypePlugin
		}
	}

	// Default to CSV for backwards compatibility
	return FileTypeCSV
}

// DetectFileTypeAndCompression determines both the file type and compression type.
// It first checks for double extensions (e.g., .csv.gz) and falls back to magic byte
// detection if no compression extension is found but the file might be compressed.
//
// Supported compression formats:
//   - gzip (.gz)
//   - bzip2 (.bz2)
//   - xz (.xz)
//
// Returns the inner file type and compression type.
func DetectFileTypeAndCompression(filePath string) (FileType, CompressionType) {
	if filePath == "" {
		return FileTypeUnknown, CompressionNone
	}

	lower := strings.ToLower(filePath)

	// First, check for compression extension
	compressionType := CompressionNone
	innerPath := lower

	for ext, ct := range compressionExtensions {
		if strings.HasSuffix(lower, ext) {
			compressionType = ct
			innerPath = strings.TrimSuffix(lower, ext)
			break
		}
	}

	// If no compression extension found, check magic bytes
	if compressionType == CompressionNone {
		if magicType, err := DetectCompressionByMagic(filePath); err == nil && magicType != CompressionNone {
			compressionType = magicType
			// For magic byte detection, we can't determine inner type from extension
			// Try to detect based on content after decompression
			// For now, default to CSV for backwards compatibility
			return FileTypeCSV, compressionType
		}
	}

	// Detect inner file type from the path without compression extension
	fileType := detectFileTypeFromPath(innerPath)

	return fileType, compressionType
}

// detectFileTypeFromPath determines file type from a path (without compression extension)
func detectFileTypeFromPath(path string) FileType {
	if strings.HasSuffix(path, ".csv") {
		return FileTypeCSV
	}

	if strings.HasSuffix(path, ".xlsx") {
		return FileTypeXLSX
	}

	if strings.HasSuffix(path, ".json") {
		return FileTypeJSON
	}

	// Check plugin registry for custom file types
	ext := filepath.Ext(path)
	if registry := plugin.GetPluginRegistry(); registry != nil {
		if _, ok := registry.GetPluginForExtension(ext); ok {
			return FileTypePlugin
		}
	}

	// Default to CSV for backwards compatibility
	return FileTypeCSV
}

// IsCompressedFile checks if a file is compressed based on extension or magic bytes
func IsCompressedFile(filePath string) bool {
	_, compression := DetectFileTypeAndCompression(filePath)
	return compression != CompressionNone
}

// GetUncompressedExtension returns the file extension without compression suffix
// e.g., "data.csv.gz" -> ".csv", "data.json.bz2" -> ".json"
func GetUncompressedExtension(filePath string) string {
	lower := strings.ToLower(filePath)

	// Strip compression extension if present
	for ext := range compressionExtensions {
		if strings.HasSuffix(lower, ext) {
			lower = strings.TrimSuffix(lower, ext)
			break
		}
	}

	// Find the last extension
	lastDot := strings.LastIndex(lower, ".")
	if lastDot == -1 {
		return ""
	}

	return lower[lastDot:]
}
