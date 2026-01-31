package cache

import (
	"strings"
)

// ExtractStageCount extracts the number of stages from a cache key
func ExtractStageCount(key string) int {
	parts := strings.Split(key, "|")
	for _, part := range parts {
		if strings.HasPrefix(part, "stages:") {
			// Count the number of stage separators
			return strings.Count(key, "|")
		}
	}
	// For simple keys, count pipe separators
	return strings.Count(key, "|")
}

// IsCacheKeyPrefix checks if prefixKey is a prefix of fullKey
func IsCacheKeyPrefix(prefixKey, fullKey string) bool {
	if !strings.HasPrefix(fullKey, prefixKey) {
		return false
	}
	remainder := strings.TrimPrefix(fullKey, prefixKey)
	return remainder == "" || strings.HasPrefix(remainder, "|")
}

// IsHistogramCacheKey checks if a cache key includes histogram parameters
func IsHistogramCacheKey(key string) bool {
	return strings.Contains(key, "|hist:")
}

// ExtractStageNameFromKey extracts the stage name from a cache key
func ExtractStageNameFromKey(key string) string {
	parts := strings.Split(key, "|")
	for _, part := range parts {
		if strings.Contains(part, ":") {
			stageParts := strings.SplitN(part, ":", 2)
			if len(stageParts) == 2 {
				return stageParts[0]
			}
		}
	}
	return ""
}
