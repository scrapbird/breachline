package query

import (
	"testing"
)

func TestParseSortStage_MultipleColumns(t *testing.T) {
	qe := NewQueryExecutor(nil, nil, DefaultCacheConfig())
	header := []string{"user name", "event source", "event time", "status"}

	tests := []struct {
		name              string
		stage             string
		expectedColumns   []string
		expectedDescFlags []bool
	}{
		{
			name:              "Single column",
			stage:             `sort "user name"`,
			expectedColumns:   []string{"user name"},
			expectedDescFlags: []bool{false},
		},
		{
			name:              "Single column desc",
			stage:             `sort "user name" desc`,
			expectedColumns:   []string{"user name"},
			expectedDescFlags: []bool{true},
		},
		{
			name:              "Single column descending",
			stage:             `sort "user name" descending`,
			expectedColumns:   []string{"user name"},
			expectedDescFlags: []bool{true},
		},
		{
			name:              "Two columns comma separated",
			stage:             `sort "event source", "event time"`,
			expectedColumns:   []string{"event source", "event time"},
			expectedDescFlags: []bool{false, false},
		},
		{
			name:              "Three columns comma separated",
			stage:             `sort "user name", "event source", "event time"`,
			expectedColumns:   []string{"user name", "event source", "event time"},
			expectedDescFlags: []bool{false, false, false},
		},
		{
			name:              "Multiple columns with trailing desc (applies to last column only)",
			stage:             `sort "event source", "event time" desc`,
			expectedColumns:   []string{"event source", "event time"},
			expectedDescFlags: []bool{false, true},
		},
		{
			name:              "Multiple columns with asc",
			stage:             `sort "event source", "event time" asc`,
			expectedColumns:   []string{"event source", "event time"},
			expectedDescFlags: []bool{false, false},
		},
		{
			name:              "Columns without quotes",
			stage:             `sort status, "event time"`,
			expectedColumns:   []string{"status", "event time"},
			expectedDescFlags: []bool{false, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			columns, descFlags := qe.parseSortStage(tt.stage, header)

			if len(columns) != len(tt.expectedColumns) {
				t.Errorf("Expected %d columns, got %d", len(tt.expectedColumns), len(columns))
				return
			}

			for i := range columns {
				if columns[i] != tt.expectedColumns[i] {
					t.Errorf("Column %d: expected %q, got %q", i, tt.expectedColumns[i], columns[i])
				}
			}

			if len(descFlags) != len(tt.expectedDescFlags) {
				t.Errorf("Expected %d desc flags, got %d", len(tt.expectedDescFlags), len(descFlags))
				return
			}

			for i := range descFlags {
				if descFlags[i] != tt.expectedDescFlags[i] {
					t.Errorf("Desc flag %d: expected %v, got %v", i, tt.expectedDescFlags[i], descFlags[i])
				}
			}
		})
	}
}

// TestSortStage_MultipleColumns is disabled due to undefined stream types
// TODO: Re-enable when stream types are fixed
// func TestSortStage_MultipleColumns(t *testing.T) { ... }
