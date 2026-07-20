package app

import (
	"fmt"
	"math"

	"breachline/app/histogram"
	"breachline/app/interfaces"
	"breachline/app/mcpserver"
)

// MCPAppBridge adapts *App to mcpserver.AppBridge. It exposes only the
// read-only surface the MCP server answers on the backend; state-changing MCP
// actions are handled by the frontend so the visible window stays in sync.
type MCPAppBridge struct{ app *App }

// NewMCPAppBridge wraps an App for use by the MCP server.
func NewMCPAppBridge(a *App) *MCPAppBridge { return &MCPAppBridge{app: a} }

func (b *MCPAppBridge) IsLicensed() bool { return b.app.IsLicensed() }

func (b *MCPAppBridge) ListTabs() []mcpserver.TabSummary {
	tabs := b.app.GetTabs()
	out := make([]mcpserver.TabSummary, 0, len(tabs))
	for _, t := range tabs {
		out = append(out, mcpserver.TabSummary{
			TabID:    t.ID,
			FileName: t.FileName,
			FilePath: t.FilePath,
			FileType: t.DetectedFileType,
		})
	}
	return out
}

func (b *MCPAppBridge) GetSchema(tabID string) (*mcpserver.SchemaInfo, error) {
	tab := b.app.GetTab(tabID)
	if tab == nil {
		return nil, fmt.Errorf("no open tab with id %q", tabID)
	}
	res, err := b.app.ExecuteQueryForTabWithMetadata(tab, "", "")
	if err != nil {
		return nil, err
	}
	header := res.OriginalHeader
	if len(header) == 0 {
		header = res.Header
	}
	ts := ""
	if idx := b.app.DetectTimestampIndex(header); idx >= 0 && idx < len(header) {
		ts = header[idx]
	}
	return &mcpserver.SchemaInfo{
		TabID:           tabID,
		Columns:         header,
		TimestampColumn: ts,
		RowCount:        res.Total,
	}, nil
}

func (b *MCPAppBridge) GetRows(tabID, query string, offset, limit int) (*mcpserver.RowsResult, error) {
	tab := b.app.GetTab(tabID)
	if tab == nil {
		return nil, fmt.Errorf("no open tab with id %q", tabID)
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}
	res, err := b.app.ExecuteQueryForTabWithMetadata(tab, query, "")
	if err != nil {
		return nil, err
	}

	start := offset
	if start > len(res.Rows) {
		start = len(res.Rows)
	}
	end := start + limit
	if end > len(res.Rows) {
		end = len(res.Rows)
	}
	page := res.Rows[start:end]

	// Rows carry all columns; project to the display columns if the query used `columns`.
	if len(res.DisplayColumns) > 0 {
		projected := make([][]string, len(page))
		for i, row := range page {
			pr := make([]string, len(res.DisplayColumns))
			for j, ci := range res.DisplayColumns {
				if ci >= 0 && ci < len(row) {
					pr[j] = row[ci]
				}
			}
			projected[i] = pr
		}
		page = projected
	}

	columns := res.Header
	if len(columns) == 0 {
		columns = res.OriginalHeader
	}
	return &mcpserver.RowsResult{
		Columns:   columns,
		Rows:      page,
		TotalRows: res.Total,
		Offset:    offset,
		Returned:  len(page),
		Truncated: int64(offset+len(page)) < res.Total,
		AppliedTo: query,
	}, nil
}

func (b *MCPAppBridge) GetHistogram(tabID, query string, bucketSeconds int) (*mcpserver.HistogramResult, error) {
	tab := b.app.GetTab(tabID)
	if tab == nil {
		return nil, fmt.Errorf("no open tab with id %q", tabID)
	}
	res, err := b.app.ExecuteQueryForTabWithMetadata(tab, query, "")
	if err != nil {
		return nil, err
	}
	header := res.OriginalHeader
	if len(header) == 0 {
		header = res.Header
	}
	tsIdx := b.app.DetectTimestampIndex(header)
	if tsIdx < 0 {
		return nil, fmt.Errorf("no timestamp column detected for this tab")
	}

	var minTs, maxTs int64 = math.MaxInt64, math.MinInt64
	times := make([]int64, 0, len(res.Rows))
	for _, row := range res.Rows {
		if tsIdx >= len(row) {
			continue
		}
		ms, ok := b.app.ParseTimestampMillis(row[tsIdx], nil)
		if !ok {
			continue
		}
		times = append(times, ms)
		if ms < minTs {
			minTs = ms
		}
		if ms > maxTs {
			maxTs = ms
		}
	}
	if len(times) == 0 {
		return &mcpserver.HistogramResult{BucketSeconds: bucketSeconds}, nil
	}
	if bucketSeconds <= 0 {
		bucketSeconds = histogram.ChooseBucketSizeForSpan((maxTs-minTs)/1000, 200)
		if bucketSeconds <= 0 {
			bucketSeconds = 3600
		}
	}
	bw := int64(bucketSeconds) * 1000
	counts := make(map[int64]int64)
	for _, ms := range times {
		counts[(ms/bw)*bw]++
	}
	buckets := make([]mcpserver.HistogramBucket, 0)
	for start := (minTs / bw) * bw; start <= maxTs; start += bw {
		buckets = append(buckets, mcpserver.HistogramBucket{StartMillis: start, Count: counts[start]})
	}
	return &mcpserver.HistogramResult{BucketSeconds: bucketSeconds, Buckets: buckets}, nil
}

func (b *MCPAppBridge) ListWorkspaceFiles() ([]mcpserver.WorkspaceFileSummary, error) {
	files, err := b.app.GetWorkspaceFiles()
	if err != nil {
		return nil, err
	}
	out := make([]mcpserver.WorkspaceFileSummary, 0, len(files))
	for _, f := range files {
		if f == nil {
			continue
		}
		out = append(out, mcpserver.WorkspaceFileSummary{
			FilePath:        f.FilePath,
			FileHash:        f.FileHash,
			Description:     f.Description,
			AnnotationCount: f.AnnotationCount,
			Options:         fileOptionsToMCP(f.Options),
		})
	}
	return out, nil
}

// fileOptionsToMCP maps the app's FileOptions to the identity-only subset the MCP
// server round-trips through list_workspace_files and the remove/update tools.
func fileOptionsToMCP(o interfaces.FileOptions) mcpserver.WorkspaceFileOptions {
	return mcpserver.WorkspaceFileOptions{
		Jpath:                  o.JPath,
		NoHeaderRow:            o.NoHeaderRow,
		IngestTimezoneOverride: o.IngestTimezoneOverride,
		IsDirectory:            o.IsDirectory,
		FilePattern:            o.FilePattern,
		IncludeSourceColumn:    o.IncludeSourceColumn,
	}
}

func (b *MCPAppBridge) GetAnnotations(tabID string, rowIndices []int) ([]mcpserver.AnnotationInfo, error) {
	tab := b.app.GetTab(tabID)
	if tab == nil {
		return nil, fmt.Errorf("no open tab with id %q", tabID)
	}
	m, err := b.app.GetRowAnnotations(tab.FileHash, tab.Options, rowIndices, "", "")
	if err != nil {
		return nil, err
	}
	out := make([]mcpserver.AnnotationInfo, 0, len(m))
	for idx, ann := range m {
		if ann == nil {
			continue
		}
		out = append(out, mcpserver.AnnotationInfo{RowIndex: idx, Note: ann.Note, Color: ann.Color})
	}
	return out, nil
}

func (b *MCPAppBridge) GetFileAnnotations(tabID string) ([]mcpserver.FileAnnotation, error) {
	tab := b.app.GetTab(tabID)
	if tab == nil {
		return nil, fmt.Errorf("no open tab with id %q", tabID)
	}
	anns, err := b.app.GetFileAnnotations(tab.FileHash, tab.Options)
	if err != nil {
		return nil, err
	}
	out := make([]mcpserver.FileAnnotation, 0, len(anns))
	for _, a := range anns {
		if a == nil {
			continue
		}
		out = append(out, mcpserver.FileAnnotation{
			OriginalRowIndex: a.OriginalRowIndex,
			DisplayRowIndex:  a.DisplayRowIndex,
			Note:             a.Note,
			Color:            a.Color,
		})
	}
	return out, nil
}

func (b *MCPAppBridge) HasAnnotationsForFile(tabID string) (bool, error) {
	tab := b.app.GetTab(tabID)
	if tab == nil {
		return false, fmt.Errorf("no open tab with id %q", tabID)
	}
	return b.app.HasAnnotationsForFile(tab.FileHash, tab.Options), nil
}

func (b *MCPAppBridge) SearchInFile(tabID, term string, isRegex bool, query string, page int) (*mcpserver.SearchResults, error) {
	if b.app.GetTab(tabID) == nil {
		return nil, fmt.Errorf("no open tab with id %q", tabID)
	}
	if page < 0 {
		page = 0
	}
	resp, err := b.app.SearchInFile(tabID, term, isRegex, page, query)
	if err != nil {
		return nil, err
	}
	matches := make([]mcpserver.SearchMatch, 0, len(resp.Results))
	for _, r := range resp.Results {
		matches = append(matches, mcpserver.SearchMatch{
			RowIndex:   r.RowIndex,
			ColumnName: r.ColumnName,
			Snippet:    r.Snippet,
			MatchStart: r.MatchStart,
			MatchEnd:   r.MatchEnd,
		})
	}
	return &mcpserver.SearchResults{
		Matches:      matches,
		TotalMatches: resp.TotalCount,
		Returned:     len(matches),
		// The engine returns one SearchPageSize page at a time; more exist when the
		// total exceeds what this page covers.
		Truncated: resp.TotalCount > (page+1)*resp.PageSize,
	}, nil
}

func (b *MCPAppBridge) ValidateTimestampColumn(tabID, columnName string) (*mcpserver.TimestampValidation, error) {
	tab := b.app.GetTab(tabID)
	if tab == nil {
		return nil, fmt.Errorf("no open tab with id %q", tabID)
	}
	res, err := b.app.validateTimestampColumnForTab(tab, columnName)
	if err != nil {
		return nil, err
	}
	return &mcpserver.TimestampValidation{Valid: res.Valid, ErrorMessage: res.ErrorMessage}, nil
}
