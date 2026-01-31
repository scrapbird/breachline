package query

import "breachline/app/interfaces"

// Type aliases to interfaces package to avoid duplication and circular dependencies
type ProgressCallback = interfaces.ProgressCallback
type Row = interfaces.Row
type TimestampStats = interfaces.TimestampStats
type StageResult = interfaces.StageResult
type FileTab = interfaces.FileTab

// PipelineStage represents a single stage in the query pipeline
type PipelineStage interface {
	// Execute processes the input data and returns a stage result with metadata
	Execute(input *StageResult) (*StageResult, error)

	// CanCache returns true if this stage's results can be cached
	CanCache() bool

	// CacheKey returns a unique key for caching this stage's results
	CacheKey() string

	// Name returns the stage name for progress reporting
	Name() string

	// EstimateOutputSize estimates the output size relative to input (0.0-1.0+)
	EstimateOutputSize() float64
}

// QueryPipeline is defined in pipeline.go

// QueryResult contains the final result of pipeline execution
type QueryResult struct {
	OriginalHeader []string        // Full original file header
	Header         []string        // Display header (filtered)
	DisplayColumns []int           // Which columns to display from full rows
	Rows           []*Row          // Rows with pre-parsed timestamps
	TimestampStats *TimestampStats // Pre-calculated timestamp statistics
	Total          int64
	Cached         bool
}

// GetDisplayRow returns only the columns that should be displayed
// NEW METHOD - for frontend rendering
func (r *QueryResult) GetDisplayRow(rowIdx int) []string {
	if rowIdx < 0 || rowIdx >= len(r.Rows) {
		return []string{}
	}

	row := r.Rows[rowIdx]
	fullRow := row.Data

	// If no display columns specified, return full row
	if len(r.DisplayColumns) == 0 {
		return fullRow
	}

	// Build displayed row from specified columns
	displayRow := make([]string, len(r.DisplayColumns))
	for i, origIdx := range r.DisplayColumns {
		if origIdx >= 0 && origIdx < len(fullRow) {
			displayRow[i] = fullRow[origIdx]
		}
	}

	return displayRow
}

// GetDisplayRows returns all rows with only displayed columns
// NEW METHOD - for frontend rendering
func (r *QueryResult) GetDisplayRows() [][]string {
	displayRows := make([][]string, len(r.Rows))
	for i := range r.Rows {
		displayRows[i] = r.GetDisplayRow(i)
	}
	return displayRows
}

// FileReader is an alias to interfaces.FileReader
type FileReader = interfaces.FileReader

const (
	// ProgressUpdateInterval defines how often to report progress
	ProgressUpdateInterval = interfaces.ProgressUpdateInterval

	// DefaultCacheMaxSize is the default cache size limit (100MB)
	DefaultCacheMaxSize = 100 * 1024 * 1024

	// MinRowsForProgress is the minimum rows before showing progress
	MinRowsForProgress = 5000

	// EnableIncrementalCaching enables per-stage caching
	EnableIncrementalCaching = true

	// MaxStageResultSize limits individual stage cache entries
	MaxStageResultSize = 10 * 1024 * 1024 // 10MB
)

// StageType represents different types of pipeline stages
type StageType int

const (
	StageFilter StageType = iota
	StageSort
	StageDedup
	StageLimit
	StageStrip
	StageColumns
)

// SortDirection represents sort order
type SortDirection int

const (
	SortAsc SortDirection = iota
	SortDesc
)

// CacheConfig controls caching behavior
type CacheConfig struct {
	EnablePipelineCache bool  // Cache full pipeline results
	EnableStageCache    bool  // Cache individual stage results
	CacheSizeLimit      int64 // Unified cache size limit (applies to all cache types)
}

// DefaultCacheConfig returns default cache configuration
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		EnablePipelineCache: true,
		EnableStageCache:    true,
		CacheSizeLimit:      100 * 1024 * 1024, // 100MB unified cache limit
	}
}

// CacheConfigFromSettings creates cache config based on user settings
func CacheConfigFromSettings(enableCache bool, sizeMB int) CacheConfig {
	return CacheConfig{
		EnablePipelineCache: enableCache,
		EnableStageCache:    enableCache,
		CacheSizeLimit:      int64(sizeMB) * 1024 * 1024, // Convert MB to bytes
	}
}

// DefaultCacheConfigWithSize returns cache configuration with custom size
func DefaultCacheConfigWithSize(sizeMB int) CacheConfig {
	return CacheConfigFromSettings(true, sizeMB) // Maintain backward compatibility
}
