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
}
