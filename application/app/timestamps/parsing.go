package timestamps

import (
	"breachline/app/settings"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ParseTimestampMillis tries several common formats and returns epoch milliseconds.
// If loc is nil, timezone-less formats will be interpreted using DefaultIngestTimezone.
func ParseTimestampMillis(s string, loc *time.Location) (int64, bool) {
	ss := strings.TrimSpace(s)
	if ss == "" {
		return 0, false
	}

	// PERFORMANCE: Try integer epoch seconds/milliseconds FIRST
	// This avoids 20+ failed time.Parse attempts for numeric timestamps
	// which are very common in log files and data exports
	if n, err := strconv.ParseInt(ss, 10, 64); err == nil {
		if n > 1_000_000_000_000 {
			// Epoch milliseconds (13+ digits)
			return n, true
		}
		// Epoch seconds (10 digits or less)
		return n * 1000, true
	}

	// Try explicit timezone formats first
	if t, err := time.Parse(time.RFC3339, ss); err == nil {
		return t.UnixMilli(), true
	}
	if t, err := time.Parse(time.RFC3339Nano, ss); err == nil {
		return t.UnixMilli(), true
	}
	// Try space-separated with explicit Z or offset e.g. "2006-01-02 15:04:05Z" or with numeric offset
	if t, err := time.Parse("2006-01-02 15:04:05Z07:00", ss); err == nil {
		return t.UnixMilli(), true
	}
	if t, err := time.Parse("2006-01-02T15:04:05Z07:00", ss); err == nil {
		return t.UnixMilli(), true
	}

	// Try various numbers of millisecond digits
	if t, err := time.Parse("2006-01-02T15:04:05.000 MST", ss); err == nil {
		return t.UnixMilli(), true
	}
	if t, err := time.Parse("2006-01-02T15:04:05.00 MST", ss); err == nil {
		return t.UnixMilli(), true
	}
	if t, err := time.Parse("2006-01-02T15:04:05.0 MST", ss); err == nil {
		return t.UnixMilli(), true
	}
	if t, err := time.Parse("2006-01-02 15:04:05.000 MST", ss); err == nil {
		return t.UnixMilli(), true
	}
	if t, err := time.Parse("2006-01-02 15:04:05.00 MST", ss); err == nil {
		return t.UnixMilli(), true
	}
	if t, err := time.Parse("2006-01-02 15:04:05.0 MST", ss); err == nil {
		return t.UnixMilli(), true
	}
	if t, err := time.Parse("2006-01-02 15:04:05 MST", ss); err == nil {
		return t.UnixMilli(), true
	}

	// With Z suffix
	if strings.HasSuffix(ss, "Z") {
		if t, err := time.Parse("2006-01-02 15:04:05Z", ss); err == nil {
			return t.UnixMilli(), true
		}
		if t, err := time.Parse("2006-01-02T15:04:05Z", ss); err == nil {
			return t.UnixMilli(), true
		}
	}

	// Determine ingest timezone for timezone-less formats if not provided
	if loc == nil {
		effective := settings.GetEffectiveSettings()
		tzName := strings.TrimSpace(effective.DefaultIngestTimezone)
		switch strings.ToUpper(tzName) {
		case "", "LOCAL":
			loc = time.Local
		case "UTC":
			loc = time.UTC
		default:
			if l, err := time.LoadLocation(tzName); err == nil {
				loc = l
			} else {
				loc = time.Local
			}
		}
	}

	// Try various numbers of millisecond digits
	if t, err := time.ParseInLocation("2006-01-02T15:04:05.000", ss, loc); err == nil {
		return t.UnixMilli(), true
	}
	if t, err := time.ParseInLocation("2006-01-02T15:04:05.00", ss, loc); err == nil {
		return t.UnixMilli(), true
	}
	if t, err := time.ParseInLocation("2006-01-02T15:04:05.0", ss, loc); err == nil {
		return t.UnixMilli(), true
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05.000", ss, loc); err == nil {
		return t.UnixMilli(), true
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05.00", ss, loc); err == nil {
		return t.UnixMilli(), true
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05.0", ss, loc); err == nil {
		return t.UnixMilli(), true
	}

	// Support ISO-like without timezone and common space-separated formats using loc
	if t, err := time.ParseInLocation("2006-01-02T15:04:05", ss, loc); err == nil {
		return t.UnixMilli(), true
	}
	if t, err := time.ParseInLocation("2006-01-02T15:04", ss, loc); err == nil {
		return t.UnixMilli(), true
	}

	// Try common "2006-01-02 15:04:05"
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", ss, loc); err == nil {
		return t.UnixMilli(), true
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04", ss, loc); err == nil {
		return t.UnixMilli(), true
	}
	if t, err := time.ParseInLocation("2006-01-02", ss, loc); err == nil {
		return t.UnixMilli(), true
	}

	// Try weirdo formats
	if t, err := time.ParseInLocation("02/01/2006 3:04pm", ss, loc); err == nil {
		return t.UnixMilli(), true
	}
	if t, err := time.ParseInLocation("02/01/2006 03:04pm", ss, loc); err == nil {
		return t.UnixMilli(), true
	}
	if t, err := time.ParseInLocation("02/01/2006 3:04 pm", ss, loc); err == nil {
		return t.UnixMilli(), true
	}
	if t, err := time.ParseInLocation("02/01/2006 03:04 pm", ss, loc); err == nil {
		return t.UnixMilli(), true
	}

	// No format matched
	return 0, false
}

// ParseTimestampMillisWithSettings is a convenience wrapper that uses the default ingest timezone from settings
func ParseTimestampMillisWithSettings(s string) (int64, bool) {
	return ParseTimestampMillis(s, nil)
}

// TimestampParser represents a cached timestamp parsing function
type TimestampParser func(s string) (int64, bool)

// TimestampParserInfo contains metadata about the cached parser
type TimestampParserInfo struct {
	Parser       TimestampParser
	FormatName   string // For debugging/logging
	UsesLocation bool   // Whether this parser needs timezone info
	mu           sync.RWMutex
}

// Parse safely calls the cached parser function
func (tpi *TimestampParserInfo) Parse(s string) (int64, bool) {
	if tpi == nil {
		return 0, false
	}
	tpi.mu.RLock()
	parser := tpi.Parser
	tpi.mu.RUnlock()

	if parser == nil {
		return 0, false
	}
	return parser(s)
}

// DetectAndCacheTimestampParser tries all formats in the same order as ParseTimestampMillis
// and returns a cached parser function for the first successful format
func DetectAndCacheTimestampParser(s string, loc *time.Location) (*TimestampParserInfo, int64, bool) {
	ss := strings.TrimSpace(s)
	if ss == "" {
		return nil, 0, false
	}

	// PERFORMANCE: Try integer epoch seconds/milliseconds FIRST
	// This matches the optimization in ParseTimestampMillis
	if n, err := strconv.ParseInt(ss, 10, 64); err == nil {
		if n > 1_000_000_000_000 {
			// Epoch milliseconds (13+ digits)
			return &TimestampParserInfo{
				Parser: func(input string) (int64, bool) {
					if n, err := strconv.ParseInt(strings.TrimSpace(input), 10, 64); err == nil && n > 1_000_000_000_000 {
						return n, true
					}
					return 0, false
				},
				FormatName:   "epoch_milliseconds",
				UsesLocation: false,
			}, n, true
		}
		// Epoch seconds (10 digits or less)
		return &TimestampParserInfo{
			Parser: func(input string) (int64, bool) {
				if n, err := strconv.ParseInt(strings.TrimSpace(input), 10, 64); err == nil && n <= 1_000_000_000_000 {
					return n * 1000, true
				}
				return 0, false
			},
			FormatName:   "epoch_seconds",
			UsesLocation: false,
		}, n * 1000, true
	}

	// Try explicit timezone formats first
	if t, err := time.Parse(time.RFC3339, ss); err == nil {
		return &TimestampParserInfo{
			Parser: func(input string) (int64, bool) {
				if t, err := time.Parse(time.RFC3339, strings.TrimSpace(input)); err == nil {
					return t.UnixMilli(), true
				}
				return 0, false
			},
			FormatName:   "RFC3339",
			UsesLocation: false,
		}, t.UnixMilli(), true
	}

	if t, err := time.Parse(time.RFC3339Nano, ss); err == nil {
		return &TimestampParserInfo{
			Parser: func(input string) (int64, bool) {
				if t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(input)); err == nil {
					return t.UnixMilli(), true
				}
				return 0, false
			},
			FormatName:   "RFC3339Nano",
			UsesLocation: false,
		}, t.UnixMilli(), true
	}

	// Try space-separated with explicit Z or offset
	if t, err := time.Parse("2006-01-02 15:04:05Z07:00", ss); err == nil {
		return &TimestampParserInfo{
			Parser: func(input string) (int64, bool) {
				if t, err := time.Parse("2006-01-02 15:04:05Z07:00", strings.TrimSpace(input)); err == nil {
					return t.UnixMilli(), true
				}
				return 0, false
			},
			FormatName:   "2006-01-02 15:04:05Z07:00",
			UsesLocation: false,
		}, t.UnixMilli(), true
	}

	if t, err := time.Parse("2006-01-02T15:04:05Z07:00", ss); err == nil {
		return &TimestampParserInfo{
			Parser: func(input string) (int64, bool) {
				if t, err := time.Parse("2006-01-02T15:04:05Z07:00", strings.TrimSpace(input)); err == nil {
					return t.UnixMilli(), true
				}
				return 0, false
			},
			FormatName:   "2006-01-02T15:04:05Z07:00",
			UsesLocation: false,
		}, t.UnixMilli(), true
	}

	// Try various numbers of millisecond digits with timezone
	if t, err := time.Parse("2006-01-02T15:04:05.000 MST", ss); err == nil {
		return &TimestampParserInfo{
			Parser: func(input string) (int64, bool) {
				if t, err := time.Parse("2006-01-02T15:04:05.000 MST", strings.TrimSpace(input)); err == nil {
					return t.UnixMilli(), true
				}
				return 0, false
			},
			FormatName:   "2006-01-02T15:04:05.000 MST",
			UsesLocation: false,
		}, t.UnixMilli(), true
	}

	if t, err := time.Parse("2006-01-02T15:04:05.00 MST", ss); err == nil {
		return &TimestampParserInfo{
			Parser: func(input string) (int64, bool) {
				if t, err := time.Parse("2006-01-02T15:04:05.00 MST", strings.TrimSpace(input)); err == nil {
					return t.UnixMilli(), true
				}
				return 0, false
			},
			FormatName:   "2006-01-02T15:04:05.00 MST",
			UsesLocation: false,
		}, t.UnixMilli(), true
	}

	if t, err := time.Parse("2006-01-02T15:04:05.0 MST", ss); err == nil {
		return &TimestampParserInfo{
			Parser: func(input string) (int64, bool) {
				if t, err := time.Parse("2006-01-02T15:04:05.0 MST", strings.TrimSpace(input)); err == nil {
					return t.UnixMilli(), true
				}
				return 0, false
			},
			FormatName:   "2006-01-02T15:04:05.0 MST",
			UsesLocation: false,
		}, t.UnixMilli(), true
	}

	if t, err := time.Parse("2006-01-02 15:04:05.000 MST", ss); err == nil {
		return &TimestampParserInfo{
			Parser: func(input string) (int64, bool) {
				if t, err := time.Parse("2006-01-02 15:04:05.000 MST", strings.TrimSpace(input)); err == nil {
					return t.UnixMilli(), true
				}
				return 0, false
			},
			FormatName:   "2006-01-02 15:04:05.000 MST",
			UsesLocation: false,
		}, t.UnixMilli(), true
	}

	if t, err := time.Parse("2006-01-02 15:04:05.00 MST", ss); err == nil {
		return &TimestampParserInfo{
			Parser: func(input string) (int64, bool) {
				if t, err := time.Parse("2006-01-02 15:04:05.00 MST", strings.TrimSpace(input)); err == nil {
					return t.UnixMilli(), true
				}
				return 0, false
			},
			FormatName:   "2006-01-02 15:04:05.00 MST",
			UsesLocation: false,
		}, t.UnixMilli(), true
	}

	if t, err := time.Parse("2006-01-02 15:04:05.0 MST", ss); err == nil {
		return &TimestampParserInfo{
			Parser: func(input string) (int64, bool) {
				if t, err := time.Parse("2006-01-02 15:04:05.0 MST", strings.TrimSpace(input)); err == nil {
					return t.UnixMilli(), true
				}
				return 0, false
			},
			FormatName:   "2006-01-02 15:04:05.0 MST",
			UsesLocation: false,
		}, t.UnixMilli(), true
	}

	if t, err := time.Parse("2006-01-02 15:04:05 MST", ss); err == nil {
		return &TimestampParserInfo{
			Parser: func(input string) (int64, bool) {
				if t, err := time.Parse("2006-01-02 15:04:05 MST", strings.TrimSpace(input)); err == nil {
					return t.UnixMilli(), true
				}
				return 0, false
			},
			FormatName:   "2006-01-02 15:04:05.0 MST",
			UsesLocation: false,
		}, t.UnixMilli(), true
	}

	// With Z suffix
	if strings.HasSuffix(ss, "Z") {
		if t, err := time.Parse("2006-01-02 15:04:05Z", ss); err == nil {
			return &TimestampParserInfo{
				Parser: func(input string) (int64, bool) {
					if t, err := time.Parse("2006-01-02 15:04:05Z", strings.TrimSpace(input)); err == nil {
						return t.UnixMilli(), true
					}
					return 0, false
				},
				FormatName:   "2006-01-02 15:04:05Z",
				UsesLocation: false,
			}, t.UnixMilli(), true
		}

		if t, err := time.Parse("2006-01-02T15:04:05Z", ss); err == nil {
			return &TimestampParserInfo{
				Parser: func(input string) (int64, bool) {
					if t, err := time.Parse("2006-01-02T15:04:05Z", strings.TrimSpace(input)); err == nil {
						return t.UnixMilli(), true
					}
					return 0, false
				},
				FormatName:   "2006-01-02T15:04:05Z",
				UsesLocation: false,
			}, t.UnixMilli(), true
		}
	}

	// Determine ingest timezone for timezone-less formats if not provided
	if loc == nil {
		effective := settings.GetEffectiveSettings()
		tzName := strings.TrimSpace(effective.DefaultIngestTimezone)
		switch strings.ToUpper(tzName) {
		case "", "LOCAL":
			loc = time.Local
		case "UTC":
			loc = time.UTC
		default:
			if l, err := time.LoadLocation(tzName); err == nil {
				loc = l
			} else {
				loc = time.Local
			}
		}
	}

	// Try various numbers of millisecond digits with location
	if t, err := time.ParseInLocation("2006-01-02T15:04:05.000", ss, loc); err == nil {
		// Capture loc in closure
		locCopy := loc
		return &TimestampParserInfo{
			Parser: func(input string) (int64, bool) {
				if t, err := time.ParseInLocation("2006-01-02T15:04:05.000", strings.TrimSpace(input), locCopy); err == nil {
					return t.UnixMilli(), true
				}
				return 0, false
			},
			FormatName:   "2006-01-02T15:04:05.000",
			UsesLocation: true,
		}, t.UnixMilli(), true
	}

	if t, err := time.ParseInLocation("2006-01-02T15:04:05.00", ss, loc); err == nil {
		locCopy := loc
		return &TimestampParserInfo{
			Parser: func(input string) (int64, bool) {
				if t, err := time.ParseInLocation("2006-01-02T15:04:05.00", strings.TrimSpace(input), locCopy); err == nil {
					return t.UnixMilli(), true
				}
				return 0, false
			},
			FormatName:   "2006-01-02T15:04:05.00",
			UsesLocation: true,
		}, t.UnixMilli(), true
	}

	if t, err := time.ParseInLocation("2006-01-02T15:04:05.0", ss, loc); err == nil {
		locCopy := loc
		return &TimestampParserInfo{
			Parser: func(input string) (int64, bool) {
				if t, err := time.ParseInLocation("2006-01-02T15:04:05.0", strings.TrimSpace(input), locCopy); err == nil {
					return t.UnixMilli(), true
				}
				return 0, false
			},
			FormatName:   "2006-01-02T15:04:05.0",
			UsesLocation: true,
		}, t.UnixMilli(), true
	}

	if t, err := time.ParseInLocation("2006-01-02 15:04:05.000", ss, loc); err == nil {
		locCopy := loc
		return &TimestampParserInfo{
			Parser: func(input string) (int64, bool) {
				if t, err := time.ParseInLocation("2006-01-02 15:04:05.000", strings.TrimSpace(input), locCopy); err == nil {
					return t.UnixMilli(), true
				}
				return 0, false
			},
			FormatName:   "2006-01-02 15:04:05.000",
			UsesLocation: true,
		}, t.UnixMilli(), true
	}

	if t, err := time.ParseInLocation("2006-01-02 15:04:05.00", ss, loc); err == nil {
		locCopy := loc
		return &TimestampParserInfo{
			Parser: func(input string) (int64, bool) {
				if t, err := time.ParseInLocation("2006-01-02 15:04:05.00", strings.TrimSpace(input), locCopy); err == nil {
					return t.UnixMilli(), true
				}
				return 0, false
			},
			FormatName:   "2006-01-02 15:04:05.00",
			UsesLocation: true,
		}, t.UnixMilli(), true
	}

	if t, err := time.ParseInLocation("2006-01-02 15:04:05.0", ss, loc); err == nil {
		locCopy := loc
		return &TimestampParserInfo{
			Parser: func(input string) (int64, bool) {
				if t, err := time.ParseInLocation("2006-01-02 15:04:05.0", strings.TrimSpace(input), locCopy); err == nil {
					return t.UnixMilli(), true
				}
				return 0, false
			},
			FormatName:   "2006-01-02 15:04:05.0",
			UsesLocation: true,
		}, t.UnixMilli(), true
	}

	// Support ISO-like without timezone and common space-separated formats using loc
	if t, err := time.ParseInLocation("2006-01-02T15:04:05", ss, loc); err == nil {
		locCopy := loc
		return &TimestampParserInfo{
			Parser: func(input string) (int64, bool) {
				if t, err := time.ParseInLocation("2006-01-02T15:04:05", strings.TrimSpace(input), locCopy); err == nil {
					return t.UnixMilli(), true
				}
				return 0, false
			},
			FormatName:   "2006-01-02T15:04:05",
			UsesLocation: true,
		}, t.UnixMilli(), true
	}

	if t, err := time.ParseInLocation("2006-01-02T15:04", ss, loc); err == nil {
		locCopy := loc
		return &TimestampParserInfo{
			Parser: func(input string) (int64, bool) {
				if t, err := time.ParseInLocation("2006-01-02T15:04", strings.TrimSpace(input), locCopy); err == nil {
					return t.UnixMilli(), true
				}
				return 0, false
			},
			FormatName:   "2006-01-02T15:04",
			UsesLocation: true,
		}, t.UnixMilli(), true
	}

	// Try common "2006-01-02 15:04:05"
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", ss, loc); err == nil {
		locCopy := loc
		return &TimestampParserInfo{
			Parser: func(input string) (int64, bool) {
				if t, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(input), locCopy); err == nil {
					return t.UnixMilli(), true
				}
				return 0, false
			},
			FormatName:   "2006-01-02 15:04:05",
			UsesLocation: true,
		}, t.UnixMilli(), true
	}

	if t, err := time.ParseInLocation("2006-01-02 15:04", ss, loc); err == nil {
		locCopy := loc
		return &TimestampParserInfo{
			Parser: func(input string) (int64, bool) {
				if t, err := time.ParseInLocation("2006-01-02 15:04", strings.TrimSpace(input), locCopy); err == nil {
					return t.UnixMilli(), true
				}
				return 0, false
			},
			FormatName:   "2006-01-02 15:04",
			UsesLocation: true,
		}, t.UnixMilli(), true
	}

	if t, err := time.ParseInLocation("2006-01-02", ss, loc); err == nil {
		locCopy := loc
		return &TimestampParserInfo{
			Parser: func(input string) (int64, bool) {
				if t, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(input), locCopy); err == nil {
					return t.UnixMilli(), true
				}
				return 0, false
			},
			FormatName:   "2006-01-02",
			UsesLocation: true,
		}, t.UnixMilli(), true
	}

	// These timestamp formats are often interchangable, so if either matches we should provide a parser than supports both
	if t, err := time.ParseInLocation("02/01/2006 3:04pm", ss, loc); err == nil {
		locCopy := loc
		return &TimestampParserInfo{
			Parser: func(input string) (int64, bool) {
				if t, err := time.ParseInLocation("02/01/2006 3:04pm", strings.TrimSpace(input), locCopy); err == nil {
					return t.UnixMilli(), true
				}
				if t, err := time.ParseInLocation("02/01/2006 03:04pm", strings.TrimSpace(input), locCopy); err == nil {
					return t.UnixMilli(), true
				}
				return 0, false
			},
			FormatName:   "02/01/2006 3:04pm",
			UsesLocation: true,
		}, t.UnixMilli(), true
	}
	// These timestamp formats are often interchangable, so if either matches we should provide a parser than supports both
	if t, err := time.ParseInLocation("02/01/2006 03:04pm", ss, loc); err == nil {
		locCopy := loc
		return &TimestampParserInfo{
			Parser: func(input string) (int64, bool) {
				if t, err := time.ParseInLocation("02/01/2006 3:04pm", strings.TrimSpace(input), locCopy); err == nil {
					return t.UnixMilli(), true
				}
				if t, err := time.ParseInLocation("02/01/2006 03:04pm", strings.TrimSpace(input), locCopy); err == nil {
					return t.UnixMilli(), true
				}
				return 0, false
			},
			FormatName:   "02/01/2006 03:04pm",
			UsesLocation: true,
		}, t.UnixMilli(), true
	}
	// These timestamp formats are often interchangable, so if either matches we should provide a parser than supports both
	if t, err := time.ParseInLocation("02/01/2006 3:04 pm", ss, loc); err == nil {
		locCopy := loc
		return &TimestampParserInfo{
			Parser: func(input string) (int64, bool) {
				if t, err := time.ParseInLocation("02/01/2006 3:04 pm", strings.TrimSpace(input), locCopy); err == nil {
					return t.UnixMilli(), true
				}
				if t, err := time.ParseInLocation("02/01/2006 03:04 pm", strings.TrimSpace(input), locCopy); err == nil {
					return t.UnixMilli(), true
				}
				return 0, false
			},
			FormatName:   "02/01/2006 3:04 pm",
			UsesLocation: true,
		}, t.UnixMilli(), true
	}
	// These timestamp formats are often interchangable, so if either matches we should provide a parser than supports both
	if t, err := time.ParseInLocation("02/01/2006 03:04 pm", ss, loc); err == nil {
		locCopy := loc
		return &TimestampParserInfo{
			Parser: func(input string) (int64, bool) {
				if t, err := time.ParseInLocation("02/01/2006 3:04 pm", strings.TrimSpace(input), locCopy); err == nil {
					return t.UnixMilli(), true
				}
				if t, err := time.ParseInLocation("02/01/2006 03:04 pm", strings.TrimSpace(input), locCopy); err == nil {
					return t.UnixMilli(), true
				}
				return 0, false
			},
			FormatName:   "02/01/2006 03:04 pm",
			UsesLocation: true,
		}, t.UnixMilli(), true
	}

	// No format matched
	return nil, 0, false
}

// FormatTimestampForComparison formats a millisecond timestamp to a display string
// using the specified timezone and format pattern. This is used for filter matching
// where we need to compare user-entered filter values against displayed timestamps.
// The format uses the same pattern style as the display settings (e.g., "yyyy-MM-dd HH:mm:ss").
func FormatTimestampForComparison(ms int64, loc *time.Location, formatPattern string) string {
	if loc == nil {
		loc = time.Local
	}

	t := time.UnixMilli(ms).In(loc)

	// Convert pattern to Go layout
	pattern := strings.TrimSpace(formatPattern)
	if pattern == "" {
		pattern = "yyyy-MM-dd HH:mm:ss"
	}

	r := strings.NewReplacer(
		"yyyy", "2006",
		"yy", "06",
		"MM", "01",
		"dd", "02",
		"HH", "15",
		"mm", "04",
		"ss", "05",
		"SSS", "000",
		"zzz", "MST",
	)
	layout := r.Replace(pattern)

	return t.Format(layout)
}

// TimestampMatchInfo contains pre-computed information for timestamp column filtering
type TimestampMatchInfo struct {
	FilterMs       int64  // Parsed filter value in milliseconds (0 if partial match)
	FilterPrefix   string // For partial matches, the prefix to match against formatted timestamp
	IsPartialMatch bool   // True if this is a prefix/partial match (e.g., "2025-09-11*" or just "2025-09-11")
	IsExactMatch   bool   // True if filter parsed to exact milliseconds
}

// PrepareTimestampMatch prepares matching info for a filter value on a timestamp column.
// filterValue: the user's filter input (e.g., "2025-09-11 14:30:00" or "2025-09-11*")
// displayTz: the user's display timezone (for parsing the filter value)
// Returns matching info that can be used to efficiently compare against Row.Timestamp
func PrepareTimestampMatch(filterValue string, displayTz *time.Location) *TimestampMatchInfo {
	if displayTz == nil {
		displayTz = time.Local
	}

	trimmed := strings.TrimSpace(filterValue)
	if trimmed == "" {
		return nil
	}

	// Check for explicit wildcard suffix
	isWildcard := strings.HasSuffix(trimmed, "*")
	if isWildcard {
		trimmed = trimmed[:len(trimmed)-1]
	}

	// Try to parse as exact timestamp
	if ms, ok := ParseTimestampMillis(trimmed, displayTz); ok {
		if isWildcard {
			// User explicitly wants prefix match - use the filter string as prefix
			return &TimestampMatchInfo{
				FilterPrefix:   trimmed,
				IsPartialMatch: true,
				IsExactMatch:   false,
			}
		}
		// Check if this is a partial date (no time component) by checking the format
		// If the filter is just a date like "2025-09-11", treat it as a prefix match
		if isPartialTimestamp(trimmed) {
			return &TimestampMatchInfo{
				FilterPrefix:   trimmed,
				IsPartialMatch: true,
				IsExactMatch:   false,
			}
		}
		// Full timestamp - do exact millisecond comparison
		return &TimestampMatchInfo{
			FilterMs:       ms,
			IsPartialMatch: false,
			IsExactMatch:   true,
		}
	}

	// Couldn't parse - treat as prefix match if it looks like a partial timestamp
	if isWildcard || isPartialTimestamp(trimmed) {
		return &TimestampMatchInfo{
			FilterPrefix:   trimmed,
			IsPartialMatch: true,
			IsExactMatch:   false,
		}
	}

	return nil
}

// isPartialTimestamp checks if a string looks like a partial timestamp (date only, or date+hour, etc.)
func isPartialTimestamp(s string) bool {
	// Common partial timestamp patterns
	partialPatterns := []string{
		"2006-01-02",       // Date only
		"2006-01-02 15",    // Date + hour
		"2006-01-02 15:04", // Date + hour:minute
		"2006-01-02T15",    // ISO date + hour
		"2006-01-02T15:04", // ISO date + hour:minute
	}

	for _, pattern := range partialPatterns {
		if _, err := time.Parse(pattern, s); err == nil {
			return true
		}
	}
	return false
}

// MatchTimestamp checks if a Row's timestamp matches the prepared filter info.
// rowMs: the pre-parsed timestamp from Row.Timestamp
// hasTime: from Row.HasTime - whether the row has a valid timestamp
// displayTz: the user's display timezone
// formatPattern: the display format pattern (e.g., "yyyy-MM-dd HH:mm:ss")
// matchInfo: pre-computed matching info from PrepareTimestampMatch
func MatchTimestamp(rowMs int64, hasTime bool, displayTz *time.Location, formatPattern string, matchInfo *TimestampMatchInfo) bool {
	if matchInfo == nil || !hasTime {
		return false
	}

	if matchInfo.IsExactMatch {
		// Exact millisecond comparison
		return rowMs == matchInfo.FilterMs
	}

	if matchInfo.IsPartialMatch {
		// Format the row's timestamp and do prefix comparison
		formatted := FormatTimestampForComparison(rowMs, displayTz, formatPattern)
		return strings.HasPrefix(strings.ToLower(formatted), strings.ToLower(matchInfo.FilterPrefix))
	}

	return false
}
