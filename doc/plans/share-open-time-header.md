# Plan: Share the open-time header with the first-load reader

## Summary

At open we read the file header once, hand it to the frontend inside `TabInfo.Headers`, and then throw it away. The header is never stored on the `FileTab`, so the first query re-reads it from disk two more times (once to build the pipeline, once to detect the timestamp column). For CSV that is a cheap first-row read, but for XLSX each of those reads is a full workbook parse, and for directories each is a full union-header scan across every file. This plan stores the header on the tab at open time and seeds it into the `FileReader` so the first-load path reuses it instead of re-reading. The change is format-agnostic and small, and it also removes redundant header reads for directories and plugins as a side benefit.

## Current behavior

Open path:
- `OpenFileTabWithOptions` (application/app/app_tabs.go:96) creates the tab, then calls `readHeaderForTab` (application/app/app_tab_helpers.go:16) which dispatches to `fileloader.ReadHeaderWithOptions` / `ReadHeaderForPath`. This is the first header read.
- The result is returned to the frontend as `TabInfo.Headers` (application/app/app_tabs.go:148-156) but is NOT written back onto the `FileTab`. The `interfaces.FileTab` struct (application/app/interfaces/types.go:41) has no persistent `Headers` field (only `SortedHeader`, which is sort-cache state).

First-query path (application/app/query/integration.go):
- `buildPipelineFromQuery` (integration.go:337) constructs a throwaway `FileReader` purely to call `.Header()` (integration.go:342-344). Header read number two.
- The main read path constructs a second `FileReader` (integration.go:278), calls `detectTimestampIndex` which calls `.Header()` again (integration.go:544-545). Header read number three. The subsequent `ReadRowsWithTimeIdx` / `ReadRowsWithSort` call `r.Header()` internally (application/app/fileloader/reader.go:149, 336) but that hits the same instance cache (`r.header`, reader.go:19, 86-90) so it does not add a disk read.

Net header reads for open plus first query, per format:
- CSV: 3 reads, each first-row-only. Cheap but wasteful.
- XLSX: 3 reads, each a full `excelize.OpenFile` + `GetRows` parse of the whole workbook. Expensive.
- Directory: 3 union-header scans, each reading the header of every matched file (and for XLSX/JSON members each of those is a full parse). Very expensive.
- Plugin: 3 subprocess executions in header mode.
- JSON: effectively deduped already because `GetOrParseJSONAsRows` (application/app/fileloader/json.go:313) caches header and rows keyed by path plus jpath plus timeIdx plus tz, so reads two and three are cache hits.

The per-instance cache exists (`FileReader.header`, reader.go:19) but there is no way to seed it, so it never helps across the three separate reader instances.

## Proposed approach

Store the header on the tab at open, then seed the reader from it.

1. Add a persistent `Headers []string` field to `interfaces.FileTab` (application/app/interfaces/types.go:41). Document it as "header captured at open, immutable for the life of the tab".
2. In `OpenFileTabWithOptions` (and the directory open path `OpenDirectoryTabWithOptions`) write the header returned by `readHeaderForTab` onto `tab.Headers` before returning `TabInfo`. This is the same value already placed in `TabInfo.Headers`, so no extra read.
3. In `NewFileReader` (application/app/fileloader/reader.go:28), when the incoming `tab` is an `*interfaces.FileTab` with a non-empty `Headers`, pre-populate `r.header` from it. Because `Header()` (reader.go:85-114) returns `r.header` immediately when non-nil, all three call sites stop hitting disk for the header. `SimpleFileTab` has no header and keeps returning nil, so it is unaffected.

Notes and safeguards:
- Only seed when the header is non-empty, so an open that failed to capture a header still falls back to reading from disk.
- The seeded header was produced from the same `tab.Options` (NoHeaderRow, JPath, IncludeSourceColumn, FilePattern) that the reader uses, so synthetic-header (NoHeaderRow=true) and jpath cases stay consistent. Tabs are effectively immutable per option combination: the frontend opens a new tab for each filepath plus options combination (see the note at application/app/app_tabs.go:118-121), so a live tab's header does not drift as options change.
- Header content is timezone-independent for every format (column names for CSV/XLSX/JSON, source column appended for directories), so seeding does not interfere with the timezone-sensitive cache keys used elsewhere. Timestamp column detection still runs on the seeded header exactly as before.
- This mostly benefits XLSX, directories, and plugins. It overlaps with the separate "XLSX row/header caching" plan: if XLSX gains its own Row/header cache, the duplicate parse disappears there too, but header seeding is simpler, lands first, and helps directories and plugins which the XLSX plan does not touch. The two are complementary, not conflicting.
- Keep the change confined to seeding an existing cache field. Do not change the `FileReader` public interface if avoidable; reading `tab.Headers` inside `NewFileReader` requires no signature change. An alternative (add a `NewFileReaderWithHeader` constructor or an optional header argument) is more invasive across call sites and is not recommended unless seeding from the tab proves insufficient.

## Affected files and functions

- application/app/interfaces/types.go: add `Headers []string` to `FileTab`.
- application/app/app_tabs.go: `OpenFileTabWithOptions` and `OpenDirectoryTabWithOptions` set `tab.Headers` from the value already computed for `TabInfo.Headers`.
- application/app/fileloader/reader.go: `NewFileReader` seeds `r.header` from `*interfaces.FileTab.Headers` when present.
- No change required in application/app/query/integration.go: both reader instances already receive the tab, so seeding is transparent to them.

## Edge cases and risks

- File changed on disk between open and first query: the seeded header could be stale relative to freshly read rows. This is an existing risk (the current code re-reads the header from the same possibly-changed file) and the window is small within a session. Acceptable; note it. If we want to be strict, gate seeding behind an unchanged mtime/size check, but that adds a stat and is probably not worth it.
- NoHeaderRow true: header is synthetic (unnamed_a, unnamed_b). Produced from the same option, so seeding is correct.
- Directory tabs: the seeded union header must match the files the reader discovers. Discovery is re-run at query time with the same pattern and file limit, so as long as the directory contents are stable the union matches. If files were added or removed mid-session the union could differ; this is the same staleness risk as above and equally low.
- Plugins: seeding skips a header-mode subprocess exec. Plugin output for header mode is deterministic for a given file, so this is safe.
- JSON: seeding is redundant (Row cache already dedupes) but harmless; leaving it seeded keeps the code uniform.
- Options mutated in place: if any code path mutates `tab.Options` after open without creating a new tab, the seeded header could become inconsistent. Verify no such path exists (search for assignments to `tab.Options` outside open). If one exists, clear `tab.Headers` there.

## Testing plan

- Unit: a `FileReader` created from a `FileTab` with `Headers` set returns that header from `Header()` without opening the file (assert via a path that does not exist on disk but has seeded headers, or via a read counter).
- Unit: `FileTab` with empty `Headers` still reads the header from disk (regression guard for the fallback).
- Integration: open a CSV, XLSX, JSON, directory, and plugin file, run the first query, and assert results and detected timestamp column are unchanged from current behavior.
- Instrumentation check: add a temporary counter (or reuse existing debug logging) to confirm XLSX goes from three full parses to one across open plus first query.
- Verify NoHeaderRow true and a jpath JSON tab still produce identical headers and row counts before and after the change.

## Unresolved questions

- Any path mutating `tab.Options` in place after open (would require invalidating `tab.Headers`)?
- Directory/file staleness within a session: acceptable, or gate seeding behind mtime/size check?
- Seed for JSON too (uniform code) or skip since Row cache already dedupes?
- Land this before or after the XLSX row/header caching plan, given overlap?
