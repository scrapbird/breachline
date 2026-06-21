package tui

import (
	"breachline/app"
	"breachline/app/interfaces"
	"breachline/tui/adapters"
	T "breachline/tui/types"

	tea "github.com/charmbracelet/bubbletea"
)

// openFileCmd returns a command that opens a file via the backend adapter.
func openFileCmd(backend *adapters.Backend, path string, opts interfaces.FileOptions) tea.Cmd {
	return func() tea.Msg {
		info, err := backend.OpenFile(path, opts)
		if err != nil {
			return T.FileErrorMsg{FilePath: path, Err: err}
		}
		return T.FileOpenedMsg{
			TabID:    info.TabID,
			FilePath: info.FilePath,
			FileName: info.FileName,
			FileHash: info.FileHash,
			Headers:  info.Headers,
		}
	}
}

// loadRowsCmd fetches a page of rows for a tab.
func loadRowsCmd(backend *adapters.Backend, tabID string, query string, startRow int, count int) tea.Cmd {
	return func() tea.Msg {
		page, err := backend.GetRows(tabID, query, startRow, count)
		if err != nil {
			return T.RowsErrorMsg{TabID: tabID, Err: err}
		}
		rowsMsg := T.RowsLoadedMsg{
			TabID:            tabID,
			Header:           page.Header,
			Rows:             page.Rows,
			StartRow:         startRow,
			Total:            page.Total,
			ReachedEnd:       page.ReachedEnd,
			Annotations:      page.Annotations,
			AnnotationColors: page.AnnotationColors,
		}
		// Also carry histogram buckets if present (synchronous path — async path
		// uses the histogramReadyFunc callback registered on SetProgram).
		if len(page.HistogramBuckets) > 0 {
			var bucketMs int64
			if len(page.HistogramBuckets) > 1 {
				bucketMs = page.HistogramBuckets[1].Start - page.HistogramBuckets[0].Start
			}
			buckets := make([]adapters.HistogramBucket, len(page.HistogramBuckets))
			for i, b := range page.HistogramBuckets {
				buckets[i] = adapters.HistogramBucket{
					TabID:     tabID,
					StartTime: b.Start,
					EndTime:   b.Start + bucketMs,
					Count:     b.Count,
				}
			}
			histMsg := adapters.HistogramReadyMsg{TabID: tabID, Buckets: buckets}
			return tea.BatchMsg{
				func() tea.Msg { return rowsMsg },
				func() tea.Msg { return histMsg },
			}
		}
		return rowsMsg
	}
}

// searchCmd performs a search in a tab's data.
func searchCmd(backend *adapters.Backend, tabID, term string, isRegex bool, query string) tea.Cmd {
	return func() tea.Msg {
		resp, err := backend.SearchInFile(tabID, term, isRegex, 0, query)
		if err != nil {
			return searchResultMsg{err: err}
		}
		return searchResultMsg{
			totalCount: resp.TotalCount,
		}
	}
}

// searchResultMsg carries search results.
type searchResultMsg struct {
	totalCount int
	err        error
}

// annotateCmd adds annotations to selected rows.
func annotateCmd(backend *adapters.Backend, note, color, query, timeField string, ranges []app.RangeSpec) tea.Cmd {
	return func() tea.Msg {
		req := app.AnnotationSelectionRequest{
			Ranges:    ranges,
			Query:     query,
			TimeField: timeField,
			Note:      note,
			Color:     color,
		}
		result, err := backend.AddAnnotations(req)
		if err != nil {
			return T.StatusMsg{Text: "Annotation failed: " + err.Error(), Level: "error"}
		}
		return annotationDoneMsg{count: result.RowsAnnotated}
	}
}

type annotationDoneMsg struct{ count int }

// copySelectionCmd copies selected rows to clipboard.
func copySelectionCmd(backend *adapters.Backend, ranges []app.RangeSpec, headers []string, query, timeField string) tea.Cmd {
	return func() tea.Msg {
		req := app.CopySelectionRequest{
			Ranges:    ranges,
			Headers:   headers,
			Query:     query,
			TimeField: timeField,
		}
		result, err := backend.CopySelection(req)
		if err != nil {
			return T.StatusMsg{Text: "Copy failed: " + err.Error(), Level: "error"}
		}
		return copyDoneMsg{count: result.RowsCopied}
	}
}

type copyDoneMsg struct{ count int }
