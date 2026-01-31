# Multi-Tab Hooks Architecture

This directory contains custom hooks that encapsulate multi-tab state management for the BreachLine application.

## Hook Overview

### 1. `useTabState` - Core State Management
The central hook that manages all tab state using a `Map<tabId, TabState>`.

**Key Features:**
- Creates, switches, and closes tabs
- Maintains refs synchronized with active tab for performance-critical code
- Provides convenient setters for all tab properties

**Usage:**
```typescript
const tabState = useTabState();

// Access current tab
const currentTab = tabState.currentTab;

// Update current tab state
tabState.setQuery('status=error');
tabState.setAppliedQuery('status=error');

// Access refs for sync operations
const currentQuery = tabState.appliedQueryRef.current;
```

### 2. `useGridOperations` - Grid Interactions
Handles AG Grid operations with tab-aware context.

**Key Features:**
- Row selection (single, range, all displayed)
- Datasource creation
- Row mapping and range calculations

**Usage:**
```typescript
const gridOps = useGridOperations({ tabState, pageSize, addLog });

// Select a range
gridOps.selectRange(0, 100);

// Create datasource for current query
const ds = gridOps.createDataSource(query, header, onGridPending);
```

### 3. `useHistogram` - Histogram Management
Manages histogram state and refresh logic with smart bucket sizing.

**Key Features:**
- Dynamic bucket size selection based on time span
- Tracks histogram pending state
- Invalidates stale histogram requests

**Usage:**
```typescript
const { refreshHistogram, histPending, bucketSeconds } = useHistogram(tabState);

// Refresh histogram for current query
await refreshHistogram(timeField, query);
```

### 4. `useFileOperations` - File Opening
Handles file dialog, tab creation, and initial setup.

**Key Features:**
- Opens files and creates tabs
- Detects timestamp columns
- Builds column definitions with custom headers

**Usage:**
```typescript
const fileOps = useFileOperations({ tabState, appliedDisplayTZ, TimeHeader, addLog });

// Open file with dialog
const tabId = await fileOps.openCsvWithDialog();

// Detect timestamp field
const timeField = fileOps.detectTimestampField(header);
```

### 5. `useSearchOperations` - Query Processing
Handles search query parsing and application with column projection support.

**Key Features:**
- Parses SPL-like query syntax
- Handles `columns` stage for projection
- Manages query application with cooldown

**Usage:**
```typescript
const searchOps = useSearchOperations({ 
  tabState, 
  buildColumnDefs, 
  detectTimestampField, 
  createDataSource, 
  onGridPending, 
  addLog 
});

// Apply search query
await searchOps.applySearch('status=error | columns timestamp, message');
```

### 6. `useClipboardOperations` - Copy to Clipboard
Manages clipboard copy with selection handling.

**Key Features:**
- Handles various selection modes (single, range, virtual select-all)
- Respects applied query for copy
- Shows progress feedback

**Usage:**
```typescript
const { copySelectedToClipboard, copyPending } = useClipboardOperations({ 
  tabState, 
  getSelectedRowIndexes, 
  indexesToRanges, 
  gridContainerRef, 
  addLog 
});

// Copy selected rows
await copySelectedToClipboard();
```

## Integration Pattern

Here's how to integrate these hooks in your component:

```typescript
function App() {
  // 1. Core tab state
  const tabState = useTabState();
  
  // 2. Global state (not tab-specific)
  const [showSettings, setShowSettings] = useState(false);
  const [settings, setSettings] = useState({...});
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [gridPending, setGridPending] = useState(0);
  
  // 3. Refs for performance
  const gridContainerRef = useRef<HTMLDivElement>(null);
  
  // 4. Histogram management
  const histogram = useHistogram(tabState);
  
  // 5. File operations
  const fileOps = useFileOperations({
    tabState,
    appliedDisplayTZ: settings.display_timezone,
    TimeHeader: TimeHeaderComponent,
    addLog,
  });
  
  // 6. Grid operations
  const gridOps = useGridOperations({
    tabState,
    pageSize: 1000,
    addLog,
  });
  
  // 7. Search operations
  const searchOps = useSearchOperations({
    tabState,
    buildColumnDefs: fileOps.buildColumnDefs,
    detectTimestampField: fileOps.detectTimestampField,
    createDataSource: gridOps.createDataSource,
    onGridPending: (delta) => setGridPending(n => n + delta),
    addLog,
  });
  
  // 8. Clipboard operations
  const clipboardOps = useClipboardOperations({
    tabState,
    getSelectedRowIndexes: gridOps.getSelectedRowIndexes,
    indexesToRanges: gridOps.indexesToRanges,
    gridContainerRef,
    addLog,
  });
  
  // Event handlers become simple delegations
  const handleOpenFile = async () => {
    await fileOps.openCsvWithDialog();
  };
  
  const handleSearch = async () => {
    await searchOps.applySearch();
    await histogram.refreshHistogram(
      tabState.currentTab?.timeField || '',
      tabState.currentTab?.appliedQuery || ''
    );
  };
  
  const handleCopy = async () => {
    await clipboardOps.copySelectedToClipboard();
  };
  
  const handleTabChange = (tabId: string) => {
    tabState.switchTab(tabId);
    // Restore grid state for the switched tab
    const tab = tabState.getTabState(tabId);
    if (tab?.gridApi && tab?.datasource) {
      tab.gridApi.setDatasource(tab.datasource);
    }
  };
  
  const handleTabClose = async (tabId: string) => {
    // Call backend to close tab
    const AppAPI = await import('../wailsjs/go/main/App');
    await (AppAPI as any).CloseTab(tabId);
    tabState.closeTab(tabId);
  };
  
  const handleNewTab = async () => {
    await fileOps.openCsvWithDialog();
  };
  
  return (
    <div className="app">
      <TabBar
        tabs={tabState.tabs}
        activeTabId={tabState.activeTabId || ''}
        onTabChange={handleTabChange}
        onTabClose={handleTabClose}
        onNewTab={handleNewTab}
      />
      
      <SearchBar
        query={tabState.currentTab?.query || ''}
        onQueryChange={tabState.setQuery}
        onApply={handleSearch}
      />
      
      <Grid
        ref={gridContainerRef}
        columnDefs={tabState.currentTab?.columnDefs || []}
        onGridReady={(params) => {
          tabState.setGridApi(params.api);
          tabState.setColumnApi(params.columnApi);
        }}
      />
      
      <Histogram
        buckets={tabState.currentTab?.histBuckets || []}
        pending={histogram.histPending}
      />
    </div>
  );
}
```

## Benefits of This Architecture

1. **Separation of Concerns**: Each hook handles a specific domain
2. **Testability**: Hooks can be tested independently
3. **Reusability**: Hooks can be composed in different ways
4. **Type Safety**: Full TypeScript support with proper types
5. **Performance**: Refs synchronized with active tab for zero-delay access
6. **Maintainability**: Changes to one feature don't affect others

## Migration from Single-Tab App

To migrate from the existing single-tab App.tsx:

1. Replace individual `useState` calls with `tabState` hook
2. Replace direct state access with `tabState.currentTab?.property`
3. Replace state setters with `tabState.setProperty(value)`
4. Move domain logic into appropriate hooks
5. Update event handlers to use hook methods
6. Add TabBar component to UI

The hooks are designed to minimize changes to existing logic while providing clean multi-tab support.
