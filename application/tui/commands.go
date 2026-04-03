package tui

import (
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
		return T.RowsLoadedMsg{
			TabID:            tabID,
			Header:           page.Header,
			Rows:             page.Rows,
			StartRow:         startRow,
			Total:            page.Total,
			ReachedEnd:       page.ReachedEnd,
			Annotations:      page.Annotations,
			AnnotationColors: page.AnnotationColors,
		}
	}
}
