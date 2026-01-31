package app

import (
	"fmt"
)

// addAnnotationsToSelectionForTab adds annotations to selected rows from a specific tab
func (a *App) addAnnotationsToSelectionForTab(tab *FileTab, req AnnotationSelectionRequest) (*AnnotationSelectionResult, error) {
	if a == nil {
		return nil, fmt.Errorf("app not initialised")
	}
	if tab == nil || tab.FilePath == "" {
		return &AnnotationSelectionResult{RowsAnnotated: 0}, nil
	}

	// Validate tab has required fields
	if tab.FileHash == "" {
		return nil, fmt.Errorf("tab missing file hash")
	}

	effectiveQuery := req.Query
	rowsAnnotated := 0

	// IMPORTANT: Use ExecuteQueryForTabWithMetadata instead of GetDataAndHistogram
	// GetDataAndHistogram applies timestamp formatting to rows, which causes column hash
	// mismatches when looking up annotations (annotations store formatted hashes, but
	// the 'annotated' query uses raw unformatted data).
	// ExecuteQueryForTabWithMetadata returns raw unformatted data that matches
	// what the query pipeline uses for annotation lookups.

	if req.VirtualSelectAll {
		// Handle virtual select all - annotate ALL rows matching the query
		// Execute query once to get all matching rows (unformatted)
		result, err := a.ExecuteQueryForTabWithMetadata(tab, effectiveQuery, req.TimeField)
		if err != nil {
			return nil, fmt.Errorf("failed to execute query: %w", err)
		}

		allRows := result.Rows
		total := len(allRows)

		// CRITICAL: Extract actual file row indices from StageResult.Rows
		// Display indices (0, 1, 2...) may differ from original file row indices when sorted/filtered
		allRowIndices := make([]int, total)
		if result.StageResult != nil && len(result.StageResult.Rows) == total {
			for i, row := range result.StageResult.Rows {
				allRowIndices[i] = row.RowIndex
			}
		} else {
			// Fallback: use display indices (won't work correctly with sorted data)
			for i := range allRows {
				allRowIndices[i] = i
			}
			a.Log("warn", "StageResult not available for bulk annotation - using display indices (may be incorrect if data is sorted)")
		}

		// Add all annotations in one call (optimized bulk operation)
		if total > 0 {
			err := a.workspaceService.AddAnnotationsWithRows(tab.FileHash, tab.Options, allRowIndices, allRows, result.OriginalHeader, req.TimeField, req.Note, req.Color, effectiveQuery)
			if err != nil {
				return nil, fmt.Errorf("failed to add annotations: %w", err)
			}
			rowsAnnotated = total
		}
	} else if len(req.Ranges) > 0 {
		// Handle specific ranges
		// Execute query once to get all matching rows (unformatted)
		result, err := a.ExecuteQueryForTabWithMetadata(tab, effectiveQuery, req.TimeField)
		if err != nil {
			return nil, fmt.Errorf("failed to execute query: %w", err)
		}

		allRows := result.Rows
		total := len(allRows)

		for _, rng := range req.Ranges {
			start, end := rng.Start, rng.End
			if start < 0 {
				start = 0
			}
			if end > total {
				end = total
			}
			if end <= start {
				continue
			}

			// Get rows for this range
			rangeRows := allRows[start:end]

			// CRITICAL: Extract actual file row indices from StageResult.Rows
			// Display indices (0, 1, 2...) may differ from original file row indices when sorted/filtered
			rangeRowIndices := make([]int, len(rangeRows))
			if result.StageResult != nil && end <= len(result.StageResult.Rows) {
				for i := 0; i < len(rangeRows); i++ {
					rangeRowIndices[i] = result.StageResult.Rows[start+i].RowIndex
				}
			} else {
				// Fallback: use indices relative to the range (won't work correctly with sorted data)
				for i := range rangeRows {
					rangeRowIndices[i] = start + i
				}
				a.Log("warn", "StageResult not available for range annotation - using display indices (may be incorrect if data is sorted)")
			}

			// Add annotations for this range using pre-fetched row data
			if len(rangeRowIndices) > 0 {
				err := a.workspaceService.AddAnnotationsWithRows(tab.FileHash, tab.Options, rangeRowIndices, rangeRows, result.OriginalHeader, req.TimeField, req.Note, req.Color, effectiveQuery)
				if err != nil {
					return nil, fmt.Errorf("failed to add annotations for range %d-%d: %w", start, end, err)
				}
				rowsAnnotated += len(rangeRowIndices)
			}
		}
	}

	a.Log("info", fmt.Sprintf("Added annotations to %d rows in tab %s", rowsAnnotated, tab.ID))
	return &AnnotationSelectionResult{RowsAnnotated: rowsAnnotated}, nil
}

// deleteAnnotationsFromSelectionForTab deletes annotations from selected rows from a specific tab
func (a *App) deleteAnnotationsFromSelectionForTab(tab *FileTab, req AnnotationSelectionRequest) (*AnnotationSelectionResult, error) {
	if a == nil {
		return nil, fmt.Errorf("app not initialised")
	}
	if tab == nil || tab.FilePath == "" {
		return &AnnotationSelectionResult{RowsAnnotated: 0}, nil
	}

	// Validate tab has required fields
	if tab.FileHash == "" {
		return nil, fmt.Errorf("tab missing file hash")
	}

	effectiveQuery := req.Query
	rowsDeleted := 0

	// IMPORTANT: Use ExecuteQueryForTabWithMetadata to get raw unformatted row data
	// This ensures column hashes match when looking up annotations to delete.

	if req.VirtualSelectAll {
		// Handle virtual select all - delete annotations from ALL rows matching the query
		// Execute query once to get all matching rows (unformatted)
		result, err := a.ExecuteQueryForTabWithMetadata(tab, effectiveQuery, req.TimeField)
		if err != nil {
			return nil, fmt.Errorf("failed to execute query: %w", err)
		}

		allRows := result.Rows
		total := len(allRows)

		// CRITICAL: Extract actual file row indices from StageResult.Rows
		// Display indices (0, 1, 2...) may differ from original file row indices when sorted/filtered
		allRowIndices := make([]int, total)
		if result.StageResult != nil && len(result.StageResult.Rows) == total {
			for i, row := range result.StageResult.Rows {
				allRowIndices[i] = row.RowIndex
			}
		} else {
			// Fallback: use display indices (won't work correctly with sorted data)
			for i := range allRows {
				allRowIndices[i] = i
			}
			a.Log("warn", "StageResult not available for bulk delete - using display indices (may be incorrect if data is sorted)")
		}

		// Delete all annotations in one call (optimized O(n) bulk deletion)
		if total > 0 {
			err := a.workspaceService.DeleteRowAnnotationsWithRows(tab.FileHash, tab.Options, allRowIndices, allRows, result.OriginalHeader, req.TimeField, effectiveQuery)
			if err != nil {
				return nil, fmt.Errorf("failed to delete annotations: %w", err)
			}
			rowsDeleted = total
		}
	} else if len(req.Ranges) > 0 {
		// Handle specific ranges
		// Execute query once to get all matching rows (unformatted)
		result, err := a.ExecuteQueryForTabWithMetadata(tab, effectiveQuery, req.TimeField)
		if err != nil {
			return nil, fmt.Errorf("failed to execute query: %w", err)
		}

		allRows := result.Rows
		total := len(allRows)

		for _, rng := range req.Ranges {
			start, end := rng.Start, rng.End
			if start < 0 {
				start = 0
			}
			if end > total {
				end = total
			}
			if end <= start {
				continue
			}

			// Get rows for this range
			rangeRows := allRows[start:end]

			// CRITICAL: Extract actual file row indices from StageResult.Rows
			rangeRowIndices := make([]int, len(rangeRows))
			if result.StageResult != nil && end <= len(result.StageResult.Rows) {
				for i := 0; i < len(rangeRows); i++ {
					rangeRowIndices[i] = result.StageResult.Rows[start+i].RowIndex
				}
			} else {
				// Fallback: use indices relative to the range (won't work correctly with sorted data)
				for i := range rangeRows {
					rangeRowIndices[i] = start + i
				}
				a.Log("warn", "StageResult not available for range delete - using display indices (may be incorrect if data is sorted)")
			}

			// Delete annotations for this range using pre-fetched row data
			if len(rangeRowIndices) > 0 {
				err := a.workspaceService.DeleteRowAnnotationsWithRows(tab.FileHash, tab.Options, rangeRowIndices, rangeRows, result.OriginalHeader, req.TimeField, effectiveQuery)
				if err != nil {
					return nil, fmt.Errorf("failed to delete annotations for range %d-%d: %w", start, end, err)
				}
				rowsDeleted += len(rangeRowIndices)
			}
		}
	}

	a.Log("info", fmt.Sprintf("Deleted annotations from %d rows in tab %s", rowsDeleted, tab.ID))
	return &AnnotationSelectionResult{RowsAnnotated: rowsDeleted}, nil
}
