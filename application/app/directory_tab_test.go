package app

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// recordedEvent is one load lifecycle event as seen by the frontend.
type recordedEvent struct {
	name  string
	kind  string
	phase string
}

// recordDirectoryEvents captures the load lifecycle events emitted during a test
// and returns an accessor for them.
func recordDirectoryEvents(t *testing.T) func() []recordedEvent {
	t.Helper()

	var mu sync.Mutex
	var events []recordedEvent

	previous := directoryEventObserver
	directoryEventObserver = func(name string, payload map[string]interface{}) {
		e := recordedEvent{name: name}
		if kind, ok := payload["kind"].(string); ok {
			e.kind = kind
		}
		if phase, ok := payload["phase"].(string); ok {
			e.phase = phase
		}
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	}
	t.Cleanup(func() { directoryEventObserver = previous })

	return func() []recordedEvent {
		mu.Lock()
		defer mu.Unlock()
		out := make([]recordedEvent, len(events))
		copy(out, events)
		return out
	}
}

// eventNames extracts just the event names, for sequence assertions.
func eventNames(events []recordedEvent) []string {
	names := make([]string, len(events))
	for i, e := range events {
		names[i] = e.name
	}
	return names
}

// reportedPhases extracts the phases reported for one load kind.
func reportedPhases(events []recordedEvent, kind string) []string {
	var phases []string
	for _, e := range events {
		if e.name == loadProgressEvent && e.kind == kind {
			phases = append(phases, e.phase)
		}
	}
	return phases
}

// assertProgressSequencesClosed checks that every run of progress events is
// terminated by a done event. The frontend shows a modal progress dialog while a
// sequence is open, so an unterminated sequence means a dialog the user cannot get
// rid of.
func assertProgressSequencesClosed(t *testing.T, events []recordedEvent) {
	t.Helper()

	open := false
	for _, e := range eventNames(events) {
		switch e {
		case loadProgressEvent:
			open = true
		case loadDoneEvent:
			open = false
		}
	}
	if open {
		t.Fatalf("progress sequence never closed; events: %v", eventNames(events))
	}
}

// TestDirectoryProgressSequenceAlwaysCloses covers the reported bug: the rows of a
// directory are read during the first query after the open, and that read reported
// progress without ever reporting completion, so the progress dialog stayed up
// after loading had finished.
func TestDirectoryProgressSequenceAlwaysCloses(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 8; i++ {
		writeGzipRecord(t, filepath.Join(dir, fmt.Sprintf("%03d.json.gz", i)), i)
	}

	a := newTestApp(t)
	events := recordDirectoryEvents(t)

	opts := interfaces.FileOptions{
		FilePattern: "*.json.gz",
		JPath:       "$.Records",
	}

	info, err := a.OpenDirectoryTabWithOptions(dir, opts)
	if err != nil {
		t.Fatalf("OpenDirectoryTabWithOptions: %v", err)
	}
	assertProgressSequencesClosed(t, events())

	afterOpen := eventNames(events())
	if len(afterOpen) == 0 || afterOpen[len(afterOpen)-1] != loadDoneEvent {
		t.Fatalf("open did not end with a done event; events: %v", afterOpen)
	}

	// The load happens here. This is the sequence that used to be left open.
	tab := a.GetActiveTab()
	if tab == nil {
		t.Fatal("no active tab")
	}
	if _, _, err := a.ExecuteQueryForTab(tab, "filter *", ""); err != nil {
		t.Fatalf("ExecuteQueryForTab: %v", err)
	}

	afterQuery := events()
	assertProgressSequencesClosed(t, afterQuery)
	if names := eventNames(afterQuery); names[len(names)-1] != loadDoneEvent {
		t.Fatalf("query did not end with a done event; events: %v", names)
	}

	_ = info
}

// TestDirectoryProgressClosesOnFailedOpen verifies a failed open also takes the
// progress dialog down. The done event used to sit on the success path only, so
// every error return left it on screen.
func TestDirectoryProgressClosesOnFailedOpen(t *testing.T) {
	a := newTestApp(t)
	events := recordDirectoryEvents(t)

	// No files match, so the open fails after reporting discovery progress.
	dir := t.TempDir()
	if _, err := a.OpenDirectoryTabWithOptions(dir, interfaces.FileOptions{
		FilePattern: "*.json.gz",
		JPath:       "$.Records",
	}); err == nil {
		t.Fatal("expected an error opening a directory with no matching files")
	}

	got := events()
	assertProgressSequencesClosed(t, got)
	names := eventNames(got)
	if len(names) == 0 || names[len(names)-1] != loadDoneEvent {
		t.Fatalf("failed open did not end with a done event; events: %v", names)
	}
}

// TestSingleFileOpenReportsProgress verifies that opening one file reports phases
// and closes its sequence, the same contract a directory open follows. Opening a
// large JSON or XLSX file parses the whole thing up front, so without this the UI
// has nothing to show for the entire wait.
func TestSingleFileOpenReportsProgress(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.json.gz")

	// Enough records that the row-building loops cross a progress reporting
	// interval and actually emit.
	var sb strings.Builder
	sb.WriteString(`{"Records":[`)
	for i := 0; i < 9000; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, `{"eventTime":"2026-08-17T00:00:00Z","id":"row-%05d"}`, i)
	}
	sb.WriteString("]}")
	writeGzipBody(t, path, sb.String())

	a := newTestApp(t)

	events := recordDirectoryEvents(t)

	info, err := a.OpenFileTabWithOptions(path, interfaces.FileOptions{JPath: "$.Records"})
	if err != nil {
		t.Fatalf("OpenFileTabWithOptions: %v", err)
	}
	if len(info.Headers) == 0 {
		t.Fatal("open returned no headers")
	}

	got := reportedPhases(events(), loadKindFile)

	// Decompression and row building are countable and must report; the JSON parse
	// between them can only be named.
	for _, want := range []string{
		fileloader.PhaseDecompressing,
		fileloader.PhaseParsing,
		fileloader.PhaseMapping,
	} {
		if !containsString(got, want) {
			t.Errorf("phase %q never reported; got %v", want, got)
		}
	}

	assertProgressSequencesClosed(t, events())
	seen := eventNames(events())
	if len(seen) == 0 || seen[len(seen)-1] != loadDoneEvent {
		t.Fatalf("file open did not end with a done event; events: %v", seen)
	}
}

// TestSingleFileProgressIsScopedToItsFile verifies the per-file progress sink is
// not picked up by other files, so a directory load cannot spray progress through
// a callback registered for something else.
func TestSingleFileProgressIsScopedToItsFile(t *testing.T) {
	dir := t.TempDir()
	watched := filepath.Join(dir, "watched.json.gz")
	other := filepath.Join(dir, "other.json.gz")
	writeGzipRecord(t, watched, 1)
	writeGzipRecord(t, other, 2)

	a := newTestApp(t)

	var mu sync.Mutex
	count := 0
	fileloader.SetFileProgressCallback(watched, func(fileloader.LoadProgress) {
		mu.Lock()
		count++
		mu.Unlock()
	})
	t.Cleanup(func() { fileloader.ClearFileProgressCallback(watched) })

	if _, err := a.OpenFileTabWithOptions(other, interfaces.FileOptions{JPath: "$.Records"}); err != nil {
		t.Fatalf("OpenFileTabWithOptions: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if count != 0 {
		t.Fatalf("opening %s reported %d progress events through the sink registered for %s",
			filepath.Base(other), count, filepath.Base(watched))
	}
}

// writeGzipBody writes an arbitrary gzipped body to path.
func writeGzipBody(t *testing.T, path, body string) {
	t.Helper()
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

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
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
