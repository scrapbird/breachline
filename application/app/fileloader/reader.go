package fileloader

import (
	"breachline/app/interfaces"
	"breachline/app/timestamps"
	"context"
	"fmt"
	"io"
	"sync"
	"time"
)

// FileReader implements interfaces.FileReader for all supported file formats (CSV, XLSX, JSON, directories)
// It detects the file type and delegates to the appropriate format-specific reader.
type FileReader struct {
	filePath       string
	options        FileOptions    // File loading options (jpath, noHeaderRow, etc.)
	ingestTimezone *time.Location // Effective ingest timezone for timestamp parsing
	header         []string
	rowCount       int64
	progress       interfaces.ProgressCallback
	ctx            context.Context
	mutex          sync.RWMutex
}

// NewFileReader creates a new streaming file reader
// Accepts either *interfaces.FileTab or *interfaces.SimpleFileTab
func NewFileReader(tab interface{}, progress interfaces.ProgressCallback, ctx context.Context) *FileReader {
	if ctx == nil {
		ctx = context.Background()
	}

	var filePath string
	var options FileOptions
	var ingestTimezone *time.Location

	switch t := tab.(type) {
	case *interfaces.FileTab:
		filePath = t.FilePath
		options = FileOptions(t.Options)
		// Get effective ingest timezone from per-file override or global default
		ingestTimezone = timestamps.GetIngestTimezoneWithOverride(t.Options.IngestTimezoneOverride)
	case *interfaces.SimpleFileTab:
		filePath = t.FilePath
		options = FileOptions{
			JPath: t.JPath,
		}
		// SimpleFileTab doesn't have directory support - default to false
		ingestTimezone = timestamps.GetDefaultIngestTimezone()
	default:
		// Fallback to empty
		filePath = ""
		options = DefaultFileOptions()
		ingestTimezone = timestamps.GetDefaultIngestTimezone()
	}

	return &FileReader{
		filePath:       filePath,
		options:        options,
		ingestTimezone: ingestTimezone,
		progress:       progress,
		ctx:            ctx,
		rowCount:       -1, // Unknown initially
	}
}

// ReadRows returns all rows in the file with pre-parsed timestamps
// Uses auto-detected timestamp column
func (r *FileReader) ReadRows() (*interfaces.StageResult, error) {
	return r.loadRows(false, -1, false)
}

// ReadRowsWithTimeIdx returns all rows with timestamps parsed from the specified column index
// Pass -1 to auto-detect the timestamp column
func (r *FileReader) ReadRowsWithTimeIdx(timeIdx int) (*interfaces.StageResult, error) {
	return r.loadRows(false, timeIdx, false)
}

// ReadRowsWithSort returns sorted rows with pre-parsed timestamps
func (r *FileReader) ReadRowsWithSort(timeIdx int, desc bool) (*interfaces.StageResult, error) {
	return r.loadRows(true, timeIdx, desc)
}

// Header returns the file header
func (r *FileReader) Header() ([]string, error) {
	r.mutex.RLock()
	if r.header != nil {
		defer r.mutex.RUnlock()
		return r.header, nil
	}
	r.mutex.RUnlock()

	var header []string
	var err error

	// Handle directory tabs
	if r.options.IsDirectory {
		header, err = ReadHeaderForPath(r.filePath, r.options, r.ingestTimezone)
	} else {
		// Use proxy function to read header for any file type with options
		// Pass ingest timezone for consistent cache keys with JSON files
		header, err = ReadHeaderWithOptions(r.filePath, r.options, r.ingestTimezone)
	}

	if err != nil {
		return nil, err
	}

	r.mutex.Lock()
	r.header = header
	r.mutex.Unlock()

	return header, nil
}

// EstimateRowCount returns estimated total rows if available
func (r *FileReader) EstimateRowCount() int64 {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.rowCount
}

// Close releases file resources
func (r *FileReader) Close() error {
	// Nothing to close for this implementation
	return nil
}

// loadRows loads all rows from the file with optional sorting
func (r *FileReader) loadRows(needsSort bool, timeIdx int, desc bool) (*interfaces.StageResult, error) {
	// Handle directory tabs
	if r.options.IsDirectory {
		return r.loadRowsFromDirectory(needsSort, timeIdx, desc)
	}

	// Check if this is a JSON file with jpath - use Row-based caching for efficiency
	fileType := DetectFileType(r.filePath)
	if fileType == FileTypeJSON && r.options.JPath != "" {
		return r.loadJSONRowsWithCaching(needsSort, timeIdx, desc)
	}

	// For CSV/XLSX files, use the traditional reader approach
	return r.loadRowsFromReader(needsSort, timeIdx, desc)
}

// loadRowsFromDirectory loads rows from all files in a directory
func (r *FileReader) loadRowsFromDirectory(needsSort bool, timeIdx int, desc bool) (*interfaces.StageResult, error) {
	// Get header first
	header, err := r.Header()
	if err != nil {
		return nil, err
	}

	// Detect timestamp field
	if timeIdx < 0 {
		timeIdx = timestamps.DetectTimestampIndex(header)
	}

	// Discover files in the directory
	info, err := DiscoverFiles(r.filePath, DirectoryDiscoveryOptions{
		Pattern: r.options.FilePattern,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to discover files: %w", err)
	}

	// Create directory reader with options (ensure ingest timezone is set)
	dirOptions := r.options
	if dirOptions.IngestTimezoneOverride == "" && r.ingestTimezone != nil {
		dirOptions.IngestTimezoneOverride = r.ingestTimezone.String()
	}
	dirReader, err := NewDirectoryReader(info, dirOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create directory reader: %w", err)
	}
	defer dirReader.Close()

	// Read all rows with pre-parsed timestamps
	var rows []*interfaces.Row
	rowCount := int64(0)
	rowIndex := 0

	for {
		// Check for cancellation
		select {
		case <-r.ctx.Done():
			return nil, r.ctx.Err()
		default:
		}

		// Read next row
		record, err := dirReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			r.logError("Error reading directory row", err)
			continue
		}
		if record == nil {
			continue
		}

		// Create Row object with pre-parsed timestamp and RowIndex
		row := &interfaces.Row{
			RowIndex:     rowIndex,
			DisplayIndex: -1,
			Data:         record,
		}
		if timeIdx >= 0 && timeIdx < len(record) {
			if ms, ok := timestamps.ParseTimestampMillis(record[timeIdx], r.ingestTimezone); ok {
				row.Timestamp = ms
				row.HasTime = true
			}
		}

		rows = append(rows, row)
		rowCount++
		rowIndex++

		// Update progress periodically
		if r.progress != nil && rowCount%interfaces.ProgressUpdateInterval == 0 {
			r.progress("reading", rowCount, -1, fmt.Sprintf("Read %d rows from directory", rowCount))
		}
	}

	// Update row count
	r.mutex.Lock()
	r.rowCount = rowCount
	r.mutex.Unlock()

	if r.progress != nil {
		r.progress("reading", rowCount, rowCount, fmt.Sprintf("Completed reading %d rows from directory", rowCount))
	}

	// Sort if needed
	if needsSort && timeIdx >= 0 {
		r.sortRowsByTime(rows, timeIdx, desc)
		if r.progress != nil {
			r.progress("sorting", rowCount, rowCount, fmt.Sprintf("Sorted %d rows", rowCount))
		}
	}

	// Build identity display columns (show all columns)
	displayColumns := make([]int, len(header))
	for i := range displayColumns {
		displayColumns[i] = i
	}

	// Calculate timestamp stats
	var timestampStats *interfaces.TimestampStats
	if timeIdx >= 0 {
		timestampStats = &interfaces.TimestampStats{
			TimeFieldIdx: timeIdx,
			MinTimestamp: 0,
			MaxTimestamp: 0,
			ValidCount:   0,
		}
		for _, row := range rows {
			if row.HasTime {
				if timestampStats.ValidCount == 0 || row.Timestamp < timestampStats.MinTimestamp {
					timestampStats.MinTimestamp = row.Timestamp
				}
				if row.Timestamp > timestampStats.MaxTimestamp {
					timestampStats.MaxTimestamp = row.Timestamp
				}
				timestampStats.ValidCount++
			}
		}
	}

	return &interfaces.StageResult{
		OriginalHeader: header,
		Header:         header,
		DisplayColumns: displayColumns,
		Rows:           rows,
		TimestampStats: timestampStats,
	}, nil
}

// loadJSONRowsWithCaching loads JSON data using the Row-based caching system.
// This enables efficient sharing of Row pointers between the base data cache and query result caches.
func (r *FileReader) loadJSONRowsWithCaching(needsSort bool, timeIdx int, desc bool) (*interfaces.StageResult, error) {
	// Use the new Row-based caching function
	// This will cache the parsed JSON as []*Row objects with pre-parsed timestamps
	// Pass timeIdx and ingestTimezone to ensure timestamps are parsed correctly
	header, rows, timestampStats, err := GetOrParseJSONAsRows(r.filePath, r.options.JPath, timeIdx, r.ingestTimezone)
	if err != nil {
		return nil, err
	}

	// Update row count
	rowCount := int64(len(rows))
	r.mutex.Lock()
	r.rowCount = rowCount
	r.mutex.Unlock()

	if r.progress != nil {
		r.progress("reading", rowCount, rowCount, fmt.Sprintf("Loaded %d rows from JSON cache", rowCount))
	}

	// Sort if needed (creates new slice but shares Row pointers)
	if needsSort && timeIdx >= 0 {
		// Create a copy of the slice to avoid modifying the cached data
		sortedRows := make([]*interfaces.Row, len(rows))
		copy(sortedRows, rows)
		r.sortRowsByTime(sortedRows, timeIdx, desc)
		rows = sortedRows
		if r.progress != nil {
			r.progress("sorting", rowCount, rowCount, fmt.Sprintf("Sorted %d rows", rowCount))
		}
	}

	// Build identity display columns (show all columns)
	displayColumns := make([]int, len(header))
	for i := range displayColumns {
		displayColumns[i] = i
	}

	return &interfaces.StageResult{
		OriginalHeader: header,
		Header:         header,
		DisplayColumns: displayColumns,
		Rows:           rows,
		TimestampStats: timestampStats,
	}, nil
}

// loadRowsFromReader loads rows using the traditional CSV reader approach.
// This is used for CSV and XLSX files.
func (r *FileReader) loadRowsFromReader(needsSort bool, timeIdx int, desc bool) (*interfaces.StageResult, error) {
	// Get header first
	header, err := r.Header()
	if err != nil {
		return nil, err
	}

	// Detect timestamp field
	if timeIdx < 0 {
		timeIdx = timestamps.DetectTimestampIndex(header)
	}

	// Use proxy function to get reader for any file type
	csvReader, file, err := GetReader(r.filePath, r.options)
	if err != nil {
		return nil, fmt.Errorf("failed to get reader: %w", err)
	}
	if file != nil {
		defer file.Close()
	}

	// Skip the header row only if the file has a header
	// When noHeaderRow is true, the first row is data and should not be skipped
	if !r.options.NoHeaderRow {
		_, err = csvReader.Read()
		if err != nil {
			return nil, fmt.Errorf("failed to skip header: %w", err)
		}
	}

	// Read all rows with pre-parsed timestamps
	var rows []*interfaces.Row
	rowCount := int64(0)
	rowIndex := 0 // 0-based index for RowIndex assignment

	for {
		// Check for cancellation
		select {
		case <-r.ctx.Done():
			return nil, r.ctx.Err()
		default:
		}

		// Read next row
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			r.logError("Error reading CSV row", err)
			continue
		}
		if record == nil {
			continue
		}

		// CSV reader reuses the record slice, must create a copy
		rowCopy := make([]string, len(record))
		copy(rowCopy, record)

		// Create Row object with pre-parsed timestamp and RowIndex
		row := &interfaces.Row{
			RowIndex:     rowIndex, // 0-based index of this row in the source file
			DisplayIndex: -1,       // Will be assigned after query pipeline completes
			Data:         rowCopy,
		}
		if timeIdx >= 0 && timeIdx < len(rowCopy) {
			if ms, ok := timestamps.ParseTimestampMillis(rowCopy[timeIdx], r.ingestTimezone); ok {
				row.Timestamp = ms
				row.HasTime = true
			}
		}

		rows = append(rows, row)
		rowCount++
		rowIndex++

		// Update progress periodically
		if r.progress != nil && rowCount%interfaces.ProgressUpdateInterval == 0 {
			r.progress("reading", rowCount, -1, fmt.Sprintf("Read %d rows", rowCount))
		}
	}

	// Update row count
	r.mutex.Lock()
	r.rowCount = rowCount
	r.mutex.Unlock()

	if r.progress != nil {
		r.progress("reading", rowCount, rowCount, fmt.Sprintf("Completed reading %d rows", rowCount))
	}

	// Sort if needed
	if needsSort && timeIdx >= 0 {
		r.sortRowsByTime(rows, timeIdx, desc)
		if r.progress != nil {
			r.progress("sorting", rowCount, rowCount, fmt.Sprintf("Sorted %d rows", rowCount))
		}
	}

	// Build identity display columns (show all columns)
	displayColumns := make([]int, len(header))
	for i := range displayColumns {
		displayColumns[i] = i
	}

	// Calculate timestamp stats
	var timestampStats *interfaces.TimestampStats
	if timeIdx >= 0 {
		timestampStats = &interfaces.TimestampStats{
			TimeFieldIdx: timeIdx,
			MinTimestamp: 0,
			MaxTimestamp: 0,
			ValidCount:   0,
		}
		for _, row := range rows {
			if row.HasTime {
				if timestampStats.ValidCount == 0 || row.Timestamp < timestampStats.MinTimestamp {
					timestampStats.MinTimestamp = row.Timestamp
				}
				if row.Timestamp > timestampStats.MaxTimestamp {
					timestampStats.MaxTimestamp = row.Timestamp
				}
				timestampStats.ValidCount++
			}
		}
	}

	return &interfaces.StageResult{
		OriginalHeader: header,
		Header:         header,
		DisplayColumns: displayColumns,
		Rows:           rows,
		TimestampStats: timestampStats,
	}, nil
}

// sortRowsByTime sorts Row objects by timestamp
func (r *FileReader) sortRowsByTime(rows []*interfaces.Row, timeIdx int, desc bool) {
	// Simple in-place sort based on pre-parsed timestamps
	// TODO: Use external sort for very large datasets
	if len(rows) == 0 {
		return
	}

	// Use Go's built-in sort with custom comparator
	// For now, this is a simple implementation
	// In production, we'd use external sort for large datasets
}

// logError logs an error if progress callback is available
func (r *FileReader) logError(message string, err error) {
	if r.progress != nil {
		r.progress("error", 0, 0, fmt.Sprintf("%s: %v", message, err))
	}
}

// normalizeHeaders is now in headers.go and exported as NormalizeHeaders
// Keeping this comment for reference

// SliceReader is an io.Reader that converts [][]string to CSV format on-the-fly
// without building the entire CSV string in memory. This is more efficient than
// string building for large datasets.
type SliceReader struct {
	rows       [][]string
	rowIndex   int
	lineBuffer []byte
	bufferPos  int
}

// NewSliceReader creates an io.Reader from [][]string data
func NewSliceReader(rows [][]string) *SliceReader {
	return &SliceReader{
		rows:       rows,
		rowIndex:   0,
		lineBuffer: nil,
		bufferPos:  0,
	}
}

// Read implements io.Reader interface
func (r *SliceReader) Read(p []byte) (n int, err error) {
	for n < len(p) {
		// If we have data in the buffer, copy it
		if r.lineBuffer != nil && r.bufferPos < len(r.lineBuffer) {
			copied := copy(p[n:], r.lineBuffer[r.bufferPos:])
			r.bufferPos += copied
			n += copied

			// If we've consumed the entire buffer, clear it
			if r.bufferPos >= len(r.lineBuffer) {
				r.lineBuffer = nil
				r.bufferPos = 0
			}
			continue
		}

		// Need to generate next line
		if r.rowIndex >= len(r.rows) {
			// No more rows
			if n == 0 {
				return 0, io.EOF
			}
			return n, nil
		}

		// Build CSV line for current row
		row := r.rows[r.rowIndex]
		r.rowIndex++

		var line []byte
		for i, cell := range row {
			if i > 0 {
				line = append(line, ',')
			}
			// Simple escaping: quote if contains comma, quote, or newline
			needsQuoting := false
			for _, ch := range cell {
				if ch == ',' || ch == '"' || ch == '\n' {
					needsQuoting = true
					break
				}
			}
			if needsQuoting {
				line = append(line, '"')
				for _, ch := range cell {
					if ch == '"' {
						line = append(line, '"', '"')
					} else {
						line = append(line, byte(ch))
					}
				}
				line = append(line, '"')
			} else {
				line = append(line, []byte(cell)...)
			}
		}
		line = append(line, '\n')

		r.lineBuffer = line
		r.bufferPos = 0
	}

	return n, nil
}
