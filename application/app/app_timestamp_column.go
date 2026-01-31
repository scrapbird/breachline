package app

import (
	"breachline/app/timestamps"
	"fmt"
	"io"
	"strings"
)

// ValidateTimestampColumnRequest contains the column name to validate
type ValidateTimestampColumnRequest struct {
	ColumnName string `json:"columnName"`
}

// ValidateTimestampColumnResponse contains validation result
type ValidateTimestampColumnResponse struct {
	Valid        bool   `json:"valid"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// ValidateTimestampColumn checks if a column can be used as a timestamp column
// by attempting to parse the first row's value in that column
func (a *App) ValidateTimestampColumn(columnName string) (*ValidateTimestampColumnResponse, error) {
	tab := a.GetActiveTab()
	if tab == nil {
		return &ValidateTimestampColumnResponse{
			Valid:        false,
			ErrorMessage: "No active tab",
		}, nil
	}

	if tab.FilePath == "" {
		return &ValidateTimestampColumnResponse{
			Valid:        false,
			ErrorMessage: "No file opened",
		}, nil
	}

	// Read header using readHeaderForTab which properly handles NoHeaderRow setting
	// This generates synthetic headers (unnamed_a, unnamed_b, etc.) when NoHeaderRow is true
	header, err := a.readHeaderForTab(tab)
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	// Find the column index
	columnIndex := -1
	normalizedColumnName := strings.ToLower(strings.TrimSpace(columnName))
	for i, h := range header {
		if strings.ToLower(strings.TrimSpace(h)) == normalizedColumnName {
			columnIndex = i
			break
		}
	}

	if columnIndex == -1 {
		return &ValidateTimestampColumnResponse{
			Valid:        false,
			ErrorMessage: fmt.Sprintf("Column '%s' not found in file", columnName),
		}, nil
	}

	// Read first data row - handle directories differently
	var firstRow []string
	if tab.Options.IsDirectory {
		// Use DirectoryReader for directory tabs
		dirReader, err := a.getDirectoryReaderForTab(tab)
		if err != nil {
			return nil, fmt.Errorf("failed to open directory: %w", err)
		}
		defer dirReader.Close()

		// Use the DirectoryReader's header for column lookup to ensure consistency
		// The DirectoryReader may discover different files due to MaxFiles limit
		dirHeader := dirReader.Header()
		columnIndex = -1
		for i, h := range dirHeader {
			if strings.ToLower(strings.TrimSpace(h)) == normalizedColumnName {
				columnIndex = i
				break
			}
		}
		if columnIndex == -1 {
			return &ValidateTimestampColumnResponse{
				Valid:        false,
				ErrorMessage: fmt.Sprintf("Column '%s' not found in directory files", columnName),
			}, nil
		}

		// Read rows until we find one with a non-empty value for the timestamp column
		// This is needed because files are merged with a union schema, and the first
		// files may not have the selected column
		maxRowsToCheck := 1000
		rowsChecked := 0
		var value string
		for rowsChecked < maxRowsToCheck {
			firstRow, err = dirReader.Read()
			if err != nil {
				if err == io.EOF {
					break
				}
				return nil, fmt.Errorf("failed to read row: %w", err)
			}
			rowsChecked++

			if columnIndex < len(firstRow) && firstRow[columnIndex] != "" {
				value = firstRow[columnIndex]
				break
			}
		}

		if value == "" {
			return &ValidateTimestampColumnResponse{
				Valid:        false,
				ErrorMessage: fmt.Sprintf("Column '%s' has no values in the first %d rows", columnName, rowsChecked),
			}, nil
		}

		// Validate the timestamp format
		if _, ok := timestamps.ParseTimestampMillis(value, nil); !ok {
			return &ValidateTimestampColumnResponse{
				Valid:        false,
				ErrorMessage: fmt.Sprintf("Unknown timestamp format: '%s'", value),
			}, nil
		}

		return &ValidateTimestampColumnResponse{
			Valid: true,
		}, nil
	} else {
		// Use regular reader for single files
		reader, f, err := a.getReaderForTab(tab)
		if err != nil {
			return nil, fmt.Errorf("failed to open file: %w", err)
		}
		if f != nil {
			defer f.Close()
		}

		// If file has a header row, skip it to get to the data
		// If NoHeaderRow is true, the first row is already data
		if !tab.Options.NoHeaderRow {
			if _, err := reader.Read(); err != nil {
				return nil, fmt.Errorf("failed to skip header row: %w", err)
			}
		}

		// Read first data row
		firstRow, err = reader.Read()
		if err != nil {
			if err == io.EOF {
				return &ValidateTimestampColumnResponse{
					Valid:        false,
					ErrorMessage: "File has no data rows to validate",
				}, nil
			}
			return nil, fmt.Errorf("failed to read first row: %w", err)
		}
	}

	if columnIndex >= len(firstRow) {
		return &ValidateTimestampColumnResponse{
			Valid:        false,
			ErrorMessage: "Column index out of bounds",
		}, nil
	}

	// Attempt to parse the timestamp value
	value := firstRow[columnIndex]
	if _, ok := timestamps.ParseTimestampMillis(value, nil); ok {
		// No need to return an error here, as ok is true
	} else {
		return &ValidateTimestampColumnResponse{
			Valid:        false,
			ErrorMessage: fmt.Sprintf("Unknown timestamp format: '%s'", value),
		}, nil
	}

	return &ValidateTimestampColumnResponse{
		Valid: true,
	}, nil
}

// SetTimestampColumnRequest contains the column name to set as timestamp
type SetTimestampColumnRequest struct {
	ColumnName string `json:"columnName"`
}

// SetTimestampColumnResponse contains the result of setting timestamp column
type SetTimestampColumnResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// SetTimestampColumn changes the timestamp column and expires the cache
func (a *App) SetTimestampColumn(columnName string) (*SetTimestampColumnResponse, error) {
	tab := a.GetActiveTab()
	if tab == nil {
		return &SetTimestampColumnResponse{
			Success: false,
			Message: "No active tab",
		}, nil
	}

	// Validate first to ensure it's a valid timestamp column
	validation, err := a.ValidateTimestampColumn(columnName)
	if err != nil {
		return nil, err
	}

	if !validation.Valid {
		return &SetTimestampColumnResponse{
			Success: false,
			Message: validation.ErrorMessage,
		}, nil
	}

	// Expire the cache for this tab (same logic as when settings change)
	tab.CacheMu.Lock()
	// Clear sorted rows cache
	tab.SortedRows = nil
	tab.SortedHeader = nil
	tab.SortedForFile = ""
	tab.SortedTimeField = "" // Clear the timestamp column that was used for sorting
	// Clear query cache
	tab.QueryCache = nil
	tab.QueryCacheOrder = nil
	// Broadcast to wake any waiting goroutines
	if tab.SortCond != nil {
		tab.SortCond.Broadcast()
	}
	tab.CacheMu.Unlock()

	// Cancel any in-progress sort operations
	tab.SortMu.Lock()
	if tab.SortCancel != nil {
		tab.SortCancel()
		tab.SortCancel = nil
	}
	tab.SortActive = 0
	tab.SortingForFile = ""
	tab.SortingTimeField = "" // Clear the timestamp column being used for sorting
	tab.SortMu.Unlock()

	// Invalidate global query cache entries for this file
	// This ensures timestamps are re-parsed from the new column when queries are re-executed
	if a.queryCache != nil && tab.FileHash != "" {
		invalidatedCount := a.queryCache.InvalidateFileCache(tab.FileHash)
		a.Log("debug", fmt.Sprintf("Invalidated %d query cache entries for file %s", invalidatedCount, tab.FileHash))
	}

	a.Log("info", fmt.Sprintf("Timestamp column changed to '%s' and cache expired", columnName))

	return &SetTimestampColumnResponse{
		Success: true,
		Message: fmt.Sprintf("Timestamp column set to '%s'", columnName),
	}, nil
}
