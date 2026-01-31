package fileloader

import (
	"path/filepath"
	"strings"

	"breachline/app/plugin"
)

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
