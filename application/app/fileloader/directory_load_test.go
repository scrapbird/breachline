package fileloader

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"breachline/app/cache"
)

// writeGzipJSON writes a gzipped JSON document of {"Records":[...]} to path.
func writeGzipJSON(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(body)); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// makeJSONArchive builds a directory of n gzipped JSON members, each holding one
// record. Records carry an "eventTime" and an "id"; the member at index
// extraColumnAt (when >= 0) also carries an "extraCol" field, standing in for the
// schema drift a real archive develops over time.
func makeJSONArchive(t *testing.T, dir string, n int, extraColumnAt int) {
	t.Helper()
	for i := 0; i < n; i++ {
		extra := ""
		if i == extraColumnAt {
			extra = fmt.Sprintf(`,"extraCol":"extra-%d"`, i)
		}
		body := fmt.Sprintf(
			`{"Records":[{"eventTime":"2026-08-17T00:00:%02dZ","id":"row-%03d"%s}]}`,
			i%60, i, extra)
		writeGzipJSON(t, filepath.Join(dir, fmt.Sprintf("%03d.json.gz", i)), body)
	}
}

// withSchemaSample installs a schema sample size for the duration of a test.
func withSchemaSample(t *testing.T, sample int) {
	t.Helper()
	SetDirectorySchemaSampleProvider(func() int { return sample })
	t.Cleanup(func() { SetDirectorySchemaSampleProvider(nil) })
}

// resetDirectoryCaches clears state shared between tests through package globals.
func resetDirectoryCaches(t *testing.T) {
	t.Helper()
	ClearDirectorySnapshots()
	SetJSONCache(nil)
	t.Cleanup(func() {
		ClearDirectorySnapshots()
		SetJSONCache(nil)
	})
}

// TestDirectoryLoad_ReadsEachMemberOnce is the regression guard for the reason this
// package caches directory snapshots at all: a directory open followed by a data
// load used to rescan the directory and re-parse every member for each caller that
// asked for the header, so a directory of compressed JSON was decompressed and
// parsed several times over before showing a single row.
func TestDirectoryLoad_ReadsEachMemberOnce(t *testing.T) {
	resetDirectoryCaches(t)

	// The base-data cache is what lets the row load reuse the parse the schema pass
	// already did, so the property under test only holds with a cache installed.
	SetJSONCache(cache.NewCache(64 * 1024 * 1024))

	dir := t.TempDir()
	const fileCount = 12
	makeJSONArchive(t, dir, fileCount, -1)

	options := FileOptions{
		FilePattern: "*.json.gz",
		JPath:       "$.Records",
		IsDirectory: true,
	}

	var mu sync.Mutex
	parses := make(map[string]int)
	restore := setFileParseObserver(func(path string) {
		mu.Lock()
		parses[path]++
		mu.Unlock()
	})
	defer restore()

	// The sequence a directory tab performs: resolve the directory, build the union
	// header from it, then load every row.
	info, err := GetDirectorySnapshot(context.Background(), dir, options, 0, nil)
	if err != nil {
		t.Fatalf("GetDirectorySnapshot: %v", err)
	}
	if _, err := GetDirectoryHeader(info, options); err != nil {
		t.Fatalf("GetDirectoryHeader: %v", err)
	}
	// A second snapshot request must be served from cache, exactly as the query
	// pipeline's repeated header lookups are.
	if _, err := GetDirectorySnapshot(context.Background(), dir, options, 0, nil); err != nil {
		t.Fatalf("GetDirectorySnapshot (cached): %v", err)
	}
	if _, err := ReadHeaderForPath(dir, options, nil); err != nil {
		t.Fatalf("ReadHeaderForPath: %v", err)
	}

	_, rows, err := LoadDirectoryRows(context.Background(), info, options, nil, nil)
	if err != nil {
		t.Fatalf("LoadDirectoryRows: %v", err)
	}
	if len(rows) != fileCount {
		t.Fatalf("loaded %d rows, want %d", len(rows), fileCount)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(parses) != fileCount {
		t.Fatalf("parsed %d distinct files, want %d", len(parses), fileCount)
	}
	// One parse per member for the whole sequence: the schema pass parses the file
	// and the row load reuses that parse from the base-data cache. More than one
	// means a caller is re-resolving the directory or missing the cache, which is
	// what made a large archive take minutes per query.
	for path, count := range parses {
		if count != 1 {
			t.Errorf("%s parsed %d times across open + load, want exactly 1", filepath.Base(path), count)
		}
	}
}

// TestDirectoryLoad_SampledSchemaKeepsUnsampledColumns verifies that sampling the
// schema never loses data: a column that exists only in a file outside the sample
// still appears in the final header, its value still lands in that column, and rows
// read before the column appeared are padded rather than left short.
func TestDirectoryLoad_SampledSchemaKeepsUnsampledColumns(t *testing.T) {
	resetDirectoryCaches(t)
	withSchemaSample(t, 3)

	dir := t.TempDir()
	const fileCount = 20
	const extraAt = 13 // Between sample points, so the sample cannot see it.
	makeJSONArchive(t, dir, fileCount, extraAt)

	options := FileOptions{
		FilePattern: "*.json.gz",
		JPath:       "$.Records",
		IsDirectory: true,
	}

	info, err := GetDirectorySnapshot(context.Background(), dir, options, 0, nil)
	if err != nil {
		t.Fatalf("GetDirectorySnapshot: %v", err)
	}
	if !info.SampledSchema {
		t.Fatalf("schema should have been sampled (%d of %d files read)", info.SchemaSampled, len(info.Files))
	}

	sampledHeader, err := GetDirectoryHeader(info, options)
	if err != nil {
		t.Fatalf("GetDirectoryHeader: %v", err)
	}
	if columnIndex(sampledHeader, "extraCol") >= 0 {
		t.Fatalf("sample unexpectedly saw extraCol; test no longer exercises header growth")
	}

	header, rows, err := LoadDirectoryRows(context.Background(), info, options, nil, nil)
	if err != nil {
		t.Fatalf("LoadDirectoryRows: %v", err)
	}

	extraIdx := columnIndex(header, "extraCol")
	if extraIdx < 0 {
		t.Fatalf("extraCol missing from loaded header %v: a column outside the schema sample was dropped", header)
	}
	idIdx := columnIndex(header, "id")
	if idIdx < 0 {
		t.Fatalf("id column missing from header %v", header)
	}

	if len(rows) != fileCount {
		t.Fatalf("loaded %d rows, want %d", len(rows), fileCount)
	}

	for i, row := range rows {
		if len(row) != len(header) {
			t.Fatalf("row %d has %d columns, want %d (rows read before the header grew were not padded)", i, len(row), len(header))
		}
		wantID := fmt.Sprintf("row-%03d", i)
		if row[idIdx] != wantID {
			t.Fatalf("row %d id = %q, want %q (row order or column mapping is wrong)", i, row[idIdx], wantID)
		}
		wantExtra := ""
		if i == extraAt {
			wantExtra = fmt.Sprintf("extra-%d", extraAt)
		}
		if row[extraIdx] != wantExtra {
			t.Errorf("row %d extraCol = %q, want %q", i, row[extraIdx], wantExtra)
		}
	}
}

// TestLoadDirectoryRows_MatchesSequentialRead verifies that the parallel loader
// produces exactly what a sequential DirectoryReader produces. Row order is
// user-visible (annotations index against it), so parallelism must not reorder.
func TestLoadDirectoryRows_MatchesSequentialRead(t *testing.T) {
	resetDirectoryCaches(t)

	dir := t.TempDir()
	// Mixed schemas across CSV members so the union header build, the per-file
	// mapping and the source column are all exercised.
	writeFile(t, filepath.Join(dir, "a.csv"), "id,name\n1,alice\n2,carol\n")
	writeFile(t, filepath.Join(dir, "b.csv"), "name,age\nbob,30\n")
	writeFile(t, filepath.Join(dir, "c.csv"), "id,age,city\n3,41,wellington\n")

	options := FileOptions{
		FilePattern:         "*.csv",
		IncludeSourceColumn: true,
		IsDirectory:         true,
	}

	info, err := GetDirectorySnapshot(context.Background(), dir, options, 0, nil)
	if err != nil {
		t.Fatalf("GetDirectorySnapshot: %v", err)
	}

	dr, err := NewDirectoryReader(info, options)
	if err != nil {
		t.Fatalf("NewDirectoryReader: %v", err)
	}
	var sequential [][]string
	for {
		row, err := dr.Read()
		if err != nil {
			break
		}
		sequential = append(sequential, row)
	}
	sequentialHeader := dr.Header()
	dr.Close()

	header, parallel, err := LoadDirectoryRows(context.Background(), info, options, nil, nil)
	if err != nil {
		t.Fatalf("LoadDirectoryRows: %v", err)
	}

	if !reflect.DeepEqual(header, sequentialHeader) {
		t.Fatalf("parallel header = %v, sequential header = %v", header, sequentialHeader)
	}
	if !reflect.DeepEqual(parallel, sequential) {
		t.Fatalf("parallel rows = %v, sequential rows = %v", parallel, sequential)
	}
}

// TestLoadDirectoryRows_Cancellation verifies a directory load can be abandoned.
// Without this the user has no way out of a multi-minute load short of killing the app.
func TestLoadDirectoryRows_Cancellation(t *testing.T) {
	resetDirectoryCaches(t)

	dir := t.TempDir()
	makeJSONArchive(t, dir, 40, -1)

	options := FileOptions{
		FilePattern: "*.json.gz",
		JPath:       "$.Records",
		IsDirectory: true,
	}

	info, err := GetDirectorySnapshot(context.Background(), dir, options, 0, nil)
	if err != nil {
		t.Fatalf("GetDirectorySnapshot: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, _, err := LoadDirectoryRows(ctx, info, options, nil, nil); err == nil {
		t.Fatal("LoadDirectoryRows on a cancelled context returned no error")
	} else if err != context.Canceled {
		t.Fatalf("LoadDirectoryRows error = %v, want context.Canceled", err)
	}
}

// TestDirectoryMetadataHash verifies the identity a directory is stored under:
// stable across reopens, sensitive to member changes, and never colliding with the
// legacy content hash it replaced.
func TestDirectoryMetadataHash(t *testing.T) {
	resetDirectoryCaches(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.csv"), "id,name\n1,alice\n")
	writeFile(t, filepath.Join(dir, "b.csv"), "id,name\n2,bob\n")

	discover := func() *DirectoryInfo {
		t.Helper()
		info, err := DiscoverFiles(dir, DirectoryDiscoveryOptions{Pattern: "*.csv"}, nil)
		if err != nil {
			t.Fatalf("DiscoverFiles: %v", err)
		}
		return info
	}

	first, err := CalculateDirectoryMetadataHash(discover())
	if err != nil {
		t.Fatalf("CalculateDirectoryMetadataHash: %v", err)
	}

	again, err := CalculateDirectoryMetadataHash(discover())
	if err != nil {
		t.Fatalf("CalculateDirectoryMetadataHash (repeat): %v", err)
	}
	if first != again {
		t.Fatalf("hash changed across reopen of an unchanged directory: %s then %s", first, again)
	}

	contentHash, err := CalculateDirectoryContentHash(context.Background(), discover(), nil)
	if err != nil {
		t.Fatalf("CalculateDirectoryContentHash: %v", err)
	}
	if contentHash == first {
		t.Fatal("metadata hash collided with the content hash; the two must be distinguishable when matching stored hashes")
	}

	// A changed member must change the hash. Write different content and move the
	// mtime, which is what an edited file on disk looks like.
	changed := filepath.Join(dir, "b.csv")
	writeFile(t, changed, "id,name\n2,robert\n")
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(changed, future, future); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	afterEdit, err := CalculateDirectoryMetadataHash(discover())
	if err != nil {
		t.Fatalf("CalculateDirectoryMetadataHash (after edit): %v", err)
	}
	if afterEdit == first {
		t.Fatal("hash did not change after a member file was modified")
	}
}

// TestCalculateDirectoryContentHash_MatchesLegacyOutput pins the content hash to the
// exact value the original sequential implementation produced. Workspaces recorded
// before metadata hashing store these values, and annotations are keyed by them, so
// parallelising the computation must not change the result.
func TestCalculateDirectoryContentHash_MatchesLegacyOutput(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.csv"), "id,name\n1,alice\n")
	writeFile(t, filepath.Join(dir, "b.csv"), "id,name\n2,bob\n")
	writeFile(t, filepath.Join(dir, "c.csv"), "id,name\n3,carol\n")

	info, err := DiscoverFiles(dir, DirectoryDiscoveryOptions{Pattern: "*.csv"}, nil)
	if err != nil {
		t.Fatalf("DiscoverFiles: %v", err)
	}

	got, err := CalculateDirectoryContentHash(context.Background(), info, nil)
	if err != nil {
		t.Fatalf("CalculateDirectoryContentHash: %v", err)
	}

	want, err := legacyDirectoryContentHash(info)
	if err != nil {
		t.Fatalf("legacyDirectoryContentHash: %v", err)
	}

	if got != want {
		t.Fatalf("content hash = %s, legacy sequential algorithm = %s", got, want)
	}
}

// TestSchemaSampleIndexes verifies the sample spreads across the directory rather
// than taking a prefix: archives are partitioned by date, so a prefix sample would
// only ever see the oldest members.
func TestSchemaSampleIndexes(t *testing.T) {
	cases := []struct {
		total, sample int
		want          []int
	}{
		{total: 10, sample: 0, want: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}},
		{total: 4, sample: 10, want: []int{0, 1, 2, 3}},
		{total: 10, sample: 1, want: []int{0}},
		{total: 10, sample: 3, want: []int{0, 4, 9}},
		{total: 100, sample: 5, want: []int{0, 24, 49, 74, 99}},
	}

	for _, tc := range cases {
		got := schemaSampleIndexes(tc.total, tc.sample)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("schemaSampleIndexes(%d, %d) = %v, want %v", tc.total, tc.sample, got, tc.want)
		}
	}
}

// TestDirectorySnapshot_ClearedForPath verifies a reopened directory sees disk again
// rather than a snapshot left over from a previous open.
func TestDirectorySnapshot_ClearedForPath(t *testing.T) {
	resetDirectoryCaches(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.csv"), "id,name\n1,alice\n")

	options := FileOptions{FilePattern: "*.csv", IsDirectory: true}

	first, err := GetDirectorySnapshot(context.Background(), dir, options, 0, nil)
	if err != nil {
		t.Fatalf("GetDirectorySnapshot: %v", err)
	}
	if len(first.Files) != 1 {
		t.Fatalf("discovered %d files, want 1", len(first.Files))
	}

	writeFile(t, filepath.Join(dir, "b.csv"), "id,name\n2,bob\n")

	cached, err := GetDirectorySnapshot(context.Background(), dir, options, 0, nil)
	if err != nil {
		t.Fatalf("GetDirectorySnapshot (cached): %v", err)
	}
	if len(cached.Files) != 1 {
		t.Fatalf("cached snapshot returned %d files, want the 1 captured at open", len(cached.Files))
	}

	ClearDirectorySnapshotsFor(dir)

	refreshed, err := GetDirectorySnapshot(context.Background(), dir, options, 0, nil)
	if err != nil {
		t.Fatalf("GetDirectorySnapshot (after clear): %v", err)
	}
	if len(refreshed.Files) != 2 {
		t.Fatalf("refreshed snapshot returned %d files, want 2", len(refreshed.Files))
	}
}

// columnIndex returns the position of a column in a header, or -1.
func columnIndex(header []string, name string) int {
	for i, col := range header {
		if col == name {
			return i
		}
	}
	return -1
}

// legacyDirectoryContentHash is the original sequential content-hash implementation,
// kept here so the parallel one can be pinned to its exact output.
func legacyDirectoryContentHash(info *DirectoryInfo) (string, error) {
	sortedFiles := make([]string, len(info.Files))
	copy(sortedFiles, info.Files)
	sort.Strings(sortedFiles)

	var combinedData []byte
	for _, filePath := range sortedFiles {
		fileHash, err := calculateFileContentHash(filePath)
		if err != nil {
			continue
		}
		relPath, err := filepath.Rel(info.RootPath, filePath)
		if err != nil {
			continue
		}
		combinedData = append(combinedData, fileHash...)
		combinedData = append(combinedData, []byte(relPath)...)
	}

	if len(combinedData) == 0 {
		return "", fmt.Errorf("failed to hash any files in directory")
	}

	final := sha256.Sum256(combinedData)
	return hex.EncodeToString(final[:]), nil
}
