package query

import (
	"testing"
)

func TestSortStage_TimestampAscDesc(t *testing.T) {
	// Create test data with timestamps
	rows := []*Row{
		{Data: []string{"log3", "2024-01-03 10:00:00"}, Timestamp: 1704276000000, HasTime: true},
		{Data: []string{"log1", "2024-01-01 10:00:00"}, Timestamp: 1704103200000, HasTime: true},
		{Data: []string{"log2", "2024-01-02 10:00:00"}, Timestamp: 1704189600000, HasTime: true},
	}

	header := []string{"message", "timestamp"}

	// Test ascending sort
	t.Run("Ascending", func(t *testing.T) {
		testRows := make([]*Row, len(rows))
		for i, r := range rows {
			testRows[i] = &Row{Data: r.Data, Timestamp: r.Timestamp, HasTime: r.HasTime}
		}

		stage := NewTimeSortStage("timestamp", false) // false = ascending
		input := &StageResult{
			Header:         header,
			OriginalHeader: header,
			Rows:           testRows,
		}

		output, err := stage.Execute(input)
		if err != nil {
			t.Fatalf("Sort failed: %v", err)
		}

		// Verify ascending order (oldest first)
		if len(output.Rows) != 3 {
			t.Fatalf("Expected 3 rows, got %d", len(output.Rows))
		}

		if output.Rows[0].Timestamp != 1704103200000 { // log1 - oldest
			t.Errorf("First row should be log1 (oldest), got timestamp %d", output.Rows[0].Timestamp)
		}
		if output.Rows[1].Timestamp != 1704189600000 { // log2 - middle
			t.Errorf("Second row should be log2, got timestamp %d", output.Rows[1].Timestamp)
		}
		if output.Rows[2].Timestamp != 1704276000000 { // log3 - newest
			t.Errorf("Third row should be log3 (newest), got timestamp %d", output.Rows[2].Timestamp)
		}
	})

	// Test descending sort
	t.Run("Descending", func(t *testing.T) {
		testRows := make([]*Row, len(rows))
		for i, r := range rows {
			testRows[i] = &Row{Data: r.Data, Timestamp: r.Timestamp, HasTime: r.HasTime}
		}

		stage := NewTimeSortStage("timestamp", true) // true = descending
		input := &StageResult{
			Header:         header,
			OriginalHeader: header,
			Rows:           testRows,
		}

		output, err := stage.Execute(input)
		if err != nil {
			t.Fatalf("Sort failed: %v", err)
		}

		// Verify descending order (newest first)
		if len(output.Rows) != 3 {
			t.Fatalf("Expected 3 rows, got %d", len(output.Rows))
		}

		if output.Rows[0].Timestamp != 1704276000000 { // log3 - newest
			t.Errorf("First row should be log3 (newest), got timestamp %d", output.Rows[0].Timestamp)
		}
		if output.Rows[1].Timestamp != 1704189600000 { // log2 - middle
			t.Errorf("Second row should be log2, got timestamp %d", output.Rows[1].Timestamp)
		}
		if output.Rows[2].Timestamp != 1704103200000 { // log1 - oldest
			t.Errorf("Third row should be log1 (oldest), got timestamp %d", output.Rows[2].Timestamp)
		}
	})
}

func TestSortStage_MultiColumnSort(t *testing.T) {
	// Test data with multiple columns
	rows := []*Row{
		{Data: []string{"Alice", "Engineering", "100000"}},
		{Data: []string{"Bob", "Engineering", "90000"}},
		{Data: []string{"Charlie", "Sales", "80000"}},
		{Data: []string{"David", "Engineering", "95000"}},
		{Data: []string{"Eve", "Sales", "95000"}},
	}

	header := []string{"name", "department", "salary"}

	// Sort by department (asc), then salary (desc)
	stage := NewSortStage([]string{"department", "salary"}, []bool{false, true})
	input := &StageResult{
		Header:         header,
		OriginalHeader: header,
		Rows:           rows,
	}

	output, err := stage.Execute(input)
	if err != nil {
		t.Fatalf("Sort failed: %v", err)
	}

	if len(output.Rows) != 5 {
		t.Fatalf("Expected 5 rows, got %d", len(output.Rows))
	}

	// Expected order:
	// Engineering dept (asc): Alice (100000), David (95000), Bob (90000)
	// Sales dept (asc): Eve (95000), Charlie (80000)

	// Check department sorting (ascending)
	if output.Rows[0].Data[1] != "Engineering" || output.Rows[1].Data[1] != "Engineering" || output.Rows[2].Data[1] != "Engineering" {
		t.Error("First three rows should be Engineering department")
	}
	if output.Rows[3].Data[1] != "Sales" || output.Rows[4].Data[1] != "Sales" {
		t.Error("Last two rows should be Sales department")
	}

	// Check salary sorting within Engineering (descending)
	if output.Rows[0].Data[2] != "100000" {
		t.Errorf("First Engineering row should have salary 100000, got %s", output.Rows[0].Data[2])
	}
	if output.Rows[1].Data[2] != "95000" {
		t.Errorf("Second Engineering row should have salary 95000, got %s", output.Rows[1].Data[2])
	}
	if output.Rows[2].Data[2] != "90000" {
		t.Errorf("Third Engineering row should have salary 90000, got %s", output.Rows[2].Data[2])
	}

	// Check salary sorting within Sales (descending)
	if output.Rows[3].Data[2] != "95000" {
		t.Errorf("First Sales row should have salary 95000, got %s", output.Rows[3].Data[2])
	}
	if output.Rows[4].Data[2] != "80000" {
		t.Errorf("Second Sales row should have salary 80000, got %s", output.Rows[4].Data[2])
	}
}
