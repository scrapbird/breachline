package histogram

import (
	"context"
	"strings"
)


// TimestampParser interface for parsing timestamps
type TimestampParser interface {
	ParseTimestampMillis(s string, tz interface{}) (int64, bool)
}


// TimestampParserWithCache interface for parsing timestamps with caching support
type TimestampParserWithCache interface {
	ParseTimestampMillisWithCache(s string, tz interface{}, tab interface{}) (int64, bool)
}


// GetDataTimeRange extracts the min/max timestamps from data rows
// appService and tab are optional - if provided, cached timestamp parsing will be used
func GetDataTimeRange(ctx context.Context, header []string, rows [][]string, timeField string, parser TimestampParser, appService interface{}, tab interface{}) (minTs, maxTs int64) {
	if len(header) == 0 || len(rows) == 0 {
		return 0, 0
	}

	// Resolve the timestamp column with robust matching
	timeIdx := -1
	if timeField != "" {
		// Try exact match first (case-sensitive)
		for i, h := range header {
			if h == timeField {
				timeIdx = i
				break
			}
		}

		// If no exact match, try case-insensitive match
		if timeIdx < 0 {
			tfLower := strings.ToLower(strings.TrimSpace(timeField))
			for i, h := range header {
				if strings.ToLower(strings.TrimSpace(h)) == tfLower {
					timeIdx = i
					break
				}
			}
		}
	}
	
	// If still not found, try to detect timestamp column
	// This requires the timestamps package, so we'll accept -1 if not provided
	if timeIdx < 0 {
		return 0, 0
	}
	
	if timeIdx < 0 || timeIdx >= len(header) {
		return 0, 0
	}

	// Find min/max timestamps
	var foundFirst bool
	for i, rec := range rows {
		// Check for cancellation every 1000 rows
		if i%1000 == 0 {
			select {
			case <-ctx.Done():
				return 0, 0
			default:
			}
		}
		if timeIdx >= len(rec) {
			continue
		}
		tsStr := rec[timeIdx]
		
		// Use cached parsing if available, otherwise fallback to direct parsing
		var ms int64
		var ok bool
		if appService != nil && tab != nil {
			// Try to use cached parsing via app service
			if app, appOk := appService.(TimestampParserWithCache); appOk {
				ms, ok = app.ParseTimestampMillisWithCache(tsStr, nil, tab)
			} else if parser != nil {
				ms, ok = parser.ParseTimestampMillis(tsStr, nil)
			}
		} else if parser != nil {
			ms, ok = parser.ParseTimestampMillis(tsStr, nil)
		}
		
		if !ok {
			continue
		}
		if !foundFirst {
			minTs = ms
			maxTs = ms
			foundFirst = true
		} else {
			if ms < minTs {
				minTs = ms
			}
			if ms > maxTs {
				maxTs = ms
			}
		}
	}

	return minTs, maxTs
}
