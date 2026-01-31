package histogram

import (
	"context"
)

// allowedBucketSizesSec matches the frontend bucket size options
var allowedBucketSizesSec = []int{
	1,                           // 1 second
	2,                           // 2 seconds
	5,                           // 5 seconds
	10,                          // 10 seconds
	30,                          // 30 seconds
	60,                          // 1 minute
	5 * 60,                      // 5 minutes
	10 * 60,                     // 10 minutes
	30 * 60,                     // 30 minutes
	60 * 60,                     // 1 hour
	2 * 60 * 60,                 // 2 hours
	3 * 60 * 60,                 // 3 hours
	6 * 60 * 60,                 // 6 hours
	12 * 60 * 60,                // 12 hours
	24 * 60 * 60,                // 1 day
	2 * 24 * 60 * 60,            // 2 days
	5 * 24 * 60 * 60,            // 5 days
	10 * 24 * 60 * 60,           // 10 days
	30 * 24 * 60 * 60,           // 1 month (30 days)
	6 * 30 * 24 * 60 * 60,       // 6 month
	12 * 30 * 24 * 60 * 60,      // 1 year
	2 * 12 * 30 * 24 * 60 * 60,  // 2 years
	5 * 12 * 30 * 24 * 60 * 60,  // 5 years
	10 * 12 * 30 * 24 * 60 * 60, // 10 years
	20 * 12 * 30 * 24 * 60 * 60, // 20 years
	50 * 12 * 30 * 24 * 60 * 60, // 50 years
}

// ChooseBucketSizeForSpan selects optimal bucket size for a given time span
// Matches the frontend logic exactly
func ChooseBucketSizeForSpan(spanSec int64, maxBuckets int) int {
	if maxBuckets <= 0 {
		maxBuckets = 100
	}

	span := spanSec
	if span < 1 {
		span = 1
	}

	// Iterate from smallest to largest bucket size to maximize bucket count
	for _, s := range allowedBucketSizesSec {
		buckets := (span + int64(s) - 1) / int64(s) // Equivalent to Math.ceil(span / s)
		if buckets <= int64(maxBuckets) {
			return s
		}
	}

	// If even the largest bucket size gives too many buckets, use it anyway
	return allowedBucketSizesSec[len(allowedBucketSizesSec)-1]
}

// TimeFilterExtractor interface for extracting time filters from queries
type TimeFilterExtractor interface {
	ExtractTimeFilters(queryString string) (*int64, *int64, string)
}

// CalculateOptimalBucketSize determines the best bucket size for a query
// considering both time filters and actual data range
func CalculateOptimalBucketSize(ctx context.Context, queryString string, dataMinTs, dataMaxTs int64, extractor TimeFilterExtractor) int {
	// Check for cancellation
	select {
	case <-ctx.Done():
		return 300 // Return default on cancellation
	default:
	}

	// Extract time filter boundaries
	var afterMs, beforeMs *int64
	if extractor != nil {
		afterMs, beforeMs, _ = extractor.ExtractTimeFilters(queryString)
	}

	// Determine the time span to use for bucket calculation
	var spanStartMs, spanEndMs int64

	if afterMs != nil || beforeMs != nil {
		// Use filter boundaries when available (user's intended time range)
		spanStartMs = dataMinTs
		spanEndMs = dataMaxTs
		if afterMs != nil {
			spanStartMs = *afterMs
		}
		if beforeMs != nil {
			spanEndMs = *beforeMs
		}
	} else {
		// No time filters, use actual data range
		spanStartMs = dataMinTs
		spanEndMs = dataMaxTs
	}

	if spanEndMs <= spanStartMs {
		return 300 // Default 5 minutes
	}

	spanSec := (spanEndMs - spanStartMs) / 1000
	return ChooseBucketSizeForSpan(spanSec, 100)
}
