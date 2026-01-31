package fileloader

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"strings"

	"github.com/xuri/excelize/v2"
)

// XLSX (Excel) file reading and ingestion functions
// This file contains all Excel-specific operations for reading headers,
// counting rows, and creating readers for XLSX files.

// ReadXLSXHeader reads and returns only the header row from the first sheet of an XLSX file using default options.
// Empty column names are normalized to unnamed_a, unnamed_b, etc.
// This is a convenience wrapper around ReadXLSXHeaderWithOptions.
func ReadXLSXHeader(filePath string) ([]string, error) {
	return ReadXLSXHeaderWithOptions(filePath, DefaultFileOptions())
}

// ReadXLSXHeaderWithOptions reads and returns the header row from the first sheet of an XLSX file with parsing options.
// If options.NoHeaderRow is true, the first row is treated as data and synthetic headers are generated.
// Empty column names are normalized to unnamed_a, unnamed_b, etc.
func ReadXLSXHeaderWithOptions(filePath string, options FileOptions) ([]string, error) {
	if filePath == "" {
		return nil, fmt.Errorf("file path is empty")
	}

	f, err := excelize.OpenFile(filePath)
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

	if len(rows) == 0 {
		return nil, fmt.Errorf("no rows found in XLSX file")
	}

	firstRow := rows[0]

	var header []string
	if options.NoHeaderRow {
		// Generate synthetic headers based on column count
		// All columns will be named unnamed_a, unnamed_b, etc.
		emptyHeaders := make([]string, len(firstRow))
		header = NormalizeHeaders(emptyHeaders)
	} else {
		// Use first row as header
		header = NormalizeHeaders(firstRow)
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
func GetXLSXRowCountWithOptions(filePath string, options FileOptions) (int, error) {
	if filePath == "" {
		return 0, fmt.Errorf("file path is empty")
	}

	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	// Get the first sheet name
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return 0, fmt.Errorf("no sheets found in XLSX file")
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

// GetXLSXReader returns a CSV reader that reads from the first sheet of an XLSX file.
// The XLSX file is converted to CSV format in memory and returned as a csv.Reader.
// The caller is responsible for closing the returned file handle (which will be nil for XLSX).
// This is the XLSX-specific implementation.
func GetXLSXReader(filePath string) (*csv.Reader, *os.File, error) {
	if filePath == "" {
		return nil, nil, fmt.Errorf("file path is empty")
	}

	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	// Get the first sheet name
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, nil, fmt.Errorf("no sheets found in XLSX file")
	}
	sheetName := sheets[0]

	// Read all rows from the first sheet
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, nil, err
	}

	if len(rows) == 0 {
		return nil, nil, fmt.Errorf("no rows found in XLSX file")
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
	// Allow variable number of fields per record to handle corrupted files
	reader.FieldsPerRecord = -1
	return reader, nil, nil
}

// ========== FromBytes variants for decompressed data ==========

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
