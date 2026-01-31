package interfaces

import (
	"context"
	"sync"

	sharedtypes "github.com/scrapbird/breachline/shared/types"
)

// FileOptions is an alias to the shared type for backward compatibility.
// Use sharedtypes.FileOptions directly for new code.
type FileOptions = sharedtypes.FileOptions

// AnnotationResult holds the result of fetching an annotation
type AnnotationResult struct {
	ID    string `json:"id,omitempty"` // Annotation UUID
	Note  string `json:"note"`
	Color string `json:"color"`
}

// WorkspaceFile represents information about a file in the workspace
type WorkspaceFile struct {
	FilePath     string          `json:"filePath" yaml:"file_path"`
	RelativePath string          `json:"relativePath,omitempty" yaml:"-"` // Computed relative path for display
	FileHash     string          `json:"fileHash" yaml:"file_hash"`
	Options      FileOptions     `json:"options" yaml:"options"`
	Description  string          `json:"description,omitempty" yaml:"description,omitempty"` // User-provided description
	Annotations  []RowAnnotation `json:"-" yaml:"annotations,omitempty"`                     // Internal annotations (not sent to frontend)
	// Frontend compatibility fields
	AnnotationCount int `json:"annotations" yaml:"-"` // Count of annotations for frontend display
}

// RowAnnotation represents a single row annotation
type RowAnnotation struct {
	AnnotationID string `json:"annotation_id,omitempty" yaml:"annotation_id,omitempty"` // UUID for the annotation
	RowIndex     int    `json:"row_index" yaml:"row_index"`                             // 0-based index of the annotated row
	Note         string `json:"note" yaml:"note"`
	Color        string `json:"color" yaml:"color"` // grey, blue, yellow, green, orange, red
}

// FileTab encapsulates all state for a single opened file
// This includes both the full app FileTab and the simplified version for file loading
type FileTab struct {
	ID       string
	FilePath string
	FileName string      // Display name for the tab
	FileHash string      // Hash of the file content
	Options  FileOptions // File options (jpath, noHeaderRow, ingestTimezoneOverride)

	// Cached, pre-sorted rows for this tab's CSV
	CacheMu         sync.RWMutex
	SortedHeader    []string
	SortedRows      [][]string
	SortedForFile   string
	SortedByTime    bool
	SortedDesc      bool
	SortedTimeField string // Track which column was used for timestamp sorting

	// Active sort cancellation
	SortMu     sync.Mutex
	SortCancel context.CancelFunc
	SortActive int64
	SortSeq    int64
	SortCond   *sync.Cond

	// Track what configuration the active sort is building for
	SortingForFile   string
	SortingByTime    bool
	SortingDesc      bool
	SortingTimeField string // Track which column is being used for timestamp sorting

	// LRU cache of query results
	QueryCache      map[string][][]string
	QueryCacheOrder []string

	// Track last-used settings that affect cache validity
	LastDisplayTZ       string
	LastIngestTZ        string
	LastTimestampFormat string

	// Histogram async generation
	HistogramMu      sync.Mutex
	HistogramCancel  context.CancelFunc
	HistogramVersion int64 // Incremented on each query

	// Query cancellation
	QueryMu     sync.Mutex
	QueryCancel context.CancelFunc
	LastQuery   string // Track last query to detect changes

	// Timestamp parser caching
	TimestampParserMu sync.RWMutex
	TimestampParser   interface{} // *timestamps.TimestampParserInfo but using interface{} to avoid import cycle
	TimestampParserTZ string      // Timezone used for cached parser (for invalidation)

	// Original-to-display index mapping for fast lookups
	// Maps original file row index -> display row index (after sorting)
	IndexMapMu              sync.RWMutex
	OriginalToDisplayMap    map[int]int // RowIndex -> DisplayIndex
	IndexMapSortedByTime    bool        // What sort config this map was built for
	IndexMapSortedDesc      bool
	IndexMapSortedTimeField string

	// Query-specific index mapping for annotation panel
	// Maps original file row index -> display row index (after current query filtering/sorting)
	// This is updated on every query, unlike OriginalToDisplayMap which is only for sorting
	QueryIndexMapMu sync.RWMutex
	QueryIndexMap   map[int]int // RowIndex -> DisplayIndex for current query
}

// SimpleFileTab is a simplified version for file loading operations
type SimpleFileTab struct {
	ID       string
	FilePath string
	JPath    string
	FileHash string
}

// TabInfo contains metadata about a tab for frontend display
type TabInfo struct {
	ID       string   `json:"id"`
	FileName string   `json:"fileName"`
	FilePath string   `json:"filePath"`
	FileHash string   `json:"fileHash"`
	Headers  []string `json:"headers,omitempty"`
}

// FileType represents different file types
// Note: This is kept separate from fileloader.FileType to avoid import cycles
type FileType int

const (
	FileTypeUnknown FileType = iota
	FileTypeCSV
	FileTypeXLSX
	FileTypeJSON
)

// ProgressCallback provides real-time feedback during query execution
type ProgressCallback func(stage string, current, total int64, message string)

// RowAnnotationInfo contains cached annotation data for a row
// This is stored on Row to avoid repeated annotation lookups
type RowAnnotationInfo struct {
	ID    string // Annotation UUID (e.g., "ann_...")
	Color string // Annotation color
	Note  string // Annotation note
}

// FileAnnotationInfo contains annotation data with display index mapping
// Used for the annotation panel to show all annotations in a file
type FileAnnotationInfo struct {
	OriginalRowIndex int    `json:"originalRowIndex"` // Original file row index
	DisplayRowIndex  int    `json:"displayRowIndex"`  // Current display index (-1 if not visible)
	Note             string `json:"note"`
	Color            string `json:"color"`
}

// SearchResult represents a single search match in the dataset
// Used by the search/find feature to show matches in the SearchPanel
type SearchResult struct {
	RowIndex    int    `json:"rowIndex"`    // 0-based row index in the current view
	ColumnIndex int    `json:"columnIndex"` // 0-based column index
	ColumnName  string `json:"columnName"`  // Column header name
	MatchStart  int    `json:"matchStart"`  // Start position of match in cell value
	MatchEnd    int    `json:"matchEnd"`    // End position of match in cell value
	Snippet     string `json:"snippet"`     // Snippet of the cell value around the match
}

// SearchResponse represents the full response from a search operation
type SearchResponse struct {
	Results    []SearchResult `json:"results"`    // Search results (paginated)
	TotalCount int            `json:"totalCount"` // Total number of matches found
	Page       int            `json:"page"`       // Current page (0-indexed)
	PageSize   int            `json:"pageSize"`   // Results per page
	Cancelled  bool           `json:"cancelled"`  // Whether the search was cancelled
}

// Row represents a single data row with pre-parsed timestamp
// This avoids re-parsing timestamps at every pipeline stage
type Row struct {
	RowIndex     int                // 0-based index of this row in the source file (order of appearance)
	DisplayIndex int                // 0-based index in display/result set (after filters/sorts), -1 if not yet assigned
	Data         []string           // Raw string data for all columns
	Timestamp    int64              // Pre-parsed timestamp in milliseconds (0 if no valid timestamp)
	HasTime      bool               // Whether this row has a valid timestamp
	FirstColHash string             // DEPRECATED: Pre-computed hash of first column (to be removed in later stage)
	Annotation   *RowAnnotationInfo // Cached annotation info (nil if not annotated or not yet checked)
}

// TimestampStats contains min/max timestamp information for a result set
type TimestampStats struct {
	TimeFieldIdx int   // Index of the time field in original header
	MinTimestamp int64 // Minimum timestamp in milliseconds
	MaxTimestamp int64 // Maximum timestamp in milliseconds
	ValidCount   int   // Number of rows with valid timestamps
}

// StageResult represents the output of a pipeline stage with metadata
type StageResult struct {
	OriginalHeader []string        // Full original file header
	Header         []string        // Display header (filtered)
	DisplayColumns []int           // Which columns to display from full rows
	Rows           []*Row          // Rows with pre-parsed timestamps
	TimestampStats *TimestampStats // Timestamp statistics calculated during stage processing
}

// FileReader provides access to file contents
type FileReader interface {
	// ReadRows returns all rows in the file with pre-parsed timestamps
	ReadRows() (*StageResult, error)

	// ReadRowsWithSort returns sorted rows with pre-parsed timestamps
	ReadRowsWithSort(timeIdx int, desc bool) (*StageResult, error)

	// Header returns the file header
	Header() ([]string, error)

	// EstimateRowCount returns estimated total rows if available
	EstimateRowCount() int64

	// Close releases file resources
	Close() error
}

// PluginConfig represents a single plugin configuration (frontend-facing)
type PluginConfig struct {
	ID          string   `json:"id"` // Unique plugin identifier (UUID from plugin.yml)
	Name        string   `json:"name"`
	Enabled     bool     `json:"enabled"`
	Path        string   `json:"path"`
	Extensions  []string `json:"extensions"`
	Description string   `json:"description"`
}

// Settings represents application settings
type Settings struct {
	DisplayTimezone        string         `json:"displayTimezone"`
	DefaultIngestTimezone  string         `json:"defaultIngestTimezone"`
	TimestampDisplayFormat string         `json:"timestampDisplayFormat"`
	SortByTime             bool           `json:"sortByTime"`
	SortDescending         bool           `json:"sortDescending"`
	EnableQueryCache       bool           `json:"enableQueryCache"`
	License                string         `json:"license"`
	EnablePlugins          bool           `json:"enablePlugins"`
	Plugins                []PluginConfig `json:"plugins"`
}

// Reader interface for CSV/JSON readers
type Reader interface {
	Read() ([]string, error)
}

// Closer interface for file closers
type Closer interface {
	Close() error
}

// QueryExecutionResult represents a query result with full metadata
type QueryExecutionResult struct {
	OriginalHeader []string
	Header         []string
	DisplayColumns []int
	Rows           [][]string
	Total          int64
	Cached         bool
	// OPTIMIZATION: Preserve StageResult for histogram generation
	StageResult *StageResult
	// PipelineCacheKey is the cache key built from parsed pipeline stages
	// Used for histogram caching to ensure consistent keys regardless of query whitespace
	PipelineCacheKey string
}

// Constants for file loading and query processing
const (
	// ProgressUpdateInterval defines how often to report progress
	ProgressUpdateInterval = 1000
)

// AppService interface defines the methods that workspace services need from the app
// Note: DetectFileType has been removed - callers should use fileloader.DetectFileType() directly
type AppService interface {
	ExecuteQueryForTab(tab *FileTab, query, timeField string) ([]string, [][]string, error)
	ExecuteQueryForTabWithMetadata(tab *FileTab, query, timeField string) (*QueryExecutionResult, error)
	GetActiveTab() *FileTab
	Log(level, message string)
	IsLicensed() bool
	GetEffectiveSettings() *Settings
	ParseTimestampMillis(s string, fallbackLoc interface{}) (int64, bool)
	ParseTimestampMillisWithCache(s string, fallbackLoc interface{}, tab *FileTab) (int64, bool)
	ReadJSONHeader(filePath, jpath string) ([]string, error)
	GetJSONReader(filePath, jpath string) (Reader, Closer, error)
	ReadHeader(filePath string) ([]string, error)
	GetReader(filePath string) (Reader, Closer, error)
	DetectTimestampIndex(header []string) int
	GetQueryCache() interface{} // Returns *cache.Cache but using interface{} to avoid import cycle
}
