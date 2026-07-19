// Package mcpserver hosts an MCP (Model Context Protocol) server inside the
// BreachLine desktop app. When enabled in Settings it lets an AI client open
// files, run queries, and annotate rows in the running window, so AI-assisted
// and human-driven work share one live session.
//
// The package is self-contained: it defines the AppBridge interface it needs
// from the app and never imports the app package, so there is no import cycle.
// Read-only tools call AppBridge directly on the backend; tools that change what
// the user sees are dispatched to the frontend over a Wails event and wait for a
// reply, so every AI action is reflected in the UI.
package mcpserver

// AppBridge is the narrow, read-only surface the MCP server needs from the app.
// State-changing actions do not go through here; they are dispatched to the
// frontend (see Server.runCommand) so the visible window stays in sync.
type AppBridge interface {
	// IsLicensed reports whether a valid license is present. Workspace and
	// annotation tools are gated on this.
	IsLicensed() bool

	// ListTabs returns a summary of every open tab.
	ListTabs() []TabSummary
	// GetSchema returns the columns, detected timestamp column, and row count for a tab.
	GetSchema(tabID string) (*SchemaInfo, error)
	// GetRows runs query against a tab and returns a page of the result.
	GetRows(tabID, query string, offset, limit int) (*RowsResult, error)
	// GetHistogram returns time-bucketed counts for a tab under an optional query.
	GetHistogram(tabID, query string, bucketSeconds int) (*HistogramResult, error)

	// ListWorkspaceFiles lists the files tracked by the open workspace (license required).
	ListWorkspaceFiles() ([]WorkspaceFileSummary, error)
	// GetAnnotations returns any annotations on the given rows of a tab (license required).
	GetAnnotations(tabID string, rowIndices []int) ([]AnnotationInfo, error)
}

// TabSummary is a compact description of an open tab.
type TabSummary struct {
	TabID    string `json:"tabId"`
	FileName string `json:"fileName"`
	FilePath string `json:"filePath"`
	FileType string `json:"fileType,omitempty"`
}

// SchemaInfo describes a tab's columns and detected timestamp column.
type SchemaInfo struct {
	TabID           string   `json:"tabId"`
	Columns         []string `json:"columns"`
	TimestampColumn string   `json:"timestampColumn"`
	RowCount        int64    `json:"rowCount"`
}

// RowsResult is a page of query results.
type RowsResult struct {
	Columns    []string   `json:"columns"`
	Rows       [][]string `json:"rows"`
	TotalRows  int64      `json:"totalRows"`
	Offset     int        `json:"offset"`
	Returned   int        `json:"returned"`
	Truncated  bool       `json:"truncated"`
	AppliedTo  string     `json:"appliedTo"`
}

// HistogramBucket is a single time bucket.
type HistogramBucket struct {
	StartMillis int64 `json:"startMillis"`
	Count       int64 `json:"count"`
}

// HistogramResult is a set of time buckets.
type HistogramResult struct {
	BucketSeconds int               `json:"bucketSeconds"`
	Buckets       []HistogramBucket `json:"buckets"`
}

// WorkspaceFileSummary describes one file tracked by a workspace.
type WorkspaceFileSummary struct {
	FilePath        string `json:"filePath"`
	Description     string `json:"description,omitempty"`
	AnnotationCount int    `json:"annotationCount"`
}

// AnnotationInfo describes an annotation on a row.
type AnnotationInfo struct {
	RowIndex int    `json:"rowIndex"`
	Note     string `json:"note"`
	Color    string `json:"color"`
}
