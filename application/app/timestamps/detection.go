package timestamps

import (
	"strings"
)

// DetectTimestampIndex attempts to find the most likely timestamp column.
// Preference order:
// 1) Exact name: "@timestamp", "timestamp", "time"
// 2) Contains: "@timestamp", "timestamp", "datetime", "date", "time", "ts"
// Returns -1 if no timestamp column is detected.
func DetectTimestampIndex(header []string) int {
	if len(header) == 0 {
		return -1
	}
	norm := make([]string, len(header))
	for i, h := range header {
		norm[i] = strings.TrimSpace(h)
	}
	lower := make([]string, len(norm))
	for i, h := range norm {
		lower[i] = strings.ToLower(h)
	}
	exacts := []string{"@timestamp", "timestamp", "time"}
	for _, ex := range exacts {
		for i, h := range lower {
			if h == ex {
				return i
			}
		}
	}
	containsSeq := []string{"@timestamp", "timestamp", "datetime", "date", "time", "ts"}
	for _, key := range containsSeq {
		for i, h := range lower {
			if strings.Contains(h, key) {
				return i
			}
		}
	}
	return -1
}
