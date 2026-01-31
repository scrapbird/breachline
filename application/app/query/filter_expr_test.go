package query

import (
	"testing"
)

// TestTokenizer tests the tokenization of filter expressions
func TestTokenizer(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Token
	}{
		{
			name:  "simple word",
			input: "scrappy",
			expected: []Token{
				{Type: TokenLiteral, Value: "scrappy"},
				{Type: TokenEOF, Value: ""},
			},
		},
		{
			name:  "quoted field with OR",
			input: `"user name"=scrappy OR "user name"=xray*`,
			expected: []Token{
				{Type: TokenLiteral, Value: `"user name"=scrappy`},
				{Type: TokenOR, Value: "OR"},
				{Type: TokenLiteral, Value: `"user name"=xray*`},
				{Type: TokenEOF, Value: ""},
			},
		},
		{
			name:  "NOT operator",
			input: "NOT scrappy",
			expected: []Token{
				{Type: TokenNOT, Value: "NOT"},
				{Type: TokenLiteral, Value: "scrappy"},
				{Type: TokenEOF, Value: ""},
			},
		},
		{
			name:  "OR operator",
			input: "scrappy OR bob",
			expected: []Token{
				{Type: TokenLiteral, Value: "scrappy"},
				{Type: TokenOR, Value: "OR"},
				{Type: TokenLiteral, Value: "bob"},
				{Type: TokenEOF, Value: ""},
			},
		},
		{
			name:  "AND operator",
			input: "scrappy AND bob",
			expected: []Token{
				{Type: TokenLiteral, Value: "scrappy"},
				{Type: TokenAND, Value: "AND"},
				{Type: TokenLiteral, Value: "bob"},
				{Type: TokenEOF, Value: ""},
			},
		},
		{
			name:  "parentheses",
			input: "(scrappy OR bob)",
			expected: []Token{
				{Type: TokenLParen, Value: "("},
				{Type: TokenLiteral, Value: "scrappy"},
				{Type: TokenOR, Value: "OR"},
				{Type: TokenLiteral, Value: "bob"},
				{Type: TokenRParen, Value: ")"},
				{Type: TokenEOF, Value: ""},
			},
		},
		{
			name:  "quoted string",
			input: `"hello world"`,
			expected: []Token{
				{Type: TokenLiteral, Value: `"hello world"`},
				{Type: TokenEOF, Value: ""},
			},
		},
		{
			name:  "field=value condition",
			input: "username=scrappy",
			expected: []Token{
				{Type: TokenLiteral, Value: "username=scrappy"},
				{Type: TokenEOF, Value: ""},
			},
		},
		{
			name:  "complex expression",
			input: "(NOT scrappy OR bob) AND admin",
			expected: []Token{
				{Type: TokenLParen, Value: "("},
				{Type: TokenNOT, Value: "NOT"},
				{Type: TokenLiteral, Value: "scrappy"},
				{Type: TokenOR, Value: "OR"},
				{Type: TokenLiteral, Value: "bob"},
				{Type: TokenRParen, Value: ")"},
				{Type: TokenAND, Value: "AND"},
				{Type: TokenLiteral, Value: "admin"},
				{Type: TokenEOF, Value: ""},
			},
		},
		{
			name:  "case insensitive operators",
			input: "scrappy or bob and not admin",
			expected: []Token{
				{Type: TokenLiteral, Value: "scrappy"},
				{Type: TokenOR, Value: "or"},
				{Type: TokenLiteral, Value: "bob"},
				{Type: TokenAND, Value: "and"},
				{Type: TokenNOT, Value: "not"},
				{Type: TokenLiteral, Value: "admin"},
				{Type: TokenEOF, Value: ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := NewFilterExprTokenizer(tt.input)
			for i, expected := range tt.expected {
				tok := tokenizer.Next()
				if tok.Type != expected.Type {
					t.Errorf("token %d: expected type %v, got %v", i, expected.Type, tok.Type)
				}
				if tok.Value != expected.Value {
					t.Errorf("token %d: expected value %q, got %q", i, expected.Value, tok.Value)
				}
			}
		})
	}
}

// TestContainsBooleanOperators tests detection of boolean operators
func TestContainsBooleanOperators(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"scrappy", false},
		{"username=scrappy", false},
		{"NOT scrappy", true},
		{"scrappy OR bob", true},
		{"scrappy AND bob", true},
		{"(scrappy)", true},
		{"scrappy or bob", true},
		{"scrappy and bob", true},
		{"not scrappy", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ContainsBooleanOperators(tt.input)
			if result != tt.expected {
				t.Errorf("ContainsBooleanOperators(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestParseFilterExpression tests parsing of filter expressions
func TestParseFilterExpression(t *testing.T) {
	// Simple evaluator for testing
	evalCondition := func(condition string, row []string) bool {
		for _, field := range row {
			if field == condition {
				return true
			}
		}
		return false
	}

	tests := []struct {
		name     string
		expr     string
		row      []string
		expected bool
	}{
		{
			name:     "simple NOT",
			expr:     "NOT scrappy",
			row:      []string{"bob", "admin"},
			expected: true,
		},
		{
			name:     "simple NOT - negative",
			expr:     "NOT scrappy",
			row:      []string{"scrappy", "admin"},
			expected: false,
		},
		{
			name:     "simple OR - first matches",
			expr:     "scrappy OR bob",
			row:      []string{"scrappy", "admin"},
			expected: true,
		},
		{
			name:     "simple OR - second matches",
			expr:     "scrappy OR bob",
			row:      []string{"bob", "admin"},
			expected: true,
		},
		{
			name:     "simple OR - neither matches",
			expr:     "scrappy OR bob",
			row:      []string{"admin", "user"},
			expected: false,
		},
		{
			name:     "simple AND - both match",
			expr:     "scrappy AND admin",
			row:      []string{"scrappy", "admin"},
			expected: true,
		},
		{
			name:     "simple AND - only first matches",
			expr:     "scrappy AND admin",
			row:      []string{"scrappy", "user"},
			expected: false,
		},
		{
			name:     "simple AND - only second matches",
			expr:     "scrappy AND admin",
			row:      []string{"bob", "admin"},
			expected: false,
		},
		{
			name:     "NOT with OR",
			expr:     "NOT scrappy OR bob",
			row:      []string{"bob", "admin"},
			expected: true, // NOT scrappy is true (no scrappy), OR bob is true
		},
		{
			name:     "NOT with OR - only NOT matches",
			expr:     "NOT scrappy OR bob",
			row:      []string{"admin", "user"},
			expected: true, // NOT scrappy is true
		},
		{
			name:     "NOT with OR - only bob matches",
			expr:     "NOT scrappy OR bob",
			row:      []string{"scrappy", "bob"},
			expected: true, // NOT scrappy is false, but bob is true
		},
		{
			name:     "NOT with OR - neither",
			expr:     "NOT scrappy OR bob",
			row:      []string{"scrappy", "admin"},
			expected: false, // NOT scrappy is false, bob is false
		},
		{
			name:     "parentheses with OR",
			expr:     "(scrappy OR bob)",
			row:      []string{"scrappy", "admin"},
			expected: true,
		},
		{
			name:     "complex - (NOT scrappy OR bob) AND admin",
			expr:     "(NOT scrappy OR bob) AND admin",
			row:      []string{"bob", "admin"},
			expected: true, // (NOT scrappy=true OR bob=true) AND admin=true = true
		},
		{
			name:     "complex - (NOT scrappy OR bob) AND admin - fails",
			expr:     "(NOT scrappy OR bob) AND admin",
			row:      []string{"bob", "user"},
			expected: false, // (NOT scrappy=true OR bob=true) AND admin=false = false
		},
		{
			name:     "deeply nested",
			expr:     "((NOT scrappy) AND (bob OR admin))",
			row:      []string{"bob", "admin"},
			expected: true, // (NOT scrappy=true) AND (bob=true OR admin=true) = true
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, ok := ParseFilterExpression(tt.expr)
			if !ok {
				t.Fatalf("failed to parse expression: %q", tt.expr)
			}
			if ast == nil {
				t.Fatalf("parsed AST is nil for expression: %q", tt.expr)
			}
			result := ast.Eval(tt.row, evalCondition)
			if result != tt.expected {
				t.Errorf("Eval(%q, %v) = %v, expected %v", tt.expr, tt.row, result, tt.expected)
			}
		})
	}
}

// TestPrecedence tests operator precedence (NOT > AND > OR)
func TestPrecedence(t *testing.T) {
	// Simple evaluator for testing
	evalCondition := func(condition string, row []string) bool {
		for _, field := range row {
			if field == condition {
				return true
			}
		}
		return false
	}

	tests := []struct {
		name     string
		expr     string
		row      []string
		expected bool
		desc     string
	}{
		{
			name:     "AND before OR - left",
			expr:     "a AND b OR c",
			row:      []string{"c"},
			expected: true,
			desc:     "(a AND b) OR c = false OR true = true",
		},
		{
			name:     "AND before OR - right",
			expr:     "a OR b AND c",
			row:      []string{"a"},
			expected: true,
			desc:     "a OR (b AND c) = true OR false = true",
		},
		{
			name:     "NOT before AND",
			expr:     "NOT a AND b",
			row:      []string{"b"},
			expected: true,
			desc:     "(NOT a) AND b = true AND true = true",
		},
		{
			name:     "NOT before AND - with a present",
			expr:     "NOT a AND b",
			row:      []string{"a", "b"},
			expected: false,
			desc:     "(NOT a) AND b = false AND true = false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, ok := ParseFilterExpression(tt.expr)
			if !ok {
				t.Fatalf("failed to parse expression: %q", tt.expr)
			}
			result := ast.Eval(tt.row, evalCondition)
			if result != tt.expected {
				t.Errorf("%s: Eval(%q, %v) = %v, expected %v", tt.desc, tt.expr, tt.row, result, tt.expected)
			}
		})
	}
}
