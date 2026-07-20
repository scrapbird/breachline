# Plan: Directory per-file header dedup

## Summary

When a directory tab is opened and its rows are loaded, every member file has its header parsed multiple times: once (or twice) to build the union header, and once more per file inside the row iteration, and then the data reader skips the header row a further time. For CSV members a header read is a cheap first-row read, but for XLSX and JSON members each header read is a full parse of the whole file, so a directory of XLSX or JSON files is parsed many times over. The goal is to read each member's header exactly once per load and reuse that cached header both for building the union schema and for mapping each file's rows into the unified schema.

## Current behavior

Reads per member file, counted across a single open-then-load cycle:

1. Union header build at open time. `ReadHeaderForPath` (application/app/fileloader/proxy.go:261) calls `DiscoverFiles` then `GetDirectoryHeader` (application/app/fileloader/directory.go:188). `GetDirectoryHeader` loops every file calling `readFileHeader` (application/app/fileloader/directory.go:220), which is `ReadHeaderWithOptions`. That is one header read per file.
2. Reader construction at load time. `loadRowsFromDirectory` (application/app/fileloader/reader.go:147) calls `DiscoverFiles` again, then `NewDirectoryReader` (application/app/fileloader/directory.go:241), which calls `GetDirectoryHeader` again to rebuild the union header. That is a second header read per file.
3. Row iteration. `DirectoryReader.Read` (application/app/fileloader/directory.go:277) opens each file with `GetReader` (a full read for the data stream), then separately calls `readFileHeader` again to populate `dr.currentHeader` (application/app/fileloader/directory.go:299), then skips the header row from the data reader when `!NoHeaderRow` (application/app/fileloader/directory.go:306). So per file this is a third header read plus a header-row skip on the already-open data stream.

Net for a directory rendered once: roughly three separate header reads per file (steps 1, 2, 3) plus one full data read. `DiscoverFiles` is also run twice (open path and load path); discovery itself is stat-only, so it is cheap, but it is redundant work.

`dr.currentHeader` exists because `mapToUnifiedSchema` (application/app/fileloader/directory.go:334) needs the source file's own column names to map each source column index to its position in the union header. Files may have differing headers, so this per-file header is genuinely required for correctness, not just an artifact.

## Proposed approach

Read each member header once and thread it through the rest of the pipeline.

1. Cache per-file headers on `DirectoryInfo`. Add a field such as `Headers map[string][]string` (path to normalized header) to `DirectoryInfo` (application/app/fileloader/directory.go:17). Populate it in a single pass, either inside `DiscoverFiles` or in a dedicated `buildDirectoryHeaders(info, options)` helper that both the union build and the reader share. `GetDirectoryHeader` then computes the union from this map instead of re-reading files, and `NewDirectoryReader` reuses the same map rather than calling `GetDirectoryHeader` again from scratch.
2. Reuse the cached header in `DirectoryReader.Read`. Instead of calling `readFileHeader` per file (application/app/fileloader/directory.go:299), look the current file's header up in the cached map. This removes the third header read.
3. Avoid the redundant `GetReader` header skip where possible. Today `Read` skips the first data row because the header was read separately. Since we already hold the cached header, the skip is still correct (the data stream still contains the header row and must be advanced past it). The skip reads one row from the already-open stream, so it is not a separate file open; keep it, but document that it is a stream advance, not a re-read. No change needed here unless we later switch to a reader that can be told to skip the header itself.
4. Single discovery. Have the open path stash the `DirectoryInfo` (including the header map) on the tab so `loadRowsFromDirectory` does not re-run `DiscoverFiles` and header building. If threading `DirectoryInfo` onto the tab is too invasive for this change, at minimum share the header map within a single `NewDirectoryReader` call so steps 2 and 3 collapse into one read. Treat cross-call reuse (open vs load) as a stretch goal and call it out in Unresolved questions.

Correctness for `mapToUnifiedSchema` is preserved because the cached per-file header is the exact same normalized header that `readFileHeader` produced; we are only removing repeat reads, not changing what header a file maps against. The union header ordering (first appearance across files, application/app/fileloader/directory.go:199) must be computed from the files in the same discovery order to keep column positions stable.

## Affected files and functions

- application/app/fileloader/directory.go: `DirectoryInfo` struct; `DiscoverFiles` or a new `buildDirectoryHeaders` helper; `GetDirectoryHeader`; `NewDirectoryReader`; `DirectoryReader` struct (may store the header map); `DirectoryReader.Read`; `readFileHeader` (may become internal to the single-pass build).
- application/app/fileloader/reader.go: `loadRowsFromDirectory` (avoid the second `DiscoverFiles` plus union rebuild if we thread `DirectoryInfo` through).
- application/app/fileloader/proxy.go: `ReadHeaderForPath`, `GetRowCountForPath`, `GetReaderForPath` (all call `DiscoverFiles` plus `GetDirectoryHeader`; keep them working against the cached map).
- Possibly the app tab layer if we choose to stash `DirectoryInfo` on the tab for cross-call reuse.

## Edge cases and risks

- Files with differing schemas: the union header and per-file header must stay independently correct; the map must key headers by the exact file path used in `mapToUnifiedSchema`.
- Unreadable or skipped files: `GetDirectoryHeader` currently skips files whose header fails to read (continue on error). The single-pass build must preserve that skip behavior, and `Read` must handle a file that is present in `info.Files` but absent from the header map (skip it rather than panic on a nil header).
- Source column: when `IncludeSourceColumn` is set, `__source_file__` is appended to the union header only, not to any per-file header; the map must store the raw per-file headers without the synthetic column so mapping stays aligned.
- NoHeaderRow: with synthetic headers the per-file header is generated from column count; caching still applies, and the header-row skip in `Read` must remain gated on `!NoHeaderRow`.
- Compressed members: a header read for a compressed member currently decompresses the whole file; caching the header is exactly where the big win is, but note decompression caching is a separate TODO, so a cached header still avoids repeat inflation for the header path only.
- Memory: caching one header slice per file is negligible for typical directories; note it scales with file count if a directory has very many files.
- Staleness: if files change on disk between open and load, a cached header could go stale; today each read re-reads, so behavior changes slightly. Given the file hash is computed at open, treating the open-time snapshot as authoritative is consistent with the rest of the app.

## Testing plan

- Extend the existing application/app/fileloader/directory_truncation_test.go with header-focused cases, or add a new `directory_header_test.go`.
- Unit test: a directory of CSV files with differing columns yields the correct union header and correctly mapped rows (verify a value lands in the right union column for a file whose column order differs).
- Regression test: confirm `IncludeSourceColumn` still appends `__source_file__` and populates the relative path.
- Regression test: `NoHeaderRow` directories still generate synthetic headers and do not skip a data row.
- Instrumentation or a counting fake `readFileHeader` to assert each member header is read once per load (the core goal), guarding against future re-introduction of repeat reads.
- Run the full fileloader package tests plus any app-level directory tab tests.

## Unresolved questions

- Store header map on `DirectoryInfo` or on `DirectoryReader` only? Former enables cross-call reuse, latter is smaller blast radius.
- Worth threading `DirectoryInfo` onto the tab to kill the second `DiscoverFiles` plus union rebuild, or leave open-vs-load discovery duplicated for now?
- Snapshot semantics ok? Cached headers assume files unchanged between open and load; acceptable given open-time hashing?
- Any caller outside proxy.go relying on `GetDirectoryHeader` re-reading files fresh each call?
