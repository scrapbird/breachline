package histogram

// HistogramBucket represents a single time bucket and its count
type HistogramBucket struct {
	// Start epoch milliseconds of the bucket start
	Start int64 `json:"start"`
	Count int   `json:"count"`
}

// HistogramResponse is returned by histogram generation functions
type HistogramResponse struct {
	Buckets []HistogramBucket `json:"buckets"`
	MinTs   int64             `json:"minTs"`
	MaxTs   int64             `json:"maxTs"`
	Version string            `json:"version"` // tab_id:version_number
}

// HistogramReadyEvent is emitted when histogram generation completes
type HistogramReadyEvent struct {
	TabID   string            `json:"tabId"`
	Version string            `json:"version"`
	Buckets []HistogramBucket `json:"buckets"`
	MinTs   int64             `json:"minTs"`
	MaxTs   int64             `json:"maxTs"`
	Error   string            `json:"error,omitempty"`
}
