# Plan: Speed up directory loads

## Summary

Opening a large directory of compressed JSON (measured against a CloudTrail S3 archive) decompresses, JSON-parses and row-converts every member file **five times** for a single open-plus-first-query, holds the entire dataset in RAM as `[]string`, does all of it single threaded, and gives the user no progress or cancel while it happens. On a 5 GB gzipped archive (~50 GB of JSON) that is roughly 35 minutes of pure CPU and tens of GB of resident memory, so the app effectively never becomes usable.

Measured on a synthetic CloudTrail archive (200 `.json.gz`, 3.9 MB on disk, 117 MB uncompressed JSON, 80k records), with counters on `DecompressFile` / `parseJSONData` / `ApplyJSONPath`:

```
open dir tab               931ms | decompress=200   json-parse=200  (117 MB)
first query (cumulative)  3.949s | decompress=1000  json-parse=1000 (586 MB)
```

586 MB parsed to display a 117 MB dataset. Resident rows measured at 130 MB per 117 MB of JSON with 2.9 GB of allocation churn.

## Current behaviour

### The five passes

| # | Call site | What it does |
|---|---|---|
| 1 | `app_tabs.go` `OpenDirectoryTabWithOptions` -> `GetDirectoryHeader` | parses every file, keeps only the header |
| 2 | `query/integration.go` `buildPipelineFromQuery` -> `reader.Header()` | new `FileReader`; header is deliberately not seeded for directory tabs (`reader.go` `NewFileReader`), so full pass again |
| 3 | `query/integration.go` `executeQuery` -> `detectTimestampIndex` -> `reader.Header()` | another new `FileReader`, again |
| 4 | `reader.go` `loadRowsFromDirectory` -> `NewDirectoryReader` -> `GetDirectoryHeader` | fresh `DirectoryInfo` from a fresh `DiscoverFiles`, so `Headers` cache is nil, again |
| 5 | `DirectoryReader.Read` -> `GetReader` per file | the actual data read, again |

`DiscoverFiles` re-globs the whole tree on each of those.

### Amplifiers

- **Compressed files bypass the Row cache entirely.** `proxy.go` `ReadHeaderWithOptions` / `GetRowCountWithOptions` / `GetReader` branch on compression *before* file type, sending every `.gz` to `DecompressFile` plus the uncached `*FromBytes` variants. The "parse once" caching in `GetOrParseJSONAsRows` is dead for any compressed JSON, which is why the five passes never collapse.
- **JSON is round-tripped through CSV text.** `GetJSONReader` builds rows, serializes them to CSV with `SliceReader`, then re-parses with `csv.Reader`. Pass 5 pays a full serialize plus reparse.
- **`CalculateDirectoryHash` SHA256s every byte** of the archive at open, and again on workspace restore (`app.go` `AddFileToWorkspace`, `processDirectoryForMatch`, `CheckDirectoryHashMismatch`).
- **Everything is serial**, despite member files being completely independent.
- **`CacheSizeLimitMB` defaults to 100**, so a large dataset never fits the base-data cache and every subsequent query repeats the whole thing.
- **No progress or cancel.** `directory:discovery:progress` fires only during the glob; the UI then goes dark for the entire multi-minute parse with no way out.

## Fixes in scope

Numbered as in the diagnosis. 7 (streaming / on-disk store) is explicitly out of scope.

### 1. One discovery and one header pass per directory

Add a snapshot cache in `fileloader` keyed by `(absDirPath, pattern, maxFiles, options)` holding the `*DirectoryInfo` with its `Headers` map already populated. Every entry point (`ReadHeaderForPath`, `GetRowCountForPath`, `GetReaderForPath`, `loadRowsFromDirectory`, `getDirectoryReaderForTab`, `OpenDirectoryTabWithOptions`) resolves through it, so passes 1 to 4 collapse into one. Also seed `FileReader.header` for directory tabs, which `NewFileReader` currently refuses to do.

Snapshot is authoritative for the life of the tab, matching the existing convention that the open-time hash is authoritative. Invalidated on tab close, on file-limit setting change, and via an explicit clear.

### 2. Sample-based schema instead of parsing every file

New setting `DirectorySchemaSampleFiles` (default 25, 0 = scan all). The union header is built from the sampled files only. Correctness is preserved because `DirectoryReader` grows the union header when a later file introduces unseen columns: new columns append at the end so existing indices stay stable, and `loadRowsFromDirectory` pads short rows once at the end (only when growth actually happened, which is never for a homogeneous archive). No column is ever dropped.

### 3. Metadata directory hash

`CalculateDirectoryHash` hashes sorted `(relPath, size, mtime)` instead of file contents; discovery already stats every file, so it is free. The legacy content hash stays available as `CalculateDirectoryContentHash` behind a `DirectoryContentHash` setting.

Migration matters: existing workspaces store content hashes, and annotations are keyed by hash. `OpenDirectoryTabWithOptions` computes the fast hash, and if the workspace has no file under it, falls back to the legacy content hash and adopts the stored value so annotations keep resolving. `CheckDirectoryHashMismatch` and `processDirectoryForMatch` do the same before declaring a mismatch.

### 4. Compressed JSON through the Row cache

Dispatch on file type before compression in `proxy.go`, mirroring what XLSX already does, so `.json.gz` goes through `GetOrParseJSONAsRows` (which decompresses internally via `parseJSONFile`). Decompression warnings must be preserved by moving `SetDecompressionWarning` into `parseJSONFile`. `reader.go` `loadRows` and `query/integration.go`'s `sharedFromBaseData` flag both use extension-only `DetectFileType` and must switch to the compression-aware detection, or the cache will mis-account row sizes.

### 5. Rows-native `DirectoryReader`

Add `GetRowsForFile` returning `(header, [][]string)` straight from the parse, and give `DirectoryReader` a rows-native mode for JSON/XLSX/plugin members, dropping the CSV serialize plus reparse. CSV members keep the streaming `csv.Reader` path so a single huge CSV member is not materialized unnecessarily.

### 6. Parallelism

Worker pool at `min(NumCPU, 8)` for the header/schema sample pass, for the legacy content hash, and for the row load. The row load uses chunked fan-out: files are processed in chunks of `workers`, each chunk collected in file order before the next starts, which keeps output order deterministic and memory bounded to `workers` files in flight.

### 8. Progress, cancel, and a memory warning

- Cancellable open: `context.Context` threaded through discovery, hashing, schema and row load; `CancelDirectoryOpen()` exposed to the frontend.
- `directory:open:progress` events carrying `{phase, current, total, message}` for the `discovering`, `hashing`, `schema` and `loading` phases, plus `directory:open:done`.
- A `DirectoryOpenProgress` overlay with a live phase, counts and a Cancel button.
- Memory estimate: sample a few members to measure the real compression ratio, extrapolate to uncompressed bytes, compare against available system memory (per-OS helper: `/proc/meminfo` on Linux, `sysctl hw.memsize` on Darwin, `GlobalMemoryStatusEx` on Windows). Surface it in `PreviewDirectory` and as an acknowledgeable warning on open, mirroring the existing `truncated` dialog.
- `MaxDirectoryFiles` stays at 0 (unlimited). Silently truncating a forensic dataset is worse than a slow load, so this warns rather than caps.

## Affected files

- `application/app/fileloader/directory.go`: snapshot cache, sampled schema, growing union header, metadata hash, parallel helpers, ctx and progress.
- `application/app/fileloader/proxy.go`: type-before-compression dispatch, snapshot-aware `*ForPath` helpers.
- `application/app/fileloader/json.go`: decompression warning in `parseJSONFile`.
- `application/app/fileloader/reader.go`: seed directory header, parallel row load, padding.
- `application/app/fileloader/rows.go` (new): `GetRowsForFile`.
- `application/app/fileloader/meminfo_*.go` (new): per-OS available memory.
- `application/app/app_tabs.go`, `app_tab_helpers.go`, `app.go`: snapshot reuse, hash migration, progress events, cancel.
- `application/app/settings/types.go`, `service.go`: `DirectorySchemaSampleFiles`, `DirectoryContentHash`.
- `application/app/query/integration.go`: compression-aware file type for `sharedFromBaseData`.
- `application/frontend/src/`: `DirectoryOpenProgress` component, `DirectoryIngestDialog` estimate, `App.tsx` wiring, Settings UI.

## Testing

- Counter-based regression test asserting exactly one decompress plus parse per member file across open plus first query (the core guarantee; guards against reintroducing passes).
- Sampled schema: directory where a file beyond the sample window has an extra column; assert the column survives and lands in the right place with earlier rows padded.
- Metadata hash: changes on content edit (mtime moves), stable across a no-op reopen; legacy fallback resolves an existing content hash.
- Parallel load: row order identical to serial for a mixed-schema directory.
- Cancellation: cancel mid-load returns `context.Canceled` and leaves no tab behind.
- Existing `fileloader`, `query`, `settings` suites green.

## Outcome

All of 1 to 6 and 8 are implemented. Measured on the same synthetic archive as above (200 `.json.gz`, 117 MB of JSON, 80k records), with the same counters:

| | before | after |
|---|---|---|
| decompress + parse operations | 1000 (5 full passes) | 200 (1 full pass) |
| JSON parsed | 586 MB | 117 MB |
| open | 931 ms | 37 ms |
| first query | 3.949 s | 287 ms |
| total | 4.88 s | 0.32 s |

End to end through `OpenDirectoryTabWithOptions` plus `ExecuteQueryForTab`, on a 588 MB (1000 file) archive: open 35 ms, first query 1.5 s for 400k rows. Per-MB cost went from about 42 ms to about 2.6 ms, so the projected time for a 5 GB gzipped archive (~50 GB of JSON) drops from roughly 35 minutes of CPU to a couple of minutes, with the open itself effectively instant.

Memory is now the binding constraint rather than CPU, which is what fix 7 (out of scope here) addresses and what the new warning tells the user about up front.

### Changes from the plan as written

- **Header seeding was defeated at the app layer.** `executeQueryStreamingOptimized` rebuilds a `query.FileTab` from scratch and was dropping `Headers`, so the seeded header never reached the reader. Fixed by carrying `Headers` across.
- **The schema pass and the row load keyed the base-data cache differently.** `readFileHeader` passed a nil ingest timezone while the row load passed the tab's, so with a timezone override set every member would have been parsed twice despite the cache. `readFileHeader` now resolves the timezone from the options.
- **`DirectoryIngestDialog` is dead code** (App routes directories through `FileOptionsDialog`), so the preview-side UI work moved to `App.tsx` and a new `DirectoryOpenProgress` overlay rather than that component.
- **Row-loading progress comes from the query path, not the open path.** The rows are read during the first query, so `executeQueryStreamingOptimized` forwards loader phases to the same `directory:open:progress` event the overlay listens on. Without that the overlay vanished before the longest phase started.
- **A test-only parse observer** (`fileParseObserver`, called from `parseJSONFile` and `readXLSXAllRows`) was added so "each member is parsed once per load" is directly assertable. It is the property the whole change exists to establish and is otherwise invisible in output.
- **Available memory is read per OS without new dependencies** except promoting `golang.org/x/sys` from indirect to direct for the Darwin `sysctl` call. Linux parses `/proc/meminfo`, Windows calls `GlobalMemoryStatusEx` through `syscall`.

### Verified

- `go build ./...`, `go vet ./app/...`, `go test ./app/...` all green; `wails build -tags webkit2_41` succeeds; frontend `tsc --noEmit` and `vite build` clean.
- The GUI itself was not launched from this environment (no X authorization available), so the overlay and warning dialogs are verified by typecheck and build only, not visually.

## Unresolved questions

- Snapshot TTL, or invalidate only on tab close plus explicit clear? Implemented as the latter.
- Default sample size 25 too small for heterogeneous dirs? Growth path makes it safe but a low sample means more late growth.
- Should the memory warning hard-block the open above some multiple of available RAM rather than just warning? Currently warns only.
- `CacheSizeLimitMB` still defaults to 100, so a dataset larger than that is re-parsed on every query (measured: 282 ms per repeat query on the 117 MB set). Worth raising the default or scaling it to available memory.
