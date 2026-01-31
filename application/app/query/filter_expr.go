package query

import (
	"strings"
)

// TokenType represents the type of a token in the filter expression
type TokenType int

const (
	TokenLiteral TokenType = iota // A value or condition (e.g., "scrappy", "username=admin")
	TokenAND                      // AND operator
	TokenOR                       // OR operator
	TokenNOT                      // NOT operator
	TokenLParen                   // Left parenthesis (
	TokenRParen                   // Right parenthesis )
	TokenEOF                      // End of expression
)

// Token represents a token in the filter expression
type Token struct {
	Type  TokenType
	Value string
}

// ExprNode represents a node in the filter expression AST
type ExprNode interface {
	Eval(row []string, evalCondition func(string, []string) bool) bool
}

// LiteralNode represents a literal condition (e.g., "scrappy", "username=admin")
type LiteralNode struct {
	Value string
}

func (n *LiteralNode) Eval(row []string, evalCondition func(string, []string) bool) bool {
	return evalCondition(n.Value, row)
}

// NotNode represents a NOT expression
type NotNode struct {
	Child ExprNode
}

func (n *NotNode) Eval(row []string, evalCondition func(string, []string) bool) bool {
	return !n.Child.Eval(row, evalCondition)
}

// AndNode represents an AND expression
type AndNode struct {
	Left  ExprNode
	Right ExprNode
}

func (n *AndNode) Eval(row []string, evalCondition func(string, []string) bool) bool {
	return n.Left.Eval(row, evalCondition) && n.Right.Eval(row, evalCondition)
}

// OrNode represents an OR expression
type OrNode struct {
	Left  ExprNode
	Right ExprNode
}

func (n *OrNode) Eval(row []string, evalCondition func(string, []string) bool) bool {
	return n.Left.Eval(row, evalCondition) || n.Right.Eval(row, evalCondition)
}

// FilterExprTokenizer tokenizes a filter expression
type FilterExprTokenizer struct {
	input    string
	pos      int
	tokens   []Token
	tokenPos int
}

// NewFilterExprTokenizer creates a new tokenizer for a filter expression
func NewFilterExprTokenizer(input string) *FilterExprTokenizer {
	t := &FilterExprTokenizer{
		input: input,
		pos:   0,
	}
	t.tokenize()
	return t
}

// tokenize splits the input into tokens
func (t *FilterExprTokenizer) tokenize() {
	t.tokens = nil
	t.pos = 0

	for t.pos < len(t.input) {
		// Skip whitespace
		if t.isWhitespace(t.input[t.pos]) {
			t.pos++
			continue
		}

		// Check for parentheses
		if t.input[t.pos] == '(' {
			t.tokens = append(t.tokens, Token{Type: TokenLParen, Value: "("})
			t.pos++
			continue
		}
		if t.input[t.pos] == ')' {
			t.tokens = append(t.tokens, Token{Type: TokenRParen, Value: ")"})
			t.pos++
			continue
		}

		// Read a word (up to whitespace, parenthesis, or end)
		// This handles both plain words and conditions with quoted field names like "user name"=value
		start := t.pos
		for t.pos < len(t.input) && !t.isWhitespace(t.input[t.pos]) && t.input[t.pos] != '(' && t.input[t.pos] != ')' {
			// Handle quoted field names in conditions like "event name"=value
			if t.input[t.pos] == '"' || t.input[t.pos] == '\'' {
				quote := t.input[t.pos]
				t.pos++
				for t.pos < len(t.input) && t.input[t.pos] != quote {
					t.pos++
				}
				if t.pos < len(t.input) {
					t.pos++ // consume closing quote
				}
				continue
			}
			t.pos++
		}

		word := t.input[start:t.pos]
		wordUpper := strings.ToUpper(word)

		switch wordUpper {
		case "AND":
			t.tokens = append(t.tokens, Token{Type: TokenAND, Value: word})
		case "OR":
			t.tokens = append(t.tokens, Token{Type: TokenOR, Value: word})
		case "NOT":
			t.tokens = append(t.tokens, Token{Type: TokenNOT, Value: word})
		default:
			if word != "" {
				t.tokens = append(t.tokens, Token{Type: TokenLiteral, Value: word})
			}
		}
	}

	t.tokens = append(t.tokens, Token{Type: TokenEOF, Value: ""})
}

func (t *FilterExprTokenizer) isWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// Peek returns the current token without consuming it
func (t *FilterExprTokenizer) Peek() Token {
	if t.tokenPos >= len(t.tokens) {
		return Token{Type: TokenEOF, Value: ""}
	}
	return t.tokens[t.tokenPos]
}

// Next returns the current token and advances to the next
func (t *FilterExprTokenizer) Next() Token {
	tok := t.Peek()
	t.tokenPos++
	return tok
}

// HasBooleanOperators checks if the expression contains any boolean operators
func (t *FilterExprTokenizer) HasBooleanOperators() bool {
	for _, tok := range t.tokens {
		switch tok.Type {
		case TokenAND, TokenOR, TokenNOT, TokenLParen, TokenRParen:
			return true
		}
	}
	return false
}

// FilterExprParser parses a filter expression into an AST
type FilterExprParser struct {
	tokenizer *FilterExprTokenizer
}

// NewFilterExprParser creates a new parser
func NewFilterExprParser(input string) *FilterExprParser {
	return &FilterExprParser{
		tokenizer: NewFilterExprTokenizer(input),
	}
}

// Parse parses the filter expression and returns the root AST node
// Returns nil if the expression is empty or contains no boolean operators (simple condition)
func (p *FilterExprParser) Parse() (ExprNode, bool) {
	// Check if there are any boolean operators
	if !p.tokenizer.HasBooleanOperators() {
		return nil, false
	}

	// Reset tokenizer position
	p.tokenizer.tokenPos = 0

	return p.parseOr(), true
}

// parseOr parses OR expressions (lowest precedence)
func (p *FilterExprParser) parseOr() ExprNode {
	left := p.parseAnd()

	for p.tokenizer.Peek().Type == TokenOR {
		p.tokenizer.Next() // consume OR
		right := p.parseAnd()
		left = &OrNode{Left: left, Right: right}
	}

	return left
}

// parseAnd parses AND expressions (medium precedence)
func (p *FilterExprParser) parseAnd() ExprNode {
	left := p.parseNot()

	for p.tokenizer.Peek().Type == TokenAND {
		p.tokenizer.Next() // consume AND
		right := p.parseNot()
		left = &AndNode{Left: left, Right: right}
	}

	return left
}

// parseNot parses NOT expressions (high precedence)
func (p *FilterExprParser) parseNot() ExprNode {
	if p.tokenizer.Peek().Type == TokenNOT {
		p.tokenizer.Next() // consume NOT
		child := p.parseNot()
		return &NotNode{Child: child}
	}

	return p.parsePrimary()
}

// parsePrimary parses primary expressions (parenthesized expressions or literals)
func (p *FilterExprParser) parsePrimary() ExprNode {
	tok := p.tokenizer.Peek()

	if tok.Type == TokenLParen {
		p.tokenizer.Next() // consume (
		node := p.parseOr()
		if p.tokenizer.Peek().Type == TokenRParen {
			p.tokenizer.Next() // consume )
		}
		return node
	}

	if tok.Type == TokenLiteral {
		p.tokenizer.Next() // consume literal
		return &LiteralNode{Value: tok.Value}
	}

	// Handle unexpected tokens - return empty literal
	return &LiteralNode{Value: ""}
}

// ContainsBooleanOperators checks if a query string contains boolean operators
// This is used to decide whether to use the simple matcher or the expression parser
func ContainsBooleanOperators(query string) bool {
	tokenizer := NewFilterExprTokenizer(query)
	return tokenizer.HasBooleanOperators()
}

// ParseFilterExpression parses a filter expression and returns an AST
func ParseFilterExpression(query string) (ExprNode, bool) {
	parser := NewFilterExprParser(query)
	return parser.Parse()
}
