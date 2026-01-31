package fileloader

import (
	sharedtypes "github.com/scrapbird/breachline/shared/types"
)

// Package fileloader provides centralized file reading and ingestion functionality
// for all supported file formats (CSV, XLSX, JSON). It abstracts file type detection,
// header reading, row counting, and basic file reading operations.
//
// Note: FileReader and FileTab remain in the query package to avoid import cycles.

// FileType represents the type of data file being processed
type FileType int

const (
	FileTypeUnknown FileType = iota
	FileTypeCSV
	FileTypeXLSX
	FileTypeJSON
	FileTypePlugin
)

// String returns the string representation of FileType
func (ft FileType) String() string {
	switch ft {
	case FileTypeCSV:
		return "CSV"
	case FileTypeXLSX:
		return "XLSX"
	case FileTypeJSON:
		return "JSON"
	case FileTypePlugin:
		return "Plugin"
	default:
		return "Unknown"
	}
}

// JSONPreviewResult contains preview data for JSON files
type JSONPreviewResult struct {
	Headers       []string
	Rows          [][]string
	Error         string
	AvailableKeys []string // Keys available at current path (when result is not an array)
}

// FileOptions is an alias to the shared type for backward compatibility.
// Use sharedtypes.FileOptions directly for new code.
type FileOptions = sharedtypes.FileOptions

// DefaultFileOptions returns the default parsing options
func DefaultFileOptions() FileOptions {
	return sharedtypes.DefaultFileOptions()
}
