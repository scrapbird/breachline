package fileloader

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os"
)

// CSV file reading and ingestion functions
// This file contains all CSV-specific operations for reading headers,
// counting rows, and creating readers for CSV files.

// ReadCSVHeader reads and returns only the header row from a CSV file using default options.
// Empty column names are normalized to unnamed_a, unnamed_b, etc.
// This is a convenience wrapper around ReadCSVHeaderWithOptions.
func ReadCSVHeader(filePath string) ([]string, error) {
	return ReadCSVHeaderWithOptions(filePath, DefaultFileOptions())
}

// ReadCSVHeaderWithOptions reads and returns the header row from a CSV file with parsing options.
// If options.NoHeaderRow is true, the first row is treated as data and synthetic headers are generated.
// Empty column names are normalized to unnamed_a, unnamed_b, etc.
func ReadCSVHeaderWithOptions(filePath string, options FileOptions) ([]string, error) {
	if filePath == "" {
		return nil, fmt.Errorf("file path is empty")
	}

	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	firstRow, err := reader.Read()
	if err != nil {
		return nil, err
	}

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

// GetCSVRowCount returns the total number of data rows in a CSV file using default options.
// This is a convenience wrapper around GetCSVRowCountWithOptions.
func GetCSVRowCount(filePath string) (int, error) {
	return GetCSVRowCountWithOptions(filePath, DefaultFileOptions())
}

// GetCSVRowCountWithOptions returns the total number of data rows in a CSV file with parsing options.
// If options.NoHeaderRow is true, all rows are counted (first row is data, not header).
// Otherwise, the header row is excluded from the count.
func GetCSVRowCountWithOptions(filePath string, options FileOptions) (int, error) {
	if filePath == "" {
		return 0, fmt.Errorf("file path is empty")
	}

	f, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	reader := csv.NewReader(f)

	// Skip header only if the file has a header row
	if !options.NoHeaderRow {
		if _, err := reader.Read(); err != nil {
			if err == io.EOF {
				return 0, nil
			}
			return 0, err
		}
	}

	count := 0
	for {
		rec, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			// Continue counting even if there's an error, as long as we got a record
			if rec == nil {
				break
			}
		}
		if rec != nil {
			count++
		}
	}
	return count, nil
}

// GetCSVReader returns a CSV reader for the specified file.
// The caller is responsible for closing the returned file handle.
// This is the CSV-specific implementation.
func GetCSVReader(filePath string) (*csv.Reader, *os.File, error) {
	if filePath == "" {
		return nil, nil, fmt.Errorf("file path is empty")
	}

	f, err := os.Open(filePath)
	if err != nil {
		return nil, nil, err
	}

	reader := csv.NewReader(f)
	// Allow variable number of fields per record to handle corrupted CSV files
	reader.FieldsPerRecord = -1
	return reader, f, nil
}

// ========== FromBytes variants for decompressed data ==========

// ReadCSVHeaderFromBytes reads and returns the header row from CSV data in memory.
// Empty column names are normalized to unnamed_a, unnamed_b, etc.
func ReadCSVHeaderFromBytes(data []byte, options FileOptions) ([]string, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("data is empty")
	}

	reader := csv.NewReader(bytes.NewReader(data))
	firstRow, err := reader.Read()
	if err != nil {
		return nil, err
	}

	var header []string
	if options.NoHeaderRow {
		// Generate synthetic headers based on column count
		emptyHeaders := make([]string, len(firstRow))
		header = NormalizeHeaders(emptyHeaders)
	} else {
		// Use first row as header
		header = NormalizeHeaders(firstRow)
	}

	return header, nil
}

// GetCSVRowCountFromBytes returns the total number of data rows from CSV data in memory.
// If options.NoHeaderRow is true, all rows are counted (first row is data, not header).
func GetCSVRowCountFromBytes(data []byte, options FileOptions) (int, error) {
	if len(data) == 0 {
		return 0, fmt.Errorf("data is empty")
	}

	reader := csv.NewReader(bytes.NewReader(data))

	// Skip header only if the file has a header row
	if !options.NoHeaderRow {
		if _, err := reader.Read(); err != nil {
			if err == io.EOF {
				return 0, nil
			}
			return 0, err
		}
	}

	count := 0
	for {
		rec, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			if rec == nil {
				break
			}
		}
		if rec != nil {
			count++
		}
	}
	return count, nil
}

// GetCSVReaderFromBytes returns a CSV reader for CSV data in memory.
// Unlike GetCSVReader, this does not return a file handle since data is in memory.
func GetCSVReaderFromBytes(data []byte) (*csv.Reader, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("data is empty")
	}

	reader := csv.NewReader(bytes.NewReader(data))
	// Allow variable number of fields per record to handle corrupted CSV files
	reader.FieldsPerRecord = -1
	return reader, nil
}
