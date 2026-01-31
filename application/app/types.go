package app

import (
	"breachline/app/histogram"
)

// RowsPage represents a page of CSV rows with metadata
type RowsPage struct {
	OriginalHeader   []string   `json:"originalHeader"` // Original file header (all columns)
	Header           []string   `json:"header"`         // Result header (may be modified by queries)
	DisplayColumns   []int      `json:"displayColumns"` // Mapping from result columns to original columns
	Rows             [][]string `json:"rows"`
	ReachedEnd       bool       `json:"reachedEnd"`
	Total            int        `json:"total"`
	Annotations      []bool     `json:"annotations"`      // Parallel array indicating if each row is annotated
	AnnotationColors []string   `json:"annotationColors"` // Parallel array with color for each row (empty if not annotated)
}

// DataAndHistogramResponse combines grid data and histogram data in a single response
// Query results are returned immediately, histogram may be generated asynchronously
type DataAndHistogramResponse struct {
	// Grid data (from RowsPage)
	OriginalHeader   []string   `json:"originalHeader"`
	Header           []string   `json:"header"`
	DisplayColumns   []int      `json:"displayColumns"`
	Rows             [][]string `json:"rows"`
	OriginalIndices  []int      `json:"originalIndices"` // Original file position for each row (0-based)
	DisplayIndices   []int      `json:"displayIndices"`  // Display position for each row (0-based)
	ReachedEnd       bool       `json:"reachedEnd"`
	Total            int        `json:"total"`
	Annotations      []bool     `json:"annotations"`
	AnnotationColors []string   `json:"annotationColors"`

	// Histogram data (from HistogramResponse or cache)
	HistogramBuckets []histogram.HistogramBucket `json:"histogramBuckets"`
	MinTs            int64                       `json:"minTs"`
	MaxTs            int64                       `json:"maxTs"`

	// Histogram metadata for async updates
	HistogramVersion string `json:"histogramVersion"` // Version string for matching async updates
	HistogramCached  bool   `json:"histogramCached"`  // True if histogram is from cache, false if pending async generation
}
