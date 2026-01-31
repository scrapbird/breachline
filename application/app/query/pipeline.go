package query

import (
	"breachline/app/cache"
	"breachline/app/interfaces"
	"context"
	"fmt"
	"log"
)

// QueryPipeline orchestrates the execution of multiple pipeline stages
type QueryPipeline struct {
	stages           []PipelineStage
	cache            *cache.Cache
	progress         ProgressCallback
	ctx              context.Context
	fileHash         string
	timeField        string      // User-selected timestamp column (empty for auto-detect)
	noHeaderRow      bool        // True if file has no header row (affects cache key)
	ingestTzOverride string      // Per-file timezone override (affects cache key)
	cacheConfig      CacheConfig // Cache configuration
}

// NewQueryPipeline creates a new query pipeline
func NewQueryPipeline(ctx context.Context, fileHash string, timeField string, noHeaderRow bool, ingestTzOverride string, c *cache.Cache, progress ProgressCallback, config CacheConfig) *QueryPipeline {
	if progress == nil {
		progress = NoOpProgressCallback
	}

	return &QueryPipeline{
		ctx:              ctx,
		fileHash:         fileHash,
		timeField:        timeField,
		noHeaderRow:      noHeaderRow,
		ingestTzOverride: ingestTzOverride,
		cache:            c,
		progress:         progress,
		cacheConfig:      config,
	}
}

// AddStage adds a pipeline stage
func (p *QueryPipeline) AddStage(stage PipelineStage) {
	p.stages = append(p.stages, stage)
}

// Execute runs the pipeline with the given input data
func (p *QueryPipeline) Execute(input *StageResult) (*QueryResult, error) {
	// Capture original header at pipeline start
	originalHeader := input.OriginalHeader
	if len(originalHeader) == 0 {
		originalHeader = make([]string, len(input.Header))
		copy(originalHeader, input.Header)
	}

	if len(p.stages) == 0 {
		// No stages, return input as-is
		// Assign display indices for all rows
		for i, row := range input.Rows {
			row.DisplayIndex = i
		}
		return &QueryResult{
			OriginalHeader: originalHeader,
			Header:         input.Header,
			DisplayColumns: input.DisplayColumns,
			Rows:           input.Rows,
			TimestampStats: input.TimestampStats,
			Total:          int64(len(input.Rows)),
			Cached:         false,
		}, nil
	}

	// Check if pipeline contains annotated stages (needed for cache marking)
	hasAnnotatedStage := false
	for _, stage := range p.stages {
		if _, ok := stage.(*AnnotatedStage); ok {
			hasAnnotatedStage = true
			break
		}
	}

	// Check if we can use cached results (full pipeline cache)
	log.Printf("[CACHE_CONFIG] Pipeline cache enabled: %v, Stage cache enabled: %v", p.cacheConfig.EnablePipelineCache, p.cacheConfig.EnableStageCache)
	if p.cache != nil && p.cacheConfig.EnablePipelineCache {
		cacheKey := BuildCacheKeyFull(p.fileHash, p.stages, p.timeField, p.noHeaderRow, p.ingestTzOverride)

		// Mark as annotation-dependent BEFORE checking cache
		// This ensures the key is tracked even on cache hits
		if hasAnnotatedStage {
			log.Printf("[CACHE_MARK_EARLY] Marking pipeline cache as annotation-dependent (hasAnnotatedStage=%v)", hasAnnotatedStage)
			p.cache.MarkAnnotationDependent(cacheKey)
			log.Printf("[CACHE_MARK_EARLY_DONE] Marked cache key: %s", cacheKey)
		} else {
			log.Printf("[CACHE_MARK_SKIP] Skipping annotation marking (hasAnnotatedStage=%v)", hasAnnotatedStage)
		}

		log.Printf("[CACHE_LOOKUP] Checking cache for key: %s", cacheKey)
		if entry, found := p.cache.Get(cacheKey); found && entry.IsComplete {
			log.Printf("[CACHE_HIT] Using cached result for key: %s (%d rows)", cacheKey, len(entry.Rows))
			// Use cached original header if available, otherwise fall back to original from input
			cachedOriginalHeader := entry.OriginalHeader
			if len(cachedOriginalHeader) == 0 {
				cachedOriginalHeader = originalHeader
			}
			return &QueryResult{
				OriginalHeader: cachedOriginalHeader,
				Header:         entry.Header,
				DisplayColumns: entry.DisplayColumns,
				Rows:           entry.Rows,
				TimestampStats: entry.TimestampStats,
				Total:          int64(len(entry.Rows)),
				Cached:         true,
			}, nil
		}
		log.Printf("[CACHE_MISS] No cached result for key: %s", cacheKey)
	} else if !p.cacheConfig.EnablePipelineCache {
		log.Printf("[CACHE_DISABLED] Pipeline cache disabled by user settings")
	}

	// Set up progress tracking
	tracker := NewProgressTracker(p.progress, len(p.stages))

	// Execute stages sequentially with incremental caching
	currentResult := input
	estimator := NewProgressEstimator(int64(len(input.Rows)))
	executedStages := []PipelineStage{} // Track which stages we've executed

	for _, stage := range p.stages {
		// Check for cancellation
		select {
		case <-p.ctx.Done():
			return nil, p.ctx.Err()
		default:
		}

		// Build cache key for this stage's output
		// Key format: "file:X:time:T:noheader:B:tz:Z|stage1:key1|stage2:key2|...|stageN:keyN"
		stageKey := BuildCacheKeyFull(p.fileHash, append(executedStages, stage), p.timeField, p.noHeaderRow, p.ingestTzOverride)

		// Check if this stage's output is cached
		if p.cache != nil && p.cacheConfig.EnableStageCache && stage.CanCache() {
			if entry, found := p.cache.Get(stageKey); found && entry.IsComplete {
				log.Printf("[CACHE_HIT_STAGE] Using cached result for stage %s: %s (%d rows)",
					stage.Name(), stageKey, len(entry.Rows))

				// Use cached result
				currentResult = &StageResult{
					OriginalHeader: entry.OriginalHeader,
					Header:         entry.Header,
					DisplayColumns: entry.DisplayColumns,
					Rows:           entry.Rows,
					TimestampStats: entry.TimestampStats, // Restored from cache
				}
				executedStages = append(executedStages, stage)
				tracker.CompleteStage(stage.Name(), int64(len(entry.Rows)))
				continue // Skip to next stage
			}
		} else if !p.cacheConfig.EnableStageCache && stage.CanCache() {
			log.Printf("[CACHE_DISABLED] Stage cache disabled by user settings for stage %s", stage.Name())
		}

		// Cache miss - execute stage
		log.Printf("[CACHE_MISS_STAGE] Executing stage %s", stage.Name())
		estimatedOutput := estimator.EstimateStageOutput(stage)
		tracker.StartStage(stage.Name(), estimatedOutput)

		stageResult, err := stage.Execute(currentResult)
		if err != nil {
			return nil, fmt.Errorf("stage %s failed: %w", stage.Name(), err)
		}

		// Ensure original header propagates through stages
		if len(stageResult.OriginalHeader) == 0 && len(originalHeader) > 0 {
			stageResult.OriginalHeader = originalHeader
		}

		// Cache this stage's output if possible
		if p.cache != nil && p.cacheConfig.EnableStageCache && stage.CanCache() {
			// Calculate average row size for cache decisions
			avgRowSize := int64(100) // Default estimate
			if len(stageResult.Rows) > 0 && len(stageResult.Rows[0].Data) > 0 {
				totalSize := int64(0)
				for _, row := range stageResult.Rows {
					for _, cell := range row.Data {
						totalSize += int64(len(cell))
					}
				}
				avgRowSize = totalSize / int64(len(stageResult.Rows))
			}

			if p.shouldCacheStage(stage, len(stageResult.Rows), avgRowSize) {
				// Mark as annotation-dependent if pipeline contains ANY annotated stage
				// This ensures all stages in annotation-dependent pipelines get invalidated together
				if hasAnnotatedStage {
					p.cache.MarkAnnotationDependent(stageKey)
				}

				// Store with full metadata (rows are already []*Row)
				// sharedFromBaseData=true because rows are pointers to base data cache entries
				p.cache.StoreWithMetadata(stageKey, stageResult.OriginalHeader, stageResult.Header, stageResult.DisplayColumns, stageResult.Rows, stageResult.TimestampStats, true)
				log.Printf("[CACHE_STORE_STAGE] Cached stage %s: %s (%d rows, displayCols=%v)",
					stage.Name(), stageKey, len(stageResult.Rows), stageResult.DisplayColumns)
			}
		}

		// Use stage result for next stage
		currentResult = stageResult
		executedStages = append(executedStages, stage)
		tracker.CompleteStage(stage.Name(), int64(len(stageResult.Rows)))

		// Update estimator
		if len(stageResult.Rows) > 0 {
			estimator = NewProgressEstimator(int64(len(stageResult.Rows)))
		}
	}

	// Assign display indices after all stages complete
	// DisplayIndex represents the 0-based position in the final result set
	for i, row := range currentResult.Rows {
		row.DisplayIndex = i
	}

	// Final result is ready
	displayColumns := currentResult.DisplayColumns
	if len(displayColumns) == 0 {
		// No filtering - identity mapping
		displayColumns = make([]int, len(originalHeader))
		for i := range displayColumns {
			displayColumns[i] = i
		}
	}

	result := &QueryResult{
		OriginalHeader: originalHeader,
		Header:         currentResult.Header,
		DisplayColumns: displayColumns,
		Rows:           currentResult.Rows,
		TimestampStats: currentResult.TimestampStats,
		Total:          int64(len(currentResult.Rows)),
		Cached:         false,
	}

	// Cache the result if possible
	if p.cache != nil && p.cacheConfig.EnablePipelineCache && p.canCacheResult() {
		cacheKey := BuildCacheKeyFull(p.fileHash, p.stages, p.timeField, p.noHeaderRow, p.ingestTzOverride)

		// Check if pipeline contains annotated stages
		hasAnnotatedStage := false
		for _, stage := range p.stages {
			if _, ok := stage.(*AnnotatedStage); ok {
				hasAnnotatedStage = true
				break
			}
		}

		// Mark as annotation-dependent before storing
		if hasAnnotatedStage {
			p.cache.MarkAnnotationDependent(cacheKey)
		}

		log.Printf("[CACHE_STORE] Storing result for key: %s (%d rows)", cacheKey, len(result.Rows))
		// Use sharedFromBaseData=true since result rows are pointers to base data cache entries
		// This avoids copying all row data and only stores pointer overhead
		p.cache.StoreWithMetadata(cacheKey, result.OriginalHeader, result.Header, result.DisplayColumns, result.Rows, result.TimestampStats, true)
		log.Printf("[CACHE_STORE_SUCCESS] Successfully cached result for key: %s", cacheKey)
	} else if !p.cacheConfig.EnablePipelineCache {
		log.Printf("[CACHE_DISABLED] Result not cached - pipeline cache disabled by user settings")
	}

	return result, nil
}

// canCacheResult determines if the pipeline result should be cached
func (p *QueryPipeline) canCacheResult() bool {
	// Only cache if all stages support caching
	for _, stage := range p.stages {
		if !stage.CanCache() {
			return false
		}
	}
	return true
}

// shouldCacheStage determines if a stage result should be cached
func (p *QueryPipeline) shouldCacheStage(stage PipelineStage, rowCount int, avgRowSize int64) bool {
	if !p.cacheConfig.EnableStageCache {
		return false
	}

	if !stage.CanCache() {
		return false
	}

	// Estimate total size
	estimatedSize := int64(rowCount) * avgRowSize

	if estimatedSize > p.cacheConfig.CacheSizeLimit {
		log.Printf("[CACHE_SKIP] Stage %s result too large (%d bytes > %d limit)",
			stage.Name(), estimatedSize, p.cacheConfig.CacheSizeLimit)
		return false
	}

	return true
}

// GetStages returns a copy of the pipeline stages
func (p *QueryPipeline) GetStages() []PipelineStage {
	stages := make([]PipelineStage, len(p.stages))
	copy(stages, p.stages)
	return stages
}

// Clear removes all stages from the pipeline
func (p *QueryPipeline) Clear() {
	p.stages = nil
}

// PipelineBuilder helps construct query pipelines
type PipelineBuilder struct {
	pipeline *QueryPipeline
}

// NewPipelineBuilder creates a new pipeline builder
func NewPipelineBuilder(ctx context.Context, fileHash string, timeField string, noHeaderRow bool, ingestTzOverride string, c *cache.Cache, progress ProgressCallback, cacheConfig CacheConfig) *PipelineBuilder {
	return &PipelineBuilder{
		pipeline: NewQueryPipeline(ctx, fileHash, timeField, noHeaderRow, ingestTzOverride, c, progress, cacheConfig),
	}
}

// AddFilter adds a filter stage to the pipeline
// displayTimezone is the timezone used for parsing time filters in the query (needed for cache key)
func (b *PipelineBuilder) AddFilter(matcher func(*Row) bool, displayIdx int, formatFunc func(string) (string, bool), workspace interface{}, fileHash string, opts interfaces.FileOptions, filterQuery string, displayTimezone string) *PipelineBuilder {
	stage := NewFilterStage(matcher, displayIdx, formatFunc, workspace, fileHash, opts, filterQuery, displayTimezone)
	b.pipeline.AddStage(stage)
	return b
}

// AddSort adds a sort stage to the pipeline
// columnNames: list of column names to sort by
// descending: corresponding sort directions for each column
func (b *PipelineBuilder) AddSort(columnNames []string, descending []bool) *PipelineBuilder {
	stage := NewSortStage(columnNames, descending)
	b.pipeline.AddStage(stage)
	return b
}

// AddTimeSort adds a time-based sort stage to the pipeline
// timeColumnName: name of the time column to sort by
// desc: true for descending order
func (b *PipelineBuilder) AddTimeSort(timeColumnName string, desc bool) *PipelineBuilder {
	stage := NewTimeSortStage(timeColumnName, desc)
	b.pipeline.AddStage(stage)
	return b
}

// AddDedup adds a deduplication stage to the pipeline
// keyColumnNames: list of column names to deduplicate on, empty means all columns
func (b *PipelineBuilder) AddDedup(keyColumnNames []string) *PipelineBuilder {
	stage := NewDedupStage(keyColumnNames)
	b.pipeline.AddStage(stage)
	return b
}

// AddLimit adds a limit stage to the pipeline
func (b *PipelineBuilder) AddLimit(count int) *PipelineBuilder {
	stage := NewLimitStage(count)
	b.pipeline.AddStage(stage)
	return b
}

// AddStrip adds a strip stage to the pipeline
func (b *PipelineBuilder) AddStrip() *PipelineBuilder {
	stage := NewStripStage()
	b.pipeline.AddStage(stage)
	return b
}

// AddColumns adds a columns stage to the pipeline
func (b *PipelineBuilder) AddColumns(columnNames []string) *PipelineBuilder {
	stage := NewColumnsStage(columnNames)
	b.pipeline.AddStage(stage)
	return b
}

// AddAnnotated adds an annotated stage to the pipeline
func (b *PipelineBuilder) AddAnnotated(workspace interface{}, fileHash string, opts interfaces.FileOptions, negated bool) *PipelineBuilder {
	stage := NewAnnotatedStage(workspace, fileHash, opts, negated)
	b.pipeline.AddStage(stage)
	return b
}

// Build returns the constructed pipeline
func (b *PipelineBuilder) Build() *QueryPipeline {
	return b.pipeline
}
