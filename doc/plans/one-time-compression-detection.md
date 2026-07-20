# Plan: One-time compression detection

## Summary

Every file dispatcher call re-detects file type and compression, and for any file without a `.gz/.bz2/.xz` extension that detection opens the file to peek at 6 magic bytes. A single tab open triggers this peek 2 to 3 times (header read, data read, and for JSON a parse), plus once per member file for directories. The fix is to detect type and compression once and reuse the result, eliminating the redundant tiny opens while preserving magic-byte detection for compressed files with non-standard names. Approach: a small path-keyed memoization cache inside the `fileloader` package, guarded by file mtime and size, keeping the concern entirely inside `fileloader` and off the persisted `FileOptions` type.

## Current behavior

Detection has two entry points in `application/app/fileloader/detection.go`:

- `DetectFileType(filePath)` (detection.go:28) is extension-only. It never opens the file. Callers: `reader.go:137`, `query/integration.go:305`, `app_tabs.go:1039`, `workspace/local.go:1468`, `workspace/remote.go:2196`.
- `DetectFileTypeAndCompression(filePath)` (detection.go:71) checks for a compression extension first; if none is found it calls `DetectCompressionByMagic(filePath)` (detection.go:92).

`DetectCompressionByMagic` (`application/app/fileloader/compression.go:55`) does `os.Open` + read 6 bytes + close, every time it is called.

`DetectFileTypeAndCompression` call sites (each incurs one magic peek for a non-compression-extension file):

- `proxy.go:71` inside `ReadHeaderWithOptions`
- `proxy.go:140` inside `GetRowCountWithOptions`
- `proxy.go:200` inside `GetReader`
- `json.go:63` inside `parseJSONFile`

Extra opens per open lifecycle (after the single-read change, so no separate row-count scan and no async preload):

- CSV/XLSX plain file: header dispatch (`proxy.go:71`) + data dispatch (`proxy.go:200`) = 2 magic peeks. `GetRowCountWithOptions` (`proxy.go:140`) adds a 3rd only if a caller still requests a count.
- JSON: header dispatch + data dispatch, plus `parseJSONFile` (`json.go:63`) on cache miss = up to 3 peeks. The Row cache means later queries skip re-parsing but the dispatchers still re-detect.
- Directory: `DirectoryReader.Read` calls `GetReader` (`proxy.go:200`) per member file, so every member file gets its own magic peek on top of the union-header reads.

Each peek is small (6 bytes), so the cost is the syscall and open/close overhead rather than IO volume. It is pure redundancy: the answer does not change between calls within one tab session.

## Proposed approach

Path-keyed memoization inside fileloader. Add an unexported cache in the `fileloader` package mapping absolute path to a detected `(FileType, CompressionType)` pair, guarded by the file's mtime and size so a changed-on-disk file re-detects.

- New internal function, roughly `detectFileTypeAndCompressionCached(filePath)`, that does one `os.Stat`, builds a key from `path + mtime + size`, returns the cached pair on hit, and on miss runs the existing `DetectFileTypeAndCompression` logic and stores the result.
- Route the four `DetectFileTypeAndCompression` call sites (`proxy.go:71`, `proxy.go:140`, `proxy.go:200`, `json.go:63`) through the cached variant. Keep the public `DetectFileTypeAndCompression` unchanged for external callers and as the miss path.
- Guard concurrency with a `sync.RWMutex`, matching the existing pattern used by `SetJSONCache` and `decompressionWarnings` in this package.
- Provide a `ClearDetectionCache(filePath)` (and maybe a clear-all) for tab close and file re-open, mirroring `ClearDecompressionWarning`.

Why memoization inside `fileloader` rather than carrying detection on the tab or `FileOptions`: `FileOptions` (`shared/types/file_options.go:11`) is the canonical cross-service type. It is serialized to JSON and persisted to DynamoDB (it carries `dynamodbav` tags and is used by sync-api), and its `Key()` / `Equals()` define virtual-file identity. Detection state is transient and derived from disk, so it does not belong on a persisted identity type; adding a field there risks polluting cache keys, sync payloads, and workspace storage. Threading a detected pair on the tab through the dispatchers would also require new arguments on `ReadHeaderWithOptions`, `GetRowCountWithOptions`, `GetReader`, and the directory/JSON paths, and would not naturally cover directory member files (each member is a distinct path discovered lazily). Memoization keeps the concern entirely inside `fileloader`, benefits directory member reads for free, and does not touch any serialized type.

Cost: one `os.Stat` per detection to validate the guard. Stat is cheaper than open+read+close and is often already warm, so this is still a net reduction. If even the stat is considered wasteful, a session-scoped variant (no mtime guard, cleared on tab open/close) is possible but is more fragile against files mutating underneath an open tab.

## Affected files and functions

- `application/app/fileloader/detection.go`: add the cached wrapper and cache struct; optionally a clear function.
- `application/app/fileloader/proxy.go`: switch `proxy.go:71`, `proxy.go:140`, `proxy.go:200` to the cached wrapper.
- `application/app/fileloader/json.go`: switch `parseJSONFile` (`json.go:63`) to the cached wrapper.
- `application/app/fileloader/compression.go`: no logic change; `DetectCompressionByMagic` stays the miss path.
- Tab close / re-open path (`application/app/app_tabs.go`, wherever tabs are closed): optional `ClearDetectionCache` call so a re-opened, edited file re-detects even inside the mtime window.
- No change to `shared/types/file_options.go`.

## Edge cases and risks

- File changed on disk between detection and read: the mtime + size guard covers the common case. A file edited within the same second and identical in size could theoretically serve a stale detection; acceptable given detection only distinguishes format/compression, and the existing code already re-reads content per dispatch anyway.
- Compressed file with a non-standard name (no `.gz` extension): must still hit the magic peek on first detection. The plan preserves this because the miss path is the unchanged `DetectFileTypeAndCompression`; only repeats are elided.
- Magic-detected compressed files return `FileTypeCSV` for the inner type (detection.go:97). Caching does not change this behavior; it just stops re-deriving it.
- Directory members: each member path is cached independently. Confirm the cache is keyed by absolute path so two directories referencing the same file agree. `DiscoverFiles` already resolves absolute paths (`directory.go`), so callers should pass absolute paths or the cache should `filepath.Abs` internally.
- Plugin extensions: `detectFileTypeFromPath` consults the plugin registry (detection.go:122). If a plugin is registered or unregistered mid-session, a cached `FileTypePlugin` vs `FileTypeCSV` could go stale. Mitigation: clear the detection cache when the plugin registry changes, or exclude plugin-extension files from caching. Low frequency, but call it out.
- Concurrency: multiple tabs opening the same file concurrently must not race on the map. Use `sync.RWMutex`; a duplicate miss that both compute is harmless.
- Memory: the cache holds tiny entries keyed by path. Add eviction on tab close (via clear) so long sessions opening many files do not grow unbounded; a size cap is optional.

## Testing plan

- Unit test the cached wrapper: first call for a plain `.csv` performs detection, second call for the same unchanged file returns the same result without a second magic peek. Assert peek count by injecting a counting hook or by using a temp file and checking behavior.
- Unit test invalidation: detect, then modify the file (change size or mtime), then detect again and assert re-detection occurred.
- Compressed-without-extension case: a gzip stream in a file named `data.csv` still detects gzip on first call and stays cached.
- Directory test: opening a directory of N members triggers at most one detection per member across the whole load, not one per dispatch.
- Plugin case (if not excluded): registering a plugin after a file was detected as CSV, then clearing the cache, yields `FileTypePlugin`.
- Regression: existing `repro_test.go`, `directory_truncation_test.go`, and format tests still pass; verify header/count/reader results are byte-identical to pre-change.
- Benchmark or log-based check: count `os.Open` calls (or magic peeks) across a representative CSV open before and after to confirm the reduction.

## Unresolved questions

- Keep mtime+size guard (robust, one stat per detection) or go session-scoped no-guard (fewer syscalls, cleared on tab close)?
- Cache scope: package-global vs per-App instance? Global simpler; per-App cleaner for teardown and tests.
- Plugin-extension files: exclude from cache, or wire cache-clear into registry add/remove/toggle?
- Who owns cache-clear on tab close and file re-open: add explicit `ClearDetectionCache` calls, or rely solely on the mtime guard?
- Should the public `DetectFileTypeAndCompression` itself become cached, or only the internal callers, to avoid surprising external callers with memoized results?
