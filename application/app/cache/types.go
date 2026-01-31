package cache

import (
	"breachline/app/interfaces"
	"time"
)

// Logger interface for cache logging
type Logger interface {
	Log(level, message string)
}

// HistogramBucket represents a single time bucket and its count
type HistogramBucket struct {
	Start int64 `json:"start"` // Start epoch milliseconds of the bucket start
	Count int   `json:"count"`
}

// CacheEntry represents a cached pipeline result
type CacheEntry struct {
	OriginalHeader []string          // Original file header (all columns)
	Header         []string          // Result header (may be modified by queries)
	DisplayColumns []int             // Indices of columns to display (maps result to original)
	Rows           []*interfaces.Row // Rows with pre-parsed timestamps
	IsComplete     bool
	Size           int64
	AccessTime     int64
	CreateTime     time.Time

	// Timestamp statistics for query optimization
	TimestampStats *interfaces.TimestampStats `json:"timestampStats,omitempty"`

	// Histogram caching fields
	HistogramBuckets    []HistogramBucket `json:"histogramBuckets,omitempty"`
	HistogramMinTs      int64             `json:"histogramMinTs,omitempty"`
	HistogramMaxTs      int64             `json:"histogramMaxTs,omitempty"`
	HistogramTimeField  string            `json:"histogramTimeField,omitempty"`
	HistogramBucketSecs int               `json:"histogramBucketSecs,omitempty"`
	HasHistogram        bool              `json:"hasHistogram,omitempty"`
}

// CacheStats contains detailed cache statistics
type CacheStats struct {
	TotalEntries int
	TotalSize    int64
	MaxSize      int64
	UsagePercent float64
	StageStats   map[string]StageStats

	// Incremental cache metrics
	PipelineCacheHits int64   // Full pipeline cache hits
	StageCacheHits    int64   // Individual stage cache hits
	CacheMisses       int64   // Total cache misses
	HitRate           float64 // Overall hit rate
	StageHitRate      float64 // Stage-level hit rate
}

// StageStats contains statistics for a specific stage type
type StageStats struct {
	EntryCount int
	TotalSize  int64
}

// DefaultCacheMaxSize is the default cache size limit (100MB)
const DefaultCacheMaxSize = 100 * 1024 * 1024

// BaseDataEntry interface for accessing cached base file data.
// This interface allows the cache to be used without import cycles.
type BaseDataEntry interface {
	GetHeader() []string
	GetRows() []*interfaces.Row
	GetTimestampStats() *interfaces.TimestampStats
}

// BaseFileCacheEntry represents cached base file data with pre-parsed Row objects.
// This enables efficient sharing of Row pointers between the base data cache and query result caches,
// avoiding duplication of string data and timestamp re-parsing.
type BaseFileCacheEntry struct {
	Header         []string          // File header
	Rows           []*interfaces.Row // Row pointers with pre-parsed timestamps
	TimestampStats *interfaces.TimestampStats
	ModTime        time.Time // File modification time for invalidation
	Size           int64
	AccessTime     int64
	CreateTime     time.Time
}

// GetHeader implements BaseDataEntry interface
func (e *BaseFileCacheEntry) GetHeader() []string {
	return e.Header
}

// GetRows implements BaseDataEntry interface
func (e *BaseFileCacheEntry) GetRows() []*interfaces.Row {
	return e.Rows
}

// GetTimestampStats implements BaseDataEntry interface
func (e *BaseFileCacheEntry) GetTimestampStats() *interfaces.TimestampStats {
	return e.TimestampStats
}
