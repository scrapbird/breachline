package query

import (
	"breachline/app/timestamps"
	"time"
)

// StringsToRows converts [][]string to []*Row with timestamp parsing
// ingestTimezone is used for parsing timestamps without timezone info; if nil, uses default from settings
func StringsToRows(strings [][]string, timeFieldIdx int, ingestTimezone *time.Location) []*Row {
	rows := make([]*Row, len(strings))
	for i, data := range strings {
		row := &Row{
			Data:      data,
			Timestamp: 0,
			HasTime:   false,
		}
		// Parse timestamp if time field exists
		if timeFieldIdx >= 0 && timeFieldIdx < len(data) {
			if ms, ok := timestamps.ParseTimestampMillis(data[timeFieldIdx], ingestTimezone); ok {
				row.Timestamp = ms
				row.HasTime = true
			}
		}
		rows[i] = row
	}
	return rows
}

// RowsToStrings converts []*Row to [][]string (extracts raw data)
func RowsToStrings(rows []*Row) [][]string {
	strings := make([][]string, len(rows))
	for i, row := range rows {
		strings[i] = row.Data
	}
	return strings
}
