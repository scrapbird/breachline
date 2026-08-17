package fileloader

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// Per-file load progress.
//
// Opening a single large file is one long synchronous stretch of work, so the UI
// could only show a spinner. Some of that work is countable and some is not: a
// decompression can be measured against the compressed size, and the row-building
// loops know how many rows they have to get through, but the JSON parse and the
// XLSX workbook read are single opaque library calls. Measured on a 79 MB JSON
// document, that splits roughly 19/42/38 (decompress / parse / map) when gzipped
// and 5/55/40 when not, so a little over half the wait can carry a real
// percentage and the rest can only be named.
//
// Callbacks are registered per file path rather than threaded through every
// signature, mirroring how this package already tracks decompression warnings.
// That keeps the reporting out of the ingestion APIs, which are called from many
// places that have no interest in progress.

// Phases reported while loading a single file. Directory phases live in
// directory.go; these describe work inside one member or one standalone file.
const (
	PhaseReading       = "reading"       // Pulling bytes off disk
	PhaseDecompressing = "decompressing" // Inflating a compressed file
	PhaseParsing       = "parsing"       // Opaque parse of the whole document
	PhaseMapping       = "mapping"       // Turning parsed data into rows
	PhasePreparing     = "preparing"     // Attaching timestamps to those rows
)

// progressReportInterval is how many rows pass between progress reports. Frequent
// enough to animate, rare enough that the callback is not a measurable cost in the
// row-building loops.
const progressReportInterval = 4096

// readChunkSize bounds a single read so a whole-file read still reports as it goes.
// Large enough that the syscall count is irrelevant next to the read itself.
const readChunkSize = 4 << 20

var (
	fileProgressMu sync.RWMutex
	fileProgress   = make(map[string]LoadProgressCallback)
)

// progressKey normalizes a path so the callback registered by the app layer is
// found by loader code that may have been handed a different spelling of it.
func progressKey(filePath string) string {
	abs, err := filepath.Abs(filePath)
	if err != nil {
		return filePath
	}
	return abs
}

// SetFileProgressCallback registers a progress sink for one file. Callers must
// pair this with ClearFileProgressCallback, normally via defer.
func SetFileProgressCallback(filePath string, cb LoadProgressCallback) {
	if filePath == "" {
		return
	}
	fileProgressMu.Lock()
	defer fileProgressMu.Unlock()
	if cb == nil {
		delete(fileProgress, progressKey(filePath))
		return
	}
	fileProgress[progressKey(filePath)] = cb
}

// ClearFileProgressCallback removes the progress sink for one file.
func ClearFileProgressCallback(filePath string) {
	if filePath == "" {
		return
	}
	fileProgressMu.Lock()
	defer fileProgressMu.Unlock()
	delete(fileProgress, progressKey(filePath))
}

// fileProgressCallback returns the sink for a file, or nil when nothing is
// listening (the common case, including every directory member).
func fileProgressCallback(filePath string) LoadProgressCallback {
	if filePath == "" {
		return nil
	}
	fileProgressMu.RLock()
	defer fileProgressMu.RUnlock()
	if len(fileProgress) == 0 {
		return nil
	}
	return fileProgress[progressKey(filePath)]
}

// reportFileProgress sends one progress update for a file, if anything is
// listening. Total is -1 for work whose size is not knowable in advance.
func reportFileProgress(filePath, phase string, current, total int64, message string) {
	if cb := fileProgressCallback(filePath); cb != nil {
		cb(LoadProgress{Phase: phase, Current: current, Total: total, Message: message})
	}
}

// progressReader counts bytes as they are consumed and reports them against a
// known total. Wrapping the compressed source (rather than the decompressed
// output) is what makes a decompression measurable: the output size is unknown
// until the stream ends, but the input size is just the file size.
type progressReader struct {
	inner    io.Reader
	filePath string
	phase    string
	message  string
	total    int64
	read     int64
	nextTick int64
}

// newProgressReader wraps r, reporting progress against total bytes. It returns r
// unchanged when nothing is listening or the total is unknown, so the counting
// costs nothing on the paths that do not report.
func newProgressReader(r io.Reader, filePath, phase, message string, total int64) io.Reader {
	if total <= 0 || fileProgressCallback(filePath) == nil {
		return r
	}
	return &progressReader{
		inner:    r,
		filePath: filePath,
		phase:    phase,
		message:  message,
		total:    total,
		nextTick: total / 100,
	}
}

// readFileWithProgress reads a whole file into memory, reporting bytes consumed
// against its size. Equivalent to os.ReadFile except that os.ReadFile is a single
// opaque call which can report nothing while it runs.
func readFileWithProgress(filePath string) ([]byte, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var size int64
	if info, statErr := f.Stat(); statErr == nil {
		size = info.Size()
	}

	reportFileProgress(filePath, PhaseReading, 0, size, "Reading file")

	// Pre-sized so the read does not repeatedly reallocate a buffer that may be
	// hundreds of megabytes.
	buf := bytes.NewBuffer(make([]byte, 0, size+1))

	// Copied in bounded chunks rather than by buf.ReadFrom: handed a buffer the size
	// of the file, one Read can satisfy the whole thing in a single syscall, which
	// reports no progress at all until it returns. Wrapping the buffer hides its
	// ReadFrom so io.CopyBuffer honours the chunk size instead of bypassing it.
	dst := struct{ io.Writer }{buf}
	if _, err := io.CopyBuffer(dst, newProgressReader(f, filePath, PhaseReading, "Reading file", size), make([]byte, readChunkSize)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.inner.Read(p)
	pr.read += int64(n)

	// Report at most once per percent so a fast stream does not flood the sink.
	if pr.read >= pr.nextTick {
		pr.nextTick = pr.read + pr.total/100 + 1
		reportFileProgress(pr.filePath, pr.phase, pr.read, pr.total, pr.message)
	}

	return n, err
}
