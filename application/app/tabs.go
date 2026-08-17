package app

import (
	"fmt"
	"path/filepath"
	"sync"

	"breachline/app/interfaces"
)

// FileTab is an alias to the interfaces.FileTab
type FileTab = interfaces.FileTab

// NewFileTab creates a new file tab with the given path
func NewFileTab(id, path string) *FileTab {
	return NewFileTabWithHashKey(id, path, nil)
}

// NewFileTabWithHashKey creates a new file tab with the given path
// The hashKey parameter is deprecated and ignored - file hashing now uses a hardcoded key
func NewFileTabWithHashKey(id, path string, hashKey []byte) *FileTab {
	// Calculate file hash using hardcoded key for consistent hashing
	// This ensures the same file always has the same hash, regardless of workspace context
	fileHash, err := CalculateFileHash(path)
	if err != nil {
		// If we can't calculate hash, log but continue
		// Annotations won't work for this file but other features will
		fileHash = ""
		fmt.Println("Failed to calculate file hash:", err)
	}

	tab := &FileTab{
		ID:       id,
		FilePath: path,
		FileName: filepath.Base(path),
		FileHash: fileHash,
	}
	tab.SortCond = sync.NewCond(&tab.CacheMu)
	return tab
}

// TabInfo contains metadata about a tab for frontend display
type TabInfo struct {
	ID                     string   `json:"id"`
	FileName               string   `json:"fileName"`
	FilePath               string   `json:"filePath"`
	FileHash               string   `json:"fileHash"`
	Headers                []string `json:"headers,omitempty"`
	IngestTimezoneOverride string   `json:"ingestTimezoneOverride,omitempty"`
	DecompressionWarning   string   `json:"decompressionWarning,omitempty"`
	DetectedFileType       string   `json:"detectedFileType,omitempty"` // "csv", "json", "xlsx" - detected from actual file loader used
	// PluginID/PluginName report the plugin actually used to load the file, even
	// when the caller did not pin one. The frontend adopts these into the tab's
	// options so its identity (dedup key, workspace entry, annotation lookup)
	// reflects the real loader instead of "no plugin".
	PluginID   string `json:"pluginId,omitempty"`
	PluginName string `json:"pluginName,omitempty"`
	// Truncated is true when the directory held more matching files than the
	// configured limit, so only FilesLoaded of them were opened. The frontend
	// warns the user that the dataset is incomplete.
	Truncated   bool `json:"truncated,omitempty"`
	FilesLoaded int  `json:"filesLoaded,omitempty"`
	// SampledSchema is true when the directory's column list was resolved from a
	// sample of its files rather than all of them, so a column unique to an
	// unsampled file is not in Headers yet. Such columns are still loaded: they are
	// appended to the header as the files carrying them are read.
	SampledSchema bool `json:"sampledSchema,omitempty"`
	SchemaSampled int  `json:"schemaSampled,omitempty"`
	// EstimatedUncompressedSize is the projected size of the directory's data once
	// decompressed, and MemoryWarning is set when loading it is projected to need
	// more memory than the machine has available.
	EstimatedUncompressedSize int64  `json:"estimatedUncompressedSize,omitempty"`
	MemoryWarning             string `json:"memoryWarning,omitempty"`
}
