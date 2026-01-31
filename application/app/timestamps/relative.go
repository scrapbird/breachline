package timestamps

import (
	"strconv"
	"strings"
	"time"
)

// ParseFlexibleTime parses absolute or relative phrases, using loc for timezone-less absolute formats.
func ParseFlexibleTime(s string, now time.Time, loc *time.Location) (int64, bool) {
	ss := strings.TrimSpace(strings.ToLower(s))
	if ss == "" {
		return 0, false
	}
	if ss == "now" {
		return now.UnixMilli(), true
	}
	if ms, ok := ParseTimestampMillis(s, loc); ok {
		return ms, true
	}

	ss = strings.TrimSpace(strings.TrimSuffix(ss, "ago"))
	numStr := ""
	unitStr := ""
	parts := strings.Fields(ss)
	if len(parts) >= 2 {
		numStr = parts[0]
		unitStr = parts[1]
	} else {
		for i, r := range ss {
			if r < '0' || r > '9' {
				numStr = ss[:i]
				unitStr = ss[i:]
				break
			}
		}
		if numStr == "" {
			numStr = ss
			unitStr = "s"
		}
	}
	n, err := strconv.ParseInt(strings.TrimSpace(numStr), 10, 64)
	if err != nil || n < 0 {
		return 0, false
	}
	u := strings.TrimSpace(unitStr)
	if u == "" {
		u = "s"
	}
	switch u {
	case "s", "sec", "secs", "second", "seconds":
		return now.Add(-time.Duration(n) * time.Second).UnixMilli(), true
	case "m", "min", "mins", "minute", "minutes":
		return now.Add(-time.Duration(n) * time.Minute).UnixMilli(), true
	case "h", "hr", "hrs", "hour", "hours":
		return now.Add(-time.Duration(n) * time.Hour).UnixMilli(), true
	case "d", "day", "days":
		return now.Add(-time.Duration(n) * 24 * time.Hour).UnixMilli(), true
	case "w", "wk", "wks", "week", "weeks":
		return now.Add(-time.Duration(n) * 7 * 24 * time.Hour).UnixMilli(), true
	case "mo", "mon", "month", "months":
		return now.Add(-time.Duration(n) * 30 * 24 * time.Hour).UnixMilli(), true
	case "y", "yr", "yrs", "year", "years":
		return now.Add(-time.Duration(n) * 365 * 24 * time.Hour).UnixMilli(), true
	default:
		return 0, false
	}
}

// ParseFlexibleTimeWithSettings is a convenience wrapper that uses the default ingest timezone from settings
func ParseFlexibleTimeWithSettings(s string, now time.Time) (int64, bool) {
	return ParseFlexibleTime(s, now, GetDefaultIngestTimezone())
}
