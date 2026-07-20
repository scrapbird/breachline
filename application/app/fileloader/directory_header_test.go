package fileloader

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestDirectoryReader_DifferingSchemas verifies that a directory of CSV members
// with different headers produces the correct first-appearance union header,
// maps each row against its own file's header, counts rows across files, and
// populates the synthetic __source_file__ column at the end of the union header.
func TestDirectoryReader_DifferingSchemas(t *testing.T) {
	dir := t.TempDir()

	// a.csv: columns id,name. b.csv: columns name,age. Discovery order is
	// alphabetical, so the union by first appearance is id, name, age.
	writeFile(t, filepath.Join(dir, "a.csv"), "id,name\n1,alice\n2,carol\n")
	writeFile(t, filepath.Join(dir, "b.csv"), "name,age\nbob,30\n")

	options := FileOptions{
		FilePattern:         "*.csv",
		IncludeSourceColumn: true,
	}

	info, err := DiscoverFiles(dir, DirectoryDiscoveryOptions{Pattern: options.FilePattern}, nil)
	if err != nil {
		t.Fatalf("DiscoverFiles: %v", err)
	}
	if len(info.Files) != 2 {
		t.Fatalf("discovered %d files, want 2", len(info.Files))
	}

	dr, err := NewDirectoryReader(info, options)
	if err != nil {
		t.Fatalf("NewDirectoryReader: %v", err)
	}
	defer dr.Close()

	// Union header: first appearance across files, with source column last.
	wantHeader := []string{"id", "name", "age", "__source_file__"}
	if got := dr.Header(); !reflect.DeepEqual(got, wantHeader) {
		t.Fatalf("union header = %v, want %v", got, wantHeader)
	}

	// The per-file headers must be cached exactly once (populated by the union
	// build) and must exclude the synthetic source column.
	if len(info.Headers) != 2 {
		t.Fatalf("cached headers = %d, want 2", len(info.Headers))
	}
	for path, h := range info.Headers {
		for _, col := range h {
			if col == "__source_file__" {
				t.Fatalf("cached header for %s must not contain __source_file__: %v", path, h)
			}
		}
	}

	var rows [][]string
	for {
		row, err := dr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		rows = append(rows, row)
	}

	// 2 rows from a.csv + 1 row from b.csv.
	if len(rows) != 3 {
		t.Fatalf("row count = %d, want 3", len(rows))
	}

	// Each value must land in its own file's column position within the union.
	want := [][]string{
		{"1", "alice", "", "a.csv"},
		{"2", "carol", "", "a.csv"},
		{"", "bob", "30", "b.csv"},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("mapped rows = %v, want %v", rows, want)
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
