package fileloader

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"breachline/app/cache"

	"github.com/xuri/excelize/v2"
)

// writeTinyXLSX writes a 3-row (1 header + 2 data) workbook to path.
func writeTinyXLSX(t *testing.T, path string) {
	t.Helper()
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	cells := [][2]string{
		{"A1", "name"}, {"B1", "timestamp"},
		{"A2", "alice"}, {"B2", "2021-01-01T00:00:00Z"},
		{"A3", "bob"}, {"B3", "2021-01-02T00:00:00Z"},
	}
	for _, c := range cells {
		if err := f.SetCellValue(sheet, c[0], c[1]); err != nil {
			t.Fatalf("SetCellValue: %v", err)
		}
	}
	if err := f.SaveAs(path); err != nil {
		t.Fatalf("SaveAs: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestGetOrParseXLSXAsRows_CacheHitAndInvalidation(t *testing.T) {
	// Install a real cache, restoring whatever was there afterwards.
	prev := getJSONCache()
	SetJSONCache(cache.NewCache(cache.DefaultCacheMaxSize))
	defer SetJSONCache(prev)

	dir := t.TempDir()
	path := filepath.Join(dir, "tiny.xlsx")
	writeTinyXLSX(t, path)

	opts := DefaultFileOptions() // NoHeaderRow = false

	// First parse (cache miss).
	header1, rows1, stats1, err := GetOrParseXLSXAsRows(path, opts, -1, nil)
	if err != nil {
		t.Fatalf("first parse: %v", err)
	}
	if want := []string{"name", "timestamp"}; len(header1) != 2 || header1[0] != want[0] || header1[1] != want[1] {
		t.Fatalf("header = %v, want %v", header1, want)
	}
	if len(rows1) != 2 {
		t.Fatalf("row count = %d, want 2", len(rows1))
	}
	if rows1[0].Data[0] != "alice" || rows1[1].Data[0] != "bob" {
		t.Fatalf("row values = %q/%q, want alice/bob", rows1[0].Data[0], rows1[1].Data[0])
	}
	if stats1 == nil || stats1.ValidCount != 2 {
		t.Fatalf("timestamp stats = %+v, want ValidCount 2", stats1)
	}

	// Second parse (cache hit) must return the identical Row pointers.
	_, rows2, _, err := GetOrParseXLSXAsRows(path, opts, -1, nil)
	if err != nil {
		t.Fatalf("second parse: %v", err)
	}
	if &rows2[0] != &rows1[0] || rows2[0] != rows1[0] {
		t.Fatalf("cache hit did not return shared Row pointers")
	}

	// Touching the file's mod-time must force a re-parse (new pointers).
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}
	_, rows3, _, err := GetOrParseXLSXAsRows(path, opts, -1, nil)
	if err != nil {
		t.Fatalf("third parse: %v", err)
	}
	if rows3[0] == rows1[0] {
		t.Fatalf("mod-time change did not invalidate the cache")
	}
}

func TestGetOrParseXLSXAsRows_NoHeaderRowDistinctEntry(t *testing.T) {
	prev := getJSONCache()
	SetJSONCache(cache.NewCache(cache.DefaultCacheMaxSize))
	defer SetJSONCache(prev)

	dir := t.TempDir()
	path := filepath.Join(dir, "tiny.xlsx")
	writeTinyXLSX(t, path)

	headered := DefaultFileOptions()
	hHeader, hRows, _, err := GetOrParseXLSXAsRows(path, headered, -1, nil)
	if err != nil {
		t.Fatalf("headered parse: %v", err)
	}
	if len(hRows) != 2 {
		t.Fatalf("headered row count = %d, want 2", len(hRows))
	}

	headerless := DefaultFileOptions()
	headerless.NoHeaderRow = true
	nHeader, nRows, _, err := GetOrParseXLSXAsRows(path, headerless, -1, nil)
	if err != nil {
		t.Fatalf("headerless parse: %v", err)
	}
	// Headerless treats the first row as data, so it has one more row.
	if len(nRows) != 3 {
		t.Fatalf("headerless row count = %d, want 3", len(nRows))
	}
	// Synthetic headers must differ from the real header, proving the two modes did not
	// collide in the cache.
	if nHeader[0] == hHeader[0] {
		t.Fatalf("headerless header[0]=%q collided with headered header[0]=%q", nHeader[0], hHeader[0])
	}
	// The headerless first data row is the original header row.
	if nRows[0].Data[0] != "name" {
		t.Fatalf("headerless first row = %q, want the original header value %q", nRows[0].Data[0], "name")
	}
}
