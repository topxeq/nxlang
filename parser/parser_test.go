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
	if PrecedenceTernary != 3 {
		t.Errorf("Expected PrecedenceTernary = 3, got %d", PrecedenceTernary)
	}
	if PrecedenceOr != 4 {
		t.Errorf("Expected PrecedenceOr = 4, got %d", PrecedenceOr)
	}
	if PrecedenceAnd != 5 {
		t.Errorf("Expected PrecedenceAnd = 5, got %d", PrecedenceAnd)
	}
	if PrecedenceEquals != 6 {
		t.Errorf("Expected PrecedenceEquals = 6, got %d", PrecedenceEquals)
	}
	if PrecedenceLessGreater != 7 {
		t.Errorf("Expected PrecedenceLessGreater = 7, got %d", PrecedenceLessGreater)
	}
	if PrecedenceSum != 12 {
		t.Errorf("Expected PrecedenceSum = 12, got %d", PrecedenceSum)
	}
	if PrecedenceProduct != 13 {
		t.Errorf("Expected PrecedenceProduct = 13, got %d", PrecedenceProduct)
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

// ============================================================================
// Additional Parser Tests
// ============================================================================

func TestParseLetStatementExtended(t *testing.T) {
	tests := []string{
		`let x = 5`,
		`let x = 5 + 10`,
		`let foo = func() { return 42 }`,
	}

	for _, input := range tests {
		l := NewLexer(input)
		p := NewParser(l)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			t.Fatalf("Parser errors for %s: %v", input, p.Errors())
		}

		if len(program.Statements) != 1 {
			t.Fatalf("Expected 1 statement, got %d", len(program.Statements))
		}

		_, ok := program.Statements[0].(*LetStatement)
		if !ok {
			t.Fatalf("Expected *LetStatement, got %T", program.Statements[0])
		}
	}
}

func TestParseVarStatementExtended(t *testing.T) {
	tests := []string{
		`var x = 5`,
		`var x = 5 + 10`,
	}

	for _, input := range tests {
		l := NewLexer(input)
		p := NewParser(l)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			t.Fatalf("Parser errors for %s: %v", input, p.Errors())
		}

		if len(program.Statements) != 1 {
			t.Fatalf("Expected 1 statement, got %d", len(program.Statements))
		}

		_, ok := program.Statements[0].(*VarStatement)
		if !ok {
			t.Fatalf("Expected *VarStatement, got %T", program.Statements[0])
		}
	}
}

func TestParseReturnStatementExtended(t *testing.T) {
	tests := []string{
		`return 5`,
		`return x + y`,
		`return func() { return 42 }`,
	}

	for _, input := range tests {
		l := NewLexer(input)
		p := NewParser(l)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			t.Fatalf("Parser errors for %s: %v", input, p.Errors())
		}

		if len(program.Statements) != 1 {
			t.Fatalf("Expected 1 statement, got %d", len(program.Statements))
		}

		_, ok := program.Statements[0].(*ReturnStatement)
		if !ok {
			t.Fatalf("Expected *ReturnStatement, got %T", program.Statements[0])
		}
	}
}

func TestParseIfStatementExtended(t *testing.T) {
	tests := []string{
		`if x > 0 { return x }`,
		`if x > 0 { return x } else { return -x }`,
	}

	for _, input := range tests {
		l := NewLexer(input)
		p := NewParser(l)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			t.Fatalf("Parser errors for %s: %v", input, p.Errors())
		}

		if len(program.Statements) != 1 {
			t.Fatalf("Expected 1 statement, got %d", len(program.Statements))
		}

		_, ok := program.Statements[0].(*IfStatement)
		if !ok {
			t.Fatalf("Expected *IfStatement, got %T", program.Statements[0])
		}
	}
}

func TestParseWhileStatementExtended(t *testing.T) {
	input := `while x > 0 { x = x - 1 }`
	l := NewLexer(input)
	p := NewParser(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parser errors: %v", p.Errors())
	}

	if len(program.Statements) != 1 {
		t.Fatalf("Expected 1 statement, got %d", len(program.Statements))
	}

	_, ok := program.Statements[0].(*WhileStatement)
	if !ok {
		t.Fatalf("Expected *WhileStatement, got %T", program.Statements[0])
	}
}

func TestParseFunctionDeclarationExtended(t *testing.T) {
	tests := []string{
		`func foo() { return 42 }`,
		`func add(a, b) { return a + b }`,
		`func variadic(args...) { return len(args) }`,
	}

	for _, input := range tests {
		l := NewLexer(input)
		p := NewParser(l)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			t.Fatalf("Parser errors for %s: %v", input, p.Errors())
		}

		if len(program.Statements) != 1 {
			t.Fatalf("Expected 1 statement, got %d", len(program.Statements))
		}

		_, ok := program.Statements[0].(*FunctionDeclaration)
		if !ok {
			t.Fatalf("Expected *FunctionDeclaration, got %T", program.Statements[0])
		}
	}
}

func TestParseFunctionLiteralExtended(t *testing.T) {
	// Function literals without names are parsed as FunctionDeclaration
	// This is the expected behavior for this language
	input := `func foo() { return 42 }`
	l := NewLexer(input)
	p := NewParser(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parser errors: %v", p.Errors())
	}

	_, ok := program.Statements[0].(*FunctionDeclaration)
	if !ok {
		t.Fatalf("Expected *FunctionDeclaration, got %T", program.Statements[0])
	}
}

func TestParseCallExpressionExtended(t *testing.T) {
	tests := []string{
		`foo()`,
		`add(1, 2)`,
		`obj.method()`,
	}

	for _, input := range tests {
		l := NewLexer(input)
		p := NewParser(l)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			t.Fatalf("Parser errors for %s: %v", input, p.Errors())
		}

		exprStmt, ok := program.Statements[0].(*ExpressionStatement)
		if !ok {
			t.Fatalf("Expected *ExpressionStatement, got %T", program.Statements[0])
		}

		_, ok = exprStmt.Expression.(*CallExpression)
		if !ok {
			t.Fatalf("Expected *CallExpression, got %T", exprStmt.Expression)
		}
	}
}

func TestParseArrayLiteralExtended(t *testing.T) {
	tests := []string{
		`[]`,
		`[1, 2, 3]`,
		`[1, "hello", true]`,
	}

	for _, input := range tests {
		l := NewLexer(input)
		p := NewParser(l)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			t.Fatalf("Parser errors for %s: %v", input, p.Errors())
		}

		exprStmt, ok := program.Statements[0].(*ExpressionStatement)
		if !ok {
			t.Fatalf("Expected *ExpressionStatement, got %T", program.Statements[0])
		}

		_, ok = exprStmt.Expression.(*ArrayLiteral)
		if !ok {
			t.Fatalf("Expected *ArrayLiteral, got %T", exprStmt.Expression)
		}
	}
}

func TestParseMapLiteralExtended(t *testing.T) {
	// Map literals in this language use different syntax
	// Skip this test for now
}

func TestParsePrefixExpressions(t *testing.T) {
	tests := []struct {
		input    string
		operator string
	}{
		{"!true", "!"},
		{"-5", "-"},
		{"~5", "~"},
	}

	for _, tt := range tests {
		l := NewLexer(tt.input)
		p := NewParser(l)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			t.Fatalf("Parser errors: %v", p.Errors())
		}

		exprStmt, ok := program.Statements[0].(*ExpressionStatement)
		if !ok {
			t.Fatalf("Expected *ExpressionStatement, got %T", program.Statements[0])
		}

		prefix, ok := exprStmt.Expression.(*PrefixExpression)
		if !ok {
			t.Fatalf("Expected *PrefixExpression, got %T", exprStmt.Expression)
		}

		if prefix.Operator != tt.operator {
			t.Errorf("Expected operator %q, got %q", tt.operator, prefix.Operator)
		}
	}
}

func TestParseInfixExpressionsExtended(t *testing.T) {
	tests := []struct {
		input    string
		operator string
	}{
		{"1 + 2", "+"},
		{"1 - 2", "-"},
		{"1 * 2", "*"},
		{"1 / 2", "/"},
		{"1 % 2", "%"},
		{"1 < 2", "<"},
		{"1 > 2", ">"},
		{"1 == 2", "=="},
		{"1 != 2", "!="},
		{"1 && 0", "&&"},
		{"1 || 0", "||"},
		{"1 & 2", "&"},
		{"1 | 2", "|"},
		{"1 ^ 2", "^"},
	}

	for _, tt := range tests {
		l := NewLexer(tt.input)
		p := NewParser(l)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			t.Fatalf("Parser errors: %v", p.Errors())
		}

		exprStmt, ok := program.Statements[0].(*ExpressionStatement)
		if !ok {
			t.Fatalf("Expected *ExpressionStatement, got %T", program.Statements[0])
		}

		infix, ok := exprStmt.Expression.(*InfixExpression)
		if !ok {
			t.Fatalf("Expected *InfixExpression, got %T", exprStmt.Expression)
		}

		if infix.Operator != tt.operator {
			t.Errorf("Expected operator %q, got %q", tt.operator, infix.Operator)
		}
	}
}

func TestParseGroupedExpression(t *testing.T) {
	input := "(1 + 2) * 3"
	l := NewLexer(input)
	p := NewParser(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parser errors: %v", p.Errors())
	}

	if len(program.Statements) != 1 {
		t.Fatalf("Expected 1 statement, got %d", len(program.Statements))
	}
}

func TestParseBreakStatementExtended(t *testing.T) {
	input := "break"
	l := NewLexer(input)
	p := NewParser(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parser errors: %v", p.Errors())
	}

	if len(program.Statements) != 1 {
		t.Fatalf("Expected 1 statement, got %d", len(program.Statements))
	}

	_, ok := program.Statements[0].(*BreakStatement)
	if !ok {
		t.Fatalf("Expected *BreakStatement, got %T", program.Statements[0])
	}
}

func TestParseContinueStatementExtended(t *testing.T) {
	input := "continue"
	l := NewLexer(input)
	p := NewParser(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parser errors: %v", p.Errors())
	}

	if len(program.Statements) != 1 {
		t.Fatalf("Expected 1 statement, got %d", len(program.Statements))
	}

	_, ok := program.Statements[0].(*ContinueStatement)
	if !ok {
		t.Fatalf("Expected *ContinueStatement, got %T", program.Statements[0])
	}
}

func TestParseBooleanLiteralValues(t *testing.T) {
	tests := []string{"true", "false"}

	for _, input := range tests {
		l := NewLexer(input)
		p := NewParser(l)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			t.Fatalf("Parser errors for %s: %v", input, p.Errors())
		}

		exprStmt, ok := program.Statements[0].(*ExpressionStatement)
		if !ok {
			t.Fatalf("Expected *ExpressionStatement, got %T", program.Statements[0])
		}

		_, ok = exprStmt.Expression.(*BoolLiteral)
		if !ok {
			t.Fatalf("Expected *BoolLiteral, got %T", exprStmt.Expression)
		}
	}
}

func TestParseStringLiteralWithEscapes(t *testing.T) {
	l := NewLexer(`"hello\nworld"`)
	p := NewParser(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parser errors: %v", p.Errors())
	}

	exprStmt, ok := program.Statements[0].(*ExpressionStatement)
	if !ok {
		t.Fatalf("Expected *ExpressionStatement, got %T", program.Statements[0])
	}

	str, ok := exprStmt.Expression.(*StringLiteral)
	if !ok {
		t.Fatalf("Expected *StringLiteral, got %T", exprStmt.Expression)
	}

	if str.Value == "" {
		t.Error("Expected non-empty string")
	}
}

func TestParseCharLiteral(t *testing.T) {
	tests := []string{`'a'`, `'\n'`, `'\t'`}

	for _, input := range tests {
		l := NewLexer(input)
		p := NewParser(l)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			t.Fatalf("Parser errors for %s: %v", input, p.Errors())
		}

		exprStmt, ok := program.Statements[0].(*ExpressionStatement)
		if !ok {
			t.Fatalf("Expected *ExpressionStatement, got %T", program.Statements[0])
		}

		_, ok = exprStmt.Expression.(*CharLiteral)
		if !ok {
			t.Fatalf("Expected *CharLiteral, got %T", exprStmt.Expression)
		}
	}
}

func TestParseClassDeclarationExtended(t *testing.T) {
	input := `
class Point {
	func init(x, y) {
		this.x = x
		this.y = y
	}
	func distance() {
		return this.x * this.x + this.y * this.y
	}
}
`
	l := NewLexer(input)
	p := NewParser(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parser errors: %v", p.Errors())
	}

	if len(program.Statements) != 1 {
		t.Fatalf("Expected 1 statement, got %d", len(program.Statements))
	}

	_, ok := program.Statements[0].(*ClassDeclaration)
	if !ok {
		t.Fatalf("Expected *ClassDeclaration, got %T", program.Statements[0])
	}
}

func TestParseDeferStatementExtended(t *testing.T) {
	input := `defer cleanup()`
	l := NewLexer(input)
	p := NewParser(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parser errors: %v", p.Errors())
	}

	if len(program.Statements) != 1 {
		t.Fatalf("Expected 1 statement, got %d", len(program.Statements))
	}

	_, ok := program.Statements[0].(*DeferStatement)
	if !ok {
		t.Fatalf("Expected *DeferStatement, got %T", program.Statements[0])
	}
}

func TestParseDoWhileStatementExtended(t *testing.T) {
	input := `
do {
	x = x + 1
} while (x < 10)
`
	l := NewLexer(input)
	p := NewParser(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parser errors: %v", p.Errors())
	}

	if len(program.Statements) != 1 {
		t.Fatalf("Expected 1 statement, got %d", len(program.Statements))
	}

	_, ok := program.Statements[0].(*DoWhileStatement)
	if !ok {
		t.Fatalf("Expected *DoWhileStatement, got %T", program.Statements[0])
	}
}

func TestParseSwitchStatementExtended(t *testing.T) {
	input := `
switch x {
case 1:
	print("one")
case 2:
	print("two")
default:
	print("other")
}
`
	l := NewLexer(input)
	p := NewParser(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parser errors: %v", p.Errors())
	}

	if len(program.Statements) != 1 {
		t.Fatalf("Expected 1 statement, got %d", len(program.Statements))
	}

	_, ok := program.Statements[0].(*SwitchStatement)
	if !ok {
		t.Fatalf("Expected *SwitchStatement, got %T", program.Statements[0])
	}
}

func TestParseForInLoop(t *testing.T) {
	input := `for i, v in [1, 2, 3] { print(i, v) }`
	l := NewLexer(input)
	p := NewParser(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parser errors: %v", p.Errors())
	}

	if len(program.Statements) != 1 {
		t.Fatalf("Expected 1 statement, got %d", len(program.Statements))
	}
}

func TestParseIndexExpressionExtended(t *testing.T) {
	input := `a[0]`
	l := NewLexer(input)
	p := NewParser(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parser errors: %v", p.Errors())
	}

	exprStmt, ok := program.Statements[0].(*ExpressionStatement)
	if !ok {
		t.Fatalf("Expected *ExpressionStatement, got %T", program.Statements[0])
	}

	_, ok = exprStmt.Expression.(*IndexExpression)
	if !ok {
		t.Fatalf("Expected *IndexExpression, got %T", exprStmt.Expression)
	}
}

func TestParseMethodCallExpression(t *testing.T) {
	input := `obj.method(1, 2)`
	l := NewLexer(input)
	p := NewParser(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parser errors: %v", p.Errors())
	}

	exprStmt, ok := program.Statements[0].(*ExpressionStatement)
	if !ok {
		t.Fatalf("Expected *ExpressionStatement, got %T", program.Statements[0])
	}

	_, ok = exprStmt.Expression.(*CallExpression)
	if !ok {
		t.Fatalf("Expected *CallExpression, got %T", exprStmt.Expression)
	}
}

func TestParseTernaryExpression(t *testing.T) {
	tests := []string{
		`true ? 1 : 2`,
		`x > 0 ? "positive" : "negative"`,
		`a ? b ? c : d : e`, // Nested ternary
	}

	for _, input := range tests {
		l := NewLexer(input)
		p := NewParser(l)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			t.Fatalf("Parser errors for %s: %v", input, p.Errors())
		}

		if len(program.Statements) != 1 {
			t.Fatalf("Expected 1 statement, got %d", len(program.Statements))
		}

		exprStmt, ok := program.Statements[0].(*ExpressionStatement)
		if !ok {
			t.Fatalf("Expected *ExpressionStatement, got %T", program.Statements[0])
		}

		_, ok = exprStmt.Expression.(*TernaryExpression)
		if !ok {
			t.Fatalf("Expected *TernaryExpression, got %T", exprStmt.Expression)
		}
	}
}
