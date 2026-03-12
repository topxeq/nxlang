package parser

import (
	"testing"
)

func TestLexerBasic(t *testing.T) {
	l := NewLexer("let x = 5")

	tok := l.NextToken()
	if tok.Type != TokenLet {
		t.Errorf("Expected TokenLet, got %v", tok.Type)
	}

	tok = l.NextToken()
	if tok.Type != TokenIdentifier || tok.Literal != "x" {
		t.Errorf("Expected identifier 'x', got %v %q", tok.Type, tok.Literal)
	}

	tok = l.NextToken()
	if tok.Type != TokenAssign {
		t.Errorf("Expected TokenAssign, got %v", tok.Type)
	}

	tok = l.NextToken()
	if tok.Type != TokenInt || tok.Literal != "5" {
		t.Errorf("Expected int '5', got %v %q", tok.Type, tok.Literal)
	}
}

func TestLexerIntegers(t *testing.T) {
	l := NewLexer("42")
	tok := l.NextToken()
	if tok.Type != TokenInt {
		t.Errorf("Expected TokenInt, got %v", tok.Type)
	}
	if tok.Literal != "42" {
		t.Errorf("Expected '42', got %q", tok.Literal)
	}
}

func TestLexerFloats(t *testing.T) {
	l := NewLexer("3.14")
	tok := l.NextToken()
	if tok.Type != TokenFloat {
		t.Errorf("Expected TokenFloat, got %v", tok.Type)
	}
	if tok.Literal != "3.14" {
		t.Errorf("Expected '3.14', got %q", tok.Literal)
	}
}

func TestLexerStrings(t *testing.T) {
	l := NewLexer(`"hello"`)
	tok := l.NextToken()
	if tok.Type != TokenString {
		t.Errorf("Expected TokenString, got %v", tok.Type)
	}
	if tok.Literal != "hello" {
		t.Errorf("Expected 'hello', got %q", tok.Literal)
	}
}

func TestLexerChars(t *testing.T) {
	l := NewLexer("'A'")
	tok := l.NextToken()
	if tok.Type != TokenChar {
		t.Errorf("Expected TokenChar, got %v", tok.Type)
	}
}

func TestLexerOperators(t *testing.T) {
	l := NewLexer("+ - * / %")

	tests := []TokenType{
		TokenPlus, TokenMinus, TokenAsterisk, TokenSlash, TokenPercent,
	}

	for _, expected := range tests {
		tok := l.NextToken()
		if tok.Type != expected {
			t.Errorf("Expected %v, got %v", expected, tok.Type)
		}
	}
}

func TestLexerComparison(t *testing.T) {
	l := NewLexer("== != < <= > >=")

	tests := []TokenType{
		TokenEqual, TokenNotEqual, TokenLT, TokenLTE, TokenGT, TokenGTE,
	}

	for _, expected := range tests {
		tok := l.NextToken()
		if tok.Type != expected {
			t.Errorf("Expected %v, got %v", expected, tok.Type)
		}
	}
}

func TestLexerLogical(t *testing.T) {
	l := NewLexer("&& || !")

	tests := []TokenType{
		TokenAnd, TokenOr, TokenBang,
	}

	for _, expected := range tests {
		tok := l.NextToken()
		if tok.Type != expected {
			t.Errorf("Expected %v, got %v", expected, tok.Type)
		}
	}
}

func TestLexerBitwise(t *testing.T) {
	l := NewLexer("& | ^ ~ << >>")

	tests := []TokenType{
		TokenAmpersand, TokenPipe, TokenCaret, TokenTilde, TokenLeftShift, TokenRightShift,
	}

	for _, expected := range tests {
		tok := l.NextToken()
		if tok.Type != expected {
			t.Errorf("Expected %v, got %v", expected, tok.Type)
		}
	}
}

func TestLexerKeywords(t *testing.T) {
	l := NewLexer("func let var if else for while return switch case default break continue")

	tests := []TokenType{
		TokenFunc, TokenLet, TokenVar, TokenIf, TokenElse, TokenFor, TokenWhile, TokenReturn, TokenSwitch, TokenCase, TokenDefault, TokenBreak, TokenContinue,
	}

	for _, expected := range tests {
		tok := l.NextToken()
		if tok.Type != expected {
			t.Errorf("Expected %v, got %v", expected, tok.Type)
		}
	}
}

func TestLexerIdentifiers(t *testing.T) {
	l := NewLexer("foo bar baz")

	idents := []string{"foo", "bar", "baz"}

	for _, expected := range idents {
		tok := l.NextToken()
		if tok.Type != TokenIdentifier {
			t.Errorf("Expected TokenIdentifier, got %v", tok.Type)
		}
		if tok.Literal != expected {
			t.Errorf("Expected %q, got %q", expected, tok.Literal)
		}
	}
}

func TestLexerEOF(t *testing.T) {
	l := NewLexer("x")
	l.NextToken()        // x
	tok := l.NextToken() // EOF

	if tok.Type != TokenEOF {
		t.Errorf("Expected TokenEOF, got %v", tok.Type)
	}
}

func TestLexerErrors(t *testing.T) {
	l := NewLexer("x @ y")
	l.NextToken()        // x
	tok := l.NextToken() // @

	if tok.Type != TokenError {
		t.Errorf("Expected TokenError for invalid char @, got %v", tok.Type)
	}
}

func TestTokenTypes(t *testing.T) {
	tests := []TokenType{
		TokenEOF,
		TokenError,
		TokenIdentifier,
		TokenInt,
		TokenFloat,
		TokenString,
		TokenLet,
		TokenFunc,
		TokenIf,
		TokenElse,
		TokenFor,
		TokenWhile,
		TokenReturn,
		TokenTrue,
		TokenFalse,
	}

	for _, tt := range tests {
		name := tt.String()
		if name == "" {
			t.Errorf("Empty string for TokenType %v", tt)
		}
	}
}

func TestPrecedenceConstants(t *testing.T) {
	if PrecedenceLowest != 1 {
		t.Errorf("Expected PrecedenceLowest = 1, got %d", PrecedenceLowest)
	}
	if PrecedenceAssignment != 2 {
		t.Errorf("Expected PrecedenceAssignment = 2, got %d", PrecedenceAssignment)
	}
	if PrecedenceOr != 3 {
		t.Errorf("Expected PrecedenceOr = 3, got %d", PrecedenceOr)
	}
	if PrecedenceAnd != 4 {
		t.Errorf("Expected PrecedenceAnd = 4, got %d", PrecedenceAnd)
	}
	if PrecedenceEquals != 5 {
		t.Errorf("Expected PrecedenceEquals = 5, got %d", PrecedenceEquals)
	}
	if PrecedenceLessGreater != 6 {
		t.Errorf("Expected PrecedenceLessGreater = 6, got %d", PrecedenceLessGreater)
	}
	if PrecedenceSum != 11 {
		t.Errorf("Expected PrecedenceSum = 11, got %d", PrecedenceSum)
	}
	if PrecedenceProduct != 12 {
		t.Errorf("Expected PrecedenceProduct = 12, got %d", PrecedenceProduct)
	}
}

func TestPrecedences(t *testing.T) {
	tests := []struct {
		tt         TokenType
		precedence int
	}{
		{TokenAssign, PrecedenceAssignment},
		{TokenDefine, PrecedenceAssignment},
		{TokenOr, PrecedenceOr},
		{TokenAnd, PrecedenceAnd},
		{TokenEqual, PrecedenceEquals},
		{TokenNotEqual, PrecedenceEquals},
		{TokenLT, PrecedenceLessGreater},
		{TokenGT, PrecedenceLessGreater},
		{TokenPlus, PrecedenceSum},
		{TokenMinus, PrecedenceSum},
		{TokenAsterisk, PrecedenceProduct},
		{TokenSlash, PrecedenceProduct},
	}

	for _, tt := range tests {
		prec, ok := precedences[tt.tt]
		if !ok {
			t.Errorf("Expected precedence for %v, got none", tt.tt)
		}
		if prec != tt.precedence {
			t.Errorf("Expected precedence %d for %v, got %d", tt.precedence, tt.tt, prec)
		}
	}
}
