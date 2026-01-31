package app

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"breachline/app/interfaces"
)

const (
	// SearchPageSize is the number of results per page
	SearchPageSize = 1000
	// SearchMaxResults is the maximum total results to track
	SearchMaxResults = 100000
	// SnippetContextLength is the number of characters to show around a match
	SnippetContextLength = 30
)

// searchState holds the state for an in-progress search
type searchState struct {
	mu        sync.Mutex
	ctx       context.Context
	cancel    context.CancelFunc
	results   []interfaces.SearchResult
	total     int
	completed bool
	err       error
}

// activeSearches tracks in-progress searches by tab ID
var activeSearchesMu sync.Mutex
var activeSearches = make(map[string]*searchState)

// SearchInFile searches for a term in all rows of the current tab's file
// Returns paginated results with snippets showing the match context
// The query parameter filters the data to search within (pass empty string for full file)
func (a *App) SearchInFile(tabID string, searchTerm string, isRegex bool, page int, query string) (*interfaces.SearchResponse, error) {
	if a == nil {
		return nil, fmt.Errorf("app not initialized")
	}

	tab := a.GetTab(tabID)
	if tab == nil {
		return nil, fmt.Errorf("tab not found: %s", tabID)
	}

	if searchTerm == "" {
		return &interfaces.SearchResponse{
			Results:    []interfaces.SearchResult{},
			TotalCount: 0,
			Page:       page,
			PageSize:   SearchPageSize,
		}, nil
	}

	// Cancel any existing search for this tab
	a.CancelSearch(tabID)

	// Create new search context
	ctx, cancel := context.WithCancel(context.Background())
	state := &searchState{
		ctx:    ctx,
		cancel: cancel,
	}

	activeSearchesMu.Lock()
	activeSearches[tabID] = state
	activeSearchesMu.Unlock()

	// Compile regex if needed
	var re *regexp.Regexp
	var err error
	if isRegex {
		re, err = regexp.Compile(searchTerm)
		if err != nil {
			return nil, fmt.Errorf("invalid regex: %w", err)
		}
	}

	// Get the time field for this tab
	// We search within the current query results, not the raw file
	timeField := ""
	if tab.SortedTimeField != "" {
		timeField = tab.SortedTimeField
	}

	// Execute query to get current view data (use the passed query to filter results)
	result, err := a.ExecuteQueryForTabWithMetadata(tab, query, timeField)
	if err != nil {
		return nil, fmt.Errorf("failed to get file data: %w", err)
	}

	header := result.Header
	if len(result.OriginalHeader) > 0 {
		header = result.OriginalHeader
	}
	rows := result.Rows

	// Apply timestamp formatting to search data (same as grid display)
	// This ensures search operates on the same formatted values the user sees
	if a.shouldFormatTimestamps(header, timeField) {
		rows = a.formatTimestampsInRows(rows, header, timeField, tab.Options.IngestTimezoneOverride)
	}

	// Perform search
	var allResults []interfaces.SearchResult
	searchTermLower := strings.ToLower(searchTerm)

	for rowIdx, row := range rows {
		// Check for cancellation
		select {
		case <-ctx.Done():
			state.mu.Lock()
			state.completed = true
			state.mu.Unlock()
			return &interfaces.SearchResponse{
				Results:    []interfaces.SearchResult{},
				TotalCount: len(allResults),
				Page:       page,
				PageSize:   SearchPageSize,
				Cancelled:  true,
			}, nil
		default:
		}

		for colIdx, cellValue := range row {
			if cellValue == "" {
				continue
			}

			var matchStart, matchEnd int
			var found bool

			if isRegex && re != nil {
				loc := re.FindStringIndex(cellValue)
				if loc != nil {
					found = true
					matchStart = loc[0]
					matchEnd = loc[1]
				}
			} else {
				// Case-insensitive string search
				cellLower := strings.ToLower(cellValue)
				idx := strings.Index(cellLower, searchTermLower)
				if idx >= 0 {
					found = true
					matchStart = idx
					matchEnd = idx + len(searchTerm)
				}
			}

			if found {
				// Get column name
				colName := ""
				if colIdx < len(header) {
					colName = header[colIdx]
				}
				if colName == "" {
					colName = fmt.Sprintf("Column %d", colIdx+1)
				}

				// Generate snippet with context
				snippet := generateSnippet(cellValue, matchStart, matchEnd, SnippetContextLength)

				// Get display index from StageResult if available
				displayIdx := rowIdx
				if result.StageResult != nil && rowIdx < len(result.StageResult.Rows) {
					displayIdx = result.StageResult.Rows[rowIdx].DisplayIndex
					if displayIdx < 0 {
						displayIdx = rowIdx
					}
				}

				allResults = append(allResults, interfaces.SearchResult{
					RowIndex:    displayIdx,
					ColumnIndex: colIdx,
					ColumnName:  colName,
					MatchStart:  matchStart,
					MatchEnd:    matchEnd,
					Snippet:     snippet,
				})

				// Limit total results to prevent memory issues
				if len(allResults) >= SearchMaxResults {
					break
				}
			}
		}

		if len(allResults) >= SearchMaxResults {
			break
		}
	}

	// Mark search as completed
	state.mu.Lock()
	state.results = allResults
	state.total = len(allResults)
	state.completed = true
	state.mu.Unlock()

	// Calculate pagination
	totalCount := len(allResults)
	startIdx := page * SearchPageSize
	endIdx := startIdx + SearchPageSize
	if startIdx > totalCount {
		startIdx = totalCount
	}
	if endIdx > totalCount {
		endIdx = totalCount
	}

	var pageResults []interfaces.SearchResult
	if startIdx < totalCount {
		pageResults = allResults[startIdx:endIdx]
	}

	a.Log("info", fmt.Sprintf("Search completed: found %d matches for '%s' in tab %s", totalCount, searchTerm, tabID))

	return &interfaces.SearchResponse{
		Results:    pageResults,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   SearchPageSize,
		Cancelled:  false,
	}, nil
}

// GetSearchResultsPage returns a specific page of search results for a tab
// This is used for pagination after the initial search
func (a *App) GetSearchResultsPage(tabID string, page int) (*interfaces.SearchResponse, error) {
	activeSearchesMu.Lock()
	state, exists := activeSearches[tabID]
	activeSearchesMu.Unlock()

	if !exists || state == nil {
		return &interfaces.SearchResponse{
			Results:    []interfaces.SearchResult{},
			TotalCount: 0,
			Page:       page,
			PageSize:   SearchPageSize,
		}, nil
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	if !state.completed {
		return nil, fmt.Errorf("search still in progress")
	}

	// Calculate pagination
	totalCount := len(state.results)
	startIdx := page * SearchPageSize
	endIdx := startIdx + SearchPageSize
	if startIdx > totalCount {
		startIdx = totalCount
	}
	if endIdx > totalCount {
		endIdx = totalCount
	}

	var pageResults []interfaces.SearchResult
	if startIdx < totalCount {
		pageResults = state.results[startIdx:endIdx]
	}

	return &interfaces.SearchResponse{
		Results:    pageResults,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   SearchPageSize,
		Cancelled:  false,
	}, nil
}

// CancelSearch cancels any in-progress search for a tab
func (a *App) CancelSearch(tabID string) {
	activeSearchesMu.Lock()
	state, exists := activeSearches[tabID]
	if exists && state != nil {
		state.cancel()
		delete(activeSearches, tabID)
	}
	activeSearchesMu.Unlock()
}

// ClearSearchResults clears the cached search results for a tab
func (a *App) ClearSearchResults(tabID string) {
	activeSearchesMu.Lock()
	state, exists := activeSearches[tabID]
	if exists && state != nil {
		state.cancel()
		delete(activeSearches, tabID)
	}
	activeSearchesMu.Unlock()
}

// generateSnippet creates a snippet of text around a match with context
func generateSnippet(text string, matchStart, matchEnd, contextLen int) string {
	textLen := len(text)
	if textLen == 0 {
		return ""
	}

	// Calculate snippet boundaries
	snippetStart := matchStart - contextLen
	snippetEnd := matchEnd + contextLen

	if snippetStart < 0 {
		snippetStart = 0
	}
	if snippetEnd > textLen {
		snippetEnd = textLen
	}

	// Build snippet with ellipsis indicators
	var builder strings.Builder

	if snippetStart > 0 {
		builder.WriteString("…")
	}

	builder.WriteString(text[snippetStart:snippetEnd])

	if snippetEnd < textLen {
		builder.WriteString("…")
	}

	return builder.String()
}
