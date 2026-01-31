package query

import (
	"breachline/app/timestamps"
	"fmt"
	"strings"
)

// BuildCacheKeyWithTimeField creates a cache key from pipeline stages and includes timeField
// The timeField is critical for ensuring cached results use the correct timestamp column
// DEPRECATED: Use BuildCacheKeyFull which includes noHeaderRow and ingestTzOverride for proper cache isolation
func BuildCacheKeyWithTimeField(fileHash string, stages []PipelineStage, timeField string) string {
	return BuildCacheKeyFull(fileHash, stages, timeField, false, "")
}

// BuildCacheKeyFull creates a cache key from pipeline stages including timeField, noHeaderRow, and ingestTzOverride
// The noHeaderRow and ingestTzOverride parameters are critical to ensure tabs with different settings don't share cache
func BuildCacheKeyFull(fileHash string, stages []PipelineStage, timeField string, noHeaderRow bool, ingestTzOverride string) string {
	// Use the EFFECTIVE ingest timezone (per-file override or global setting)
	// This ensures cache is invalidated when global timezone setting changes
	effectiveIngestTz := timestamps.GetIngestTimezoneWithOverride(ingestTzOverride)
	tzKey := effectiveIngestTz.String()
	key := fmt.Sprintf("file:%s:time:%s:noheader:%t:tz:%s", fileHash, timeField, noHeaderRow, tzKey)

	for _, stage := range stages {
		if stage.CanCache() {
			key += fmt.Sprintf("|%s:%s", stage.Name(), stage.CacheKey())
		}
	}

	return key
}

// BuildStageCacheKey creates a cache key for a specific stage
func BuildStageCacheKey(fileHash string, stageName string, stageKey string) string {
	return fmt.Sprintf("file:%s|%s:%s", fileHash, stageName, stageKey)
}

// BuildHistogramCacheKey builds a cache key that includes histogram parameters
// DEPRECATED: Use BuildHistogramCacheKeyFull which includes noHeaderRow and ingestTzOverride
func BuildHistogramCacheKey(fileHash string, stages []PipelineStage, timeField string, bucketSeconds int) string {
	return BuildHistogramCacheKeyFull(fileHash, stages, timeField, false, "", bucketSeconds)
}

// BuildHistogramCacheKeyFull builds a cache key that includes histogram parameters, noHeaderRow, and ingestTzOverride
func BuildHistogramCacheKeyFull(fileHash string, stages []PipelineStage, timeField string, noHeaderRow bool, ingestTzOverride string, bucketSeconds int) string {
	baseKey := BuildCacheKeyFull(fileHash, stages, timeField, noHeaderRow, ingestTzOverride)
	histTimeField := timeField
	if histTimeField == "" {
		histTimeField = "auto"
	}
	return fmt.Sprintf("%s|hist:%s:%d", baseKey, histTimeField, bucketSeconds)
}

// IsHistogramCacheKey checks if a cache key includes histogram parameters
func IsHistogramCacheKey(key string) bool {
	return strings.Contains(key, "|hist:")
}

// ExtractHistogramParams extracts histogram parameters from a cache key
func ExtractHistogramParams(key string) (timeField string, bucketSeconds int, found bool) {
	parts := strings.Split(key, "|")
	for _, part := range parts {
		if strings.HasPrefix(part, "hist:") {
			histPart := part[5:] // Remove "hist:" prefix
			histParts := strings.Split(histPart, ":")
			if len(histParts) == 2 {
				timeField = histParts[0]
				if timeField == "auto" {
					timeField = ""
				}
				fmt.Sscanf(histParts[1], "%d", &bucketSeconds)
				return timeField, bucketSeconds, true
			}
		}
	}
	return "", 0, false
}
