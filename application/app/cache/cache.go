package cache

import (
	"breachline/app/interfaces"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Cache provides LRU caching for pipeline results
type Cache struct {
	storage         map[string]*CacheEntry
	baseDataStorage map[string]*BaseFileCacheEntry // For base file data with pre-parsed Row objects
	headerCache     map[string]*HeaderCacheEntry   // For headers only (timeIdx-independent)
	maxSize         int64
	currentSize     int64
	lru             *LRUList
	mutex           sync.RWMutex
	logger          Logger

	// Performance counters
	pipelineHits int64
	stageHits    int64
	misses       int64
	baseDataHits int64 // Hits on base file data cache

	// Annotation-dependent cache tracking
	annotationDependentKeys map[string]bool // cache keys that contain annotated operations
	depsMutex               sync.RWMutex
}

// HeaderCacheEntry stores just the header for a file (timeIdx-independent)
type HeaderCacheEntry struct {
	Header     []string
	ModTime    time.Time
	AccessTime int64
}

// NewCache creates a new cache
func NewCache(maxSize int64) *Cache {
	if maxSize <= 0 {
		maxSize = DefaultCacheMaxSize
	}

	return &Cache{
		storage:                 make(map[string]*CacheEntry),
		baseDataStorage:         make(map[string]*BaseFileCacheEntry),
		headerCache:             make(map[string]*HeaderCacheEntry),
		maxSize:                 maxSize,
		lru:                     NewLRUList(),
		logger:                  nil, // No logger by default
		annotationDependentKeys: make(map[string]bool),
	}
}

// NewCacheWithLogger creates a new cache with a logger
func NewCacheWithLogger(maxSize int64, logger Logger) *Cache {
	if maxSize <= 0 {
		maxSize = DefaultCacheMaxSize
	}

	return &Cache{
		storage:                 make(map[string]*CacheEntry),
		baseDataStorage:         make(map[string]*BaseFileCacheEntry),
		headerCache:             make(map[string]*HeaderCacheEntry),
		maxSize:                 maxSize,
		lru:                     NewLRUList(),
		logger:                  logger,
		annotationDependentKeys: make(map[string]bool),
	}
}

// SetLogger sets the logger for the cache
func (c *Cache) SetLogger(logger Logger) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.logger = logger
}

// Get retrieves a cache entry and marks it as recently used
func (c *Cache) Get(key string) (*CacheEntry, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	entry, exists := c.storage[key]
	if !exists {
		atomic.AddInt64(&c.misses, 1)
		if c.logger != nil {
			c.logger.Log("debug", fmt.Sprintf("[CACHE_MISS] Key: %s", key))
		}
		return nil, false
	}

	// Determine if pipeline, stage, or histogram cache hit
	stageCount := ExtractStageCount(key)
	isHistogramKey := IsHistogramCacheKey(key)

	if isHistogramKey {
		atomic.AddInt64(&c.pipelineHits, 1) // Count histogram hits as pipeline hits
		if c.logger != nil {
			histogramInfo := ""
			if entry.HasHistogram {
				histogramInfo = fmt.Sprintf(", Buckets: %d", len(entry.HistogramBuckets))
			}
			c.logger.Log("debug", fmt.Sprintf("[CACHE_HIT_HISTOGRAM] Key: %s, Rows: %d%s, Size: %d bytes",
				key, len(entry.Rows), histogramInfo, entry.Size))
		}
	} else if stageCount == 0 {
		atomic.AddInt64(&c.pipelineHits, 1)
		if c.logger != nil {
			c.logger.Log("debug", fmt.Sprintf("[CACHE_HIT_PIPELINE] Key: %s, Rows: %d, Size: %d bytes",
				key, len(entry.Rows), entry.Size))
		}
	} else {
		atomic.AddInt64(&c.stageHits, 1)
		if c.logger != nil {
			c.logger.Log("debug", fmt.Sprintf("[CACHE_HIT_STAGE] Key: %s, Rows: %d, Size: %d bytes",
				key, len(entry.Rows), entry.Size))
		}
	}

	// Update access time and move to front of LRU
	entry.AccessTime = time.Now().Unix()
	c.lru.MoveToFront(key)

	return entry, true
}

// StoreWithMetadata adds or updates a cache entry preserving full metadata
// Accepts Row pointers and stores them in cache
// If sharedFromBaseData is true, rows are pointers to base data cache - only count pointer overhead
func (c *Cache) StoreWithMetadata(key string, originalHeader []string, header []string, displayColumns []int, rows []*interfaces.Row, timestampStats *interfaces.TimestampStats, sharedFromBaseData bool) {
	if sharedFromBaseData {
		c.storeWithSharedRows(key, originalHeader, header, displayColumns, rows, timestampStats)
	} else {
		c.storeWithOriginalAndStats(key, originalHeader, header, displayColumns, rows, timestampStats)
	}
}

// storeWithSharedRows stores a cache entry where rows are shared pointers from base data cache
// Only counts pointer overhead, not the underlying string data
func (c *Cache) storeWithSharedRows(key string, originalHeader []string, header []string, displayColumns []int, rows []*interfaces.Row, timestampStats *interfaces.TimestampStats) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Calculate entry size - only pointer overhead since rows are shared
	size := c.calculateSharedRowsSize(originalHeader, header, len(rows))

	// PRE-VALIDATION: Reject if entry is larger than entire cache
	if size > c.maxSize {
		log.Printf("[CACHE_REJECT_SHARED] Entry too large: %d bytes > %d cache limit", size, c.maxSize)
		return
	}

	// Remove existing entry if it exists
	if existing, exists := c.storage[key]; exists {
		c.currentSize -= existing.Size
		c.lru.Remove(key)
	}

	// ENHANCED EVICTION: Ensure we have space
	if !c.evictToMakeSpace(size) {
		log.Printf("[CACHE_REJECT_SHARED] Could not make space for entry: %d bytes needed, %d available", size, c.maxSize-c.currentSize)
		return
	}

	// Create new entry
	entry := &CacheEntry{
		OriginalHeader: originalHeader,
		Header:         header,
		DisplayColumns: displayColumns,
		Rows:           rows,
		TimestampStats: timestampStats, // Preserve timestamp stats for histogram
		IsComplete:     true,
		Size:           size,
		AccessTime:     time.Now().Unix(),
		CreateTime:     time.Now(),
	}

	// Store entry
	c.storage[key] = entry
	c.currentSize += size
	c.lru.AddToFront(key)

	// Log with indicator that this uses shared rows
	if c.logger != nil {
		stageCount := ExtractStageCount(key)
		if stageCount == 0 {
			c.logger.Log("debug", fmt.Sprintf("[CACHE_STORE_PIPELINE_SHARED] Key: %s, Rows: %d, Size: %d bytes (shared), Total Cache: %d/%d bytes",
				key, len(rows), size, c.currentSize, c.maxSize))
		} else {
			c.logger.Log("debug", fmt.Sprintf("[CACHE_STORE_STAGE_SHARED] Key: %s, Rows: %d, Size: %d bytes (shared), Total Cache: %d/%d bytes",
				key, len(rows), size, c.currentSize, c.maxSize))
		}
	}
}

// storeWithOriginalAndStats stores a cache entry with timestamp stats (not shared rows)
func (c *Cache) storeWithOriginalAndStats(key string, originalHeader []string, header []string, displayColumns []int, rows []*interfaces.Row, timestampStats *interfaces.TimestampStats) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Calculate entry size
	size := c.calculateEntrySizeWithOriginal(originalHeader, header, rows)

	// PRE-VALIDATION: Reject if entry is larger than entire cache
	if size > c.maxSize {
		log.Printf("[CACHE_REJECT] Entry too large: %d bytes > %d cache limit", size, c.maxSize)
		return
	}

	// Remove existing entry if it exists
	if existing, exists := c.storage[key]; exists {
		c.currentSize -= existing.Size
		c.lru.Remove(key)
	}

	// ENHANCED EVICTION: Ensure we have space
	if !c.evictToMakeSpace(size) {
		log.Printf("[CACHE_REJECT] Could not make space for entry: %d bytes needed, %d available", size, c.maxSize-c.currentSize)
		return
	}

	// Create new entry
	entry := &CacheEntry{
		OriginalHeader: originalHeader,
		Header:         header,
		DisplayColumns: displayColumns,
		Rows:           rows,
		TimestampStats: timestampStats,
		IsComplete:     true,
		Size:           size,
		AccessTime:     time.Now().Unix(),
		CreateTime:     time.Now(),
	}

	// Store entry
	c.storage[key] = entry
	c.currentSize += size
	c.lru.AddToFront(key)

	// Log successful cache store
	if c.logger != nil {
		stageCount := ExtractStageCount(key)
		if stageCount == 0 {
			c.logger.Log("debug", fmt.Sprintf("[CACHE_STORE_PIPELINE] Key: %s, Rows: %d, Size: %d bytes, Total Cache: %d/%d bytes",
				key, len(rows), size, c.currentSize, c.maxSize))
		} else {
			c.logger.Log("debug", fmt.Sprintf("[CACHE_STORE_STAGE] Key: %s, Rows: %d, Size: %d bytes, Total Cache: %d/%d bytes",
				key, len(rows), size, c.currentSize, c.maxSize))
		}
	}
}

// StoreWithHistogram adds or updates a cache entry with histogram data
// Note: This typically creates new Row objects from [][]string, so sharedRows is false
func (c *Cache) StoreWithHistogram(key string, originalHeader []string, header []string, displayColumns []int, rows []*interfaces.Row, histogramBuckets []HistogramBucket, histogramMinTs, histogramMaxTs int64, timeField string, bucketSeconds int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Calculate entry size including histogram data
	// sharedRows=false because this method typically receives newly created Row objects
	size := c.calculateEntrySizeWithHistogram(originalHeader, header, rows, histogramBuckets, timeField, false)

	// PRE-VALIDATION: Reject if entry is larger than entire cache
	if size > c.maxSize {
		if c.logger != nil {
			c.logger.Log("warning", fmt.Sprintf("[CACHE_REJECT_HISTOGRAM] Entry too large: %d bytes > %d cache limit", size, c.maxSize))
		}
		return
	}

	// Remove existing entry if it exists
	if existing, exists := c.storage[key]; exists {
		c.currentSize -= existing.Size
		c.lru.Remove(key)
	}

	// ENHANCED EVICTION: Ensure we have space
	if !c.evictToMakeSpace(size) {
		if c.logger != nil {
			c.logger.Log("warning", fmt.Sprintf("[CACHE_REJECT_HISTOGRAM] Could not make space for entry: %d bytes needed, %d available", size, c.maxSize-c.currentSize))
		}
		return
	}

	// Create new entry with histogram data
	entry := &CacheEntry{
		OriginalHeader:      originalHeader,
		Header:              header,
		DisplayColumns:      displayColumns,
		Rows:                rows,
		IsComplete:          true,
		Size:                size,
		AccessTime:          time.Now().Unix(),
		CreateTime:          time.Now(),
		HistogramBuckets:    histogramBuckets,
		HistogramMinTs:      histogramMinTs,
		HistogramMaxTs:      histogramMaxTs,
		HistogramTimeField:  timeField,
		HistogramBucketSecs: bucketSeconds,
		HasHistogram:        len(histogramBuckets) > 0,
	}

	// Store entry
	c.storage[key] = entry
	c.currentSize += size
	c.lru.AddToFront(key)

	// Log successful cache store
	if c.logger != nil {
		stageCount := ExtractStageCount(key)
		if IsHistogramCacheKey(key) {
			c.logger.Log("debug", fmt.Sprintf("[CACHE_STORE_HISTOGRAM] Key: %s, Rows: %d, Buckets: %d, Size: %d bytes, Total Cache: %d/%d bytes",
				key, len(rows), len(histogramBuckets), size, c.currentSize, c.maxSize))
		} else if stageCount == 0 {
			c.logger.Log("debug", fmt.Sprintf("[CACHE_STORE_PIPELINE] Key: %s, Rows: %d, Size: %d bytes, Total Cache: %d/%d bytes",
				key, len(rows), size, c.currentSize, c.maxSize))
		} else {
			c.logger.Log("debug", fmt.Sprintf("[CACHE_STORE_STAGE] Key: %s, Rows: %d, Size: %d bytes, Total Cache: %d/%d bytes",
				key, len(rows), size, c.currentSize, c.maxSize))
		}
	}

	// POST-VALIDATION: Emergency check
	if c.currentSize > c.maxSize {
		if c.logger != nil {
			c.logger.Log("warning", fmt.Sprintf("[CACHE_EMERGENCY] Cache exceeded limit: %d > %d, emergency eviction", c.currentSize, c.maxSize))
		}
		c.emergencyEvict()
	}
}

// StoreWithOriginal adds or updates a cache entry with original header tracking
func (c *Cache) StoreWithOriginal(key string, originalHeader []string, header []string, displayColumns []int, rows []*interfaces.Row) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Calculate entry size
	size := c.calculateEntrySizeWithOriginal(originalHeader, header, rows)

	// PRE-VALIDATION: Reject if entry is larger than entire cache
	if size > c.maxSize {
		log.Printf("[CACHE_REJECT] Entry too large: %d bytes > %d cache limit", size, c.maxSize)
		return
	}

	// Remove existing entry if it exists
	if existing, exists := c.storage[key]; exists {
		c.currentSize -= existing.Size
		c.lru.Remove(key)
	}

	// ENHANCED EVICTION: Ensure we have space
	if !c.evictToMakeSpace(size) {
		log.Printf("[CACHE_REJECT] Could not make space for entry: %d bytes needed, %d available", size, c.maxSize-c.currentSize)
		return
	}

	// Create new entry
	entry := &CacheEntry{
		OriginalHeader: originalHeader,
		Header:         header,
		DisplayColumns: displayColumns,
		Rows:           rows,
		IsComplete:     true,
		Size:           size,
		AccessTime:     time.Now().Unix(),
		CreateTime:     time.Now(),
	}

	// Store entry
	c.storage[key] = entry
	c.currentSize += size
	c.lru.AddToFront(key)

	// Log successful cache store
	if c.logger != nil {
		stageCount := ExtractStageCount(key)
		if stageCount == 0 {
			c.logger.Log("debug", fmt.Sprintf("[CACHE_STORE_PIPELINE] Key: %s, Rows: %d, Size: %d bytes, Total Cache: %d/%d bytes",
				key, len(rows), size, c.currentSize, c.maxSize))
		} else {
			c.logger.Log("debug", fmt.Sprintf("[CACHE_STORE_STAGE] Key: %s, Rows: %d, Size: %d bytes, Total Cache: %d/%d bytes",
				key, len(rows), size, c.currentSize, c.maxSize))
		}
	}

	// POST-VALIDATION: Emergency check
	if c.currentSize > c.maxSize {
		log.Printf("[CACHE_EMERGENCY] Cache exceeded limit: %d > %d, emergency eviction", c.currentSize, c.maxSize)
		c.emergencyEvict()
	}
}

// Remove removes a cache entry
func (c *Cache) Remove(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if entry, exists := c.storage[key]; exists {
		delete(c.storage, key)
		c.currentSize -= entry.Size
		c.lru.Remove(key)
	}
}

// Clear removes all cache entries
func (c *Cache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.storage = make(map[string]*CacheEntry)
	c.currentSize = 0
	c.lru = NewLRUList()
}

// Size returns the current cache size in bytes
func (c *Cache) Size() int64 {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.currentSize
}

// MaxSize returns the maximum cache size
func (c *Cache) MaxSize() int64 {
	return c.maxSize
}

// EntryCount returns the number of cached entries
func (c *Cache) EntryCount() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return len(c.storage)
}

// AddHistogramToEntry adds histogram data to an existing cache entry
// Returns true if the entry exists and histogram was added, false otherwise
func (c *Cache) AddHistogramToEntry(key string, histogramBuckets []HistogramBucket, histogramMinTs, histogramMaxTs int64, timeField string, bucketSeconds int) bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	entry, exists := c.storage[key]
	if !exists {
		return false
	}

	// Add histogram data to existing entry
	entry.HistogramBuckets = histogramBuckets
	entry.HistogramMinTs = histogramMinTs
	entry.HistogramMaxTs = histogramMaxTs
	entry.HistogramTimeField = timeField
	entry.HistogramBucketSecs = bucketSeconds
	entry.HasHistogram = len(histogramBuckets) > 0

	// Update access time and move to front of LRU
	entry.AccessTime = time.Now().Unix()
	c.lru.MoveToFront(key)

	if c.logger != nil {
		c.logger.Log("debug", fmt.Sprintf("[CACHE_ENHANCE_HISTOGRAM] Added histogram to existing entry: %s, Buckets: %d", key, len(histogramBuckets)))
	}

	return true
}

// evictToMakeSpace removes entries until there's enough space
func (c *Cache) evictToMakeSpace(neededSize int64) bool {
	// Check if we can possibly make space
	if neededSize > c.maxSize {
		return false
	}

	// Evict until we have enough space
	for c.currentSize+neededSize > c.maxSize {
		if c.lru.Size() == 0 {
			// No more entries to evict
			return c.currentSize+neededSize <= c.maxSize
		}

		oldestKey := c.lru.RemoveOldest()
		if oldestKey != "" {
			if entry, exists := c.storage[oldestKey]; exists {
				delete(c.storage, oldestKey)
				c.currentSize -= entry.Size
				if c.logger != nil {
					c.logger.Log("debug", fmt.Sprintf("[CACHE_EVICT] Evicted entry: %s, Size: %d bytes, Remaining Cache: %d/%d bytes",
						oldestKey, entry.Size, c.currentSize, c.maxSize))
				} else {
					log.Printf("[CACHE_EVICT] Evicted entry: %s (%d bytes)", oldestKey, entry.Size)
				}
			}
		}
	}

	return true
}

// calculateEntrySizeWithOriginal estimates the memory size of a cache entry with original header
// This counts full string content size - use calculateSharedRowsSize for entries sharing Row pointers
func (c *Cache) calculateEntrySizeWithOriginal(originalHeader []string, header []string, rows []*interfaces.Row) int64 {
	size := int64(0)

	// Original header size
	for _, h := range originalHeader {
		size += int64(len(h))
	}

	// Result header size
	for _, h := range header {
		size += int64(len(h))
	}

	// Rows size - iterate through Row objects
	for _, row := range rows {
		for _, cell := range row.Data {
			size += int64(len(cell))
		}
		size += int64(len(row.Data) * 8) // Slice overhead
		size += 32                       // Row struct overhead (RowIndex + DisplayIndex + Timestamp + HasTime + pointers)
	}

	// Add some overhead for the entry structure and display columns
	size += 300

	return size
}

// calculateSharedRowsSize estimates the memory size of a cache entry when rows are shared
// from the base data cache. Only counts pointer overhead, not string content.
func (c *Cache) calculateSharedRowsSize(originalHeader []string, header []string, rowCount int) int64 {
	size := int64(0)

	// Original header size (these are copied, so count them)
	for _, h := range originalHeader {
		size += int64(len(h))
	}

	// Result header size (these are copied, so count them)
	for _, h := range header {
		size += int64(len(h))
	}

	// Row pointers only - the actual Row data is owned by baseDataStorage
	// 8 bytes per pointer + 24 bytes slice header
	size += int64(rowCount*8) + 24

	// Add some overhead for the entry structure and display columns
	size += 300

	return size
}

// calculateEntrySizeWithHistogram estimates the memory size of a cache entry including histogram data
// If sharedRows is true, only counts pointer overhead for rows (not string content)
func (c *Cache) calculateEntrySizeWithHistogram(originalHeader []string, header []string, rows []*interfaces.Row, histogramBuckets []HistogramBucket, timeField string, sharedRows bool) int64 {
	// Base size calculation
	var size int64
	if sharedRows {
		size = c.calculateSharedRowsSize(originalHeader, header, len(rows))
	} else {
		size = c.calculateEntrySizeWithOriginal(originalHeader, header, rows)
	}

	// Add histogram data size
	if len(histogramBuckets) > 0 {
		// Each bucket: 8 bytes (int64) + 4 bytes (int) = 12 bytes
		size += int64(len(histogramBuckets) * 12)

		// Time field string
		size += int64(len(timeField))

		// Additional histogram metadata overhead
		size += 50
	}

	return size
}

// emergencyEvict performs emergency eviction when cache exceeds limits
func (c *Cache) emergencyEvict() {
	for c.currentSize > c.maxSize && c.lru.Size() > 0 {
		oldestKey := c.lru.RemoveOldest()
		if oldestKey != "" {
			if entry, exists := c.storage[oldestKey]; exists {
				delete(c.storage, oldestKey)
				c.currentSize -= entry.Size
				if c.logger != nil {
					c.logger.Log("warning", fmt.Sprintf("[CACHE_EMERGENCY_EVICT] Emergency evicted: %s, Size: %d bytes, Remaining Cache: %d/%d bytes",
						oldestKey, entry.Size, c.currentSize, c.maxSize))
				} else {
					log.Printf("[CACHE_EMERGENCY_EVICT] Emergency evicted: %s (%d bytes)", oldestKey, entry.Size)
				}
			}
		}
	}
}

// UpdateMaxSize updates the maximum cache size and triggers eviction if necessary
func (c *Cache) UpdateMaxSize(newMaxSize int64) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if newMaxSize <= 0 {
		newMaxSize = DefaultCacheMaxSize
	}

	oldMaxSize := c.maxSize
	c.maxSize = newMaxSize

	if c.logger != nil {
		c.logger.Log("info", fmt.Sprintf("[CACHE_RESIZE] Cache size updated from %d to %d bytes", oldMaxSize, newMaxSize))
	}

	// If current size exceeds new limit, evict entries until we're under the limit
	evictedCount := 0
	for c.currentSize > c.maxSize && c.lru.Size() > 0 {
		// Remove least recently used entry
		oldestKey := c.lru.RemoveOldest()
		if oldestKey != "" {
			if entry, exists := c.storage[oldestKey]; exists {
				delete(c.storage, oldestKey)
				c.currentSize -= entry.Size
				evictedCount++
			}
		}
	}

	if evictedCount > 0 && c.logger != nil {
		c.logger.Log("info", fmt.Sprintf("[CACHE_RESIZE_EVICT] Evicted %d entries due to cache size reduction, Final Cache: %d/%d bytes",
			evictedCount, c.currentSize, c.maxSize))
	}
}

// InvalidateFileCache removes all cache entries for a specific file
// This includes both query cache entries and base data cache entries
func (c *Cache) InvalidateFileCache(fileHash string) int {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	prefix := fmt.Sprintf("file:%s", fileHash)
	var keysToRemove []string

	// Find keys in query cache (storage)
	for key := range c.storage {
		if strings.HasPrefix(key, prefix) {
			keysToRemove = append(keysToRemove, key)
		}
	}

	// Remove from query cache
	for _, key := range keysToRemove {
		if entry, exists := c.storage[key]; exists {
			delete(c.storage, key)
			c.currentSize -= entry.Size
			c.lru.Remove(key)
		}
	}

	// Also clear base data cache entries for this file
	var baseKeysToRemove []string
	for key := range c.baseDataStorage {
		if strings.HasPrefix(key, prefix) {
			baseKeysToRemove = append(baseKeysToRemove, key)
		}
	}

	for _, key := range baseKeysToRemove {
		if entry, exists := c.baseDataStorage[key]; exists {
			delete(c.baseDataStorage, key)
			c.currentSize -= entry.Size
			c.lru.Remove(key)
		}
	}

	return len(keysToRemove) + len(baseKeysToRemove)
}

// InvalidateStageCache removes cache entries for a specific stage type
func (c *Cache) InvalidateStageCache(stageName string) int {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	var keysToRemove []string

	for key := range c.storage {
		if strings.Contains(key, fmt.Sprintf("|%s:", stageName)) {
			keysToRemove = append(keysToRemove, key)
		}
	}

	for _, key := range keysToRemove {
		if entry, exists := c.storage[key]; exists {
			delete(c.storage, key)
			c.currentSize -= entry.Size
			c.lru.Remove(key)
		}
	}

	return len(keysToRemove)
}

// InvalidateExpiredEntries removes entries older than the specified duration
func (c *Cache) InvalidateExpiredEntries(maxAge time.Duration) int {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	cutoff := time.Now().Add(-maxAge)
	var keysToRemove []string

	for key, entry := range c.storage {
		if entry.CreateTime.Before(cutoff) {
			keysToRemove = append(keysToRemove, key)
		}
	}

	for _, key := range keysToRemove {
		if entry, exists := c.storage[key]; exists {
			delete(c.storage, key)
			c.currentSize -= entry.Size
			c.lru.Remove(key)
		}
	}

	return len(keysToRemove)
}

// GetCacheStats returns detailed cache statistics
func (c *Cache) GetCacheStats() CacheStats {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	stats := CacheStats{
		TotalEntries:      len(c.storage),
		TotalSize:         c.currentSize,
		MaxSize:           c.maxSize,
		UsagePercent:      float64(c.currentSize) / float64(c.maxSize) * 100,
		StageStats:        make(map[string]StageStats),
		PipelineCacheHits: atomic.LoadInt64(&c.pipelineHits),
		StageCacheHits:    atomic.LoadInt64(&c.stageHits),
		CacheMisses:       atomic.LoadInt64(&c.misses),
	}

	// Calculate hit rates
	total := stats.PipelineCacheHits + stats.StageCacheHits + stats.CacheMisses
	if total > 0 {
		stats.HitRate = float64(stats.PipelineCacheHits+stats.StageCacheHits) / float64(total)
		stats.StageHitRate = float64(stats.StageCacheHits) / float64(total)
	}

	// Analyze entries by stage type
	for key, entry := range c.storage {
		stageName := ExtractStageNameFromKey(key)
		if stageName != "" {
			stageStats := stats.StageStats[stageName]
			stageStats.EntryCount++
			stageStats.TotalSize += entry.Size
			stats.StageStats[stageName] = stageStats
		}
	}

	return stats
}

// GetBaseData retrieves cached base file data (pre-parsed Row objects) and validates file hasn't changed.
// Returns the cache entry if found and valid, nil otherwise.
func (c *Cache) GetBaseData(key string, filePath string) (BaseDataEntry, bool) {
	c.mutex.RLock()
	entry, exists := c.baseDataStorage[key]
	c.mutex.RUnlock()

	if !exists {
		atomic.AddInt64(&c.misses, 1)
		return nil, false
	}

	// Validate file hasn't changed by checking modification time
	if filePath != "" {
		info, err := os.Stat(filePath)
		if err != nil {
			// File no longer accessible, remove from cache
			c.RemoveBaseData(key)
			return nil, false
		}
		if !info.ModTime().Equal(entry.ModTime) {
			// File changed, invalidate cache
			c.RemoveBaseData(key)
			return nil, false
		}
	}

	// Update access time
	c.mutex.Lock()
	entry.AccessTime = time.Now().Unix()
	c.lru.MoveToFront(key)
	c.mutex.Unlock()

	atomic.AddInt64(&c.baseDataHits, 1)
	if c.logger != nil {
		c.logger.Log("debug", fmt.Sprintf("[CACHE_HIT_BASEDATA] Key: %s, Rows: %d, Size: %d bytes",
			key, len(entry.Rows), entry.Size))
	}
	return entry, true
}

// StoreBaseData stores base file data (pre-parsed Row objects) in the cache.
// This enables efficient sharing of Row pointers between base data and query result caches.
func (c *Cache) StoreBaseData(key string, filePath string, header []string, rows []*interfaces.Row, timestampStats *interfaces.TimestampStats) {
	// Get file modification time for cache invalidation
	var modTime time.Time
	if filePath != "" {
		info, err := os.Stat(filePath)
		if err == nil {
			modTime = info.ModTime()
		}
	}

	// Calculate size
	size := c.calculateBaseDataSize(header, rows)

	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Check if entry would be too large
	if size > c.maxSize {
		if c.logger != nil {
			c.logger.Log("warning", fmt.Sprintf("[CACHE_REJECT_BASEDATA] Entry too large: %d bytes > %d cache limit", size, c.maxSize))
		}
		return
	}

	// Remove existing entry if present
	if existing, exists := c.baseDataStorage[key]; exists {
		c.currentSize -= existing.Size
		c.lru.Remove(key)
	}

	// Evict entries if needed
	if !c.evictToMakeSpace(size) {
		if c.logger != nil {
			c.logger.Log("warning", fmt.Sprintf("[CACHE_REJECT_BASEDATA] Could not make space for entry: %d bytes needed, %d available", size, c.maxSize-c.currentSize))
		}
		return
	}

	// Store new entry
	c.baseDataStorage[key] = &BaseFileCacheEntry{
		Header:         header,
		Rows:           rows,
		TimestampStats: timestampStats,
		ModTime:        modTime,
		Size:           size,
		AccessTime:     time.Now().Unix(),
		CreateTime:     time.Now(),
	}
	c.currentSize += size
	c.lru.AddToFront(key)

	if c.logger != nil {
		c.logger.Log("debug", fmt.Sprintf("[CACHE_STORE_BASEDATA] Key: %s, Rows: %d, Size: %d bytes, Total Cache: %d/%d bytes",
			key, len(rows), size, c.currentSize, c.maxSize))
	}
}

// RemoveBaseData removes a base data cache entry
func (c *Cache) RemoveBaseData(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if entry, exists := c.baseDataStorage[key]; exists {
		delete(c.baseDataStorage, key)
		c.currentSize -= entry.Size
		c.lru.Remove(key)
	}
}

// GetHeader retrieves a cached header for a file (timeIdx-independent).
// Returns the header if found and file hasn't changed, nil otherwise.
func (c *Cache) GetHeader(key string, filePath string) ([]string, bool) {
	c.mutex.RLock()
	entry, exists := c.headerCache[key]
	c.mutex.RUnlock()

	if !exists {
		return nil, false
	}

	// Validate file hasn't changed by checking modification time
	if filePath != "" {
		info, err := os.Stat(filePath)
		if err != nil {
			// File no longer accessible, remove from cache
			c.mutex.Lock()
			delete(c.headerCache, key)
			c.mutex.Unlock()
			return nil, false
		}
		if !info.ModTime().Equal(entry.ModTime) {
			// File changed, invalidate cache
			c.mutex.Lock()
			delete(c.headerCache, key)
			c.mutex.Unlock()
			return nil, false
		}
	}

	// Update access time
	c.mutex.Lock()
	entry.AccessTime = time.Now().Unix()
	c.mutex.Unlock()

	return entry.Header, true
}

// StoreHeader stores a header in the cache (timeIdx-independent).
// Headers are stored separately from base data to allow quick timestamp column detection
// without needing to parse the entire file.
func (c *Cache) StoreHeader(key string, filePath string, header []string) {
	var modTime time.Time
	if filePath != "" {
		info, err := os.Stat(filePath)
		if err == nil {
			modTime = info.ModTime()
		}
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.headerCache[key] = &HeaderCacheEntry{
		Header:     header,
		ModTime:    modTime,
		AccessTime: time.Now().Unix(),
	}
}

// calculateBaseDataSize estimates the memory size of base file data with Row objects
func (c *Cache) calculateBaseDataSize(header []string, rows []*interfaces.Row) int64 {
	size := int64(0)

	// Header size
	for _, h := range header {
		size += int64(len(h))
	}
	size += int64(len(header) * 16) // Slice header overhead

	// Rows size - iterate through Row objects
	for _, row := range rows {
		for _, cell := range row.Data {
			size += int64(len(cell))
		}
		size += int64(len(row.Data) * 16) // Slice overhead for Data
		size += 40                        // Row struct overhead (RowIndex + DisplayIndex + Timestamp + HasTime + pointers + padding)
	}
	size += int64(len(rows) * 8) // Pointer overhead per row

	// Add overhead for entry structure and timestamp stats
	size += 200

	return size
}

// InvalidateAnnotationCaches implements the CacheInvalidator interface
func (c *Cache) InvalidateAnnotationCaches(workspaceID string) {
	c.depsMutex.RLock()
	keysToInvalidate := make([]string, 0, len(c.annotationDependentKeys))
	for key := range c.annotationDependentKeys {
		keysToInvalidate = append(keysToInvalidate, key)
	}
	mapSize := len(c.annotationDependentKeys)
	c.depsMutex.RUnlock()

	// Use log.Printf for stdout debugging
	log.Printf("[CACHE_INVALIDATE_CHECK] Found %d annotation-dependent keys to invalidate (map size: %d, workspace: %s)", len(keysToInvalidate), mapSize, workspaceID)

	if c.logger != nil {
		c.logger.Log("debug", fmt.Sprintf("[CACHE_INVALIDATE_CHECK] Found %d annotation-dependent keys to invalidate (workspace: %s)", len(keysToInvalidate), workspaceID))
	}

	if len(keysToInvalidate) == 0 {
		log.Printf("[CACHE_INVALIDATE_SKIP] No annotation-dependent keys found (workspace: %s)", workspaceID)
		if c.logger != nil {
			c.logger.Log("debug", fmt.Sprintf("[CACHE_INVALIDATE_SKIP] No annotation-dependent keys found (workspace: %s)", workspaceID))
		}
		return
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	for _, cacheKey := range keysToInvalidate {
		if entry, exists := c.storage[cacheKey]; exists {
			delete(c.storage, cacheKey)
			c.currentSize -= entry.Size
			c.lru.Remove(cacheKey)

			log.Printf("[CACHE_INVALIDATE_ANNOTATION] Key: %s (workspace: %s)", cacheKey, workspaceID)
			if c.logger != nil {
				c.logger.Log("debug", fmt.Sprintf("[CACHE_INVALIDATE_ANNOTATION] Key: %s (workspace: %s)",
					cacheKey, workspaceID))
			}
		} else {
			log.Printf("[CACHE_INVALIDATE_NOTFOUND] Key not in storage: %s", cacheKey)
		}
	}

	// Clear the tracking map since all annotation-dependent caches are now invalid
	c.depsMutex.Lock()
	c.annotationDependentKeys = make(map[string]bool)
	c.depsMutex.Unlock()
	log.Printf("[CACHE_INVALIDATE_COMPLETE] Cleared annotation tracking map (workspace: %s)", workspaceID)
}

// MarkAnnotationDependent tracks annotation-dependent cache keys
func (c *Cache) MarkAnnotationDependent(cacheKey string) {
	c.depsMutex.Lock()
	defer c.depsMutex.Unlock()
	c.annotationDependentKeys[cacheKey] = true
	log.Printf("[CACHE_MARK_DEPENDENT_INTERNAL] Added key to tracking map (total: %d): %s", len(c.annotationDependentKeys), cacheKey)
	if c.logger != nil {
		c.logger.Log("debug", fmt.Sprintf("[CACHE_MARK_DEPENDENT] Marked cache key as annotation-dependent: %s", cacheKey))
	}
}
