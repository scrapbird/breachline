package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// licenseError is returned by workspace/annotation tools without a license.
var licenseError = errors.New("this feature requires a valid BreachLine license; import one in the app first")

// emptyIn is the input type for tools that take no arguments.
type emptyIn struct{}

// dispatch sends a state-changing action to the frontend and decodes the reply.
func (s *Server) dispatch(ctx context.Context, action string, params any, out any) error {
	raw, err := s.runCommand(ctx, action, params)
	if err != nil {
		return err
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("could not read the app's reply for %q: %w", action, err)
		}
	}
	return nil
}

func (s *Server) registerTools() {
	// --- Read-only tools (answered on the backend) ---

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "get_query_help",
		Description: "Return a cheatsheet for the BreachLine query language. Call this before writing a query.",
	}, s.handleQueryHelp)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "list_tabs",
		Description: "List the open tabs (each is one loaded file or directory) with their ids.",
	}, s.handleListTabs)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "get_schema",
		Description: "Return the columns, detected timestamp column, and total row count for a tab.",
	}, s.handleGetSchema)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "get_rows",
		Description: "Run a query against a tab and return a page of matching rows as structured data. Use apply_query first if you also want the user to see it.",
	}, s.handleGetRows)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "get_histogram",
		Description: "Return time-bucketed event counts for a tab under an optional query.",
	}, s.handleGetHistogram)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "list_workspace_files",
		Description: "List the files tracked by the open workspace (requires a license and an open workspace).",
	}, s.handleListWorkspaceFiles)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "get_annotations",
		Description: "Return any annotations on the given rows of a tab (requires a license).",
	}, s.handleGetAnnotations)

	// --- State-changing tools (performed by the visible app window) ---

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "open_file",
		Description: "Open a single file (CSV, XLSX, or JSON) in a new tab. For JSON, pass jpath (e.g. $.Records).",
	}, s.handleOpenFile)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "open_directory",
		Description: "Open a directory of files as one merged, time-sorted tab. filePattern is a glob like **/*.json.gz; jpath locates records in JSON files.",
	}, s.handleOpenDirectory)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "apply_query",
		Description: "Apply a query to a tab in the visible window (updates the search bar, grid, and histogram). See get_query_help for syntax.",
	}, s.handleApplyQuery)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "set_active_tab",
		Description: "Switch the visible window to the given tab.",
	}, s.handleSetActiveTab)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "close_tab",
		Description: "Close the given tab.",
	}, s.handleCloseTab)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "open_workspace",
		Description: "Open a .breachline workspace file by path (requires a license).",
	}, s.handleOpenWorkspace)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "close_workspace",
		Description: "Close the open workspace (requires a license).",
	}, s.handleCloseWorkspace)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "add_file_to_workspace",
		Description: "Add a file to the open workspace (requires a license and an open workspace).",
	}, s.handleAddFileToWorkspace)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "annotate_rows",
		Description: "Annotate rows of a tab with a note and color (grey|blue|yellow|green|orange|red). Requires a license and an open workspace.",
	}, s.handleAnnotateRows)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "delete_annotations",
		Description: "Remove annotations from rows of a tab (requires a license).",
	}, s.handleDeleteAnnotations)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "export_workspace_timeline",
		Description: "Export the annotated rows across the workspace to a combined timeline file (requires a license).",
	}, s.handleExportTimeline)
}

// ---------------- Read tool inputs/outputs ----------------

// QueryHelp documents the query language for the model.
type QueryHelp struct {
	Summary   string   `json:"summary"`
	Stages    []string `json:"stages"`
	Operators []string `json:"operators"`
	Examples  []string `json:"examples"`
}

func (s *Server) handleQueryHelp(_ context.Context, _ *mcp.CallToolRequest, _ emptyIn) (*mcp.CallToolResult, QueryHelp, error) {
	return nil, QueryHelp{
		Summary: "A query is stages joined by '|'. Every stage starts with a keyword. To match rows use the 'filter' stage: a bare expression on its own is NOT valid.",
		Stages: []string{
			"filter <expr> - keep matching rows (required keyword; e.g. filter eventName=ConsoleLogin)",
			"columns a, b, c - project to these columns",
			"after <time> / before <time> - time window (relative like 24h or 7d, or absolute quoted timestamp)",
			"dedup a, b - one row per unique combination",
			"limit N - cap rows",
		},
		Operators: []string{
			"field=value (equals, case-insensitive)", "field!=value", "field~value (contains)", "field!~value (not contains)",
			"field=* (field is present/non-empty)", "field=prefix* (prefix match)",
			"AND / OR / NOT and parentheses; a bare quoted term is full-text across all columns",
			"No numeric comparisons (>, <, >=, <=) are supported.",
		},
		Examples: []string{
			"filter errorCode=* | columns eventTime, eventName, errorCode",
			"filter eventName=Delete* | after 7d",
			"filter \"powershell\" AND eventName=RunInstances | dedup sourceIPAddress",
		},
	}, nil
}

type tabIn struct {
	TabID string `json:"tabId" jsonschema:"the id of an open tab (from list_tabs or an open_ result)"`
}

type listTabsOut struct {
	Tabs []TabSummary `json:"tabs"`
}

func (s *Server) handleListTabs(_ context.Context, _ *mcp.CallToolRequest, _ emptyIn) (*mcp.CallToolResult, listTabsOut, error) {
	return nil, listTabsOut{Tabs: s.bridge.ListTabs()}, nil
}

func (s *Server) handleGetSchema(_ context.Context, _ *mcp.CallToolRequest, in tabIn) (*mcp.CallToolResult, SchemaInfo, error) {
	info, err := s.bridge.GetSchema(in.TabID)
	if err != nil {
		return nil, SchemaInfo{}, err
	}
	return nil, *info, nil
}

type getRowsIn struct {
	TabID  string `json:"tabId" jsonschema:"the id of an open tab"`
	Query  string `json:"query,omitempty" jsonschema:"optional query; empty returns all rows in time order"`
	Offset int    `json:"offset,omitempty" jsonschema:"row offset into the result (default 0)"`
	Limit  int    `json:"limit,omitempty" jsonschema:"max rows to return (default 100, capped at 1000)"`
}

func (s *Server) handleGetRows(_ context.Context, _ *mcp.CallToolRequest, in getRowsIn) (*mcp.CallToolResult, RowsResult, error) {
	res, err := s.bridge.GetRows(in.TabID, in.Query, in.Offset, in.Limit)
	if err != nil {
		return nil, RowsResult{}, err
	}
	return nil, *res, nil
}

type getHistogramIn struct {
	TabID         string `json:"tabId" jsonschema:"the id of an open tab"`
	Query         string `json:"query,omitempty" jsonschema:"optional query to filter before bucketing"`
	BucketSeconds int    `json:"bucketSeconds,omitempty" jsonschema:"bucket width in seconds (0 lets the app choose)"`
}

func (s *Server) handleGetHistogram(_ context.Context, _ *mcp.CallToolRequest, in getHistogramIn) (*mcp.CallToolResult, HistogramResult, error) {
	res, err := s.bridge.GetHistogram(in.TabID, in.Query, in.BucketSeconds)
	if err != nil {
		return nil, HistogramResult{}, err
	}
	return nil, *res, nil
}

type listWorkspaceFilesOut struct {
	Files []WorkspaceFileSummary `json:"files"`
}

func (s *Server) handleListWorkspaceFiles(_ context.Context, _ *mcp.CallToolRequest, _ emptyIn) (*mcp.CallToolResult, listWorkspaceFilesOut, error) {
	if !s.bridge.IsLicensed() {
		return nil, listWorkspaceFilesOut{}, licenseError
	}
	files, err := s.bridge.ListWorkspaceFiles()
	if err != nil {
		return nil, listWorkspaceFilesOut{}, err
	}
	return nil, listWorkspaceFilesOut{Files: files}, nil
}

type getAnnotationsIn struct {
	TabID      string `json:"tabId" jsonschema:"the id of an open tab"`
	RowIndices []int  `json:"rowIndices" jsonschema:"row indices to look up"`
}

type getAnnotationsOut struct {
	Annotations []AnnotationInfo `json:"annotations"`
}

func (s *Server) handleGetAnnotations(_ context.Context, _ *mcp.CallToolRequest, in getAnnotationsIn) (*mcp.CallToolResult, getAnnotationsOut, error) {
	if !s.bridge.IsLicensed() {
		return nil, getAnnotationsOut{}, licenseError
	}
	anns, err := s.bridge.GetAnnotations(in.TabID, in.RowIndices)
	if err != nil {
		return nil, getAnnotationsOut{}, err
	}
	return nil, getAnnotationsOut{Annotations: anns}, nil
}

// ---------------- State-changing tool inputs/outputs ----------------

// OpenResult is returned by open_file and open_directory.
type OpenResult struct {
	TabID           string   `json:"tabId"`
	FileName        string   `json:"fileName"`
	Columns         []string `json:"columns"`
	TimestampColumn string   `json:"timestampColumn"`
	RowCount        int64    `json:"rowCount"`
	FileCount       int      `json:"fileCount,omitempty"`
}

type okOut struct {
	OK    bool `json:"ok"`
	Count int  `json:"count,omitempty"`
}

type openFileIn struct {
	Path           string `json:"path" jsonschema:"absolute path to the file"`
	Jpath          string `json:"jpath,omitempty" jsonschema:"JSONPath to the records for JSON files, e.g. $.Records"`
	NoHeaderRow    bool   `json:"noHeaderRow,omitempty" jsonschema:"true if a CSV has no header row"`
	IngestTimezone string `json:"ingestTimezone,omitempty" jsonschema:"timezone for timestamps without an offset, e.g. UTC"`
}

func (s *Server) handleOpenFile(ctx context.Context, _ *mcp.CallToolRequest, in openFileIn) (*mcp.CallToolResult, OpenResult, error) {
	var out OpenResult
	if err := s.dispatch(ctx, "open_file", in, &out); err != nil {
		return nil, OpenResult{}, err
	}
	return nil, out, nil
}

type openDirectoryIn struct {
	Path                string `json:"path" jsonschema:"absolute path to the directory"`
	Jpath               string `json:"jpath,omitempty" jsonschema:"JSONPath to records for JSON files, e.g. $.Records"`
	FilePattern         string `json:"filePattern" jsonschema:"glob for files to include, e.g. **/*.json.gz"`
	IncludeSourceColumn bool   `json:"includeSourceColumn,omitempty" jsonschema:"add a __source_file__ column showing each row's origin file"`
}

func (s *Server) handleOpenDirectory(ctx context.Context, _ *mcp.CallToolRequest, in openDirectoryIn) (*mcp.CallToolResult, OpenResult, error) {
	var out OpenResult
	if err := s.dispatch(ctx, "open_directory", in, &out); err != nil {
		return nil, OpenResult{}, err
	}
	return nil, out, nil
}

type applyQueryIn struct {
	TabID string `json:"tabId" jsonschema:"the id of an open tab"`
	Query string `json:"query" jsonschema:"the query to apply; see get_query_help"`
}

type applyQueryOut struct {
	TabID        string `json:"tabId"`
	AppliedQuery string `json:"appliedQuery"`
	MatchedRows  int64  `json:"matchedRows"`
}

func (s *Server) handleApplyQuery(ctx context.Context, _ *mcp.CallToolRequest, in applyQueryIn) (*mcp.CallToolResult, applyQueryOut, error) {
	var out applyQueryOut
	if err := s.dispatch(ctx, "apply_query", in, &out); err != nil {
		return nil, applyQueryOut{}, err
	}
	return nil, out, nil
}

func (s *Server) handleSetActiveTab(ctx context.Context, _ *mcp.CallToolRequest, in tabIn) (*mcp.CallToolResult, okOut, error) {
	var out okOut
	if err := s.dispatch(ctx, "set_active_tab", in, &out); err != nil {
		return nil, okOut{}, err
	}
	return nil, out, nil
}

func (s *Server) handleCloseTab(ctx context.Context, _ *mcp.CallToolRequest, in tabIn) (*mcp.CallToolResult, okOut, error) {
	var out okOut
	if err := s.dispatch(ctx, "close_tab", in, &out); err != nil {
		return nil, okOut{}, err
	}
	return nil, out, nil
}

type openWorkspaceIn struct {
	Path string `json:"path" jsonschema:"absolute path to a .breachline workspace file"`
}

func (s *Server) handleOpenWorkspace(ctx context.Context, _ *mcp.CallToolRequest, in openWorkspaceIn) (*mcp.CallToolResult, okOut, error) {
	if !s.bridge.IsLicensed() {
		return nil, okOut{}, licenseError
	}
	var out okOut
	if err := s.dispatch(ctx, "open_workspace", in, &out); err != nil {
		return nil, okOut{}, err
	}
	return nil, out, nil
}

func (s *Server) handleCloseWorkspace(ctx context.Context, _ *mcp.CallToolRequest, _ emptyIn) (*mcp.CallToolResult, okOut, error) {
	if !s.bridge.IsLicensed() {
		return nil, okOut{}, licenseError
	}
	var out okOut
	if err := s.dispatch(ctx, "close_workspace", struct{}{}, &out); err != nil {
		return nil, okOut{}, err
	}
	return nil, out, nil
}

type addFileToWorkspaceIn struct {
	Path        string `json:"path" jsonschema:"absolute path to the file to add"`
	Jpath       string `json:"jpath,omitempty" jsonschema:"JSONPath for JSON files"`
	FilePattern string `json:"filePattern,omitempty" jsonschema:"glob if adding a directory"`
	Description string `json:"description,omitempty" jsonschema:"optional note describing the file"`
}

func (s *Server) handleAddFileToWorkspace(ctx context.Context, _ *mcp.CallToolRequest, in addFileToWorkspaceIn) (*mcp.CallToolResult, okOut, error) {
	if !s.bridge.IsLicensed() {
		return nil, okOut{}, licenseError
	}
	var out okOut
	if err := s.dispatch(ctx, "add_file_to_workspace", in, &out); err != nil {
		return nil, okOut{}, err
	}
	return nil, out, nil
}

type annotateRowsIn struct {
	TabID      string `json:"tabId" jsonschema:"the id of an open tab"`
	RowIndices []int  `json:"rowIndices" jsonschema:"row indices to annotate"`
	Note       string `json:"note" jsonschema:"the annotation text"`
	Color      string `json:"color,omitempty" jsonschema:"grey|blue|yellow|green|orange|red (default grey)"`
}

func (s *Server) handleAnnotateRows(ctx context.Context, _ *mcp.CallToolRequest, in annotateRowsIn) (*mcp.CallToolResult, okOut, error) {
	if !s.bridge.IsLicensed() {
		return nil, okOut{}, licenseError
	}
	var out okOut
	if err := s.dispatch(ctx, "annotate_rows", in, &out); err != nil {
		return nil, okOut{}, err
	}
	return nil, out, nil
}

type deleteAnnotationsIn struct {
	TabID      string `json:"tabId" jsonschema:"the id of an open tab"`
	RowIndices []int  `json:"rowIndices" jsonschema:"row indices to clear"`
}

func (s *Server) handleDeleteAnnotations(ctx context.Context, _ *mcp.CallToolRequest, in deleteAnnotationsIn) (*mcp.CallToolResult, okOut, error) {
	if !s.bridge.IsLicensed() {
		return nil, okOut{}, licenseError
	}
	var out okOut
	if err := s.dispatch(ctx, "delete_annotations", in, &out); err != nil {
		return nil, okOut{}, err
	}
	return nil, out, nil
}

type exportTimelineIn struct {
	OutputPath string `json:"outputPath,omitempty" jsonschema:"absolute path to write the timeline to; omit to prompt the user"`
}

func (s *Server) handleExportTimeline(ctx context.Context, _ *mcp.CallToolRequest, in exportTimelineIn) (*mcp.CallToolResult, okOut, error) {
	if !s.bridge.IsLicensed() {
		return nil, okOut{}, licenseError
	}
	var out okOut
	if err := s.dispatch(ctx, "export_workspace_timeline", in, &out); err != nil {
		return nil, okOut{}, err
	}
	return nil, out, nil
}
