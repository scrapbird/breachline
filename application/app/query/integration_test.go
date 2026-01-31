package query

import (
	"strings"
	"testing"
	"time"

	"breachline/app/interfaces"
)

// testRow creates a *Row from []string for testing
// Optionally accepts timestamp info as (ms int64, hasTime bool)
func testRow(data []string, tsInfo ...interface{}) *Row {
	row := &Row{
		Data:    data,
		HasTime: false,
	}
	if len(tsInfo) >= 2 {
		if ms, ok := tsInfo[0].(int64); ok {
			row.Timestamp = ms
		}
		if hasTime, ok := tsInfo[1].(bool); ok {
			row.HasTime = hasTime
		}
	}
	return row
}

// MockWorkspaceService implements the interface needed for annotation testing
type MockWorkspaceService struct {
	annotatedRows map[string]bool // key: "fileHash:opts:row0:row1:..." -> bool
}

func (m *MockWorkspaceService) IsRowAnnotatedByIndex(fileHash string, opts interfaces.FileOptions, rowIndex int) (bool, *interfaces.AnnotationResult) {
	// For testing, we don't have actual row data, so just return false
	// This mock is primarily used to verify that buildMatcherFromQuery doesn't call this method
	return false, nil
}

func TestAnnotatedOperator(t *testing.T) {
	// Create query executor
	qe := NewQueryExecutor(nil, nil, DefaultCacheConfig())

	// Test data
	header := []string{"user", "action"}

	// Test that buildMatcherFromQuery no longer handles "annotated" as an operator
	// It should now treat "annotated" as a literal text search
	matcher := qe.buildMatcherFromQuery("annotated", header, "", -1, nil)

	// Test row containing "annotated" as text
	rowWithAnnotatedText := testRow([]string{"annotated", "action1"})
	if !matcher(rowWithAnnotatedText) {
		t.Errorf("Expected row containing 'annotated' text to match literal 'annotated' query")
	}

	// Test row without "annotated" text (even if it's annotated in the workspace)
	annotatedRowWithoutText := testRow([]string{"user1", "action1"})
	if matcher(annotatedRowWithoutText) {
		t.Errorf("Expected annotated row without 'annotated' text to not match literal 'annotated' query")
	}
}

func TestNotAnnotatedOperator(t *testing.T) {
	// Note: The "annotated" operator has been moved to its own pipeline stage type.
	// "NOT annotated" in buildMatcherFromQuery is now parsed as a boolean expression:
	// NOT (any field contains "annotated" text)
	//
	// The actual annotation-based filtering is handled at the pipeline level,
	// not in buildMatcherFromQuery.

	// Create query executor
	qe := NewQueryExecutor(nil, nil, DefaultCacheConfig())

	// Test data
	header := []string{"user", "action"}

	// Test "NOT annotated" as a boolean expression (NOT + literal text search)
	matcher := qe.buildMatcherFromQuery("NOT annotated", header, "", -1, nil)

	// Row without the word "annotated" should match (NOT = negation of text search)
	rowWithoutText := testRow([]string{"user1", "action1"})
	if !matcher(rowWithoutText) {
		t.Errorf("Expected row without 'annotated' text to match 'NOT annotated' boolean expression")
	}

	// Row containing the word "annotated" should NOT match
	rowWithText := testRow([]string{"annotated", "action1"})
	if matcher(rowWithText) {
		t.Errorf("Expected row with 'annotated' text to NOT match 'NOT annotated' boolean expression")
	}
}

func TestAnnotatedOperatorCaseInsensitive(t *testing.T) {
	// Create query executor
	qe := NewQueryExecutor(nil, nil, DefaultCacheConfig())

	// Test data
	header := []string{"user", "action"}

	// Test different case variations - now these should work as literal text searches
	testCases := []string{"annotated", "ANNOTATED", "Annotated", "AnNoTaTeD"}

	for _, query := range testCases {
		matcher := qe.buildMatcherFromQuery(query, header, "", -1, nil)

		// Test row containing the text (should match)
		rowWithText := testRow([]string{strings.ToLower(query), "action1"})
		if !matcher(rowWithText) {
			t.Errorf("Expected row containing '%s' text to match '%s' query (case insensitive)", query, query)
		}

		// Test row without the text (should not match, even if annotated)
		annotatedRowWithoutText := testRow([]string{"user1", "action1"})
		if matcher(annotatedRowWithoutText) {
			t.Errorf("Expected annotated row without '%s' text to not match '%s' query", query, query)
		}
	}
}

func TestQuotedAnnotatedOperator(t *testing.T) {
	// Create query executor
	qe := NewQueryExecutor(nil, nil, DefaultCacheConfig())

	// Test data
	header := []string{"field1", "field2"}

	// Test quoted "annotated" - should be treated as literal text search, not operator
	query := `"annotated"`
	t.Logf("Testing with query: %s", query)
	matcher := qe.buildMatcherFromQuery(query, header, "", -1, nil)

	// Test row containing "annotated" as data
	rowWithAnnotatedText := testRow([]string{"annotated", "data"})
	if !matcher(rowWithAnnotatedText) {
		t.Errorf("Expected row containing 'annotated' text to match quoted 'annotated' query")
	}

	// Test row without "annotated" text
	rowWithoutAnnotatedText := testRow([]string{"user1", "action1"})
	if matcher(rowWithoutAnnotatedText) {
		t.Errorf("Expected row without 'annotated' text to not match quoted 'annotated' query")
	}
}

func TestAnnotatedOperatorWithTimeFilters(t *testing.T) {
	// Note: The "annotated" operator has been moved to its own pipeline stage type.
	// In buildMatcherFromQuery, "annotated after 2024-01-01" is now parsed as:
	// - Time filter: after 2024-01-01
	// - Text search: "annotated" (literal text)
	//
	// The actual annotation-based filtering is handled at the pipeline level.

	// Create query executor with timezone
	qe := NewQueryExecutorWithTimezone(nil, nil, DefaultCacheConfig(), time.UTC)

	// Test data with timestamp column
	header := []string{"timestamp", "action"}

	// Test "annotated after 2024-01-01" - this now means: time filter + literal text "annotated"
	matcher := qe.buildMatcherFromQuery("annotated after 2024-01-01", header, "", -1, nil)

	// Row with "annotated" text and timestamp after filter should match
	// 2024-01-02 10:00:00 UTC = 1704189600000 ms
	rowWithTextAfter := testRow([]string{"2024-01-02 10:00:00", "annotated"}, int64(1704189600000), true)
	if !matcher(rowWithTextAfter) {
		t.Errorf("Expected row with 'annotated' text after 2024-01-01 to match")
	}

	// Row without "annotated" text should NOT match (even with valid timestamp)
	rowWithoutTextAfter := testRow([]string{"2024-01-02 10:00:00", "action1"}, int64(1704189600000), true)
	if matcher(rowWithoutTextAfter) {
		t.Errorf("Expected row without 'annotated' text to NOT match")
	}

	// Row with "annotated" text but before the time filter should NOT match
	// 2023-12-31 10:00:00 UTC = 1704016800000 ms
	rowWithTextBefore := testRow([]string{"2023-12-31 10:00:00", "annotated"}, int64(1704016800000), true)
	if matcher(rowWithTextBefore) {
		t.Errorf("Expected row before 2024-01-01 to NOT match time filter")
	}
}

func TestAnnotatedOperatorPipelineParsing(t *testing.T) {
	// Create query executor
	qe := NewQueryExecutor(nil, nil, DefaultCacheConfig())

	// Test that "annotated" is recognized as a valid operation by checking the parsing logic
	// We'll test the stage parsing directly rather than full pipeline building
	stages := qe.splitPipesTopLevel("annotated")
	if len(stages) != 1 {
		t.Errorf("Expected 1 stage for 'annotated' query, got %d", len(stages))
	}

	// Test the stage recognition logic
	stage := stages[0]
	tokens := qe.splitRespectingQuotes(stage)
	if len(tokens) == 0 {
		t.Errorf("Expected tokens for 'annotated' stage")
		return
	}

	head := strings.ToLower(tokens[0])
	if head != "annotated" {
		t.Errorf("Expected head to be 'annotated', got '%s'", head)
	}

	// Test that "NOT annotated" is also recognized
	stages2 := qe.splitPipesTopLevel("NOT annotated")
	if len(stages2) != 1 {
		t.Errorf("Expected 1 stage for 'NOT annotated' query, got %d", len(stages2))
	}

	stage2 := stages2[0]
	tokens2 := qe.splitRespectingQuotes(stage2)
	if len(tokens2) < 2 {
		t.Errorf("Expected at least 2 tokens for 'NOT annotated' stage")
		return
	}

	head2 := strings.ToLower(tokens2[0])
	if head2 != "not" {
		t.Errorf("Expected head to be 'not', got '%s'", head2)
	}
}

func TestTimeFilterOperatorParsing(t *testing.T) {
	// Create query executor
	qe := NewQueryExecutor(nil, nil, DefaultCacheConfig())

	// Test that "after" is recognized as a valid operation
	stages := qe.splitPipesTopLevel("after 2024-01-01")
	if len(stages) != 1 {
		t.Errorf("Expected 1 stage for 'after 2024-01-01' query, got %d", len(stages))
	}

	stage := stages[0]
	tokens := qe.splitRespectingQuotes(stage)
	if len(tokens) < 2 {
		t.Errorf("Expected at least 2 tokens for 'after 2024-01-01' stage")
		return
	}

	head := strings.ToLower(tokens[0])
	if head != "after" {
		t.Errorf("Expected head to be 'after', got '%s'", head)
	}

	// Test that "before" is also recognized
	stages2 := qe.splitPipesTopLevel("before 2024-12-31")
	if len(stages2) != 1 {
		t.Errorf("Expected 1 stage for 'before 2024-12-31' query, got %d", len(stages2))
	}

	stage2 := stages2[0]
	tokens2 := qe.splitRespectingQuotes(stage2)
	if len(tokens2) < 2 {
		t.Errorf("Expected at least 2 tokens for 'before 2024-12-31' stage")
		return
	}

	head2 := strings.ToLower(tokens2[0])
	if head2 != "before" {
		t.Errorf("Expected head to be 'before', got '%s'", head2)
	}
}

// ============================================================
// Boolean Filter Expression Tests
// ============================================================

func TestBooleanFilterNOT(t *testing.T) {
	qe := NewQueryExecutor(nil, nil, DefaultCacheConfig())
	header := []string{"username", "action"}

	// Test NOT with simple text search
	matcher := qe.buildMatcherFromQuery("NOT scrappy", header, "", -1, nil)

	// Row without "scrappy" should match
	if !matcher(testRow([]string{"bob", "login"})) {
		t.Errorf("Expected row without 'scrappy' to match 'NOT scrappy'")
	}

	// Row with "scrappy" should NOT match
	if matcher(testRow([]string{"scrappy", "login"})) {
		t.Errorf("Expected row with 'scrappy' to NOT match 'NOT scrappy'")
	}
}

func TestBooleanFilterOR(t *testing.T) {
	qe := NewQueryExecutor(nil, nil, DefaultCacheConfig())
	header := []string{"username", "action"}

	// Test OR with simple text search
	matcher := qe.buildMatcherFromQuery("scrappy OR bob", header, "", -1, nil)

	// Row with "scrappy" should match
	if !matcher(testRow([]string{"scrappy", "login"})) {
		t.Errorf("Expected row with 'scrappy' to match 'scrappy OR bob'")
	}

	// Row with "bob" should match
	if !matcher(testRow([]string{"bob", "login"})) {
		t.Errorf("Expected row with 'bob' to match 'scrappy OR bob'")
	}

	// Row with neither should NOT match
	if matcher(testRow([]string{"alice", "login"})) {
		t.Errorf("Expected row with neither to NOT match 'scrappy OR bob'")
	}
}

func TestBooleanFilterAND(t *testing.T) {
	qe := NewQueryExecutor(nil, nil, DefaultCacheConfig())
	header := []string{"username", "action"}

	// Test AND with simple text search
	matcher := qe.buildMatcherFromQuery("scrappy AND login", header, "", -1, nil)

	// Row with both should match
	if !matcher(testRow([]string{"scrappy", "login"})) {
		t.Errorf("Expected row with both to match 'scrappy AND login'")
	}

	// Row with only "scrappy" should NOT match
	if matcher(testRow([]string{"scrappy", "logout"})) {
		t.Errorf("Expected row with only 'scrappy' to NOT match 'scrappy AND login'")
	}

	// Row with only "login" should NOT match
	if matcher(testRow([]string{"bob", "login"})) {
		t.Errorf("Expected row with only 'login' to NOT match 'scrappy AND login'")
	}
}

func TestBooleanFilterComplex(t *testing.T) {
	qe := NewQueryExecutor(nil, nil, DefaultCacheConfig())
	header := []string{"username", "action"}

	// Test complex expression: (scrappy OR bob) AND login
	matcher := qe.buildMatcherFromQuery("(scrappy OR bob) AND login", header, "", -1, nil)

	// scrappy + login should match
	if !matcher(testRow([]string{"scrappy", "login"})) {
		t.Errorf("Expected 'scrappy+login' to match '(scrappy OR bob) AND login'")
	}

	// bob + login should match
	if !matcher(testRow([]string{"bob", "login"})) {
		t.Errorf("Expected 'bob+login' to match '(scrappy OR bob) AND login'")
	}

	// scrappy + logout should NOT match
	if matcher(testRow([]string{"scrappy", "logout"})) {
		t.Errorf("Expected 'scrappy+logout' to NOT match '(scrappy OR bob) AND login'")
	}

	// alice + login should NOT match
	if matcher(testRow([]string{"alice", "login"})) {
		t.Errorf("Expected 'alice+login' to NOT match '(scrappy OR bob) AND login'")
	}
}

func TestBooleanFilterNOTWithOR(t *testing.T) {
	qe := NewQueryExecutor(nil, nil, DefaultCacheConfig())
	header := []string{"username", "action"}

	// Test: scrappy OR NOT bob
	// Should match if contains "scrappy" OR does not contain "bob"
	matcher := qe.buildMatcherFromQuery("scrappy OR NOT bob", header, "", -1, nil)

	// Row with "scrappy" should match (first condition true)
	if !matcher(testRow([]string{"scrappy", "login"})) {
		t.Errorf("Expected 'scrappy' row to match 'scrappy OR NOT bob'")
	}

	// Row without "bob" should match (second condition true)
	if !matcher(testRow([]string{"alice", "login"})) {
		t.Errorf("Expected row without 'bob' to match 'scrappy OR NOT bob'")
	}

	// Row with "bob" but not "scrappy" should NOT match
	if matcher(testRow([]string{"bob", "login"})) {
		t.Errorf("Expected 'bob' row without 'scrappy' to NOT match 'scrappy OR NOT bob'")
	}
}

func TestBooleanFilterDeepNesting(t *testing.T) {
	qe := NewQueryExecutor(nil, nil, DefaultCacheConfig())
	header := []string{"username", "action", "status"}

	// Test deeply nested: ((NOT scrappy) AND (bob OR admin)) OR error
	matcher := qe.buildMatcherFromQuery("((NOT scrappy) AND (bob OR admin)) OR error", header, "", -1, nil)

	// Row with "error" should match (last OR condition)
	if !matcher(testRow([]string{"scrappy", "login", "error"})) {
		t.Errorf("Expected row with 'error' to match")
	}

	// Row with "bob" but not "scrappy" should match
	if !matcher(testRow([]string{"bob", "login", "success"})) {
		t.Errorf("Expected 'bob' without 'scrappy' to match")
	}

	// Row with "admin" but not "scrappy" should match
	if !matcher(testRow([]string{"admin", "login", "success"})) {
		t.Errorf("Expected 'admin' without 'scrappy' to match")
	}

	// Row with "scrappy" and no error should NOT match
	if matcher(testRow([]string{"scrappy", "login", "success"})) {
		t.Errorf("Expected 'scrappy' without 'error' to NOT match")
	}
}

func TestBooleanFilterWithFieldEquals(t *testing.T) {
	qe := NewQueryExecutor(nil, nil, DefaultCacheConfig())
	header := []string{"username", "action"}

	// Test field=value conditions with OR
	matcher := qe.buildMatcherFromQuery("username=scrappy OR username=bob", header, "", -1, nil)

	// Row with username=scrappy should match
	if !matcher(testRow([]string{"scrappy", "login"})) {
		t.Errorf("Expected username=scrappy to match 'username=scrappy OR username=bob'")
	}

	// Row with username=bob should match
	if !matcher(testRow([]string{"bob", "login"})) {
		t.Errorf("Expected username=bob to match 'username=scrappy OR username=bob'")
	}

	// Row with username=alice should NOT match
	if matcher(testRow([]string{"alice", "login"})) {
		t.Errorf("Expected username=alice to NOT match 'username=scrappy OR username=bob'")
	}
}

func TestBooleanFilterWithQuotedFieldNames(t *testing.T) {
	qe := NewQueryExecutor(nil, nil, DefaultCacheConfig())
	header := []string{"user name", "action"}

	// Test quoted field names with OR - this was a bug where "user name"=value was tokenized incorrectly
	matcher := qe.buildMatcherFromQuery(`"user name"=scrappy OR "user name"=xray`, header, "", -1, nil)

	// Row with user name=scrappy should match
	if !matcher(testRow([]string{"scrappy", "login"})) {
		t.Errorf("Expected 'user name'=scrappy to match")
	}

	// Row with user name=xray should match
	if !matcher(testRow([]string{"xray", "login"})) {
		t.Errorf("Expected 'user name'=xray to match")
	}

	// Row with user name=alice should NOT match
	if matcher(testRow([]string{"alice", "login"})) {
		t.Errorf("Expected 'user name'=alice to NOT match")
	}
}

func TestBooleanFilterWithQuotedFieldNamesAndPrefix(t *testing.T) {
	qe := NewQueryExecutor(nil, nil, DefaultCacheConfig())
	header := []string{"user name", "action"}

	// Test quoted field names with OR and prefix matching
	matcher := qe.buildMatcherFromQuery(`"user name"=scrappy OR "user name"=xray*`, header, "", -1, nil)

	// Row with user name=scrappy should match (exact)
	if !matcher(testRow([]string{"scrappy", "login"})) {
		t.Errorf("Expected 'user name'=scrappy to match")
	}

	// Row with user name=xray123 should match (prefix)
	if !matcher(testRow([]string{"xray123", "login"})) {
		t.Errorf("Expected 'user name'=xray123 to match prefix 'xray*'")
	}

	// Row with user name=alice should NOT match
	if matcher(testRow([]string{"alice", "login"})) {
		t.Errorf("Expected 'user name'=alice to NOT match")
	}
}

func TestBooleanFilterWithFieldNotEquals(t *testing.T) {
	qe := NewQueryExecutor(nil, nil, DefaultCacheConfig())
	header := []string{"username", "action"}

	// Test field!=value with AND
	matcher := qe.buildMatcherFromQuery("username!=scrappy AND action=login", header, "", -1, nil)

	// Row with username!=scrappy AND action=login should match
	if !matcher(testRow([]string{"bob", "login"})) {
		t.Errorf("Expected bob+login to match 'username!=scrappy AND action=login'")
	}

	// Row with username=scrappy should NOT match
	if matcher(testRow([]string{"scrappy", "login"})) {
		t.Errorf("Expected scrappy+login to NOT match 'username!=scrappy AND action=login'")
	}

	// Row with action!=login should NOT match
	if matcher(testRow([]string{"bob", "logout"})) {
		t.Errorf("Expected bob+logout to NOT match 'username!=scrappy AND action=login'")
	}
}

func TestBooleanFilterCaseInsensitive(t *testing.T) {
	qe := NewQueryExecutor(nil, nil, DefaultCacheConfig())
	header := []string{"username", "action"}

	// Test case-insensitive operators
	tests := []struct {
		query    string
		row      *Row
		expected bool
	}{
		{"scrappy or bob", testRow([]string{"scrappy", "login"}), true},
		{"scrappy OR bob", testRow([]string{"bob", "login"}), true},
		{"scrappy Or Bob", testRow([]string{"alice", "login"}), false},
		{"NOT scrappy", testRow([]string{"bob", "login"}), true},
		{"not scrappy", testRow([]string{"scrappy", "login"}), false},
		{"scrappy AND login", testRow([]string{"scrappy", "login"}), true},
		{"scrappy and login", testRow([]string{"scrappy", "logout"}), false},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			matcher := qe.buildMatcherFromQuery(tt.query, header, "", -1, nil)
			result := matcher(tt.row)
			if result != tt.expected {
				t.Errorf("Query %q with row %v: got %v, expected %v", tt.query, tt.row, result, tt.expected)
			}
		})
	}
}

func TestBooleanFilterMixedConditions(t *testing.T) {
	qe := NewQueryExecutor(nil, nil, DefaultCacheConfig())
	header := []string{"username", "action", "status"}

	// Test mixed conditions: username=admin OR (status=error AND NOT action=ignored)
	matcher := qe.buildMatcherFromQuery("username=admin OR (status=error AND NOT action=ignored)", header, "", -1, nil)

	// admin should match
	if !matcher(testRow([]string{"admin", "login", "success"})) {
		t.Errorf("Expected admin to match")
	}

	// error status with non-ignored action should match
	if !matcher(testRow([]string{"bob", "login", "error"})) {
		t.Errorf("Expected error status with login to match")
	}

	// error status with ignored action should NOT match
	if matcher(testRow([]string{"bob", "ignored", "error"})) {
		t.Errorf("Expected error status with ignored action to NOT match")
	}

	// non-admin with success status should NOT match
	if matcher(testRow([]string{"bob", "login", "success"})) {
		t.Errorf("Expected non-admin with success to NOT match")
	}
}

func TestContainsOperator(t *testing.T) {
	qe := NewQueryExecutor(nil, nil, DefaultCacheConfig())
	header := []string{"username", "action", "status"}

	tests := []struct {
		name     string
		query    string
		row      *Row
		expected bool
	}{
		// Basic contains operator
		{"contains match", "username~scrap", testRow([]string{"scrappy", "login", "success"}), true},
		{"contains no match", "username~admin", testRow([]string{"scrappy", "login", "success"}), false},
		{"contains case insensitive", "username~SCRAP", testRow([]string{"scrappy", "login", "success"}), true},
		{"contains partial", "action~log", testRow([]string{"scrappy", "login", "success"}), true},
		{"contains full word", "status~success", testRow([]string{"scrappy", "login", "success"}), true},

		// Quoted field names
		{"quoted field contains", "\"username\"~scrap", testRow([]string{"scrappy", "login", "success"}), true},

		// Not contains operator
		{"not contains match", "username!~admin", testRow([]string{"scrappy", "login", "success"}), true},
		{"not contains no match", "username!~scrap", testRow([]string{"scrappy", "login", "success"}), false},
		{"not contains case insensitive", "username!~ADMIN", testRow([]string{"scrappy", "login", "success"}), true},

		// Field not found
		{"contains field not found", "nonexistent~value", testRow([]string{"scrappy", "login", "success"}), false},
		{"not contains field not found", "nonexistent!~value", testRow([]string{"scrappy", "login", "success"}), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := qe.buildMatcherFromQuery(tt.query, header, "", -1, nil)
			result := matcher(tt.row)
			if result != tt.expected {
				t.Errorf("Query %q with row %v: got %v, expected %v", tt.query, tt.row, result, tt.expected)
			}
		})
	}
}

func TestContainsOperatorWithBooleanExpressions(t *testing.T) {
	qe := NewQueryExecutor(nil, nil, DefaultCacheConfig())
	header := []string{"username", "action", "status"}

	tests := []struct {
		name     string
		query    string
		row      *Row
		expected bool
	}{
		// Contains with AND
		{"contains AND equals", "username~scrap AND status=success", testRow([]string{"scrappy", "login", "success"}), true},
		{"contains AND equals no match", "username~scrap AND status=error", testRow([]string{"scrappy", "login", "success"}), false},

		// Contains with OR
		{"contains OR equals", "username~admin OR status=success", testRow([]string{"scrappy", "login", "success"}), true},
		{"contains OR contains", "username~scrap OR action~out", testRow([]string{"scrappy", "login", "success"}), true},

		// Not contains with AND
		{"not contains AND equals", "username!~admin AND status=success", testRow([]string{"scrappy", "login", "success"}), true},

		// Complex expressions
		{"complex expression", "(username~scrap OR username~bob) AND status=success", testRow([]string{"scrappy", "login", "success"}), true},
		{"complex expression no match", "(username~admin OR username~bob) AND status=success", testRow([]string{"scrappy", "login", "success"}), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := qe.buildMatcherFromQuery(tt.query, header, "", -1, nil)
			result := matcher(tt.row)
			if result != tt.expected {
				t.Errorf("Query %q with row %v: got %v, expected %v", tt.query, tt.row, result, tt.expected)
			}
		})
	}
}

// TestJPathFilterParsing tests the JPath field condition parsing function
func TestJPathFilterParsing(t *testing.T) {
	tests := []struct {
		name         string
		fieldName    string
		expectColumn string
		expectJPath  string
		expectHas    bool
	}{
		{"simple field", "username", "username", "", false},
		{"basic jpath", "requestParameters{$.durationSeconds}", "requestParameters", "$.durationSeconds", true},
		{"nested jpath", "data{$.user.profile.name}", "data", "$.user.profile.name", true},
		{"array index jpath", "items{$[0].id}", "items", "$[0].id", true},
		{"empty braces", "field{}", "field{}", "", false},
		{"no closing brace", "field{$.key", "field{$.key", "", false},
		{"no opening brace", "field$.key}", "field$.key}", "", false},
		{"quoted field with jpath", `"request params"{$.key}`, `"request params"`, "$.key", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			col, jpath, hasJPath := parseJPathFieldCondition(tt.fieldName)
			if col != tt.expectColumn {
				t.Errorf("parseJPathFieldCondition(%q): column = %q, want %q", tt.fieldName, col, tt.expectColumn)
			}
			if jpath != tt.expectJPath {
				t.Errorf("parseJPathFieldCondition(%q): jpath = %q, want %q", tt.fieldName, jpath, tt.expectJPath)
			}
			if hasJPath != tt.expectHas {
				t.Errorf("parseJPathFieldCondition(%q): hasJPath = %v, want %v", tt.fieldName, hasJPath, tt.expectHas)
			}
		})
	}
}

// TestJPathEvaluation tests the JPath evaluation function
func TestJPathEvaluation(t *testing.T) {
	tests := []struct {
		name        string
		json        string
		jpath       string
		expectValue string
		expectOk    bool
	}{
		{"simple string", `{"name":"Alice"}`, "$.name", "Alice", true},
		{"simple integer", `{"count":42}`, "$.count", "42", true},
		{"simple float", `{"price":19.99}`, "$.price", "19.99", true},
		{"boolean true", `{"active":true}`, "$.active", "true", true},
		{"boolean false", `{"active":false}`, "$.active", "false", true},
		{"nested value", `{"user":{"profile":{"name":"Bob"}}}`, "$.user.profile.name", "Bob", true},
		{"missing key", `{"name":"Alice"}`, "$.age", "", false},
		{"invalid json", `not json`, "$.name", "", false},
		{"empty json", `{}`, "$.name", "", false},
		{"null value", `{"name":null}`, "$.name", "", true},
		{"integer as float64", `{"durationSeconds":3600}`, "$.durationSeconds", "3600", true},
		// Note: JSON object key order is not guaranteed, so we just check it contains expected keys
		{"object value", `{"config":{"a":1,"b":2}}`, "$.config", "", true}, // Don't compare object string directly
		{"array value", `{"tags":["x","y"]}`, "$.tags", `["x","y"]`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, ok := evaluateJPath(tt.json, tt.jpath)
			if ok != tt.expectOk {
				t.Errorf("evaluateJPath(%q, %q): ok = %v, want %v", tt.json, tt.jpath, ok, tt.expectOk)
			}
			// Skip value comparison when expectValue is empty (for object tests where ordering is non-deterministic)
			if ok && tt.expectValue != "" && value != tt.expectValue {
				t.Errorf("evaluateJPath(%q, %q): value = %q, want %q", tt.json, tt.jpath, value, tt.expectValue)
			}
		})
	}
}

// TestJPathFilterEquals tests JPath filtering with equals operator
func TestJPathFilterEquals(t *testing.T) {
	qe := NewQueryExecutor(nil, nil, DefaultCacheConfig())
	header := []string{"requestparameters", "action"}

	tests := []struct {
		name     string
		query    string
		row      *Row
		expected bool
	}{
		// Basic JPath equals
		{
			"jpath equals match",
			"requestParameters{$.durationSeconds}=3600",
			testRow([]string{`{"durationSeconds":3600,"roleArn":"arn:aws:iam::123"}`, "AssumeRole"}),
			true,
		},
		{
			"jpath equals no match",
			"requestParameters{$.durationSeconds}=7200",
			testRow([]string{`{"durationSeconds":3600,"roleArn":"arn:aws:iam::123"}`, "AssumeRole"}),
			false,
		},
		// Nested JPath
		{
			"nested jpath equals",
			"requestParameters{$.user.name}=admin",
			testRow([]string{`{"user":{"name":"admin","role":"superuser"}}`, "Login"}),
			true,
		},
		// String value
		{
			"jpath string equals",
			"requestParameters{$.roleArn}=arn:aws:iam::123",
			testRow([]string{`{"durationSeconds":3600,"roleArn":"arn:aws:iam::123"}`, "AssumeRole"}),
			true,
		},
		// Case insensitive matching
		{
			"jpath case insensitive",
			"requestParameters{$.status}=SUCCESS",
			testRow([]string{`{"status":"success","code":200}`, "GetItem"}),
			true,
		},
		// Prefix matching with *
		{
			"jpath prefix match",
			"requestParameters{$.roleArn}=arn:aws:iam*",
			testRow([]string{`{"roleArn":"arn:aws:iam::123456:role/MyRole"}`, "AssumeRole"}),
			true,
		},
		// Invalid JSON (should not match)
		{
			"invalid json no match",
			"requestParameters{$.key}=value",
			testRow([]string{`not valid json`, "Action"}),
			false,
		},
		// Missing key (should not match)
		{
			"missing key no match",
			"requestParameters{$.missing}=value",
			testRow([]string{`{"present":"value"}`, "Action"}),
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := qe.buildMatcherFromQuery(tt.query, header, "", -1, nil)
			result := matcher(tt.row)
			if result != tt.expected {
				t.Errorf("Query %q with row %v: got %v, expected %v", tt.query, tt.row, result, tt.expected)
			}
		})
	}
}

// TestJPathFilterNotEquals tests JPath filtering with not equals operator
func TestJPathFilterNotEquals(t *testing.T) {
	qe := NewQueryExecutor(nil, nil, DefaultCacheConfig())
	header := []string{"requestparameters", "action"}

	tests := []struct {
		name     string
		query    string
		row      *Row
		expected bool
	}{
		{
			"jpath not equals match",
			"requestParameters{$.durationSeconds}!=7200",
			testRow([]string{`{"durationSeconds":3600}`, "AssumeRole"}),
			true,
		},
		{
			"jpath not equals no match",
			"requestParameters{$.durationSeconds}!=3600",
			testRow([]string{`{"durationSeconds":3600}`, "AssumeRole"}),
			false,
		},
		// Invalid JSON returns true for != (nothing to compare)
		{
			"invalid json returns true for !=",
			"requestParameters{$.key}!=value",
			testRow([]string{`not json`, "Action"}),
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := qe.buildMatcherFromQuery(tt.query, header, "", -1, nil)
			result := matcher(tt.row)
			if result != tt.expected {
				t.Errorf("Query %q with row %v: got %v, expected %v", tt.query, tt.row, result, tt.expected)
			}
		})
	}
}

// TestJPathFilterContains tests JPath filtering with contains operator
func TestJPathFilterContains(t *testing.T) {
	qe := NewQueryExecutor(nil, nil, DefaultCacheConfig())
	header := []string{"requestparameters", "action"}

	tests := []struct {
		name     string
		query    string
		row      *Row
		expected bool
	}{
		{
			"jpath contains match",
			"requestParameters{$.roleArn}~admin",
			testRow([]string{`{"roleArn":"arn:aws:iam::123:role/admin-role"}`, "AssumeRole"}),
			true,
		},
		{
			"jpath contains no match",
			"requestParameters{$.roleArn}~superuser",
			testRow([]string{`{"roleArn":"arn:aws:iam::123:role/admin-role"}`, "AssumeRole"}),
			false,
		},
		{
			"jpath contains case insensitive",
			"requestParameters{$.roleArn}~ADMIN",
			testRow([]string{`{"roleArn":"arn:aws:iam::123:role/admin-role"}`, "AssumeRole"}),
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := qe.buildMatcherFromQuery(tt.query, header, "", -1, nil)
			result := matcher(tt.row)
			if result != tt.expected {
				t.Errorf("Query %q with row %v: got %v, expected %v", tt.query, tt.row, result, tt.expected)
			}
		})
	}
}

// TestJPathFilterNotContains tests JPath filtering with not contains operator
func TestJPathFilterNotContains(t *testing.T) {
	qe := NewQueryExecutor(nil, nil, DefaultCacheConfig())
	header := []string{"requestparameters", "action"}

	tests := []struct {
		name     string
		query    string
		row      *Row
		expected bool
	}{
		{
			"jpath not contains match",
			"requestParameters{$.roleArn}!~superuser",
			testRow([]string{`{"roleArn":"arn:aws:iam::123:role/admin-role"}`, "AssumeRole"}),
			true,
		},
		{
			"jpath not contains no match",
			"requestParameters{$.roleArn}!~admin",
			testRow([]string{`{"roleArn":"arn:aws:iam::123:role/admin-role"}`, "AssumeRole"}),
			false,
		},
		// Invalid JSON returns true for !~ (nothing contains anything)
		{
			"invalid json returns true for !~",
			"requestParameters{$.key}!~value",
			testRow([]string{`not json`, "Action"}),
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := qe.buildMatcherFromQuery(tt.query, header, "", -1, nil)
			result := matcher(tt.row)
			if result != tt.expected {
				t.Errorf("Query %q with row %v: got %v, expected %v", tt.query, tt.row, result, tt.expected)
			}
		})
	}
}

// TestJPathFilterWithBooleanOperators tests JPath filtering with AND, OR, NOT
func TestJPathFilterWithBooleanOperators(t *testing.T) {
	qe := NewQueryExecutor(nil, nil, DefaultCacheConfig())
	header := []string{"requestparameters", "action"}

	tests := []struct {
		name     string
		query    string
		row      *Row
		expected bool
	}{
		// JPath with AND
		{
			"jpath AND regular field",
			"requestParameters{$.durationSeconds}=3600 AND action=AssumeRole",
			testRow([]string{`{"durationSeconds":3600}`, "AssumeRole"}),
			true,
		},
		{
			"jpath AND jpath",
			"requestParameters{$.durationSeconds}=3600 AND requestParameters{$.roleSessionName}=xray-daemon",
			testRow([]string{`{"durationSeconds":3600,"roleSessionName":"xray-daemon"}`, "AssumeRole"}),
			true,
		},
		// JPath with OR
		{
			"jpath OR jpath",
			"requestParameters{$.durationSeconds}=7200 OR requestParameters{$.durationSeconds}=3600",
			testRow([]string{`{"durationSeconds":3600}`, "AssumeRole"}),
			true,
		},
		// JPath with NOT
		{
			"NOT jpath",
			"NOT requestParameters{$.durationSeconds}=7200",
			testRow([]string{`{"durationSeconds":3600}`, "AssumeRole"}),
			true,
		},
		// Complex expression
		{
			"complex jpath expression",
			"(requestParameters{$.durationSeconds}=3600 OR requestParameters{$.durationSeconds}=7200) AND action=AssumeRole",
			testRow([]string{`{"durationSeconds":3600}`, "AssumeRole"}),
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := qe.buildMatcherFromQuery(tt.query, header, "", -1, nil)
			result := matcher(tt.row)
			if result != tt.expected {
				t.Errorf("Query %q with row %v: got %v, expected %v", tt.query, tt.row, result, tt.expected)
			}
		})
	}
}

// TestJPathSortStage tests sorting with JPath expressions
func TestJPathSortStage(t *testing.T) {
	// Create test data with JSON column
	rows := []*Row{
		{Data: []string{`{"count":30}`, "row1"}, HasTime: false},
		{Data: []string{`{"count":10}`, "row2"}, HasTime: false},
		{Data: []string{`{"count":20}`, "row3"}, HasTime: false},
		{Data: []string{`{"count":5}`, "row4"}, HasTime: false},
	}

	input := &StageResult{
		OriginalHeader: []string{"data", "name"},
		Header:         []string{"data", "name"},
		Rows:           rows,
	}

	tests := []struct {
		name          string
		columnNames   []string
		descending    []bool
		expectedOrder []string // Expected order of "name" column values
	}{
		{
			"sort by jpath ascending",
			[]string{"data{$.count}"},
			[]bool{false},
			[]string{"row4", "row2", "row3", "row1"}, // 5, 10, 20, 30
		},
		{
			"sort by jpath descending",
			[]string{"data{$.count}"},
			[]bool{true},
			[]string{"row1", "row3", "row2", "row4"}, // 30, 20, 10, 5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy of input rows to avoid modifying original
			rowsCopy := make([]*Row, len(rows))
			for i, row := range rows {
				rowsCopy[i] = &Row{Data: row.Data, HasTime: row.HasTime, Timestamp: row.Timestamp}
			}
			inputCopy := &StageResult{
				OriginalHeader: input.OriginalHeader,
				Header:         input.Header,
				Rows:           rowsCopy,
			}

			stage := NewSortStage(tt.columnNames, tt.descending)
			result, err := stage.Execute(inputCopy)
			if err != nil {
				t.Fatalf("SortStage.Execute() error = %v", err)
			}

			// Check order
			for i, expectedName := range tt.expectedOrder {
				if i >= len(result.Rows) {
					t.Errorf("Expected row %d with name %q, but only got %d rows", i, expectedName, len(result.Rows))
					continue
				}
				actualName := result.Rows[i].Data[1] // name column
				if actualName != expectedName {
					t.Errorf("Row %d: expected name %q, got %q", i, expectedName, actualName)
				}
			}
		})
	}
}

// TestJPathSortStageNestedPath tests sorting with nested JPath expressions
func TestJPathSortStageNestedPath(t *testing.T) {
	rows := []*Row{
		{Data: []string{`{"user":{"score":75}}`, "alice"}, HasTime: false},
		{Data: []string{`{"user":{"score":50}}`, "bob"}, HasTime: false},
		{Data: []string{`{"user":{"score":100}}`, "charlie"}, HasTime: false},
	}

	input := &StageResult{
		OriginalHeader: []string{"data", "name"},
		Header:         []string{"data", "name"},
		Rows:           rows,
	}

	stage := NewSortStage([]string{"data{$.user.score}"}, []bool{false})
	result, err := stage.Execute(input)
	if err != nil {
		t.Fatalf("SortStage.Execute() error = %v", err)
	}

	expectedOrder := []string{"bob", "alice", "charlie"} // 50, 75, 100
	for i, expectedName := range expectedOrder {
		if result.Rows[i].Data[1] != expectedName {
			t.Errorf("Row %d: expected name %q, got %q", i, expectedName, result.Rows[i].Data[1])
		}
	}
}

// TestJPathDedupStage tests deduplication with JPath expressions
func TestJPathDedupStage(t *testing.T) {
	// Create test data with JSON column containing duplicate values for JPath expression
	rows := []*Row{
		{Data: []string{`{"type":"admin"}`, "user1"}, HasTime: false},
		{Data: []string{`{"type":"user"}`, "user2"}, HasTime: false},
		{Data: []string{`{"type":"admin"}`, "user3"}, HasTime: false}, // Duplicate type
		{Data: []string{`{"type":"guest"}`, "user4"}, HasTime: false},
		{Data: []string{`{"type":"user"}`, "user5"}, HasTime: false}, // Duplicate type
	}

	input := &StageResult{
		OriginalHeader: []string{"data", "name"},
		Header:         []string{"data", "name"},
		Rows:           rows,
	}

	stage := NewDedupStage([]string{"data{$.type}"})
	result, err := stage.Execute(input)
	if err != nil {
		t.Fatalf("DedupStage.Execute() error = %v", err)
	}

	// Should only have 3 unique types: admin, user, guest
	if len(result.Rows) != 3 {
		t.Errorf("Expected 3 unique rows, got %d", len(result.Rows))
	}

	// First occurrence of each type should be kept: user1 (admin), user2 (user), user4 (guest)
	expectedNames := []string{"user1", "user2", "user4"}
	for i, expectedName := range expectedNames {
		if i >= len(result.Rows) {
			t.Errorf("Expected row %d with name %q, but only got %d rows", i, expectedName, len(result.Rows))
			continue
		}
		actualName := result.Rows[i].Data[1]
		if actualName != expectedName {
			t.Errorf("Row %d: expected name %q, got %q", i, expectedName, actualName)
		}
	}
}

// TestJPathDedupStageNestedPath tests deduplication with nested JPath expressions
func TestJPathDedupStageNestedPath(t *testing.T) {
	rows := []*Row{
		{Data: []string{`{"user":{"role":"admin"}}`, "alice"}, HasTime: false},
		{Data: []string{`{"user":{"role":"developer"}}`, "bob"}, HasTime: false},
		{Data: []string{`{"user":{"role":"admin"}}`, "charlie"}, HasTime: false}, // Duplicate role
	}

	input := &StageResult{
		OriginalHeader: []string{"data", "name"},
		Header:         []string{"data", "name"},
		Rows:           rows,
	}

	stage := NewDedupStage([]string{"data{$.user.role}"})
	result, err := stage.Execute(input)
	if err != nil {
		t.Fatalf("DedupStage.Execute() error = %v", err)
	}

	// Should only have 2 unique roles: admin, developer
	if len(result.Rows) != 2 {
		t.Errorf("Expected 2 unique rows, got %d", len(result.Rows))
	}

	expectedNames := []string{"alice", "bob"}
	for i, expectedName := range expectedNames {
		if result.Rows[i].Data[1] != expectedName {
			t.Errorf("Row %d: expected name %q, got %q", i, expectedName, result.Rows[i].Data[1])
		}
	}
}

// TestJPathDedupAfterColumnsStage tests deduplication with JPath after a columns operation
// This simulates the pipeline: columns requestParameters | dedup requestParameters{$.roleSessionName}
func TestJPathDedupAfterColumnsStage(t *testing.T) {
	// Simulate data with multiple columns, where requestParameters is at index 2
	rows := []*Row{
		{Data: []string{"2024-01-01", "AssumeRole", `{"roleSessionName":"session-a","durationSeconds":3600}`}, HasTime: false},
		{Data: []string{"2024-01-02", "AssumeRole", `{"roleSessionName":"session-b","durationSeconds":3600}`}, HasTime: false},
		{Data: []string{"2024-01-03", "AssumeRole", `{"roleSessionName":"session-a","durationSeconds":7200}`}, HasTime: false}, // Duplicate roleSessionName
		{Data: []string{"2024-01-04", "AssumeRole", `{"roleSessionName":"session-c","durationSeconds":3600}`}, HasTime: false},
		{Data: []string{"2024-01-05", "AssumeRole", `{"roleSessionName":"session-b","durationSeconds":7200}`}, HasTime: false}, // Duplicate roleSessionName
	}

	// Simulate the output after "columns requestParameters"
	// Header only shows requestParameters, but rows still have all data
	// DisplayColumns maps display index 0 -> original index 2
	input := &StageResult{
		OriginalHeader: []string{"eventTime", "eventName", "requestParameters"},
		Header:         []string{"requestParameters"}, // After columns operation
		DisplayColumns: []int{2},                      // requestParameters is at original index 2
		Rows:           rows,
	}

	stage := NewDedupStage([]string{"requestParameters{$.roleSessionName}"})
	result, err := stage.Execute(input)
	if err != nil {
		t.Fatalf("DedupStage.Execute() error = %v", err)
	}

	// Should only have 3 unique roleSessionName values: session-a, session-b, session-c
	if len(result.Rows) != 3 {
		t.Errorf("Expected 3 unique rows, got %d", len(result.Rows))
		for i, row := range result.Rows {
			t.Logf("Row %d: %v", i, row.Data)
		}
	}

	// First occurrences should be kept: rows 0, 1, 3 (with session-a, session-b, session-c)
	expectedEventTimes := []string{"2024-01-01", "2024-01-02", "2024-01-04"}
	for i, expectedTime := range expectedEventTimes {
		if i >= len(result.Rows) {
			continue
		}
		actualTime := result.Rows[i].Data[0] // eventTime is at index 0
		if actualTime != expectedTime {
			t.Errorf("Row %d: expected eventTime %q, got %q", i, expectedTime, actualTime)
		}
	}
}

// TestJPathSortStageInvalidJSON tests sorting when JSON is invalid
func TestJPathSortStageInvalidJSON(t *testing.T) {
	rows := []*Row{
		{Data: []string{`{"count":30}`, "row1"}, HasTime: false},
		{Data: []string{`not json`, "row2"}, HasTime: false}, // Invalid JSON
		{Data: []string{`{"count":10}`, "row3"}, HasTime: false},
		{Data: []string{`{"missing":5}`, "row4"}, HasTime: false}, // Missing key
	}

	input := &StageResult{
		OriginalHeader: []string{"data", "name"},
		Header:         []string{"data", "name"},
		Rows:           rows,
	}

	stage := NewSortStage([]string{"data{$.count}"}, []bool{false})
	result, err := stage.Execute(input)
	if err != nil {
		t.Fatalf("SortStage.Execute() error = %v", err)
	}

	// Rows with valid JPath should be sorted, invalid ones go to end
	// Valid: row3 (10), row1 (30)
	// Invalid (empty): row2, row4
	if len(result.Rows) != 4 {
		t.Errorf("Expected 4 rows, got %d", len(result.Rows))
	}

	// First two rows should be the valid ones sorted by count
	if result.Rows[0].Data[1] != "row3" {
		t.Errorf("Row 0: expected 'row3' (count=10), got %q", result.Rows[0].Data[1])
	}
	if result.Rows[1].Data[1] != "row1" {
		t.Errorf("Row 1: expected 'row1' (count=30), got %q", result.Rows[1].Data[1])
	}
}

// TestParseColumnJPath tests the column JPath parsing function
func TestParseColumnJPath(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectColumn string
		expectJPath  string
		expectHas    bool
	}{
		{"simple column", "username", "username", "", false},
		{"basic jpath", "data{$.count}", "data", "$.count", true},
		{"nested jpath", "col{$.a.b.c}", "col", "$.a.b.c", true},
		{"array jpath", "col{$[0].id}", "col", "$[0].id", true},
		{"empty braces", "col{}", "col{}", "", false},
		{"no closing brace", "col{$.key", "col{$.key", "", false},
		{"spaces in column", "  data  {$.key}", "data", "$.key", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			col, jpath, hasJPath := parseColumnJPath(tt.input)
			if col != tt.expectColumn {
				t.Errorf("parseColumnJPath(%q): column = %q, want %q", tt.input, col, tt.expectColumn)
			}
			if jpath != tt.expectJPath {
				t.Errorf("parseColumnJPath(%q): jpath = %q, want %q", tt.input, jpath, tt.expectJPath)
			}
			if hasJPath != tt.expectHas {
				t.Errorf("parseColumnJPath(%q): hasJPath = %v, want %v", tt.input, hasJPath, tt.expectHas)
			}
		})
	}
}

// TestEvaluateColumnJPath tests the column JPath evaluation function
func TestEvaluateColumnJPath(t *testing.T) {
	tests := []struct {
		name        string
		json        string
		jpath       string
		expectValue string
		expectOk    bool
	}{
		{"simple string", `{"name":"test"}`, "$.name", "test", true},
		{"simple integer", `{"count":42}`, "$.count", "42", true},
		{"nested path", `{"a":{"b":{"c":"deep"}}}`, "$.a.b.c", "deep", true},
		{"missing key", `{"a":1}`, "$.b", "", false},
		{"invalid json", `not json`, "$.key", "", false},
		{"empty input", "", "$.key", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, ok := evaluateColumnJPath(tt.json, tt.jpath)
			if ok != tt.expectOk {
				t.Errorf("evaluateColumnJPath(%q, %q): ok = %v, want %v", tt.json, tt.jpath, ok, tt.expectOk)
			}
			if ok && value != tt.expectValue {
				t.Errorf("evaluateColumnJPath(%q, %q): value = %q, want %q", tt.json, tt.jpath, value, tt.expectValue)
			}
		})
	}
}
