package app

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"breachline/app/fileloader"
	"breachline/app/interfaces"
)

// writeGzipRecord writes a gzipped CloudTrail-shaped JSON document holding one record.
func writeGzipRecord(t *testing.T, path string, id int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := fmt.Sprintf(
		`{"Records":[{"eventTime":"2026-08-17T00:%02d:00Z","eventName":"GetObject","id":"row-%03d"}]}`,
		id%60, id)

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(body)); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// newTestApp builds an App wired the way Startup wires it, minus the Wails runtime
// context (which is nil here, so nothing emits frontend events).
func newTestApp(t *testing.T) *App {
	t.Helper()
	a := NewApp()
	a.workspaceService = NewWorkspaceManager()
	fileloader.SetJSONCache(a.queryCache)
	installLoaderProviders()
	t.Cleanup(func() {
		fileloader.SetJSONCache(nil)
		fileloader.ClearDirectorySnapshots()
	})
	return a
}

// TestOpenDirectoryTab_LoadsEveryRecord covers the whole directory path through the
// app: opening a nested archive of compressed JSON, then querying it. It is the
// end-to-end guard that the snapshot reuse, seeded header and rows-native loading
// still produce a complete, correctly mapped dataset.
func TestOpenDirectoryTab_LoadsEveryRecord(t *testing.T) {
	dir := t.TempDir()
	const fileCount = 30
	for i := 0; i < fileCount; i++ {
		// Nested layout, as an S3 CloudTrail export is laid out on disk.
		path := filepath.Join(dir, "AWSLogs", "2026", fmt.Sprintf("%02d", i%5), fmt.Sprintf("%03d.json.gz", i))
		writeGzipRecord(t, path, i)
	}

	a := newTestApp(t)

	info, err := a.OpenDirectoryTabWithOptions(dir, interfaces.FileOptions{
		FilePattern:         "**/*.json.gz",
		JPath:               "$.Records",
		IncludeSourceColumn: true,
	})
	if err != nil {
		t.Fatalf("OpenDirectoryTabWithOptions: %v", err)
	}

	if info.FilesLoaded != fileCount {
		t.Fatalf("filesLoaded = %d, want %d", info.FilesLoaded, fileCount)
	}
	if info.DetectedFileType != "json" {
		t.Errorf("detectedFileType = %q, want \"json\" (compressed JSON must not be reported as CSV)", info.DetectedFileType)
	}
	if info.FileHash == "" {
		t.Error("open produced no directory hash")
	}
	if columnPos(info.Headers, "__source_file__") < 0 {
		t.Errorf("source column missing from header %v", info.Headers)
	}

	tab := a.GetActiveTab()
	if tab == nil {
		t.Fatal("no active tab after opening a directory")
	}

	header, rows, err := a.ExecuteQueryForTab(tab, "filter *", "")
	if err != nil {
		t.Fatalf("ExecuteQueryForTab: %v", err)
	}
	if len(rows) != fileCount {
		t.Fatalf("query returned %d rows, want %d (one record per member file)", len(rows), fileCount)
	}

	idIdx := columnPos(header, "id")
	if idIdx < 0 {
		t.Fatalf("id column missing from query header %v", header)
	}
	srcIdx := columnPos(header, "__source_file__")
	if srcIdx < 0 {
		t.Fatalf("source column missing from query header %v", header)
	}

	// Every record must be present exactly once, and each row must be tagged with
	// the file it came from.
	seen := make(map[string]bool, len(rows))
	for i, row := range rows {
		if len(row) != len(header) {
			t.Fatalf("row %d has %d columns, want %d", i, len(row), len(header))
		}
		id := row[idIdx]
		if seen[id] {
			t.Fatalf("row %d duplicates id %q", i, id)
		}
		seen[id] = true
		if row[srcIdx] == "" {
			t.Errorf("row %d (%s) has an empty source file", i, id)
		}
	}
	for i := 0; i < fileCount; i++ {
		want := fmt.Sprintf("row-%03d", i)
		if !seen[want] {
			t.Errorf("record %s missing from the loaded dataset", want)
		}
	}

	// A filter must narrow the result rather than reload something different.
	_, filtered, err := a.ExecuteQueryForTab(tab, "filter row-005", "")
	if err != nil {
		t.Fatalf("ExecuteQueryForTab (filtered): %v", err)
	}
	if len(filtered) != 1 {
		t.Fatalf("filtered query returned %d rows, want 1", len(filtered))
	}
}

// TestOpenDirectoryTab_ClosingTabClearsSnapshot verifies a directory reopened after
// its tab is closed sees the current contents of disk. The snapshot deliberately
// outlives individual calls, so this is what stops it outliving its usefulness.
func TestOpenDirectoryTab_ClosingTabClearsSnapshot(t *testing.T) {
	dir := t.TempDir()
	writeGzipRecord(t, filepath.Join(dir, "000.json.gz"), 0)

	a := newTestApp(t)

	opts := interfaces.FileOptions{
		FilePattern: "*.json.gz",
		JPath:       "$.Records",
	}

	first, err := a.OpenDirectoryTabWithOptions(dir, opts)
	if err != nil {
		t.Fatalf("OpenDirectoryTabWithOptions: %v", err)
	}
	if first.FilesLoaded != 1 {
		t.Fatalf("filesLoaded = %d, want 1", first.FilesLoaded)
	}

	if err := a.CloseTab(first.ID); err != nil {
		t.Fatalf("CloseTab: %v", err)
	}

	writeGzipRecord(t, filepath.Join(dir, "001.json.gz"), 1)

	second, err := a.OpenDirectoryTabWithOptions(dir, opts)
	if err != nil {
		t.Fatalf("OpenDirectoryTabWithOptions (reopen): %v", err)
	}
	if second.FilesLoaded != 2 {
		t.Fatalf("reopened directory loaded %d files, want 2 (a file added while closed was not picked up)", second.FilesLoaded)
	}
	if second.FileHash == first.FileHash {
		t.Error("directory hash unchanged after a file was added")
	}
}

// columnPos returns the index of a column in a header, or -1.
func columnPos(header []string, name string) int {
	for i, col := range header {
		if col == name {
			return i
		}
	}
	return -1
}
