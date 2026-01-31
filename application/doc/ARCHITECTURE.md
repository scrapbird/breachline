# Architecture & Components

## Backend

### Package Structure & Separation of Concerns

#### [main.go](../main.go) (Entry Point)

The [main.go](../main.go) file serves as the application entry point and handles:

1. **Service Instantiation**: Creates instances of core services:
   - `App` (main application logic)
   - `SettingsService` (settings management)
   - `LicenseService` (license validation)
   - `WorkspaceService` (workspace and annotation management)

2. **Service Wiring**: Establishes bidirectional references between services:
   - Settings and License services receive a reference to App for cache clearing
   - App receives a reference to WorkspaceService for annotation lookups

3. **Menu System**: Defines the native application menu structure:
   - File menu: Open, Copy, Import License, Settings
   - Workspace menu: Open/Close workspace, Add files, Export timeline
   - View menu: Toggle Histogram, Toggle Console
   - Help menu: Syntax, Shortcuts, About
   - All menu actions emit events to the frontend rather than calling backend functions directly

4. **Wails Configuration**: Sets up the Wails runtime with:
   - Window dimensions and styling
   - Asset embedding (frontend build)
   - Service binding for frontend-backend communication
   - Startup hooks for each service

#### app Package

The `app` package contains all backend business logic, organized into focused files:

**[app.go](../app/app.go)** - Core application structure:
- `App` struct: Central coordinator with context, tab management, and workspace service reference
- Mutex-protected tab storage (`tabs map[string]*FileTab`)
- Active tab tracking (`activeTabID`)
- Clipboard initialization (lazy-loaded)
- Helper methods for tab access (`GetActiveTab`, `GetTab`)
- Cache management (`ClearAllTabCaches`) for settings changes
- JSON preview functionality for JSONPath expressions

### Tab Implementation and State Tracking

**[tabs.go](../app/tabs.go)** - Tab data structure:
- `FileTab` struct: Encapsulates all state for a single opened file
  - Identification: `ID`, `FilePath`, `FileName`
  - JSON support: `IngestExpression` for JSONPath queries
  - Sorted data cache: `sortedHeader`, `sortedRows`, `sortedForFile`, `sortedTimeField`
  - Sort state tracking: `sortedByTime`, `sortedDesc`
  - Active sort control: `sortCancel`, `sortActive`, `sortSeq`, `sortCond`
  - Query cache: LRU cache (`queryCache`, `queryCacheOrder`) for filtered results
  - Settings tracking: `lastDisplayTZ`, `lastIngestTZ`, `lastTimestampFormat` for cache invalidation

**[app_tabs.go](../app/app_tabs.go)** - Tab management operations:
- `OpenFileTab(filePath)`: Creates new tab, handles CSV/XLSX/JSON detection
- `OpenFileWithDialogTab()`: Opens file picker and delegates to OpenFileTab
- `GetTabs()`: Returns all open tabs for UI display
- `SetActiveTab(tabID)`: Switches the active tab
- `CloseTab(tabID)`: Removes tab and auto-switches to another if closing active
- `GetActiveTabID()`: Returns current active tab identifier
- Tab-specific queries: `GetCSVRowCountForTab`, `GetCSVRowsFilteredForTab`, `GetHistogramForTab`
- Active tab queries: `GetCSVRowCount`, `GetCSVRowsFiltered`, `GetHistogram` (delegate to active tab)

**[app_tab_helpers.go](../app/app_tab_helpers.go)** - Internal tab operations:
- `readHeaderForTab(tab)`: Reads headers, dispatches to CSV/XLSX/JSON handlers based on file type
- `getCSVRowCountForTab(tab)`: Gets row count, handles JSONPath for JSON files
- `getReaderForTab(tab)`: Returns appropriate reader for the tab's file type
- `materializeQueryRowsForTab(tab, query, timeField)`: Executes query and returns full result set

**[app_tab_query.go](../app/app_tab_query.go)** - Query execution engine:
- `getCSVRowsFilteredForTab()`: Main query execution with pagination
  - Executes query via `executeQueryForTab`
  - Handles column projection (`columns` stage)
  - Applies timestamp formatting with timezone conversion
  - Manages annotation decoration via workspace service
  - Returns paginated results with total count
- `executeQueryForTab()`: Core query processor
  - Checks cache validity (settings, file, timeField)
  - Handles cache hit/miss with sort state management
  - Manages concurrent sort operations with cancellation
  - Parses and applies query filters (text search, time ranges)
  - Uses external sort for large datasets
  - Caches results for subsequent queries

**[app_tab_clipboard.go](../app/app_tab_clipboard.go)** - Clipboard operations:
- `copySelectionToClipboardForTab()`: Handles row selection copying
  - Supports range selection and virtual select-all
  - Applies query filters before copying
  - Projects specified columns only
  - Formats as TSV for pasting into spreadsheets
  - Reports number of rows copied

**[app_tab_histogram.go](../app/app_tab_histogram.go)** - Histogram data generation:
- `getHistogramForTab()`: Creates time-based histogram
  - Applies query filters
  - Buckets data by configurable time intervals
  - Returns bucket counts and time ranges

**[app_timestamp_column.go](../app/app_timestamp_column.go)** - Timestamp column management:
- `ValidateTimestampColumn()`: Validates if column contains parseable timestamps
- `SetTimestampColumn()`: Changes the timestamp column and expires caches

### Workspace Implementation

**[workspace.go](../app/workspace.go)** - Annotation and workspace management:

**Core Concepts:**
- **Composite Keys**: Files are tracked by `filepath::JPATHexpression` to support same file with different JSONPath expressions
- **Hash-Based Annotation**: Annotations use HighwayHash of all column values for row identification
- **First Column Index**: Annotations indexed by first column hash for fast lookup
- **Sequential Matching**: Full row matching compares all column hashes sequentially

**WorkspaceService struct:**
- `workspacePath`: Path to .breachline workspace file
- `annotationsMap`: Nested map `[compositeKey][firstColHash][]RowAnnotation`
- `fileMetadata`: Stores hash, jpath, description per file
- `hashKey`: 32-byte HighwayHash key for consistent hashing

**Key Methods:**
- `OpenWorkspace()`: Opens .breachline file, validates license, loads annotations and hash key
- `CloseWorkspace()`: Clears all annotations and updates window title
- `AddFileToWorkspace(filePath, jpath)`: Adds file with hash and optional JSONPath
- `AddAnnotations(filePath, rowIndices, ...)`: Batch annotate multiple rows
  - Calculates column hashes for each row
  - Checks for existing annotations and updates them
  - Saves workspace file after changes
- `IsRowAnnotated(filePath, row, ...)`: Checks if row has annotation
  - Uses first column hash for index lookup
  - Sequentially validates all columns
  - Returns note and color if annotated
- `GetRowAnnotations()`: Batch retrieval for multiple rows
- `DeleteAnnotations()`: Batch deletion of annotations
- `UpdateFileDescription()`: Sets user description for workspace file
- `GetWorkspaceFiles()`: Lists all tracked files with annotation counts
- `ExportWorkspaceTimeline()`: Exports merged timeline from all workspace files

**Workspace File Format (.breachline):**
```yaml
hash_key: <base64-encoded-32-byte-key>
files:
  - file_path: /path/to/file.csv
    hash: <file-content-hash>
    jpath: $.events[*]  # for JSON files
    description: User notes
    annotations:
      - column_hashes:
          - column_name: hash_value
          - column_name: hash_value
        note: Annotation text
        color: grey|blue|yellow|green|orange|red
```

### License Implementation

**[license.go](../app/license.go)** - JWT-based license validation:

**License Format:**
- Base64-encoded JWT signed with ECDSA
- Public key embedded in application
- Claims: `email`, `id` (UUID), `nbf` (start time), `exp` (expiration)

**LicenseService:**
- `ImportLicenseFile()`: Opens file dialog, reads license, validates, saves to settings
- `IsLicenseValid(content)`: Multi-step validation:
  1. Base64 decode
  2. Parse JWT with ECDSA signature verification
  3. Validate time constraints (nbf/exp)
  4. Validate required claims (email, UUID)
- `GetLicenseDetails()`: Extracts license information for display
- `IsLicensed()`: Global function checking if valid license exists in settings

**License Enforcement:**
- Workspace features require license
- Annotations require license
- Checks performed at operation entry points

### File Ingestion Architecture

**[ingest.go](../app/ingest.go)** - Multi-format file reader abstraction:

**Design Principles:**
- Format-agnostic interface: All readers return headers and CSV-compatible data
- Extensible: Clear pattern for adding new formats
- Type detection: Extension-based with fallback to CSV

**File Type Support:**
1. **CSV** (`FileTypeCSV`):
   - Direct streaming with `encoding/csv`
   - `ReadCSVHeader()`, `GetCSVRowCount()`, `GetCSVReader()`

2. **XLSX** (`FileTypeXLSX`):
   - Uses `excelize` library for Excel files
   - Reads first sheet only
   - Converts to CSV format in memory
   - `ReadXLSXHeader()`, `GetXLSXRowCount()`, `GetXLSXReader()`

3. **JSON** (`FileTypeJSON`):
   - Requires JSONPath expression for data extraction
   - Uses `ojg` library for JSONPath and parsing
   - Preserves field order unlike standard json package
   - `ReadJSONHeader()`, `GetJSONRowCount()`, `GetJSONReader()`
   - `PreviewJSONWithExpression()`: Shows preview with available keys
   - `ApplyJSONPath()`: Converts JSON to tabular format
     - Array of objects: Keys become headers, sorted alphabetically
     - Array of arrays: First array is header row
     - Nested objects/arrays: JSON-stringified in cells

**Proxy Functions:**
- `ReadHeader(filePath)`: Detects type and dispatches to appropriate handler
- `GetRowCount(filePath)`: Type-agnostic row counting
- `GetReader(filePath)`: Returns CSV-compatible reader for any format

**Adding New Format:**
1. Add `FileTypeXXX` constant
2. Update `DetectFileType()` with extension/magic number detection
3. Implement `ReadXXXHeader()`, `GetXXXRowCount()`, `GetXXXReader()`
4. Update proxy functions to dispatch to new handlers
5. Update file dialog filters in `app_tabs.go`

### Settings Implementation

**[settings.go](../app/settings.go)** - Application configuration management:

**Settings struct:**
```go
type Settings struct {
    SortByTime              bool   // Enable timestamp-based sorting
    SortDescending          bool   // Descending sort order
    EnableQueryCache        bool   // Cache query results
    DefaultIngestTimezone   string // TZ for parsing timestamps without timezone
    DisplayTimezone         string // TZ for displaying timestamps
    TimestampDisplayFormat  string // Format string (yyyy-MM-dd HH:mm:ss)
    License                 string // Base64-encoded JWT (not shown in UI)
}
```

**Default Values:**
- SortByTime: true
- SortDescending: false
- EnableQueryCache: true
- DefaultIngestTimezone: "Local"
- DisplayTimezone: "Local"
- TimestampDisplayFormat: "yyyy-MM-dd HH:mm:ss"

**SettingsService:**
- `GetSettings()`: Loads from disk, overlays on defaults
- `SaveSettings(settings)`: Saves only non-default values
  - Detects sort setting changes
  - Triggers cache clearing via App reference
  - Preserves license field even when not displayed
- `settingsFilePath()`: Returns `breachline.yml` in executable directory

**Settings Impact:**
- Sort changes: Clear all tab caches, re-sort on next query
- Timezone changes: Regenerate column definitions, refresh grid
- Cache toggle: Enable/disable query result caching
- Format changes: Affect timestamp display in grid and exports

### Time Management ([time.go](../app/time.go))

**Timestamp Parsing:**
- `parseTimestampMillis(s, loc)`: Flexible timestamp parser
  - Tries explicit timezone formats first (RFC3339, Z suffix, offsets)
  - Falls back to timezone-less formats using provided location
  - Supports multiple millisecond precision levels
  - Handles epoch seconds and milliseconds
  - Returns epoch milliseconds

**Supported Formats:**
- ISO 8601: `2006-01-02T15:04:05Z`, with/without timezone
- Space-separated: `2006-01-02 15:04:05`, with/without timezone
- Millisecond variations: `.0`, `.00`, `.000`
- Epoch: Integer seconds or milliseconds
- Timezone formats: RFC3339, Z suffix, numeric offsets, MST names

**Timezone Utilities:**
- `getLocationForTZ(name)`: Resolves timezone name to `time.Location`
  - "Local" → system local timezone
  - "UTC" → UTC
  - IANA name → loaded location
  - Fallback to Local on error

**Relative Time Parsing:**
- `parseFlexibleTimeInLoc(s, now, loc)`: Parses absolute or relative time
  - "now" → current time
  - Absolute timestamps
  - Relative: "5m ago", "2h ago", "1d ago"
  - Units: s, m, h, d, w, mo, y (seconds to years)
  - Used in query time filters (after/before)

**Timestamp Column Detection:**
- `detectTimestampIndex(header)`: Auto-detects timestamp column
  - Priority 1: Exact matches ("@timestamp", "timestamp", "time")
  - Priority 2: Contains matches ("@timestamp", "timestamp", "datetime", "date", "time", "ts")
  - Fallback: First column (or -1 if strict mode)

### Utility Functions ([util.go](../app/util.go))

**Query Parsing:**
- `splitPipesTopLevel(s)`: Splits query by `|` outside quotes
  - Respects single and double quotes
  - Enables pipeline syntax: `query | columns a,b,c`

- `parseColumnsStage(query, header)`: Extracts column projection
  - Syntax: `columns colA, colB, colC`
  - Returns column indices and cleaned query
  - Case-insensitive column matching

- `extractTimeFilters(query)`: Parses time filters
  - Syntax: `after VALUE` and/or `before VALUE`
  - Supports absolute and relative times
  - Returns epoch milliseconds and cleaned query
  - Uses DisplayTimezone for absolute timestamps

- `splitRespectingQuotes(s)`: Tokenizes query respecting quotes
  - Splits on whitespace and `|` outside quotes
  - Preserves quoted strings intact

**String Utilities:**
- `unquoteIfQuoted(s)`: Removes matching quotes from string
- `isBoundary(s, start, end)`: Checks token boundaries for search

**Sorting:**
- `sortRows(ctx, rows, timeIdx, desc)`: External sort for large datasets
  - Uses `extsort` library for memory-efficient sorting
  - Sorts by timestamp column
  - Parseable timestamps ordered before unparseable
  - Context-aware cancellation support
  - JSON serialization for external storage

**Query Term Structure:**
- `term` struct: Represents parsed search term
  - `field`: Column name (empty = search all)
  - `op`: Operator (=, !=, contains, prefix)
  - `value`: Search value
  - `neg`: Negation flag

## Frontend

### Architecture Overview

The frontend is built with React + TypeScript using Vite as the build tool. It follows a component-based architecture with custom hooks for business logic separation.

**Key Libraries:**
- **AG-Grid**: High-performance data grid with infinite scrolling
- **Recharts**: Histogram visualization
- **Wails Runtime**: Frontend-backend communication bridge

### [App.tsx](../frontend/src/App.tsx) - Main Application Component

`App.tsx` serves as the root component and state coordinator for the entire application.

#### State Management

**Tab State** (via `useTabState` hook):
- Multi-tab management with per-tab state isolation
- Active tab tracking and switching
- Tab creation, closing, and persistence
- Per-tab data: file path, headers, query, filters, grid API, datasource

**Global Application State:**
```typescript
- error: string                    // Global error messages
- pageSize: number                 // Grid pagination size (1000)
- showHistogram: boolean           // Histogram visibility
- appliedDisplayTZ: string         // Current display timezone
```

**Modal State:**
```typescript
- showSettings: boolean            // Settings dialog
- showAbout: boolean              // About dialog
- showLicenseDialog: boolean      // License error/success dialog
- showSyntax: boolean             // Syntax help dialog
- showShortcuts: boolean          // Keyboard shortcuts dialog
- showAnnotationDialog: boolean   // Annotation creation/edit
- showIngestExpression: boolean   // JSON path expression for JSON files
- showAddToWorkspaceDialog: boolean // Add file to workspace
- showExportLoading: boolean      // Workspace export progress
```

**Settings State:**
```typescript
settings: {
  sort_by_time: boolean
  sort_ascending: boolean
  enable_query_cache: boolean
  default_ingest_timezone: string
  display_timezone: string
  timestamp_display_format: string
}
originalSettingsRef: Ref          // For cancel functionality
```

**License State:**
```typescript
- isLicensed: boolean              // Valid license present
- licenseEmail: string | null      // License holder email
- licenseEndDate: Date | null      // License expiration
```

**Workspace State:**
```typescript
- isWorkspaceOpen: boolean         // Workspace file loaded
```

**Annotation State:**
```typescript
- annotationRowIndices: number[]   // Selected rows to annotate
- annotationNote: string           // Annotation text
- annotationColor: string          // Annotation color
```

**UI Interaction State:**
```typescript
- headerContextMenu: {             // Right-click on column header
    visible: boolean
    x: number
    y: number
    columnName: string
  }
- showConsole: boolean             // Console panel visibility
- consoleHeight: number            // Console panel height in pixels
- logs: LogEntry[]                 // Console log entries
- queryHistory: string[]           // Recent query history (localStorage)
```

**Refs:**
```typescript
- searchInputRef: HTMLInputElement     // Search bar focus management
- gridContainerRef: HTMLDivElement     // Grid keyboard shortcuts
- fileTotalRowsRef: number             // Cached total row count
```

#### Custom Hooks Integration

[App.tsx](../frontend/src/App.tsx) delegates business logic to specialized hooks:

1. **useTabState**: Multi-tab state management
2. **useGridOperations**: AG-Grid datasource and operations
3. **useHistogram**: Histogram data fetching and updates
4. **useFileOperations**: File opening and column definitions
5. **useSearchOperations**: Query execution and filtering
6. **useClipboardOperations**: Copy to clipboard functionality

#### Event Handlers

**File Operations:**
- `handleOpenFile()`: Opens file picker, creates tab, loads data
- `handleDashboardFileOpen(filePath, jpath)`: Opens file from workspace dashboard
- `handleIngestExpressionSave(expression)`: Loads JSON with JSONPath
- `handleIngestExpressionClose()`: Cancels JSON import

**Tab Operations:**
- `handleTabChange(tabId)`: Switches active tab, refreshes UI
- `handleTabClose(tabId)`: Closes tab, auto-switches to another
- `handleNewTab()`: Creates new tab via file picker

**Search Operations:**
- `handleApplySearch(queryText)`: Executes query, updates grid and histogram
- `handleHistogramBrushEnd(start, end)`: Creates time range filter from histogram selection
- `handleSetTimestampColumn(columnName)`: Changes timestamp column, refreshes data

**Grid Operations:**
- `handleGridReady(params, tabId)`: Grid initialization, keyboard shortcuts setup
- `handleHeaderContextMenu(e, columnName)`: Right-click menu on column headers

**Annotation Operations:**
- `handleAnnotateRow(rowIndices)`: Opens annotation dialog for selected rows
- `handleSubmitAnnotation(note, color)`: Saves annotation to workspace
- `handleDeleteAnnotation()`: Removes annotation from workspace

**Workspace Operations:**
- `handleAddFileToWorkspace()`: Adds file to open workspace
- `handleAddToWorkspaceSaveExpression(expression)`: Saves JSON file with JSONPath to workspace

**Settings Operations:**
- `openSettings()`: Opens settings dialog, saves original values
- `saveSettings()`: Persists settings, triggers cache clear if sort changed
- `cancelSettings()`: Restores original settings without saving

#### Native Menu Event Listeners

Listens for events emitted by native menu actions:
- `menu:open`: Trigger file open dialog
- `menu:copySelected`: Copy selected rows to clipboard
- `menu:importLicense`: Open license import dialog
- `menu:settings`: Open settings dialog
- `menu:openWorkspace`: Open workspace file picker
- `menu:closeWorkspace`: Close active workspace
- `menu:addFileToWorkspace`: Add file to workspace
- `menu:exportWorkspaceTimeline`: Export merged timeline
- `menu:toggleHistogram`: Toggle histogram visibility
- `menu:toggleConsole`: Toggle console panel
- `menu:syntax`: Show syntax help
- `menu:shortcuts`: Show keyboard shortcuts
- `menu:about`: Show about dialog

#### Keyboard Shortcuts

Implemented in `handleGridReady`:
- **Ctrl/Cmd+C**: Copy selected rows
- **Ctrl/Cmd+A**: Select all visible rows (virtual)
- **g**: Jump to top of grid
- **Shift+G**: Jump to bottom of grid
- **Ctrl/Cmd+O**: Open file (via menu)
- **Ctrl/Cmd+,**: Open settings (via menu)
- **Ctrl/Cmd+H**: Toggle histogram (via menu)
- **Ctrl/Cmd+`**: Toggle console (via menu)

#### Component Hierarchy

```
App
├── TabBar (tab switching, close, new tab)
├── SearchBar (query input, history, apply)
├── RowCountIndicator (total/filtered counts)
├── Histogram (time-based visualization)
│   └── Recharts BarChart
├── Grid (AG-Grid infinite scroll)
│   ├── TimeHeader (timestamp columns)
│   └── RegularHeader (other columns)
├── ConsolePanel (log messages)
├── Dashboard (workspace files, no tabs open)
├── Dialog Components:
│   ├── Settings
│   ├── AnnotationDialog
│   ├── IngestExpression (JSON path)
│   ├── WorkspaceFileEditDialog
│   └── Generic Dialog (about, syntax, shortcuts)
```

### React Components

#### [Grid.tsx](../frontend/src/components/Grid.tsx)
**Purpose**: AG-Grid wrapper for infinite scrolling data display

**Props:**
- `tabId: string` - Current tab identifier
- `columnDefs: ColDef[]` - Column definitions
- `onGridReady: (params, tabId) => void` - Grid initialization callback

**Features:**
- Infinite row model for large datasets
- Row selection (single, range, Ctrl+Click)
- Dynamic column definitions
- Custom cell renderers for annotations
- Row styling based on annotation colors

**State:**
- Grid API and Column API stored in parent tab state
- Datasource provided by parent via `onGridReady`

#### [Histogram.tsx](../frontend/src/components/Histogram.tsx)
**Purpose**: Time-based data distribution visualization

**Props:**
- `buckets: HistogramBucket[]` - Time buckets with counts
- `onBrushEnd: (start, end) => void` - Time range selection callback
- `displayTZ: string` - Timezone for axis labels

**Features:**
- Interactive brush selection for time filtering
- Timezone-aware axis labels
- Responsive sizing
- Hover tooltips with counts and time ranges

**Implementation:**
- Uses Recharts BarChart with Brush component
- Formats timestamps according to display timezone
- Emits millisecond epoch values on brush end

#### [SearchBar.tsx](../frontend/src/components/SearchBar.tsx)
**Purpose**: Query input with history and execution

**Props:**
- `query: string` - Current query text
- `onChange: (query: string) => void` - Query change handler
- `onApply: () => void` - Execute query handler
- `queryHistory: string[]` - Recent queries
- `inputRef: RefObject` - External ref for focus management

**Features:**
- Query syntax highlighting
- History dropdown with recent queries
- Enter key to apply
- Clear button
- Syntax help link

#### [TabBar.tsx](../frontend/src/components/TabBar.tsx)
**Purpose**: Multi-tab navigation and management

**Props:**
- `tabs: TabInfo[]` - List of open tabs
- `activeTabId: string` - Currently active tab
- `onTabChange: (tabId) => void` - Tab switch handler
- `onTabClose: (tabId) => void` - Tab close handler
- `onNewTab: () => void` - New tab handler

**Features:**
- Horizontal scrolling for many tabs
- Close buttons on each tab
- Active tab highlighting
- New tab button
- Tab truncation with tooltips

#### [ConsolePanel.tsx](../frontend/src/components/ConsolePanel.tsx)
**Purpose**: Resizable log message display

**Props:**
- `logs: LogEntry[]` - Array of log messages
- `height: number` - Panel height in pixels
- `onHeightChange: (height) => void` - Resize handler

**Features:**
- Draggable resize handle
- Auto-scroll to latest log
- Level-based styling (info/warn/error)
- Timestamp display
- Clear logs button

#### [RowCountIndicator.tsx](../frontend/src/components/RowCountIndicator.tsx)
**Purpose**: Displays total and filtered row counts

**Props:**
- `total: number` - Total rows in file
- `filtered: number` - Rows matching current query

**Display:**
- "X rows" (no filter)
- "X / Y rows" (filter active)
- "Loading..." (during load)

#### [Settings.tsx](../frontend/src/components/Settings.tsx)
**Purpose**: Application settings configuration modal

**Props:**
- `settings: Settings` - Current settings object
- `onChange: (settings) => void` - Settings update handler
- `onSave: () => void` - Save handler
- `onCancel: () => void` - Cancel handler
- `tzOptions: string[]` - Available timezones

**Sections:**
1. **Sort Settings**: By time, ascending/descending
2. **Cache Settings**: Query cache enable/disable
3. **Timezone Settings**: Ingest and display timezones
4. **Format Settings**: Timestamp display format

#### [AnnotationDialog.tsx](../frontend/src/components/AnnotationDialog.tsx)
**Purpose**: Create/edit annotations for selected rows

**Props:**
- `rowCount: number` - Number of selected rows
- `note: string` - Annotation text
- `color: string` - Annotation color
- `onNoteChange: (note) => void`
- `onColorChange: (color) => void`
- `onSubmit: () => void`
- `onDelete: () => void`
- `onClose: () => void`

**Features:**
- Multi-line text input for notes
- Color picker (grey, blue, yellow, green, orange, red)
- Shows row count being annotated
- Delete button for existing annotations

#### [IngestExpression.tsx](../frontend/src/components/IngestExpression.tsx)
**Purpose**: JSONPath expression dialog for JSON files

**Props:**
- `filePath: string` - JSON file path
- `onSave: (expression) => void` - Save handler
- `onClose: () => void` - Cancel handler

**Features:**
- JSONPath expression input
- Live preview of extracted data (first 5 rows)
- Error messages for invalid expressions
- Available keys display when expression returns object
- Syntax help for JSONPath

#### [Dashboard.tsx](../frontend/src/components/Dashboard.tsx)
**Purpose**: Workspace file browser (shown when no tabs open)

**Props:**
- `isWorkspaceOpen: boolean` - Workspace loaded
- `files: WorkspaceFileInfo[]` - Workspace files
- `onFileOpen: (filePath, jpath) => void` - Open file handler
- `onWorkspaceOpen: () => void` - Open workspace handler

**Features:**
- Lists all files in workspace
- Shows annotation counts per file
- Displays file descriptions
- Click to open file in new tab
- Empty state with call-to-action

### Custom Hooks

#### [useTabState.ts](../frontend/src/hooks/useTabState.ts)
**Purpose**: Multi-tab state management

**State Per Tab:**
- tabId, filePath, fileName
- header, originalHeader, timeField
- query, appliedQuery, columnDefs
- datasource, gridApi, columnApi
- totalRows, histBuckets
- virtualSelectAll, fileToken, generation
- ingestExpression (for JSON files)

**Methods:**
- `createTab(tabId, filePath)`: Initialize new tab
- `closeTab(tabId)`: Remove tab
- `switchTab(tabId)`: Change active tab
- `getTabState(tabId)`: Get tab by ID
- `findTabByFilePath(path, jpath)`: Find existing tab
- `setXXXForTab(tabId, value)`: Per-tab setters
- `currentTab`: Computed property for active tab

**Features:**
- Isolated state per tab prevents cross-contamination
- Ref-based access for callbacks
- Atomic updates for tab switching

#### [useGridOperations.ts](../frontend/src/hooks/useGridOperations.ts)
**Purpose**: AG-Grid datasource and operations

**Methods:**
- `createDataSource(tabId, query, header, timeField)`: Creates infinite row model datasource
  - Fetches paginated data from backend
  - Handles success/fail callbacks
  - Manages loading states
- `getSelectedRowIndexes()`: Returns array of selected row indices
- `indexesToRanges(indices)`: Converts indices to range specifications
- `selectAllDisplayed()`: Selects all visible rows in grid

**Datasource Implementation:**
- Implements `IServerSideDatasource` interface
- `getRows(params)`: Fetches rows for requested range
- Calls backend `GetCSVRowsFiltered` with pagination
- Handles total row count updates
- Manages error states

#### [useHistogram.ts](../frontend/src/hooks/useHistogram.ts)
**Purpose**: Histogram data fetching and caching

**Methods:**
- `refreshHistogram(tabId, timeField, query)`: Fetches histogram data
  - Calls backend `GetHistogram`
  - Updates tab-specific histogram state
  - Handles errors gracefully

**Features:**
- Tab-specific histogram data
- Automatic refresh on query changes
- Bucket size configurable (default: 60 seconds)

#### [useFileOperations.ts](../frontend/src/hooks/useFileOperations.ts)
**Purpose**: File opening and column definition building

**Methods:**
- `openCsvWithDialog()`: Opens file picker, creates tab, reads headers
- `loadJsonFileWithExpression(tabId, path, expr)`: Loads JSON with JSONPath
- `buildColumnDefs(header, timeField, displayTZ)`: Creates AG-Grid column definitions
  - Timestamp columns get TimeHeader component
  - Other columns get RegularHeader component
  - Sets up context menu handlers
- `detectTimestampField(header)`: Auto-detects timestamp column

**Features:**
- Multi-format support (CSV, XLSX, JSON)
- Timezone-aware column headers

#### [useSearchOperations.ts](../frontend/src/hooks/useSearchOperations.ts)
**Purpose**: Query parsing and execution

**Methods:**
- `applySearch(query, newTZ?, timeFieldOverride?)`: Executes query
  - Rebuilds column definitions if timezone changed
  - Creates new datasource with query
  - Updates grid
  - Saves applied query to tab state
- `replaceExistingTimeRange(query, after, before)`: Replaces time filters in query
  - Detects existing `after` and `before` filters
  - Replaces them with new values
  - Returns modified query

**Features:**
- Query syntax parsing
- Time filter replacement
- Timezone change handling

#### [useClipboardOperations.ts](../frontend/src/hooks/useClipboardOperations.ts)
**Purpose**: Copy selected rows to clipboard

**Methods:**
- `copySelectedToClipboard()`: Copies selected rows
  - Gets selected row indices from grid
  - Handles virtual select-all
  - Projects columns if specified
  - Calls backend `CopySelectionToClipboard`
  - Shows success message with row count
- `copyHistogramToClipboard()`: Copies histogram as PNG image
  - Converts canvas to data URL
  - Calls backend `CopyPNGFromDataURL`

**Features:**
- Range-based copying for efficiency
- Virtual select-all support
- TSV format for spreadsheet compatibility
- Image copying for histogram

### State Flow

**File Opening Flow:**
1. User clicks "Open" → `handleOpenFile()`
2. Backend `OpenFileTab()` → Returns tab ID, headers
3. Frontend creates tab state → `createTab()`
4. Detect timestamp column → `detectTimestampField()`
5. Build column definitions → `buildColumnDefs()`
6. Create datasource → `createDataSource()`
7. Set datasource on grid → `gridApi.setDatasource()`
8. Fetch histogram → `refreshHistogram()`

**Query Execution Flow:**
1. User enters query → Types in SearchBar
2. User presses Enter → `handleApplySearch()`
3. Parse and execute → `applySearch()`
4. Create new datasource with query → `createDataSource(query)`
5. Update grid → `gridApi.setDatasource()`
6. Refresh histogram → `refreshHistogram(query)`
7. Grid requests rows → Datasource `getRows()`
8. Backend applies filters → `GetCSVRowsFiltered(query)`
9. Return paginated results → Update grid

**Annotation Flow:**
1. User selects rows and right-clicks → Grid context menu
2. Click "Annotate" → `handleAnnotateRow(indices)`
3. Check license and workspace → Show error if missing
4. Fetch existing annotations → `GetRowAnnotations()`
5. Pre-fill dialog if annotations exist → Open dialog
6. User edits and saves → `handleSubmitAnnotation()`
7. Backend saves to workspace → `AddAnnotations()`
8. Refresh grid → `gridApi.refreshInfiniteCache()`
9. Annotations appear with colors

### Performance Optimizations

1. **Infinite Scrolling**: Only loads visible rows, handles millions of rows
2. **Query Caching**: Backend caches filtered results per query
3. **Sort Caching**: Backend caches sorted data, reuses across queries
4. **Virtual Select-All**: Doesn't load all rows into memory
5. **External Sort**: Disk-based sorting for datasets larger than RAM
6. **Debounced Updates**: Settings changes batch cache clears
7. **Tab State Isolation**: Switching tabs doesn't reload data
8. **Lazy Clipboard**: Only initializes when first used
9. **Streaming Reads**: Files read via streaming, not loaded entirely into memory
10. **Component Memoization**: React components use useMemo/useCallback where appropriate
