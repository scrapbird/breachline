# Query Cache Implementation

## Overview

BreachLine implements a sophisticated two-level caching system to optimize query performance on large datasets. The caching system consists of:

1. **Sort Cache**: Pre-sorted rows for the entire file
2. **Query Cache**: Filtered/processed query results per pipeline stage

Both caches are per-tab, ensuring isolated state when multiple files are open simultaneously.

## Architecture

### Cache Storage Location

All caches are stored in the `FileTab` struct ([app/app_tabs.go](../app/app_tabs.go)), providing per-tab isolation:

```go
type FileTab struct {
    // Sort Cache
    cacheMu          sync.RWMutex
    sortedHeader     []string
    sortedRows       [][]string
    sortedForFile    string  // File path cache was built for
    sortedByTime     bool    // Sort by timestamp enabled
    sortedDesc       bool    // Descending sort order
    sortedTimeField  string  // Timestamp column used for sorting
    
    // Sort State Management
    sortMu           sync.Mutex
    sortCancel       context.CancelFunc
    sortActive       int64
    sortSeq          int64
    sortCond         *sync.Cond
    
    // Query Cache (LRU)
    queryCache       map[string][][]string
    queryCacheOrder  []string  // LRU ordering
    
    // Settings Tracking for Cache Invalidation
    lastDisplayTZ       string
    lastIngestTZ        string
    lastTimestampFormat string
}
```

## Sort Cache

### Purpose

The sort cache stores the entire file's rows pre-sorted by timestamp. This is critical for performance because:
- Sorting large files (millions of rows) is expensive
- Most queries benefit from time-ordered data
- External sort algorithm is used for datasets larger than RAM
- Reusing sorted data avoids redundant work across queries

### Cache Key Components

The sort cache is keyed by a composite of:
1. **File Path** (`sortedForFile`)
2. **Sort by Time Enabled** (`sortedByTime`)
3. **Sort Order** (`sortedDesc` - ascending/descending)
4. **Timestamp Column** (`sortedTimeField`)

### Cache Population

When a query requires sorted data:

1. **Check Cache Validity**:
   ```go
   cachedOK := tab.sortedRows != nil && 
               tab.sortedForFile == tab.FilePath && 
               tab.sortedByTime == effective.SortByTime && 
               tab.sortedDesc == effective.SortDescending && 
               tab.sortedTimeField == timeField
   ```

2. **Cache Miss**: Read entire file and sort
   - Read all rows from file into memory
   - Use external sort (`extsort` library) for large datasets
   - Store sorted rows in `sortedRows`
   - Store corresponding header in `sortedHeader`
   - Set cache key fields

3. **Cache Hit**: Return cached `sortedRows` immediately

### Concurrent Sort Management

The system prevents redundant concurrent sorts:

```go
sortInProgress := tab.sortActive > 0 && 
                  tab.sortingForFile == tab.FilePath && 
                  tab.sortingByTime == effective.SortByTime && 
                  tab.sortingDesc == effective.SortDescending && 
                  tab.sortingTimeField == timeField
```

- **If sort in progress**: Wait on `sortCond` condition variable
- **If no sort in progress**: Start new sort with cancellation token
- **Token-based validation**: Ensures completed sort matches current requirements

### Sort Cancellation

When settings change mid-sort:
- Previous sort operation is cancelled via `sortCancel()`
- New sort starts with fresh context
- Old sort results are discarded
- Token validation prevents stale data from being cached

## Query Cache

### Purpose

The query cache stores intermediate and final results of query pipeline execution. Benefits:
- Incremental filtering avoids reprocessing entire dataset
- Common query prefixes are shared across similar queries
- LRU eviction keeps memory usage bounded
- Per-stage caching enables flexible query composition

### Cache Key Generation

Query cache keys are built from pipeline stages, **excluding** the `columns` projection stage:

```go
cacheStages := make([]string, 0, len(stages))
for _, st := range stages {
    head := strings.ToLower(strings.TrimSpace(toks[0]))
    switch head {
    case "columns":
        continue  // Skip in cache key
    case "dedup", "limit":
        cacheStages = append(cacheStages, st)
    default:  // filter stage
        cacheStages = append(cacheStages, st)
    }
}
```

**Example Cache Keys**:
- Query: `error | after '2024-01-01'`
- Keys:
  - `error` (first filter)
  - `error | after '2024-01-01'` (second filter)

### LRU Eviction Policy

The query cache maintains a maximum of **10 entries** per tab:

```go
for len(tab.queryCacheOrder) > 10 {
    oldKey := tab.queryCacheOrder[len(tab.queryCacheOrder)-1]
    tab.queryCacheOrder = tab.queryCacheOrder[:len(tab.queryCacheOrder)-1]
    delete(tab.queryCache, oldKey)
}
```

**LRU Mechanics**:
- `queryCacheOrder`: Maintains access order (most recent first)
- **Cache Hit**: Move key to front of order
- **Cache Store**: Insert key at front of order
- **Eviction**: Remove oldest (last) entry when limit exceeded

### Pipeline Stage Caching

Each stage in the query pipeline is cached incrementally:

#### Filter Stage
```go
prefix := strings.Join(cacheStages[:i+1], " | ")
if cached, ok := getCache(prefix); ok {
    finalRows = cached
    continue
}
// Apply filter
out := make([][]string, 0, len(finalRows))
for _, r := range finalRows {
    if s.m(r) {
        out = append(out, r)
    }
}
finalRows = out
putCache(prefix, finalRows)
```

#### Dedup Stage
```go
prefix := strings.Join(cacheStages[:i+1], " | ")
if cached, ok := getCache(prefix); ok {
    finalRows = cached
    continue
}
// Apply deduplication
seen := make(map[string]struct{})
out := make([][]string, 0, len(finalRows))
for _, r := range finalRows {
    // Build composite key from specified columns
    var b strings.Builder
    for _, idx := range s.ded {
        b.WriteString("\u0001")
        if idx >= 0 && idx < len(r) {
            b.WriteString(r[idx])
        }
    }
    key := b.String()
    if _, ok := seen[key]; !ok {
        seen[key] = struct{}{}
        out = append(out, r)
    }
}
finalRows = out
putCache(prefix, finalRows)
```

#### Limit Stage
```go
prefix := strings.Join(cacheStages[:i+1], " | ")
if cached, ok := getCache(prefix); ok {
    finalRows = cached
    continue
}
// Apply limit
if s.n < len(finalRows) {
    finalRows = finalRows[:s.n]
}
putCache(prefix, finalRows)
```

### Full Query Result Caching

Before pipeline execution, the system checks if the complete query result is cached:

```go
if effective.EnableQueryCache && len(cacheStages) > 0 {
    fullKey := strings.Join(cacheStages, " | ")
    if cached, ok := getCache(fullKey); ok {
        return header, cached, nil  // Fast path: return immediately
    }
}
```

## Cache Invalidation

### Automatic Invalidation Triggers

The cache system automatically invalidates when any of these conditions change:

#### 1. Display Timezone Change
```go
displayChanged := tab.lastDisplayTZ != normDisplay
if displayChanged {
    tab.queryCache = nil
    tab.queryCacheOrder = nil
}
```
- **Reason**: Timestamp formatting changes affect cached row representation
- **Scope**: Query cache only (sort cache unaffected)

#### 2. Ingest Timezone Change
```go
ingestChanged := tab.lastIngestTZ != normIngest
if ingestChanged {
    tab.queryCache = nil
    tab.queryCacheOrder = nil
    tab.sortedRows = nil
    tab.sortedHeader = nil
}
```
- **Reason**: Timestamp parsing changes affect sort order
- **Scope**: Both query and sort caches

#### 3. Timestamp Display Format Change
```go
formatChanged := tab.lastTimestampFormat != normTimestampFormat
if formatChanged {
    tab.queryCache = nil
    tab.queryCacheOrder = nil
}
```
- **Reason**: Display format affects cached row values
- **Scope**: Query cache only

#### 4. Sort Settings Change
```go
sortChanged := tab.sortedByTime != effective.SortByTime || 
               tab.sortedDesc != effective.SortDescending
if sortChanged {
    tab.queryCache = nil
    tab.queryCacheOrder = nil
    tab.sortedRows = nil
    tab.sortedHeader = nil
}
```
- **Reason**: Sort order fundamentally changes data presentation
- **Scope**: Both query and sort caches

#### 5. File Change
```go
fileChanged := tab.sortedForFile != tab.FilePath
if fileChanged {
    tab.queryCache = nil
    tab.queryCacheOrder = nil
    tab.sortedRows = nil
    tab.sortedHeader = nil
}
```
- **Reason**: Different file means completely different data
- **Scope**: Both query and sort caches

#### 6. Timestamp Column Change

When user explicitly changes the timestamp column:

```go
// In SetTimestampColumn()
tab.cacheMu.Lock()
tab.sortedRows = nil
tab.sortedHeader = nil
tab.sortedForFile = ""
tab.sortedTimeField = ""
tab.queryCache = nil
tab.queryCacheOrder = nil
tab.cacheMu.Unlock()
```
- **Reason**: Different timestamp column changes sort order
- **Scope**: Both query and sort caches
- **Location**: [app_timestamp_column.go](../app/app_timestamp_column.go)

### Manual Cache Clearing

#### Global Cache Clear

When global settings change (triggered by `SettingsService`):

```go
func (a *App) ClearAllTabCaches() {
    a.tabsMu.RLock()
    tabs := make([]*FileTab, 0, len(a.tabs))
    for _, tab := range a.tabs {
        tabs = append(tabs, tab)
    }
    a.tabsMu.RUnlock()
    
    for _, tab := range tabs {
        tab.cacheMu.Lock()
        // Clear sort cache
        tab.sortedRows = nil
        tab.sortedHeader = nil
        tab.sortedForFile = ""
        tab.sortedTimeField = ""
        // Clear query cache
        tab.queryCache = nil
        tab.queryCacheOrder = nil
        // Wake waiting goroutines
        if tab.sortCond != nil {
            tab.sortCond.Broadcast()
        }
        tab.cacheMu.Unlock()
    }
}
```

**Triggered by**:
- Sort setting changes in `SettingsService.SaveSettings()`
- Called from [settings.go](../app/settings.go) when `sortChanged` is detected

#### Sort Cache Rebuild

When a new sort is initiated, the query cache is cleared:

```go
if active {
    tab.cacheMu.Lock()
    tab.sortedRows = allRows
    tab.sortedHeader = header
    tab.sortedForFile = tab.FilePath
    tab.sortedByTime = effective.SortByTime
    tab.sortedDesc = effective.SortDescending
    tab.sortedTimeField = timeField
    tab.queryCache = nil           // Clear query cache
    tab.queryCacheOrder = nil      // Clear LRU order
    tab.cacheMu.Unlock()
}
```

- **Reason**: Sorted base data has changed, invalidating all filtered results
- **Scope**: Query cache only (sort cache is being populated)

## Cache Settings Control

### Enable/Disable Query Cache

User-controllable via Settings UI:

```go
type Settings struct {
    EnableQueryCache bool `yaml:"enable_query_cache"`
}
```

**When Disabled**:
- Query cache checks are skipped
- Results computed directly without caching
- Sort cache still active (separate control)
- Useful for debugging or memory-constrained systems

**Code Path**:
```go
if effective.EnableQueryCache && len(cacheStages) > 0 {
    fullKey := strings.Join(cacheStages, " | ")
    if cached, ok := getCache(fullKey); ok {
        return header, cached, nil
    }
}
```

## Concurrency & Thread Safety

### Mutex Hierarchy

1. **`tabsMu`** (App-level): Read lock for accessing tabs map
2. **`cacheMu`** (Tab-level): Read/write lock for cache access
3. **`sortMu`** (Tab-level): Lock for sort state coordination

**Lock Ordering**: Always acquire in the above order to prevent deadlocks.

### Read-Write Lock Usage

```go
// Read path (cache hit)
tab.cacheMu.RLock()
rows, ok := tab.queryCache[key]
tab.cacheMu.RUnlock()

// Write path (cache store)
tab.cacheMu.Lock()
tab.queryCache[key] = rows
tab.queryCacheOrder = append([]string{key}, tab.queryCacheOrder...)
tab.cacheMu.Unlock()
```

**Benefits**:
- Multiple concurrent readers (high performance)
- Exclusive writer (data consistency)
- Minimal contention on cache hits

### Condition Variable for Sort Coordination

```go
tab.sortCond = sync.NewCond(&tab.cacheMu)

// Waiting goroutine
tab.cacheMu.Lock()
for !(tab.sortedRows != nil && tab.sortedForFile == tab.FilePath) {
    tab.sortCond.Wait()  // Releases cacheMu, waits, reacquires
}
rows = tab.sortedRows
tab.cacheMu.Unlock()

// Completing goroutine
tab.cacheMu.Lock()
tab.sortedRows = allRows
if tab.sortCond != nil {
    tab.sortCond.Broadcast()  // Wake all waiters
}
tab.cacheMu.Unlock()
```

**Purpose**:
- Multiple queries can wait for a single sort operation
- Prevents redundant concurrent sorts
- Efficient CPU utilization

## Performance Characteristics

### Cache Hit Performance

**Query Cache Hit**:
- O(1) map lookup
- O(n) LRU order update (n = cache size, max 10)
- Negligible overhead: ~microseconds

**Sort Cache Hit**:
- O(1) pointer return
- No copying (rows slice is shared)
- Sub-millisecond response time

### Cache Miss Performance

**Sort Cache Miss**:
- O(n log n) external sort
- Disk I/O for large datasets
- Can take seconds for millions of rows
- One-time cost per tab/settings combination

**Query Cache Miss**:
- O(n) pipeline stage execution
- Depends on filter selectivity
- Results cached for subsequent access

### Memory Usage

**Sort Cache**:
- Stores entire file in memory after sort
- For large files: Only sorted row references, not duplicated data
- Example: 1M rows × 10 columns × 50 bytes/cell = ~500MB per tab

**Query Cache**:
- Maximum 10 entries per tab
- Each entry stores filtered result rows
- Example: 10 queries × 100K rows × 10 columns × 50 bytes = ~500MB per tab
- LRU eviction keeps memory bounded

**Total per Tab**: ~1GB for typical large file scenarios

## Query Execution Flow with Caching

```
User Query: "error | after '2024-01-01' | limit 100"
                        |
                        v
            Check Full Query Cache
            "error | after '2024-01-01' | limit 100"
                        |
                 ┌──────┴──────┐
                 |             |
              HIT             MISS
                 |             |
              Return          |
                              v
                    Check Sort Cache Valid?
                    (file, settings, timeField)
                              |
                       ┌──────┴──────┐
                       |             |
                     YES            NO
                       |             |
                Get Sorted     Sort & Cache
                   Rows            Rows
                       └──────┬──────┘
                              v
                    Apply Pipeline Stages
                    (with incremental caching)
                              |
                  ┌───────────┼───────────┐
                  v           v           v
            Stage 1      Stage 2      Stage 3
            "error"    "+ after"    "+ limit"
                  |           |           |
            Check Cache   Check Cache   Check Cache
                  |           |           |
              HIT/MISS    HIT/MISS    HIT/MISS
                  |           |           |
            Execute if   Execute if   Execute if
              needed       needed       needed
                  |           |           |
            Store to     Store to     Store to
              Cache        Cache        Cache
                  └───────────┼───────────┘
                              v
                    Return Final Result
```

## Cache Logging

All cache operations are logged to the console (when enabled):

**Cache Hit**:
```
[CACHE] HIT for key: "error | after '2024-01-01'" (1234 rows)
```

**Cache Miss**:
```
[CACHE] MISS for key: "error | after '2024-01-01'"
```

**Cache Store**:
```
[CACHE] STORE key: "error | after '2024-01-01'" (1234 rows)
```

**Cache Eviction**:
```
[CACHE] EVICT key: "error | after '2024-01-01'" (LRU limit)
```

**Usage**: Enable console panel (View → Toggle Console) to monitor cache behavior.

## Best Practices

### For Users

1. **Keep Query Cache Enabled**: Provides significant performance benefits
2. **Use Similar Query Patterns**: Maximize cache hits through shared prefixes
3. **Monitor Console**: Watch cache hit/miss patterns to optimize queries
4. **Be Aware of Settings Changes**: Changing timezones/format clears cache

### For Developers

1. **Always Check Settings Changes**: Implement proper cache invalidation
2. **Use Appropriate Locks**: Read locks for reads, write locks for writes
3. **Handle Cache Miss Gracefully**: Always compute fresh results when needed
4. **Consider Memory Impact**: Large cached result sets can consume significant RAM
5. **Test Concurrent Access**: Ensure thread safety with concurrent queries

## Future Enhancements

Potential improvements to the caching system:

1. **Configurable LRU Size**: Allow users to adjust cache size based on available RAM
2. **Cache Persistence**: Save cache to disk for faster restarts
3. **Smart Prefetching**: Preload common query patterns
4. **Memory-Mapped Cache**: Use mmap for larger-than-RAM caching
5. **Distributed Caching**: Share cache across multiple application instances
6. **Cache Statistics**: Track hit rates, memory usage, and performance metrics
7. **Partial Sort Cache**: Cache only timestamp column for faster sorting
8. **Bloom Filters**: Speed up cache key lookups for large LRU caches
