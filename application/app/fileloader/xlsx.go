package fileloader

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"strings"
	"time"

	"breachline/app/interfaces"
	"breachline/app/timestamps"

	"github.com/xuri/excelize/v2"
)

// XLSX (Excel) file reading and ingestion functions
// This file contains all Excel-specific operations for reading headers,
// counting rows, and creating readers for XLSX files.
//
// Like JSON, XLSX uses the shared Row-based base-data cache (see json.go's
// JSONCache interface, injected via SetJSONCache) so a workbook is parsed at
// most once per (path, NoHeaderRow, timeIdx, ingest timezone) combination and
// the resulting []*interfaces.Row are shared between the base-data cache and
// query result caches. GetOrParseXLSXAsRows is the single entry point; the
// header/row-count/reader helpers below are thin wrappers over it.

// ReadXLSXHeader reads and returns only the header row from the first sheet of an XLSX file using default options.
// Empty column names are normalized to unnamed_a, unnamed_b, etc.
// This is a convenience wrapper around ReadXLSXHeaderWithOptions.
func ReadXLSXHeader(filePath string) ([]string, error) {
	return ReadXLSXHeaderWithOptions(filePath, DefaultFileOptions())
}

// ReadXLSXHeaderWithOptions reads and returns the header row from the first sheet of an XLSX file with parsing options.
// If options.NoHeaderRow is true, the first row is treated as data and synthetic headers are generated.
// Empty column names are normalized to unnamed_a, unnamed_b, etc.
// Uses the Row-based base-data cache to avoid re-parsing the workbook.
func ReadXLSXHeaderWithOptions(filePath string, options FileOptions) ([]string, error) {
	header, _, _, err := GetOrParseXLSXAsRows(filePath, options, -1, nil)
	if err != nil {
		return nil, err
	}
	return header, nil
}

// GetXLSXRowCount returns the total number of data rows in the first sheet of an XLSX file using default options.
// This is a convenience wrapper around GetXLSXRowCountWithOptions.
func GetXLSXRowCount(filePath string) (int, error) {
	return GetXLSXRowCountWithOptions(filePath, DefaultFileOptions())
}

// GetXLSXRowCountWithOptions returns the total number of data rows in the first sheet of an XLSX file with parsing options.
// If options.NoHeaderRow is true, all rows are counted (first row is data, not header).
// Otherwise, the header row is excluded from the count.
// Uses the Row-based base-data cache to avoid re-parsing the workbook.
func GetXLSXRowCountWithOptions(filePath string, options FileOptions) (int, error) {
	_, rows, _, err := GetOrParseXLSXAsRows(filePath, options, -1, nil)
	if err != nil {
		return 0, err
	}
	return len(rows), nil
}

// GetXLSXReader returns a CSV reader that reads from the first sheet of an XLSX file.
// The rows are served from the Row-based base-data cache (parsing the workbook at most
// once) and converted to CSV format on-the-fly via a SliceReader.
// The returned reader emits every source row (header row included when the file has a
// header), matching the previous full-parse behavior; callers decide whether to skip the
// header row. The returned *os.File is always nil for XLSX.
func GetXLSXReader(filePath string, options FileOptions) (*csv.Reader, *os.File, error) {
	header, rows, _, err := GetOrParseXLSXAsRows(filePath, options, -1, nil)
	if err != nil {
		return nil, nil, err
	}

	// Reconstruct the original row stream (all source rows) for reader compatibility.
	// When the file has a header row, GetOrParseXLSXAsRows split it out, so prepend it.
	// When NoHeaderRow is true, rows already contains every source row.
	stringRows := make([][]string, 0, len(rows)+1)
	if !options.NoHeaderRow {
		stringRows = append(stringRows, header)
	}
	for _, row := range rows {
		stringRows = append(stringRows, row.Data)
	}

	// Use SliceReader to convert rows to CSV format on-the-fly
	sliceReader := NewSliceReader(stringRows)
	reader := csv.NewReader(sliceReader)
	// Allow variable number of fields per record to handle corrupted files
	reader.FieldsPerRecord = -1
	return reader, nil, nil
}

// buildXLSXBaseDataCacheKey creates a cache key for XLSX base file data (Row-based).
// NoHeaderRow MUST be part of the key because it changes which rows are data vs header
// (unlike JSON, which has no header-row toggle). timeIdx and the effective ingest timezone
// are included so changing the timestamp column or timezone invalidates the cache.
// NOTE: only the first sheet is read; any future multi-sheet support must fold the sheet
// name into this key.
func buildXLSXBaseDataCacheKey(filePath string, noHeaderRow bool, timeIdx int, ingestTz *time.Location) string {
	// Use provided timezone for cache key (includes per-file overrides).
	// If nil, use the default ingest timezone from settings to ensure consistent keys.
	effectiveTz := ingestTz
	if effectiveTz == nil {
		effectiveTz = timestamps.GetDefaultIngestTimezone()
	}
	return fmt.Sprintf("basedata:xlsx:%s::noheader:%t::time:%d::tz:%s", filePath, noHeaderRow, timeIdx, effectiveTz.String())
}

// buildXLSXHeaderCacheKey creates a cache key for XLSX header-only data (timeIdx-independent).
// NoHeaderRow is part of the key because it determines whether the header is synthetic.
func buildXLSXHeaderCacheKey(filePath string, noHeaderRow bool) string {
	return fmt.Sprintf("header:xlsx:%s::noheader:%t", filePath, noHeaderRow)
}

// readXLSXAllRows opens the workbook (decompressing first for .xlsx.gz/.bz2/.xz) and
// returns every raw string row from the first sheet. This centralizes workbook parsing
// so caching keys uniformly on the real file path for both compressed and uncompressed
// XLSX, mirroring parseJSONFile.
func readXLSXAllRows(filePath string) ([][]string, error) {
	if filePath == "" {
		return nil, fmt.Errorf("file path is empty")
	}

	// Detect compression so compressed workbooks are decompressed before parsing.
	_, compression := detectFileTypeAndCompressionCached(filePath)

	var f *excelize.File
	var err error
	if compression != CompressionNone {
		result, decompressErr := DecompressFile(filePath, compression)
		if decompressErr != nil {
			return nil, fmt.Errorf("failed to decompress file: %w", decompressErr)
		}
		// Preserve incomplete-decompression warning behavior of the proxy dispatch.
		if result.Warning != "" {
			SetDecompressionWarning(filePath, result.Warning)
		}
		f, err = excelize.OpenReader(bytes.NewReader(result.Data))
	} else {
		f, err = excelize.OpenFile(filePath)
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Get the first sheet name
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("no sheets found in XLSX file")
	}
	sheetName := sheets[0]

	// Read all rows from the first sheet
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, err
	}

	return rows, nil
}

// GetOrParseXLSXAsRows retrieves cached base file data or parses the workbook, converts to
// Row objects with pre-parsed timestamps, and caches the result. This is the preferred
// method for XLSX loading as it enables efficient sharing of Row pointers between the base
// data cache and query result caches, and it parses the workbook at most once.
//
// options.NoHeaderRow controls whether the first row is treated as a header or as data; it
// is part of the cache key so headered and headerless opens of the same file never collide.
// timeIdx: the index of the timestamp column to use for parsing. Use -1 for auto-detection.
// ingestTz: the effective ingest timezone for parsing timestamps (includes per-file override).
// Returns: header, rows with pre-parsed timestamps, timestamp stats, error.
func GetOrParseXLSXAsRows(filePath string, options FileOptions, timeIdx int, ingestTz *time.Location) ([]string, []*interfaces.Row, *interfaces.TimestampStats, error) {
	if filePath == "" {
		return nil, nil, nil, fmt.Errorf("file path is empty")
	}

	cache := getJSONCache()

	// If we have a specific timeIdx (not auto-detect), check cache first
	if timeIdx >= 0 && cache != nil {
		cacheKey := buildXLSXBaseDataCacheKey(filePath, options.NoHeaderRow, timeIdx, ingestTz)
		if entry, found := cache.GetBaseData(cacheKey, filePath); found {
			// Cache hit - return Row pointers directly (no copying!)
			return entry.GetHeader(), entry.GetRows(), entry.GetTimestampStats(), nil
		}
	}

	// OPTIMIZATION: For auto-detect case (timeIdx=-1), check header cache first so we can
	// compute the timestamp column without re-parsing the whole workbook when the base data
	// for that resolved timeIdx is already cached.
	if timeIdx < 0 && cache != nil {
		headerCacheKey := buildXLSXHeaderCacheKey(filePath, options.NoHeaderRow)
		if cachedHeader, found := cache.GetHeader(headerCacheKey, filePath); found {
			effectiveTimeIdx := timestamps.DetectTimestampIndex(cachedHeader)
			cacheKey := buildXLSXBaseDataCacheKey(filePath, options.NoHeaderRow, effectiveTimeIdx, ingestTz)
			if entry, found := cache.GetBaseData(cacheKey, filePath); found {
				// Cache hit - return Row pointers directly (no copying!)
				return entry.GetHeader(), entry.GetRows(), entry.GetTimestampStats(), nil
			}
		}
	}

	// Cache miss - parse the workbook once
	allRows, err := readXLSXAllRows(filePath)
	if err != nil {
		return nil, nil, nil, err
	}

	// Split header vs data rows honoring NoHeaderRow, exactly as the previous
	// header/count/reader code did.
	var header []string
	var dataRows [][]string
	if len(allRows) == 0 {
		// Empty sheet: no header, no data.
		header = []string{}
		dataRows = nil
	} else if options.NoHeaderRow {
		// Generate synthetic headers based on column count; every row is data.
		emptyHeaders := make([]string, len(allRows[0]))
		header = NormalizeHeaders(emptyHeaders)
		dataRows = allRows
	} else {
		// First row is the header; the rest are data.
		header = NormalizeHeaders(allRows[0])
		dataRows = allRows[1:]
	}

	// Store header in cache for future auto-detect calls
	if cache != nil {
		headerCacheKey := buildXLSXHeaderCacheKey(filePath, options.NoHeaderRow)
		cache.StoreHeader(headerCacheKey, filePath, header)
	}

	// Resolve timeIdx: use provided value or auto-detect
	effectiveTimeIdx := timeIdx
	if effectiveTimeIdx < 0 {
		effectiveTimeIdx = timestamps.DetectTimestampIndex(header)
	}

	// Check if we already have base data cached with the resolved timeIdx
	// (This handles the case where header was cached but base data wasn't)
	if cache != nil {
		cacheKey := buildXLSXBaseDataCacheKey(filePath, options.NoHeaderRow, effectiveTimeIdx, ingestTz)
		if entry, found := cache.GetBaseData(cacheKey, filePath); found {
			// Cache hit - return Row pointers directly (no copying!)
			return entry.GetHeader(), entry.GetRows(), entry.GetTimestampStats(), nil
		}
	}

	// Cache miss - build rows from the already-parsed data (ONCE)
	timeFieldIdx := effectiveTimeIdx
	rows := make([]*interfaces.Row, 0, len(dataRows))
	var timestampStats *interfaces.TimestampStats

	if timeFieldIdx >= 0 {
		timestampStats = &interfaces.TimestampStats{
			TimeFieldIdx: timeFieldIdx,
			MinTimestamp: 0,
			MaxTimestamp: 0,
			ValidCount:   0,
		}
	}

	for i, record := range dataRows {
		row := &interfaces.Row{
			RowIndex:     i,  // 0-based index of this row in the data stream
			DisplayIndex: -1, // Will be assigned after query pipeline completes
			Data:         record,
		}

		// Parse timestamp if time field exists
		if timeFieldIdx >= 0 && timeFieldIdx < len(record) {
			if ms, ok := timestamps.ParseTimestampMillis(record[timeFieldIdx], ingestTz); ok {
				row.Timestamp = ms
				row.HasTime = true

				// Track timestamp stats
				if timestampStats != nil {
					if timestampStats.ValidCount == 0 || ms < timestampStats.MinTimestamp {
						timestampStats.MinTimestamp = ms
					}
					if ms > timestampStats.MaxTimestamp {
						timestampStats.MaxTimestamp = ms
					}
					timestampStats.ValidCount++
				}
			}
		}

		rows = append(rows, row)
	}

	// Store in cache with the resolved timeIdx
	if cache != nil {
		cacheKey := buildXLSXBaseDataCacheKey(filePath, options.NoHeaderRow, effectiveTimeIdx, ingestTz)
		cache.StoreBaseData(cacheKey, filePath, header, rows, timestampStats)
	}

	return header, rows, timestampStats, nil
}

// ========== FromBytes variants for decompressed data ==========
//
// These uncached in-memory variants are retained as fallbacks. The proxy dispatch now
// routes all XLSX (compressed or not) through GetOrParseXLSXAsRows, which decompresses
// internally, so these are no longer on the hot path.

// ReadXLSXHeaderFromBytes reads and returns the header row from XLSX data in memory.
// Empty column names are normalized to unnamed_a, unnamed_b, etc.
func ReadXLSXHeaderFromBytes(data []byte, options FileOptions) ([]string, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("data is empty")
	}

	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Get the first sheet name
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("no sheets found in XLSX data")
	}
	sheetName := sheets[0]

	// Read all rows from the first sheet
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("no rows found in XLSX data")
	}

	firstRow := rows[0]

	var header []string
	if options.NoHeaderRow {
		emptyHeaders := make([]string, len(firstRow))
		header = NormalizeHeaders(emptyHeaders)
	} else {
		header = NormalizeHeaders(firstRow)
	}

	return header, nil
}

// GetXLSXRowCountFromBytes returns the total number of data rows from XLSX data in memory.
// If options.NoHeaderRow is true, all rows are counted (first row is data, not header).
func GetXLSXRowCountFromBytes(data []byte, options FileOptions) (int, error) {
	if len(data) == 0 {
		return 0, fmt.Errorf("data is empty")
	}

	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return 0, err
	}
	defer f.Close()

	// Get the first sheet name
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return 0, fmt.Errorf("no sheets found in XLSX data")
	}
	sheetName := sheets[0]

	// Read all rows from the first sheet
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return 0, err
	}

	if len(rows) == 0 {
		return 0, nil
	}

	// If file has a header row, subtract 1 for the header
	if !options.NoHeaderRow {
		if len(rows) <= 1 {
			return 0, nil
		}
		return len(rows) - 1, nil
	}

	// No header row - all rows are data
	return len(rows), nil
}

// GetXLSXReaderFromBytes returns a CSV reader that reads from XLSX data in memory.
// The XLSX data is converted to CSV format in memory and returned as a csv.Reader.
func GetXLSXReaderFromBytes(data []byte) (*csv.Reader, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("data is empty")
	}

	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Get the first sheet name
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("no sheets found in XLSX data")
	}
	sheetName := sheets[0]

	// Read all rows from the first sheet
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("no rows found in XLSX data")
	}

	// Convert rows to CSV format in memory
	var sb strings.Builder
	for _, row := range rows {
		for i, cell := range row {
			if i > 0 {
				sb.WriteString(",")
			}
			// Escape quotes and wrap in quotes if necessary
			if strings.ContainsAny(cell, ",\"\n") {
				sb.WriteString("\"")
				sb.WriteString(strings.ReplaceAll(cell, "\"", "\"\""))
				sb.WriteString("\"")
			} else {
				sb.WriteString(cell)
			}
		}
		sb.WriteString("\n")
	}

	// Create a CSV reader from the string
	reader := csv.NewReader(strings.NewReader(sb.String()))
	reader.FieldsPerRecord = -1
	return reader, nil
}
