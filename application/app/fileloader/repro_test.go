package fileloader

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDirectoryReader_NoHeaderRow(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "dir_reader_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a CSV file without header
	csvContent := "data1,data2,data3\nrow2_1,row2_2,row2_3\n"
	csvPath := filepath.Join(tmpDir, "test.csv")
	if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
		t.Fatalf("Failed to write CSV file: %v", err)
	}

	// Create DirectoryInfo
	info := &DirectoryInfo{
		RootPath:   tmpDir,
		Files:      []string{csvPath},
		TotalFiles: 1,
		TotalSize:  int64(len(csvContent)),
	}

	// Create options with NoHeaderRow = true
	options := FileOptions{
		NoHeaderRow: true,
		IsDirectory: true,
		FilePattern: "*.csv",
	}

	// Create DirectoryReader
	reader, err := NewDirectoryReader(info, options)
	if err != nil {
		t.Fatalf("Failed to create DirectoryReader: %v", err)
	}
	defer reader.Close()

	// Check headers
	headers := reader.Header()
	expectedHeaders := []string{"Unnamed_A", "Unnamed_B", "Unnamed_C"}
	if !reflect.DeepEqual(headers, expectedHeaders) {
		t.Errorf("Expected headers %v, got %v", expectedHeaders, headers)
	}

	// Read first row
	row, err := reader.Read()
	if err != nil {
		t.Fatalf("Failed to read first row: %v", err)
	}

	// Verify first row is the data row
	expectedRow := []string{"data1", "data2", "data3"}
	if !reflect.DeepEqual(row, expectedRow) {
		t.Errorf("Expected first row %v, got %v", expectedRow, row)
	}
}
