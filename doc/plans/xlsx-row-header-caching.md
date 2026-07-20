# Plan: XLSX row/header caching

## Summary

XLSX is the only tabular format with no loader-level caching. Every header read, row-count, and data read re-runs `excelize.OpenFile` + `GetRows(sheet)`, which fully materializes the whole workbook in memory each time. A cold open of an XLSX file triggers roughly 4 full workbook parses. JSON already solves this with a Row-based base-data cache (`GetOrParseJSONAsRows`) keyed by file path plus options, backed by `cache.BaseFileCacheEntry` and invalidated on file mod-time change. The plan is to give XLSX the same treatment: parse the workbook once into `[]*interfaces.Row` plus header, cache it, and route the header/count/data/reader paths through the cache so repeat parses collapse to cache hits.

## Current behavior

XLSX operations, each doing an independent full parse (`excelize.OpenFile` then `GetRows(firstSheet)`):

- `ReadXLSXHeaderWithOptions` - application/app/fileloader/xlsx.go:27 (full parse, returns only header)
- `GetXLSXRowCountWithOptions` - application/app/fileloader/xlsx.go:80 (full parse, returns only count)
- `GetXLSXReader` - application/app/fileloader/xlsx.go:124 (full parse, converts every row to an in-memory CSV string, wraps in a `csv.Reader`)
- FromBytes variants for compressed `.xlsx.gz` etc: xlsx.go:182, xlsx.go:225, xlsx.go:267

Dispatch (no XLSX caching branch anywhere):

- `ReadHeaderWithOptions` routes XLSX to `ReadXLSXHeaderWithOptions` - application/app/fileloader/proxy.go:97
- `GetRowCountWithOptions` routes XLSX to `GetXLSXRowCountWithOptions` - application/app/fileloader/proxy.go:168
- `GetReader` routes XLSX to `GetXLSXReader` - application/app/fileloader/proxy.go:227
- `loadRows` only special-cases JSON+jpath (`loadJSONRowsWithCaching`); XLSX falls through to `loadRowsFromReader` which calls `GetReader` - application/app/fileloader/reader.go:136-143

Full-parse count on a cold open of an XLSX tab:

1. Open: `NewFileTab` runs `CalculateFileHash` (raw byte stream, not an excelize parse) then `readHeaderForTab` -> `ReadHeaderWithOptions` -> `ReadXLSXHeaderWithOptions` = parse #1.
2. First query build: `buildPipelineFromQuery` creates a `FileReader` and calls `reader.Header()` - application/app/query/integration.go:342-344 = parse #2.
3. First query exec: a second `FileReader` is created at integration.go:278. `detectTimestampIndex` -> `reader.Header()` populates that instance's header cache = parse #3, then `loadRowsFromReader` calls `GetReader` for the data = parse #4.

So header work alone is parsed 3 times and the data once, all on the same unchanged file. The query engine does cache the assembled base result under `baseFileCacheKey` (integration.go:262, stored at integration.go:305-307 with `sharedFromBaseData=false` for CSV/XLSX), but that only helps the second query onward within a session; it does not dedupe the multiple parses that happen during the initial open, and it does not help the standalone header/count dispatch paths (MCP, directory union headers, preview).

Contrast JSON: `loadRows` detects JSON+jpath and calls `loadJSONRowsWithCaching` (reader.go:138-139) -> `GetOrParseJSONAsRows` (application/app/fileloader/json.go:313), which checks the base-data cache and the header cache before parsing, parses at most once, and stores `[]*interfaces.Row` for reuse. Header dispatch for JSON also flows through the same cached function (`ReadJSONHeaderWithTimezone`, json.go:233). Note `parseJSONFile` (json.go:57) handles decompression internally, so caching works uniformly for compressed and uncompressed JSON via the real file path.

Cache substrate already available and format-neutral:

- Interface `JSONCache` with `GetBaseData` / `StoreBaseData` / `GetHeader` / `StoreHeader` - application/app/fileloader/json.go:19 (named for JSON but generic in shape)
- Injected once at startup via `SetJSONCache` - json.go:34, wired in `App.Startup`
- Backed by `cache.BaseFileCacheEntry` (Header, Rows, TimestampStats, ModTime, Size) - application/app/cache/types.go:78
- `GetBaseData` validates freshness by `os.Stat` mod-time compare and evicts on change - application/app/cache/cache.go:771-808; `StoreBaseData` records mod-time and enforces the size limit / LRU - cache.go:812

## Proposed approach

Mirror the JSON caching path for XLSX rather than inventing a new mechanism.

1. Add `GetOrParseXLSXAsRows(filePath string, options FileOptions, timeIdx int, ingestTz *time.Location) ([]string, []*interfaces.Row, *interfaces.TimestampStats, error)` in a new `xlsx.go` section, structurally parallel to `GetOrParseJSONAsRows`:
   - Handle decompression internally (detect via `DetectFileTypeAndCompression`, `DecompressFile` to bytes, then `excelize.OpenReader`) so caching keys on the real path uniformly for `.xlsx` and `.xlsx.gz`. This is the same pattern `parseJSONFile` uses and it removes the need to special-case the FromBytes variants for caching.
   - Parse the first sheet once, build `[]*interfaces.Row` with pre-parsed timestamps and `TimestampStats`, honoring `NoHeaderRow` (synthetic headers plus first row treated as data) exactly as the current header/reader code does.
   - Check `GetBaseData` first, store via `StoreBaseData` on miss, and use `GetHeader` / `StoreHeader` for the auto-detect fast path, identical to JSON.

2. Route `loadRows` to a new `loadXLSXRowsWithCaching` (parallel to `loadJSONRowsWithCaching`, application/app/fileloader/reader.go:284) when `DetectFileType == FileTypeXLSX`, added alongside the existing JSON branch at reader.go:136-143. This is the change that removes the data-path parse duplication and lets XLSX rows be shared pointers.

3. Point the standalone dispatch paths at the cached function so header/count/reader all hit one parse:
   - `ReadHeaderWithOptions` XLSX case (proxy.go:97) -> call `GetOrParseXLSXAsRows` and return the header.
   - `GetRowCountWithOptions` XLSX case (proxy.go:168) -> return `len(rows)`.
   - `GetReader` XLSX case (proxy.go:227) -> build a `SliceReader` from cached header+rows (same trick `GetJSONReader` uses, json.go:266-286) instead of re-parsing. This keeps the `GetReader` contract intact for any caller not yet migrated to the caching path.

4. Cache-key design (via a `buildXLSXBaseDataCacheKey` helper mirroring `buildBaseDataCacheKey`, json.go:290):
   - Include file path, `NoHeaderRow`, resolved `timeIdx`, and effective ingest timezone string. `NoHeaderRow` MUST be part of the key for XLSX because it changes which rows are data vs header (JSON omits it because JSON has no header-row toggle). Timezone and timeIdx match JSON so a timestamp-column or tz change invalidates correctly.
   - Header cache key: path plus `NoHeaderRow` (timeIdx-independent), mirroring `buildHeaderCacheKey` (json.go:302).

5. Reuse the already-injected cache instance via `getJSONCache()`. Optionally rename the `JSONCache` interface and `SetJSONCache`/`getJSONCache` to format-neutral names (`BaseDataCache`, `SetBaseDataCache`) in a mechanical follow-up; not required for correctness since the interface is already generic. If renamed, keep `SetJSONCache` as a thin alias to avoid touching the `App.Startup` wiring in the same change.

6. Set `sharedFromBaseData = (fileType == JSON || fileType == XLSX)` at application/app/query/integration.go:305-306 once XLSX rows come from `baseDataStorage`, so query-cache size accounting stays accurate (XLSX rows become shared pointers, not an authoritative copy).

## Affected files and functions

- application/app/fileloader/xlsx.go - add `GetOrParseXLSXAsRows`, `buildXLSXBaseDataCacheKey`, internal decompress+parse helper; make header/count/reader thin wrappers over it.
- application/app/fileloader/reader.go - add XLSX branch in `loadRows` (reader.go:130) and new `loadXLSXRowsWithCaching`.
- application/app/fileloader/proxy.go - XLSX cases in `ReadHeaderWithOptions`, `GetRowCountWithOptions`, `GetReader` call the cached path; keep compressed handling delegated to the internal decompress in the cached function.
- application/app/fileloader/json.go - only if the interface is renamed for neutrality (optional).
- application/app/query/integration.go - extend `sharedFromBaseData` to include XLSX (integration.go:305).
- Tests as below.

## Edge cases and risks

- Compressed XLSX (`.xlsx.gz`, `.bz2`, `.xz`): handle decompression inside `GetOrParseXLSXAsRows` so the cache keys on the original path and `GetBaseData`'s `os.Stat` mod-time check validates against the real compressed file. Avoid caching in the FromBytes variants (no path, no mod-time anchor); leave them as uncached fallbacks or delete if unused after migration.
- Multiple sheets: current code always reads `sheets[0]`. Key stays valid because sheet selection is fixed. Add a code comment that any future multi-sheet support must fold the sheet name into the cache key.
- `NoHeaderRow`: must be in the key (see approach). Missing it would let a headered and a headerless open of the same file collide.
- Cache invalidation: inherited for free from `GetBaseData` mod-time validation (cache.go:786-792). No new invalidation logic.
- Memory: excelize already loads the entire workbook into memory via `GetRows`, so the cached `[]*Row` footprint is comparable to JSON; the existing size limit and LRU (`StoreBaseData` rejects oversize entries, cache.go:826) apply unchanged.
- Timestamp parsing parity: replicate the exact `ParseTimestampMillis` + `TimestampStats` accumulation JSON uses (json.go:394-430) so histogram and sort behavior are identical to the pre-change reader path.
- Behavior parity risk: the current `GetXLSXReader` CSV-stringifies then re-parses through `csv.Reader`, which can alter quoting/whitespace edge cases. Building `SliceReader` from cached `[]string` rows preserves cell values directly and should be equal or more faithful, but diff a few real workbooks to confirm no column-count regressions (the reader sets `FieldsPerRecord = -1`).

## Testing plan

- Unit: new `xlsx_caching_test.go` asserting a second `GetOrParseXLSXAsRows` for an unchanged file is a cache hit (parse counter or spy cache), returns identical `[]*Row` pointers, and that touching the file's mod-time forces a re-parse.
- Unit: `NoHeaderRow=true` vs `false` on the same file produce distinct cache entries and correct row counts (headerless has one more data row).
- Unit: compressed `.xlsx.gz` loads and caches by original path; mod-time change invalidates.
- Parity: header, row count, and full rows match the pre-change reader output for a fixture set (headered, headerless, empty sheet, single-row, wide/ragged rows).
- Integration: open an XLSX tab and assert only one workbook parse happens across open + first query (instrument `excelize.OpenFile`/`OpenReader` call count in a test build or via a wrapper), down from ~4.
- Regression: existing `repro_test.go` and any fileloader tests still pass.

## Unresolved questions

- Rename `JSONCache`/`SetJSONCache` to neutral names now, or defer to follow-up?
- Keep the uncached XLSX FromBytes variants as fallbacks, or delete after all callers use the cached path?
- Cache eviction: XLSX cell strings can be large; keep the shared 100MB base-data budget, or size XLSX entries differently?
- Any non-loader caller depends on `GetXLSXReader`'s current CSV-restringified output semantics that `SliceReader` would change?
- Worth also caching the directory union-header path for XLSX members here, or leave that to the directory header-dedup plan?
