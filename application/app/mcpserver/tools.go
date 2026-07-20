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

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "get_file_annotations",
		Description: "Return every annotation on a tab's file, with each row's display index under the current query (-1 if not visible). Requires a license.",
	}, s.handleGetFileAnnotations)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "has_annotations",
		Description: "Report whether a tab's file has any annotations at all (a cheap check before fetching them). Requires a license.",
	}, s.handleHasAnnotations)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "search_in_file",
		Description: "Search a tab's current query results for a literal term or regex and return matching cells with context snippets. Use this for free-text/regex matching; use get_rows/apply_query for the structured query language.",
	}, s.handleSearchInFile)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "validate_timestamp_column",
		Description: "Check whether a column on a tab parses as timestamps, without changing anything. Call before set_timestamp_column.",
	}, s.handleValidateTimestampColumn)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "get_console_log",
		Description: "Return recent entries from the app's console log (backend and UI events). Useful for diagnosing why a previous action failed.",
	}, s.handleGetConsoleLog)

	// --- State-changing tools (performed by the visible app window) ---

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "open_file",
		Description: "Open a single file (CSV, XLSX, or JSON) in a new tab. For JSON, pass jpath (e.g. $.Records).",
	}, s.handleOpenFile)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "open_directory",
		Description: "Open a directory of files as one merged, time-sorted tab. filePattern is a glob like **/*.json.gz; jpath locates records in JSON files. If the result has truncated=true, more matching files existed than the configured limit and the dataset is incomplete: warn the user to raise the directory file limit in Settings (0 = unlimited) or narrow filePattern.",
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

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "create_local_workspace",
		Description: "Create a new local .breachline workspace at the given path and open it (requires a license).",
	}, s.handleCreateLocalWorkspace)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "create_remote_workspace",
		Description: "Create a new synced remote workspace with the given name and open it (requires a license and that the user is signed in to sync).",
	}, s.handleCreateRemoteWorkspace)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "remove_file_from_workspace",
		Description: "Remove a file from the open workspace. Identify it with the fileHash and options from list_workspace_files. Requires a license.",
	}, s.handleRemoveFileFromWorkspace)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "update_file_description",
		Description: "Set the description of a file in the open workspace. Identify it with the fileHash and options from list_workspace_files. Requires a license.",
	}, s.handleUpdateFileDescription)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "set_timestamp_column",
		Description: "Change which column a tab uses as its timestamp, re-sorting and refreshing the view. Validate with validate_timestamp_column first. This switches the visible window to the tab.",
	}, s.handleSetTimestampColumn)
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

type getFileAnnotationsOut struct {
	Annotations []FileAnnotation `json:"annotations"`
}

func (s *Server) handleGetFileAnnotations(_ context.Context, _ *mcp.CallToolRequest, in tabIn) (*mcp.CallToolResult, getFileAnnotationsOut, error) {
	if !s.bridge.IsLicensed() {
		return nil, getFileAnnotationsOut{}, licenseError
	}
	anns, err := s.bridge.GetFileAnnotations(in.TabID)
	if err != nil {
		return nil, getFileAnnotationsOut{}, err
	}
	return nil, getFileAnnotationsOut{Annotations: anns}, nil
}

type hasAnnotationsOut struct {
	HasAnnotations bool `json:"hasAnnotations"`
}

func (s *Server) handleHasAnnotations(_ context.Context, _ *mcp.CallToolRequest, in tabIn) (*mcp.CallToolResult, hasAnnotationsOut, error) {
	if !s.bridge.IsLicensed() {
		return nil, hasAnnotationsOut{}, licenseError
	}
	has, err := s.bridge.HasAnnotationsForFile(in.TabID)
	if err != nil {
		return nil, hasAnnotationsOut{}, err
	}
	return nil, hasAnnotationsOut{HasAnnotations: has}, nil
}

type searchInFileIn struct {
	TabID   string `json:"tabId" jsonschema:"the id of an open tab"`
	Term    string `json:"term" jsonschema:"the text or regex pattern to search for"`
	IsRegex bool   `json:"isRegex,omitempty" jsonschema:"treat term as a regular expression (default false)"`
	Query   string `json:"query,omitempty" jsonschema:"optional query to narrow the rows searched; empty searches all rows in time order"`
	Page    int    `json:"page,omitempty" jsonschema:"result page to return (0-based, 1000 matches per page)"`
}

func (s *Server) handleSearchInFile(_ context.Context, _ *mcp.CallToolRequest, in searchInFileIn) (*mcp.CallToolResult, SearchResults, error) {
	res, err := s.bridge.SearchInFile(in.TabID, in.Term, in.IsRegex, in.Query, in.Page)
	if err != nil {
		return nil, SearchResults{}, err
	}
	return nil, *res, nil
}

type validateTimestampColumnIn struct {
	TabID      string `json:"tabId" jsonschema:"the id of an open tab"`
	ColumnName string `json:"columnName" jsonschema:"the column to test as a timestamp source"`
}

func (s *Server) handleValidateTimestampColumn(_ context.Context, _ *mcp.CallToolRequest, in validateTimestampColumnIn) (*mcp.CallToolResult, TimestampValidation, error) {
	res, err := s.bridge.ValidateTimestampColumn(in.TabID, in.ColumnName)
	if err != nil {
		return nil, TimestampValidation{}, err
	}
	return nil, *res, nil
}

type getConsoleLogIn struct {
	Limit int    `json:"limit,omitempty" jsonschema:"max entries to return, most recent last (default 100)"`
	Level string `json:"level,omitempty" jsonschema:"minimum level to include: info|warn|error (default info = all)"`
}

// ConsoleLogEntry is one line from the app console.
type ConsoleLogEntry struct {
	Timestamp int64  `json:"ts"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

type getConsoleLogOut struct {
	Entries []ConsoleLogEntry `json:"entries"`
}

func (s *Server) handleGetConsoleLog(ctx context.Context, _ *mcp.CallToolRequest, in getConsoleLogIn) (*mcp.CallToolResult, getConsoleLogOut, error) {
	var out getConsoleLogOut
	if err := s.dispatch(ctx, "get_console_log", in, &out); err != nil {
		return nil, getConsoleLogOut{}, err
	}
	return nil, out, nil
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
	// Truncated is true when the directory held more matching files than the
	// configured limit, so only FilesLoaded of them were opened and this dataset
	// is INCOMPLETE. When true, do not treat results as exhaustive: tell the user
	// to raise "Maximum files when opening directory" in Settings (0 = unlimited)
	// and reopen, or narrow the filePattern to fit within the limit.
	Truncated   bool `json:"truncated,omitempty" jsonschema:"true if more matching files existed than the limit allowed, so the loaded dataset is incomplete"`
	FilesLoaded int  `json:"filesLoaded,omitempty" jsonschema:"number of files actually loaded when truncated"`
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
	// The frontend reply carries tabId/appliedQuery but not the match count, so
	// compute it server-side with the same engine get_rows uses. The frontend just
	// ran this exact query, so this hits the pipeline cache and is cheap.
	if res, err := s.bridge.GetRows(in.TabID, in.Query, 0, 1); err == nil && res != nil {
		out.MatchedRows = res.TotalRows
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

type createLocalWorkspaceIn struct {
	Path string `json:"path" jsonschema:"absolute path for the new .breachline workspace file"`
}

func (s *Server) handleCreateLocalWorkspace(ctx context.Context, _ *mcp.CallToolRequest, in createLocalWorkspaceIn) (*mcp.CallToolResult, okOut, error) {
	if !s.bridge.IsLicensed() {
		return nil, okOut{}, licenseError
	}
	var out okOut
	if err := s.dispatch(ctx, "create_local_workspace", in, &out); err != nil {
		return nil, okOut{}, err
	}
	return nil, out, nil
}

type createRemoteWorkspaceIn struct {
	Name string `json:"name" jsonschema:"name for the new remote workspace"`
}

func (s *Server) handleCreateRemoteWorkspace(ctx context.Context, _ *mcp.CallToolRequest, in createRemoteWorkspaceIn) (*mcp.CallToolResult, okOut, error) {
	if !s.bridge.IsLicensed() {
		return nil, okOut{}, licenseError
	}
	var out okOut
	if err := s.dispatch(ctx, "create_remote_workspace", in, &out); err != nil {
		return nil, okOut{}, err
	}
	return nil, out, nil
}

// workspaceFileRef identifies a workspace entry via the fileHash and options
// returned by list_workspace_files.
type workspaceFileRef struct {
	FileHash string               `json:"fileHash" jsonschema:"the file's hash from list_workspace_files"`
	Options  WorkspaceFileOptions `json:"options" jsonschema:"the file's options from list_workspace_files (needed to disambiguate variants)"`
}

func (s *Server) handleRemoveFileFromWorkspace(ctx context.Context, _ *mcp.CallToolRequest, in workspaceFileRef) (*mcp.CallToolResult, okOut, error) {
	if !s.bridge.IsLicensed() {
		return nil, okOut{}, licenseError
	}
	var out okOut
	if err := s.dispatch(ctx, "remove_file_from_workspace", in, &out); err != nil {
		return nil, okOut{}, err
	}
	return nil, out, nil
}

type updateFileDescriptionIn struct {
	FileHash    string               `json:"fileHash" jsonschema:"the file's hash from list_workspace_files"`
	Options     WorkspaceFileOptions `json:"options" jsonschema:"the file's options from list_workspace_files"`
	Description string               `json:"description" jsonschema:"the new description (empty clears it)"`
}

func (s *Server) handleUpdateFileDescription(ctx context.Context, _ *mcp.CallToolRequest, in updateFileDescriptionIn) (*mcp.CallToolResult, okOut, error) {
	if !s.bridge.IsLicensed() {
		return nil, okOut{}, licenseError
	}
	var out okOut
	if err := s.dispatch(ctx, "update_file_description", in, &out); err != nil {
		return nil, okOut{}, err
	}
	return nil, out, nil
}

type setTimestampColumnIn struct {
	TabID      string `json:"tabId" jsonschema:"the id of an open tab"`
	ColumnName string `json:"columnName" jsonschema:"the column to use as the timestamp"`
}

// setTimestampColumnOut reports whether the change succeeded; on failure Message
// explains why (e.g. the column does not parse as timestamps).
type setTimestampColumnOut struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

func (s *Server) handleSetTimestampColumn(ctx context.Context, _ *mcp.CallToolRequest, in setTimestampColumnIn) (*mcp.CallToolResult, setTimestampColumnOut, error) {
	var out setTimestampColumnOut
	if err := s.dispatch(ctx, "set_timestamp_column", in, &out); err != nil {
		return nil, setTimestampColumnOut{}, err
	}
	return nil, out, nil
}
