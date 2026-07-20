# TODO

## Data/UI Issues

- [x] Highlight search terms in results
- [x] 🔴 **Error** Fix file load failure with corrupted CSV rows
- [x] 🔴 **Error** Fix issues when file has no header row
- [ ] 🔴 **Error** Fix TabInfo struct defined in two places
- [x] 🔴 **Error** Fix timestamp parsing failure when some rows have no timestamp
- [x] 🔴 **Error** Fix issue where annotations being added when strip or columns operation is in place do not work
- [ ] Cancel query history selection if user keeps typing
- [x] Dont redraw the histogram when switching back to tabs
- [x] **Error** Fix annotation cache not invalidating on annotation changes
- [x] Figure out the cache size issue with json files
- [x] Show status of locate file scans, log of files scanned and allow cancel
- [x] Sort workspace files by all file open options
- [x] Show error dialog when trying to open unlocated workspace files
- [x] 🔴 **Error** Fix JSON file options not being applied when opening with options dialog
- [x] Fix json file with different timezone override cant be opened at the same time
- [x] Make FindDisplayIndexForOriginalRow not execute its own query just to get the matching row
- [x] Fix frontend jump to row functionality (sets timeout instead of intelligently detecting when the grid has refreshed)
- [x] Make getCSVRowCountForTab use same type for directory options as api so there is no need to translate. do the same for fileoptions
- [x] Fix when opening json file without options, the option to add a jpath is never given to the user
- [ ] fix strip operation
- [x] Add plugin used to open file to fileoptions maybe? (not sure we need this yet)
- [x] Make plugins have uuid like originally planned so they make more sense for sync workspaces
- [x] Fix open with options when opening file with a plugin not taking effect (no header, timezone override not working)

## Backend/Infrastructure

- [ ] Add proper rate limiting to sync-api endpoints
- [ ] Make sync package use sync-api types properly
- [ ] Ensure attributevalue.MarshalMap uses types everywhere
- [ ] 🔴 **Error** Fix secrets manager license signing key in license-generator
- [ ] Add proper logging to lambda functions
- [ ] Add cloudwatch alerts for critical errors
- [ ] Add lambda to forward cloudwatch alerts to discord

## Business/Pricing

- [ ] Change yearly pricing to 100 USD
- [ ] Decide on sync pricing model
- [ ] Add new pricing objects for remote workspace API
- [ ] Workspace sync service - decide pricing model

## Documentation

- [ ] Update STRIPE_INTEGRATION.md doc (remove non-existent API)
- [x] Normalize license file ending (.lic) across docs and implementation

## Security/Deployment

- [ ] Add yubikey 2 to cloudflare MFA
- [ ] ⚠️ **garble vs AV trust conflict** - obfuscation is itself an AV malware heuristic + Go binaries already trigger Defender ML false positives. garble randomizes each build (pin `GARBLE_SEED` or every release resets SmartScreen reputation + Defender FP-clearing). Decide if obfuscation is worth the distribution cost; if kept, sign/Store-distribute after obfuscating.
- [ ] Set up code signing / trusted distribution (NZ-based individual):
  - **Windows: Microsoft Store** - cheapest, registration now **free** (individuals Sept 2025, companies May 2026), MS signs for free, Store apps bypass SmartScreen. Use **own commerce** (keep existing Stripe + .lic flow) → keep 100%, MS takes 0%. Cost = MSIX packaging effort (Wails ships NSIS, not MSIX) + `broadFileSystemAccess` capability for file reads. Only buy a Windows cert (~$200/yr OV) if a signed *direct-download* .exe is also wanted (OV==EV now for SmartScreen; EV no longer instant, skip EV).
  - **macOS: Apple Developer Program ($99/yr)** - unavoidable. Notarized direct-download (Mac App Store needs sandboxing a forensics tool can't accept; Homebrew cask now *requires* notarization by Sept 2026 too).
  - **Linux: GPG signatures** - free.
  - Azure Trusted/Artifact Signing ruled out: Windows-only + NZ not in supported countries.
  - Free AV lever regardless: submit binaries to Microsoft Defender WDSI false-positive portal (+ other vendors).
  - Total minimum ~**$99/yr** (Apple only) if Windows goes Store-first.

## Features

- [x] Workspace sync service
- [x] Ingest plugins (backend complete, UI pending)
  - [x] Plugin registry and manifest validation
  - [x] Plugin executor (subprocess-based)
  - [x] FileLoader integration
  - [x] Backend API (AddPlugin, RemovePlugin, TogglePlugin)
  - [x] Example plugin (tools/example-plugin/)
  - [x] Developer documentation (doc/PLUGIN_DEVELOPER_GUIDE.md)
  - [x] Settings UI with tabs (General/Plugins)
  - [x] Plugin management UI component
- [x] ctrl-p hotkey to open workspace file picker
- [x] Add "strip" query filter which strips any columns which don't have results in any rows from the output
- [ ] Add proper wildcard search (not just prefix)
- [x] Make the application save the desired window size in the config file whenever the window is resized. This size should be used next time the application starts
- [x] Strip whitespace from each stage before comparing cache so that `filter hello ` and `filter hello` match the same cache entry

## Website/Support

- [ ] Set up domain
- [ ] Set up cloudflare+SES integration
- [ ] Finish Stripe sandbox setup
- [ ] Finish website
- [ ] Create discord for support

## File open improvements

- [ ] XLSX: add Row/header caching (like JSON) so header + data don't each re-parse the whole workbook; currently header, count, and data each do a full `excelize.OpenFile` + `GetRows`, ~4 full parses per open
- [ ] Directory: dedupe per-file header reads; union header is read at open, again in `NewDirectoryReader`, and again per-file in `DirectoryReader.Read` (3x/file, and each is a full parse for XLSX/JSON members). Reuse the union header instead of re-reading
- [ ] Add a decompression cache (or single-decompress path) so compressed CSV/XLSX don't inflate the whole file on both the header and data dispatch
- [ ] Avoid the separate 6-byte magic-byte peek (`DetectCompressionByMagic`) on every dispatcher call for files without a compression extension; detect once at open and reuse
- [ ] Share the header already read at open with the first-load `FileReader` so CSV/XLSX don't re-read the header (open uses one reader instance, first query another)

## Performance

- [x] Make JSON file parsing more efficient (avoid multiple reads, implement streaming)
- [ ] Optimize periodic sync for remote workspaces (incremental sync, timestamp-based change detection)
- [x] Optimize the query pipeline and remove duplicate code

## Large File Improvements

- [x] Disable default timestamp sorting to fix large file loading performance
- [x] Implement chunked growth strategy for row allocation
- [ ] Add progress indicators for large file operations with cancellation support
- [x] Implement lazy loading architecture with virtual scrolling and on-demand row loading
- [ ] Implement file format optimization (Parquet/Arrow) with pre-built sorted indices
