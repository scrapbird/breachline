package fileloader

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar/v4"
)

// DirectoryFileStat is the cheap on-disk identity of a member file, captured
// during discovery (which stats every match anyway) so later phases do not have
// to touch the filesystem again.
type DirectoryFileStat struct {
	Size    int64
	ModTime int64 // UnixNano
}

// DirectoryInfo contains metadata about a discovered directory
type DirectoryInfo struct {
	RootPath   string   // Absolute path to directory
	Files      []string // List of discovered file paths (absolute)
	TotalFiles int      // Total files found
	TotalSize  int64    // Total size in bytes
	Truncated  bool     // True if more matching files existed than MaxFiles allowed (some were skipped)

	// Stats holds each member file's size and mtime, keyed by absolute path.
	// Populated by discovery. Absent entries are re-stat'd on demand.
	Stats map[string]DirectoryFileStat

	// Headers caches each member file's normalized per-file header, keyed by the
	// absolute file path, so a header is parsed at most once per load and reused
	// for both the union schema and per-row mapping. It is populated lazily by
	// ensureFileHeaders. Files whose header cannot be read are absent from the map
	// (skipped). The synthetic __source_file__ column is never stored here; it is
	// only appended to the union header.
	//
	// When SampledSchema is true this map covers only a sample of the members, so
	// files absent from it are not necessarily unreadable and the union header
	// built from it may grow when an unsampled file introduces a new column.
	Headers       map[string][]string
	SampledSchema bool
	SchemaSampled int // Number of files whose header was actually read
}

// DirectoryDiscoveryOptions controls file discovery behavior
type DirectoryDiscoveryOptions struct {
	Pattern         string   // Glob pattern filter (e.g., "*.json.gz", "*.csv")
	ExcludePatterns []string // Patterns to exclude
	MaxFiles        int      // Maximum files to include (0 = unlimited)
	MaxDepth        int      // Maximum directory depth (0 = unlimited)
}

// Load phases reported through LoadProgressCallback. The frontend renders these
// as the stage of a directory open, so the values are part of the event contract.
const (
	PhaseDiscovering = "discovering"
	PhaseHashing     = "hashing"
	PhaseSchema      = "schema"
	PhaseLoading     = "loading"
)

// LoadProgress reports progress within one phase of a directory load.
// Total is -1 when the size of the phase is not known ahead of time.
type LoadProgress struct {
	Phase   string `json:"phase"`
	Current int64  `json:"current"`
	Total   int64  `json:"total"`
	Message string `json:"message"`
}

// LoadProgressCallback receives phase progress. It may be nil, and may be called
// from a worker goroutine, so implementations must be safe for concurrent use.
type LoadProgressCallback func(progress LoadProgress)

func (cb LoadProgressCallback) report(phase string, current, total int64, message string) {
	if cb != nil {
		cb(LoadProgress{Phase: phase, Current: current, Total: total, Message: message})
	}
}

// directoryFileLimitProvider supplies the effective "maximum files when opening a
// directory" setting to the data-load path. It is injected by the app layer (see
// SetDirectoryFileLimitProvider) to avoid a fileloader -> settings import cycle,
// mirroring how SetJSONCache injects the query cache. A nil provider or a
// non-positive result means unlimited (load every matching file).
var directoryFileLimitProvider func() int

// SetDirectoryFileLimitProvider installs the callback used by directory reads to
// honour the configured file limit, keeping the query/data path consistent with
// the discovery and hash performed when the directory tab was opened.
func SetDirectoryFileLimitProvider(f func() int) { directoryFileLimitProvider = f }

// EffectiveDirectoryFileLimit returns the configured max files (0 = unlimited).
func EffectiveDirectoryFileLimit() int {
	if directoryFileLimitProvider != nil {
		return directoryFileLimitProvider()
	}
	return 0
}

// directorySchemaSampleProvider supplies the "how many member files to read when
// building the union schema" setting. Injected by the app layer for the same
// reason as the file limit. A nil provider or a non-positive result means read
// every file, which is the safe default for library callers and tests.
var directorySchemaSampleProvider func() int

// SetDirectorySchemaSampleProvider installs the schema sample size callback.
func SetDirectorySchemaSampleProvider(f func() int) { directorySchemaSampleProvider = f }

// EffectiveSchemaSampleFiles returns how many files to read when building the
// union schema (0 = all of them).
func EffectiveSchemaSampleFiles() int {
	if directorySchemaSampleProvider != nil {
		return directorySchemaSampleProvider()
	}
	return 0
}

// directoryContentHashProvider reports whether directory hashing should read every
// byte of every member (the legacy behaviour) instead of hashing file metadata.
var directoryContentHashProvider func() bool

// SetDirectoryContentHashProvider installs the content-hash toggle callback.
func SetDirectoryContentHashProvider(f func() bool) { directoryContentHashProvider = f }

// EffectiveDirectoryContentHash reports whether to hash member file contents.
func EffectiveDirectoryContentHash() bool {
	if directoryContentHashProvider != nil {
		return directoryContentHashProvider()
	}
	return false
}

// directoryWorkers returns the degree of parallelism for per-file work. Member
// files are completely independent, so header reads, hashing and parsing all fan
// out; the cap keeps a large archive from saturating every core on a machine the
// user is still trying to use.
func directoryWorkers() int {
	n := runtime.NumCPU()
	if n > 8 {
		n = 8
	}
	if n < 1 {
		n = 1
	}
	return n
}

// DirectoryReader provides unified sequential access to all files in a directory
type DirectoryReader struct {
	info          *DirectoryInfo
	options       FileOptions
	ingestTz      *time.Location
	unifiedHeader []string       // Union of all headers (may grow, see growUnifiedHeader)
	headerMap     map[string]int // Column name -> index in unified header
	headerGrew    bool           // True once an unsampled file added a column
	currentIdx    int            // Current file index
	currentReader *csv.Reader    // Current file's CSV reader (streaming members)
	currentFile   *os.File       // Current file handle (for cleanup)
	currentRows   [][]string     // Current file's rows (rows-native members)
	currentRowIdx int            // Position within currentRows
	currentPath   string         // Current file path (for source column)
	currentHeader []string       // Header of current file
	sourceColIdx  int            // Index of __source_file__ column (-1 if disabled)
	rootPath      string         // For relative path calculation
}

// DiscoveryProgress reports progress during directory scanning
type DiscoveryProgress struct {
	FilesFound  int
	DirsScanned int
	CurrentPath string
	TotalSize   int64
}

// DiscoveryProgressCallback is called during directory scanning
type DiscoveryProgressCallback func(progress DiscoveryProgress)

// IsDirectory checks if the path is a directory
func IsDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// DiscoverFiles recursively finds all files matching the pattern in a directory
// Returns files in discovery order (depth-first traversal)
// Pattern is required - all files should be of the same log type for consistent timestamp parsing
// Uses doublestar library for efficient pattern matching and directory traversal
func DiscoverFiles(dirPath string, options DirectoryDiscoveryOptions, progress DiscoveryProgressCallback) (*DirectoryInfo, error) {
	return DiscoverFilesContext(context.Background(), dirPath, options, progress)
}

// DiscoverFilesContext is DiscoverFiles with cancellation. Discovery of a deep
// archive can take a while on its own, so it has to be interruptible.
func DiscoverFilesContext(ctx context.Context, dirPath string, options DirectoryDiscoveryOptions, progress DiscoveryProgressCallback) (*DirectoryInfo, error) {
	// Pattern is required
	if options.Pattern == "" {
		return nil, fmt.Errorf("file pattern is required (e.g., *.json.gz, *.csv)")
	}

	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %w", err)
	}

	// Use doublestar library for all pattern matching - it handles optimization automatically
	files, stats, totalSize, truncated, err := discoverFilesWithDoublestar(ctx, absPath, options, progress)
	if err != nil {
		return nil, err
	}

	return &DirectoryInfo{
		RootPath:   absPath,
		Files:      files,
		Stats:      stats,
		TotalFiles: len(files),
		TotalSize:  totalSize,
		Truncated:  truncated,
	}, nil
}

// discoverFilesWithDoublestar uses doublestar library for efficient pattern matching.
// Returns the discovered files, their stats, total size, and whether discovery was
// truncated (more eligible files existed than MaxFiles allowed). MaxFiles == 0
// means unlimited.
func discoverFilesWithDoublestar(ctx context.Context, rootPath string, options DirectoryDiscoveryOptions, progress DiscoveryProgressCallback) ([]string, map[string]DirectoryFileStat, int64, bool, error) {
	var files []string
	var totalSize int64
	stats := make(map[string]DirectoryFileStat)
	dirsScanned := 0
	truncated := false

	// Create the full pattern by combining rootPath with the user pattern
	fullPattern := filepath.Join(rootPath, options.Pattern)

	// Use doublestar to find all matching files - it handles directory traversal optimization
	// For v4, we need to use the filesystem-based API
	matches, err := doublestar.FilepathGlob(fullPattern)
	if err != nil {
		return nil, nil, 0, false, fmt.Errorf("pattern matching failed: %w", err)
	}

	// Process each match
	for _, match := range matches {
		if err := ctx.Err(); err != nil {
			return nil, nil, 0, false, err
		}

		// Get file info
		info, err := os.Stat(match)
		if err != nil {
			continue // Skip files we can't stat
		}

		// Skip directories
		if info.IsDir() {
			continue
		}

		// Check exclude patterns
		excluded := false
		for _, excludePattern := range options.ExcludePatterns {
			if matched, _ := filepath.Match(excludePattern, filepath.Base(match)); matched {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		// Enforce the max files limit before adding. Hitting an eligible file once
		// the cap is full means the dataset is being truncated: flag it and stop.
		if options.MaxFiles > 0 && len(files) >= options.MaxFiles {
			truncated = true
			break
		}

		files = append(files, match)
		stats[match] = DirectoryFileStat{Size: info.Size(), ModTime: info.ModTime().UnixNano()}
		totalSize += info.Size()

		// Report progress
		if progress != nil {
			progress(DiscoveryProgress{
				FilesFound:  len(files),
				DirsScanned: dirsScanned,
				CurrentPath: match,
				TotalSize:   totalSize,
			})
		}
	}

	return files, stats, totalSize, truncated, nil
}

// ========== Directory snapshot cache ==========

// A directory open used to re-run discovery and the whole per-file header pass at
// every entry point: once when the tab opened, once when the query pipeline asked
// for the header, once when the timestamp column was detected, and once more when
// the rows were finally read. For a directory of compressed JSON each of those is
// a full decompress-and-parse of every member, so a single open-then-query
// decompressed and parsed the entire dataset four times before reading any data.
//
// The snapshot cache makes discovery plus schema resolution happen once per
// (directory, pattern, limit, options) and be shared by every caller. It is
// authoritative for the life of the tab, which matches the existing convention
// that the hash taken at open time describes the dataset being worked on.
type dirSnapshotKey struct {
	root     string
	pattern  string
	maxFiles int
	optsKey  string
}

const maxCachedDirSnapshots = 8

var (
	dirSnapshotMu    sync.Mutex
	dirSnapshots     = make(map[dirSnapshotKey]*DirectoryInfo)
	dirSnapshotOrder []dirSnapshotKey
	dirSnapshotLocks = make(map[dirSnapshotKey]*sync.Mutex)
)

// snapshotOptionsKey captures the options that change what a snapshot contains.
// The union header depends on JPath and NoHeaderRow (how each member is parsed)
// and on IncludeSourceColumn (the synthetic column appended to the union).
func snapshotOptionsKey(options FileOptions) string {
	return options.JPath + "|" +
		strconv.FormatBool(options.NoHeaderRow) + "|" +
		strconv.FormatBool(options.IncludeSourceColumn) + "|" +
		options.IngestTimezoneOverride
}

func newDirSnapshotKey(root, pattern string, maxFiles int, options FileOptions) dirSnapshotKey {
	return dirSnapshotKey{
		root:     root,
		pattern:  pattern,
		maxFiles: maxFiles,
		optsKey:  snapshotOptionsKey(options),
	}
}

// snapshotBuildLock returns the per-key build lock, so two tabs opening different
// directories at once do not serialize behind each other.
func snapshotBuildLock(key dirSnapshotKey) *sync.Mutex {
	dirSnapshotMu.Lock()
	defer dirSnapshotMu.Unlock()
	l, ok := dirSnapshotLocks[key]
	if !ok {
		l = &sync.Mutex{}
		dirSnapshotLocks[key] = l
	}
	return l
}

func lookupDirSnapshot(key dirSnapshotKey) *DirectoryInfo {
	dirSnapshotMu.Lock()
	defer dirSnapshotMu.Unlock()
	return dirSnapshots[key]
}

func storeDirSnapshot(key dirSnapshotKey, info *DirectoryInfo) {
	dirSnapshotMu.Lock()
	defer dirSnapshotMu.Unlock()

	if _, exists := dirSnapshots[key]; !exists {
		dirSnapshotOrder = append(dirSnapshotOrder, key)
	}
	dirSnapshots[key] = info

	// Bound the cache: a snapshot of a very large archive holds one path, one
	// stat and (for sampled members) one header slice per file, so keeping every
	// directory ever opened would be a slow leak.
	for len(dirSnapshotOrder) > maxCachedDirSnapshots {
		oldest := dirSnapshotOrder[0]
		dirSnapshotOrder = dirSnapshotOrder[1:]
		delete(dirSnapshots, oldest)
		delete(dirSnapshotLocks, oldest)
	}
}

// ClearDirectorySnapshots drops every cached snapshot. Call when a setting that
// changes what a directory load contains (file limit, schema sample size) changes.
func ClearDirectorySnapshots() {
	dirSnapshotMu.Lock()
	defer dirSnapshotMu.Unlock()
	dirSnapshots = make(map[dirSnapshotKey]*DirectoryInfo)
	dirSnapshotOrder = nil
	dirSnapshotLocks = make(map[dirSnapshotKey]*sync.Mutex)
}

// ClearDirectorySnapshotsFor drops cached snapshots for one directory, across all
// patterns and options. Call on tab close so a reopened directory re-reads disk.
func ClearDirectorySnapshotsFor(dirPath string) {
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		absPath = dirPath
	}

	dirSnapshotMu.Lock()
	defer dirSnapshotMu.Unlock()

	kept := dirSnapshotOrder[:0]
	for _, key := range dirSnapshotOrder {
		if key.root == absPath {
			delete(dirSnapshots, key)
			delete(dirSnapshotLocks, key)
			continue
		}
		kept = append(kept, key)
	}
	dirSnapshotOrder = kept
}

// GetDirectorySnapshot returns the discovered files and resolved schema for a
// directory, building it once and reusing it for every later caller.
func GetDirectorySnapshot(ctx context.Context, dirPath string, options FileOptions, maxFiles int, progress LoadProgressCallback) (*DirectoryInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	pattern := options.FilePattern
	if pattern == "" {
		pattern = "*"
	}

	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %w", err)
	}

	key := newDirSnapshotKey(absPath, pattern, maxFiles, options)

	if info := lookupDirSnapshot(key); info != nil {
		return info, nil
	}

	lock := snapshotBuildLock(key)
	lock.Lock()
	defer lock.Unlock()

	// Another caller may have built it while we waited for the lock.
	if info := lookupDirSnapshot(key); info != nil {
		return info, nil
	}

	progress.report(PhaseDiscovering, 0, -1, "Scanning directory")
	info, err := DiscoverFilesContext(ctx, absPath, DirectoryDiscoveryOptions{
		Pattern:  pattern,
		MaxFiles: maxFiles,
	}, func(p DiscoveryProgress) {
		if p.FilesFound%500 == 0 {
			progress.report(PhaseDiscovering, int64(p.FilesFound), -1,
				fmt.Sprintf("Found %d files", p.FilesFound))
		}
	})
	if err != nil {
		return nil, err
	}
	progress.report(PhaseDiscovering, int64(info.TotalFiles), int64(info.TotalFiles),
		fmt.Sprintf("Found %d files", info.TotalFiles))

	if err := ensureFileHeaders(ctx, info, options, progress); err != nil {
		return nil, err
	}

	storeDirSnapshot(key, info)
	return info, nil
}

// ========== Schema resolution ==========

// schemaSampleIndexes picks which member files to read when building the union
// schema. Sampling is spread evenly across discovery order rather than taking the
// first N: archives are usually partitioned by date, so a prefix sample would only
// ever see the oldest files and miss schema drift. The first and last file are
// always included.
func schemaSampleIndexes(total, sample int) []int {
	if sample <= 0 || sample >= total {
		idx := make([]int, total)
		for i := range idx {
			idx[i] = i
		}
		return idx
	}
	if sample == 1 {
		return []int{0}
	}

	seen := make(map[int]bool, sample)
	idx := make([]int, 0, sample)
	for i := 0; i < sample; i++ {
		// Evenly spaced over [0, total-1], inclusive of both ends.
		pos := i * (total - 1) / (sample - 1)
		if !seen[pos] {
			seen[pos] = true
			idx = append(idx, pos)
		}
	}
	sort.Ints(idx)
	return idx
}

// ensureFileHeaders reads member headers once and caches them on info, keyed by
// absolute file path. Files whose header cannot be read are skipped (left absent
// from the map), preserving the tolerant behavior of the union build and row
// iteration. The synthetic source column is never stored here.
//
// Only a sample of the members is read when a schema sample size is configured;
// the resulting union header may then grow at read time (see growUnifiedHeader),
// which is what makes sampling safe rather than lossy.
func ensureFileHeaders(ctx context.Context, info *DirectoryInfo, options FileOptions, progress LoadProgressCallback) error {
	if info.Headers != nil {
		return nil
	}

	indexes := schemaSampleIndexes(len(info.Files), EffectiveSchemaSampleFiles())
	total := int64(len(indexes))
	progress.report(PhaseSchema, 0, total, "Resolving schema")

	type headerResult struct {
		path   string
		header []string
	}

	results := make([]headerResult, len(indexes))
	var done int64
	var doneMu sync.Mutex

	err := parallelFor(ctx, len(indexes), func(i int) error {
		filePath := info.Files[indexes[i]]
		header, err := readFileHeader(filePath, options)
		if err == nil {
			results[i] = headerResult{path: filePath, header: header}
		}
		// A file whose header cannot be read is skipped, not fatal.

		doneMu.Lock()
		done++
		current := done
		doneMu.Unlock()
		if current%25 == 0 || current == total {
			progress.report(PhaseSchema, current, total,
				fmt.Sprintf("Read schema from %d of %d files", current, total))
		}
		return nil
	})
	if err != nil {
		return err
	}

	headers := make(map[string][]string, len(results))
	for _, r := range results {
		if r.header != nil {
			headers[r.path] = r.header
		}
	}

	info.Headers = headers
	info.SchemaSampled = len(indexes)
	info.SampledSchema = len(indexes) < len(info.Files)
	return nil
}

// parallelFor runs fn for each index in [0, n) across a bounded worker pool,
// aborting early if ctx is cancelled or any call returns an error.
func parallelFor(ctx context.Context, n int, fn func(i int) error) error {
	if n <= 0 {
		return nil
	}

	workers := directoryWorkers()
	if workers > n {
		workers = n
	}

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		firstEr error
		next    int
		idxMu   sync.Mutex
	)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				idxMu.Lock()
				i := next
				next++
				idxMu.Unlock()
				if i >= n {
					return
				}

				if err := ctx.Err(); err != nil {
					mu.Lock()
					if firstEr == nil {
						firstEr = err
					}
					mu.Unlock()
					return
				}

				mu.Lock()
				aborted := firstEr != nil
				mu.Unlock()
				if aborted {
					return
				}

				if err := fn(i); err != nil {
					mu.Lock()
					if firstEr == nil {
						firstEr = err
					}
					mu.Unlock()
					return
				}
			}
		}()
	}

	wg.Wait()
	return firstEr
}

// GetDirectoryHeader reads headers from all files and returns unified union header
// Columns are ordered by first appearance across files
func GetDirectoryHeader(info *DirectoryInfo, options FileOptions) ([]string, error) {
	// Populate the per-file header cache once, then build the union from it so
	// each member file is parsed a single time.
	if err := ensureFileHeaders(context.Background(), info, options, nil); err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var unionHeader []string

	// Iterate files in discovery order so the union is ordered by first
	// appearance and column positions stay stable.
	for _, filePath := range info.Files {
		header, ok := info.Headers[filePath]
		if !ok {
			// Unreadable, or not part of the schema sample.
			continue
		}

		for _, col := range header {
			if !seen[col] {
				seen[col] = true
				unionHeader = append(unionHeader, col)
			}
		}
	}

	if len(unionHeader) == 0 {
		return nil, fmt.Errorf("no valid headers found in any files")
	}

	// Add source column if requested (at the end)
	if options.IncludeSourceColumn {
		unionHeader = append(unionHeader, "__source_file__")
	}

	return unionHeader, nil
}

// readFileHeader reads the header from a single file.
//
// The ingest timezone is resolved from the options rather than left nil: for JSON
// and XLSX members the header comes out of the Row-based base-data cache, whose key
// includes the timezone, so passing nil here would key the header read under the
// default timezone and make the subsequent row read - which uses the tab's actual
// timezone - miss the cache and parse the whole file a second time.
func readFileHeader(filePath string, options FileOptions) ([]string, error) {
	return ReadHeaderWithOptions(filePath, options, ingestTimezoneForOptions(options))
}

// GetDirectoryRowCount returns total row count across all files
func GetDirectoryRowCount(info *DirectoryInfo, options FileOptions) (int, error) {
	counts := make([]int, len(info.Files))
	err := parallelFor(context.Background(), len(info.Files), func(i int) error {
		count, err := GetRowCountWithOptions(info.Files[i], options)
		if err != nil {
			// Skip files that can't be read
			return nil
		}
		counts[i] = count
		return nil
	})
	if err != nil {
		return 0, err
	}

	totalCount := 0
	for _, c := range counts {
		totalCount += c
	}
	return totalCount, nil
}

// NewDirectoryReader creates a reader that iterates through all files
func NewDirectoryReader(info *DirectoryInfo, options FileOptions) (*DirectoryReader, error) {
	if info == nil || len(info.Files) == 0 {
		return nil, fmt.Errorf("no files to read")
	}

	// Build union header
	header, err := GetDirectoryHeader(info, options)
	if err != nil {
		return nil, fmt.Errorf("failed to build union header: %w", err)
	}

	// Build header map for fast column lookup
	headerMap := make(map[string]int)
	for i, col := range header {
		headerMap[col] = i
	}

	// Find source column index
	sourceColIdx := -1
	if options.IncludeSourceColumn {
		sourceColIdx = headerMap["__source_file__"]
	}

	return &DirectoryReader{
		info:          info,
		options:       options,
		ingestTz:      ingestTimezoneForOptions(options),
		unifiedHeader: header,
		headerMap:     headerMap,
		currentIdx:    0,
		currentRowIdx: -1,
		sourceColIdx:  sourceColIdx,
		rootPath:      info.RootPath,
	}, nil
}

// growUnifiedHeader appends any columns of a member file that the union header
// does not already contain. New columns go on the end, so every index already
// handed out stays valid; rows produced before the growth are shorter than the
// final header and must be padded by the caller (see HeaderGrew).
//
// This is what makes a sampled schema safe: a file outside the sample that
// introduces an unseen column still contributes it, rather than having the column
// silently dropped.
func (dr *DirectoryReader) growUnifiedHeader(header []string) {
	for _, col := range header {
		if _, ok := dr.headerMap[col]; ok {
			continue
		}
		dr.headerMap[col] = len(dr.unifiedHeader)
		dr.unifiedHeader = append(dr.unifiedHeader, col)
		dr.headerGrew = true
	}
}

// HeaderGrew reports whether the union header gained columns after the reader was
// created. When true, rows returned earlier are shorter than the current header
// and the caller must pad them to len(Header()).
func (dr *DirectoryReader) HeaderGrew() bool { return dr.headerGrew }

// Read returns the next row from the directory (unified schema)
// Returns io.EOF when all files are exhausted
func (dr *DirectoryReader) Read() ([]string, error) {
	for {
		// If no current file, open the next one
		if dr.currentReader == nil && dr.currentRows == nil {
			if dr.currentIdx >= len(dr.info.Files) {
				return nil, io.EOF
			}

			filePath := dr.info.Files[dr.currentIdx]
			dr.currentIdx++

			if err := dr.openFile(filePath); err != nil {
				continue // Skip files we cannot read
			}
		}

		// Rows-native members: serve from the parsed rows.
		if dr.currentRows != nil {
			if dr.currentRowIdx >= len(dr.currentRows) {
				dr.closeCurrentFile()
				continue
			}
			row := dr.currentRows[dr.currentRowIdx]
			dr.currentRowIdx++
			return dr.mapToUnifiedSchema(row), nil
		}

		// Streaming members: read the next row from the CSV reader.
		row, err := dr.currentReader.Read()
		if err == io.EOF {
			dr.closeCurrentFile()
			continue // Move to next file
		}
		if err != nil {
			dr.closeCurrentFile()
			continue // Skip problematic rows
		}

		return dr.mapToUnifiedSchema(row), nil
	}
}

// openFile prepares the reader state for one member file, choosing the rows-native
// path (JSON/XLSX, served from the base-data cache) or the streaming CSV path.
func (dr *DirectoryReader) openFile(filePath string) error {
	if SupportsRowsNative(filePath) {
		header, rows, err := GetRowsForFile(filePath, dr.options, dr.ingestTz)
		if err != nil {
			return err
		}
		dr.currentPath = filePath
		dr.currentHeader = header
		dr.currentRows = rows
		dr.currentRowIdx = 0
		dr.growUnifiedHeader(header)
		return nil
	}

	// Streaming member. The per-file header comes from the schema cache when the
	// file was sampled, otherwise it is read here (a cheap first-row read for CSV).
	header, ok := dr.info.Headers[filePath]
	if !ok {
		var err error
		header, err = readFileHeader(filePath, dr.options)
		if err != nil {
			return err
		}
	}

	reader, file, err := GetReader(filePath, dr.options)
	if err != nil {
		return err
	}

	dr.currentReader = reader
	dr.currentFile = file
	dr.currentPath = filePath
	dr.currentHeader = header
	dr.growUnifiedHeader(header)

	// Skip the header row on the data stream if present. This is a stream advance
	// past the already-consumed header row, not a header re-parse.
	if !dr.options.NoHeaderRow {
		if _, err := reader.Read(); err != nil {
			dr.closeCurrentFile()
			return err
		}
	}

	return nil
}

// mapToUnifiedSchema maps a row from the current file to the unified schema
func (dr *DirectoryReader) mapToUnifiedSchema(row []string) []string {
	unified := make([]string, len(dr.unifiedHeader))

	// Map each column from current file to unified position
	for i, val := range row {
		if i < len(dr.currentHeader) {
			colName := dr.currentHeader[i]
			if unifiedIdx, ok := dr.headerMap[colName]; ok {
				unified[unifiedIdx] = val
			}
		}
	}

	// Add source file column if enabled
	if dr.sourceColIdx >= 0 {
		relPath, err := filepath.Rel(dr.rootPath, dr.currentPath)
		if err != nil {
			relPath = dr.currentPath // Fallback to absolute path
		}
		unified[dr.sourceColIdx] = relPath
	}

	return unified
}

// Header returns the unified header
func (dr *DirectoryReader) Header() []string {
	return dr.unifiedHeader
}

// Close releases all resources
func (dr *DirectoryReader) Close() error {
	dr.closeCurrentFile()
	return nil
}

// closeCurrentFile closes the current file and resets the reader
func (dr *DirectoryReader) closeCurrentFile() {
	if dr.currentFile != nil {
		dr.currentFile.Close()
		dr.currentFile = nil
	}
	dr.currentReader = nil
	dr.currentRows = nil
	dr.currentRowIdx = -1
	dr.currentPath = ""
	dr.currentHeader = nil
}

// ========== Parallel row loading ==========

// LoadDirectoryRows reads every member file and returns the union header along
// with all rows mapped into it, padded to a uniform width.
//
// Member files are parsed in parallel because they are completely independent;
// mapping into the union schema stays serial and in discovery order so the row
// order is identical to a sequential read (row order is user-visible: it is what
// annotations index against when no sort is applied). Parsing dominates the cost,
// so serializing the cheap mapping step costs little and keeps the result
// deterministic.
func LoadDirectoryRows(ctx context.Context, info *DirectoryInfo, options FileOptions, ingestTz *time.Location, progress LoadProgressCallback) ([]string, [][]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if info == nil || len(info.Files) == 0 {
		return nil, nil, fmt.Errorf("no files to read")
	}

	dr, err := NewDirectoryReader(info, options)
	if err != nil {
		return nil, nil, err
	}
	defer dr.Close()

	total := int64(len(info.Files))
	progress.report(PhaseLoading, 0, total, "Loading rows")

	type fileRows struct {
		header []string
		rows   [][]string
	}

	var all [][]string
	workers := directoryWorkers()
	var filesDone int64

	// Files are processed in chunks the width of the worker pool: every file in a
	// chunk is parsed concurrently, then the chunk is folded into the result in
	// order before the next chunk starts. That bounds in-flight memory to `workers`
	// files rather than the whole archive, and keeps output order deterministic.
	for start := 0; start < len(info.Files); start += workers {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}

		end := start + workers
		if end > len(info.Files) {
			end = len(info.Files)
		}

		chunk := make([]fileRows, end-start)
		err := parallelFor(ctx, end-start, func(i int) error {
			filePath := info.Files[start+i]

			if SupportsRowsNative(filePath) {
				header, rows, err := GetRowsForFile(filePath, options, ingestTz)
				if err != nil {
					return nil // Skip unreadable members, as the reader path does
				}
				chunk[i] = fileRows{header: header, rows: rows}
				return nil
			}

			header, rows, err := readStreamingFileRows(filePath, options, info)
			if err != nil {
				return nil // Skip unreadable members
			}
			chunk[i] = fileRows{header: header, rows: rows}
			return nil
		})
		if err != nil {
			return nil, nil, err
		}

		for i := range chunk {
			if chunk[i].header == nil {
				continue
			}
			dr.currentPath = info.Files[start+i]
			dr.currentHeader = chunk[i].header
			dr.growUnifiedHeader(chunk[i].header)
			for _, row := range chunk[i].rows {
				all = append(all, dr.mapToUnifiedSchema(row))
			}
		}

		filesDone = int64(end)
		progress.report(PhaseLoading, filesDone, total,
			fmt.Sprintf("Loaded %d of %d files (%d rows)", filesDone, total, len(all)))
	}

	header := dr.Header()

	// A file outside the schema sample may have widened the union header partway
	// through, leaving rows produced before that point short. Pad them once here.
	if dr.HeaderGrew() {
		for i, row := range all {
			if len(row) < len(header) {
				padded := make([]string, len(header))
				copy(padded, row)
				all[i] = padded
			}
		}
	}

	return header, all, nil
}

// readStreamingFileRows materializes one CSV or plugin member's rows.
func readStreamingFileRows(filePath string, options FileOptions, info *DirectoryInfo) ([]string, [][]string, error) {
	header, ok := info.Headers[filePath]
	if !ok {
		var err error
		header, err = readFileHeader(filePath, options)
		if err != nil {
			return nil, nil, err
		}
	}

	reader, file, err := GetReader(filePath, options)
	if err != nil {
		return nil, nil, err
	}
	if file != nil {
		defer file.Close()
	}

	if !options.NoHeaderRow {
		if _, err := reader.Read(); err != nil {
			return nil, nil, err
		}
	}

	var rows [][]string
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // Skip problematic rows, matching DirectoryReader.Read
		}
		if record == nil {
			continue
		}
		// csv.Reader reuses the record slice, so it must be copied.
		row := make([]string, len(record))
		copy(row, record)
		rows = append(rows, row)
	}

	return header, rows, nil
}

// ========== Hashing ==========

// CalculateDirectoryHash generates a hash identifying the directory contents.
//
// By default this hashes each member's relative path, size and modification time
// rather than its bytes. Reading every byte of a multi-gigabyte archive purely to
// derive an identity cost minutes at open, and again on every workspace restore
// and file-relocation scan, for a value that only has to change when the data
// changes. Set the content-hash option (see SetDirectoryContentHashProvider) to
// restore byte-exact hashing.
//
// The digest is domain-separated so a metadata hash can never collide with a
// legacy content hash; see CalculateDirectoryContentHash for the legacy algorithm,
// which callers fall back to when matching against previously stored hashes.
func CalculateDirectoryHash(info *DirectoryInfo) (string, error) {
	if EffectiveDirectoryContentHash() {
		return CalculateDirectoryContentHash(context.Background(), info, nil)
	}
	return CalculateDirectoryMetadataHash(info)
}

// CalculateDirectoryMetadataHash hashes member identity (relative path, size,
// mtime) instead of member contents.
func CalculateDirectoryMetadataHash(info *DirectoryInfo) (string, error) {
	if info == nil || len(info.Files) == 0 {
		return "", fmt.Errorf("no files in directory info")
	}

	sortedFiles := make([]string, len(info.Files))
	copy(sortedFiles, info.Files)
	sort.Strings(sortedFiles)

	h := sha256.New()
	// Domain separator: keeps this value distinct from a content hash of the same
	// directory, so the two can be told apart when matching stored hashes.
	h.Write([]byte("breachline-dir-meta-v1\x00"))

	hashed := 0
	for _, filePath := range sortedFiles {
		stat, ok := info.Stats[filePath]
		if !ok {
			fi, err := os.Stat(filePath)
			if err != nil {
				// Skip files that can't be stat'd, but continue with others
				continue
			}
			stat = DirectoryFileStat{Size: fi.Size(), ModTime: fi.ModTime().UnixNano()}
		}

		relPath, err := filepath.Rel(info.RootPath, filePath)
		if err != nil {
			continue
		}

		h.Write([]byte(relPath))
		h.Write([]byte{0})
		h.Write([]byte(strconv.FormatInt(stat.Size, 10)))
		h.Write([]byte{0})
		h.Write([]byte(strconv.FormatInt(stat.ModTime, 10)))
		h.Write([]byte{0})
		hashed++
	}

	if hashed == 0 {
		return "", fmt.Errorf("failed to hash any files in directory")
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// CalculateDirectoryContentHash generates a hash from the contents of every member
// file. Algorithm:
// 1. Calculate SHA256 hash for each file's contents
// 2. Sort file paths alphabetically
// 3. For each file, append: file hash + relative path (to ensure directory structure is part of hash)
// 4. Hash the concatenated result
//
// This is the original directory hash algorithm and its output format is preserved
// exactly, so hashes stored by earlier versions still match. It is retained for the
// content-hash setting and for migrating workspaces that recorded a content hash.
func CalculateDirectoryContentHash(ctx context.Context, info *DirectoryInfo, progress LoadProgressCallback) (string, error) {
	if info == nil || len(info.Files) == 0 {
		return "", fmt.Errorf("no files in directory info")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// Sort files for consistent hashing
	sortedFiles := make([]string, len(info.Files))
	copy(sortedFiles, info.Files)
	sort.Strings(sortedFiles)

	total := int64(len(sortedFiles))
	progress.report(PhaseHashing, 0, total, "Hashing files")

	// Per-file hashes are computed in parallel and then combined in sorted order,
	// so the digest is identical to the original sequential implementation.
	type hashResult struct {
		hash    []byte
		relPath string
	}
	results := make([]hashResult, len(sortedFiles))

	var done int64
	var doneMu sync.Mutex

	err := parallelFor(ctx, len(sortedFiles), func(i int) error {
		filePath := sortedFiles[i]

		fileHash, err := calculateFileContentHash(filePath)
		if err == nil {
			// Calculate relative path from directory root
			if relPath, relErr := filepath.Rel(info.RootPath, filePath); relErr == nil {
				results[i] = hashResult{hash: fileHash, relPath: relPath}
			}
		}
		// Skip files that can't be read or resolved, but continue with others.

		doneMu.Lock()
		done++
		current := done
		doneMu.Unlock()
		if current%100 == 0 || current == total {
			progress.report(PhaseHashing, current, total,
				fmt.Sprintf("Hashed %d of %d files", current, total))
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	var combinedData []byte
	for _, r := range results {
		if r.hash == nil {
			continue
		}
		// This ensures directory structure is part of the hash
		combinedData = append(combinedData, r.hash...)
		combinedData = append(combinedData, []byte(r.relPath)...)
	}

	if len(combinedData) == 0 {
		return "", fmt.Errorf("failed to hash any files in directory")
	}

	// Hash the combined result
	finalHash := sha256.Sum256(combinedData)
	return hex.EncodeToString(finalHash[:]), nil
}

// calculateFileContentHash computes SHA256 hash of a file's contents
func calculateFileContentHash(filePath string) ([]byte, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

// ========== Preview ==========

// DirectoryPreviewResult contains preview information for a directory
type DirectoryPreviewResult struct {
	Files      []string `json:"files"`      // Sample of discovered files (relative paths)
	Headers    []string `json:"headers"`    // Unified header columns
	TotalFiles int      `json:"totalFiles"` // Total number of files found
	TotalSize  int64    `json:"totalSize"`  // Total size in bytes

	// SampledSchema is true when Headers came from a sample of the members rather
	// than all of them, so a column unique to an unsampled file may not be listed.
	// Such columns are still loaded; they are just not visible in the preview.
	SampledSchema bool `json:"sampledSchema"`
	SchemaSampled int  `json:"schemaSampled"`

	// EstimatedUncompressedSize is the projected in-memory size of the dataset, and
	// MemoryWarning is set when that projection exceeds what the machine can hold.
	EstimatedUncompressedSize int64  `json:"estimatedUncompressedSize"`
	MemoryWarning             string `json:"memoryWarning,omitempty"`
}

// PreviewDirectory returns preview information about a directory without fully loading it
func PreviewDirectory(dirPath string, pattern string, jpath string, maxFiles int) (*DirectoryPreviewResult, error) {
	return PreviewDirectoryContext(context.Background(), dirPath, pattern, jpath, maxFiles, nil)
}

// PreviewDirectoryContext is PreviewDirectory with cancellation and progress
// reporting. Previewing a large archive scans it and samples its members, which is
// not instant, so it has to be interruptible and has to say what it is doing.
func PreviewDirectoryContext(ctx context.Context, dirPath string, pattern string, jpath string, maxFiles int, progress LoadProgressCallback) (*DirectoryPreviewResult, error) {
	options := FileOptions{
		FilePattern:         pattern,
		JPath:               jpath,
		IncludeSourceColumn: false, // Don't include in preview
	}

	info, err := GetDirectorySnapshot(ctx, dirPath, options, maxFiles, progress)
	if err != nil {
		return nil, err
	}

	if len(info.Files) == 0 {
		return nil, fmt.Errorf("no compatible files found in directory")
	}

	// Get unified header
	headers, err := GetDirectoryHeader(info, options)
	if err != nil {
		return nil, err
	}

	// Convert file paths to relative paths for display
	relativeFiles := make([]string, len(info.Files))
	for i, f := range info.Files {
		rel, err := filepath.Rel(info.RootPath, f)
		if err != nil {
			rel = f
		}
		relativeFiles[i] = rel
	}

	estimate := EstimateDirectoryMemory(info)

	return &DirectoryPreviewResult{
		Files:                     relativeFiles,
		Headers:                   headers,
		TotalFiles:                info.TotalFiles,
		TotalSize:                 info.TotalSize,
		SampledSchema:             info.SampledSchema,
		SchemaSampled:             info.SchemaSampled,
		EstimatedUncompressedSize: estimate.EstimatedBytes,
		MemoryWarning:             estimate.Warning,
	}, nil
}

// ingestTimezoneForOptions is a small indirection so directory.go does not have to
// import the timestamps package in more than one place.
func ingestTimezoneForOptions(options FileOptions) *time.Location {
	return effectiveIngestTimezone(options.IngestTimezoneOverride)
}
