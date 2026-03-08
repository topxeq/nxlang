package parser

import (
	"fmt"
	"strconv"

	"github.com/topxeq/nxlang/types"
)

// Operator precedence levels
const (
	_ int = iota
	PrecedenceLowest
	PrecedenceAssignment  // =, +=, -=, *=, /=, %=, &=, |=, ^=, <<=, >>=
	PrecedenceOr          // ||
	PrecedenceAnd         // &&
	PrecedenceEquals      // == !=
	PrecedenceLessGreater // < > <= >=
	PrecedenceBitwiseOr   // |
	PrecedenceBitwiseXor  // ^
	PrecedenceBitwiseAnd  // &
	PrecedenceShift       // << >>
	PrecedenceSum         // + -
	PrecedenceProduct     // * / %
	PrecedencePrefix      // -x !x ~x
	PrecedenceCall        // function()
	PrecedenceIndex       // array[index]
	PrecedenceMember      // obj.member
	PrecedencePostfix     // i++ i--
)

var precedences = map[TokenType]int{
	// Assignment operators
	TokenAssign:      PrecedenceAssignment,
	TokenDefine:      PrecedenceAssignment,
	TokenPlusAssign:  PrecedenceAssignment,
	TokenMinusAssign: PrecedenceAssignment,
	TokenMulAssign:   PrecedenceAssignment,
	TokenDivAssign:   PrecedenceAssignment,
	TokenModAssign:   PrecedenceAssignment,
	TokenAndAssign:   PrecedenceAssignment,
	TokenOrAssign:    PrecedenceAssignment,
	TokenXorAssign:   PrecedenceAssignment,
	TokenLShiftAssign: PrecedenceAssignment,
	TokenRShiftAssign: PrecedenceAssignment,

	// Other operators
	TokenOr:         PrecedenceOr,
	TokenAnd:        PrecedenceAnd,
	TokenEqual:      PrecedenceEquals,
	TokenNotEqual:   PrecedenceEquals,
	TokenLT:         PrecedenceLessGreater,
	TokenLTE:        PrecedenceLessGreater,
	TokenGT:         PrecedenceLessGreater,
	TokenGTE:        PrecedenceLessGreater,
	TokenPipe:       PrecedenceBitwiseOr,
	TokenCaret:      PrecedenceBitwiseXor,
	TokenAmpersand:  PrecedenceBitwiseAnd,
	TokenLeftShift:  PrecedenceShift,
	TokenRightShift: PrecedenceShift,
	TokenPlus:       PrecedenceSum,
	TokenMinus:      PrecedenceSum,
	TokenAsterisk:   PrecedenceProduct,
	TokenSlash:      PrecedenceProduct,
	TokenPercent:    PrecedenceProduct,
	TokenLeftParen:  PrecedenceCall,
	TokenLeftBracket: PrecedenceIndex,
	TokenDot:        PrecedenceMember,
	TokenInc:        PrecedencePostfix,
	TokenDec:        PrecedencePostfix,
}

// Parser represents a recursive descent parser
type Parser struct {
	lexer  *Lexer
	errors []string

	curToken  Token
	peekToken Token

	prefixParseFns map[TokenType]prefixParseFn
	infixParseFns  map[TokenType]infixParseFn
}

type (
	prefixParseFn func() Expression
	infixParseFn  func(Expression) Expression
)

// NewParser creates a new parser for the given input
func NewParser(lexer *Lexer) *Parser {
	p := &Parser{
		lexer:  lexer,
		errors: []string{},
	}

	// Read two tokens to initialize curToken and peekToken
	p.nextToken()
	p.nextToken()

	// Register prefix parse functions
	p.prefixParseFns = make(map[TokenType]prefixParseFn)
	p.registerPrefix(TokenIdentifier, p.parseIdentifier)
	p.registerPrefix(TokenInt, p.parseIntegerLiteral)
	p.registerPrefix(TokenFloat, p.parseFloatLiteral)
	p.registerPrefix(TokenString, p.parseStringLiteral)
	p.registerPrefix(TokenChar, p.parseCharLiteral)
	p.registerPrefix(TokenTrue, p.parseBooleanLiteral)
	p.registerPrefix(TokenFalse, p.parseBooleanLiteral)
	p.registerPrefix(TokenNil, p.parseNullLiteral)
	p.registerPrefix(TokenBang, p.parsePrefixExpression)
	p.registerPrefix(TokenMinus, p.parsePrefixExpression)
	p.registerPrefix(TokenTilde, p.parsePrefixExpression)
	p.registerPrefix(TokenInc, p.parsePrefixExpression)
	p.registerPrefix(TokenDec, p.parsePrefixExpression)
	p.registerPrefix(TokenLeftParen, p.parseGroupedExpression)
	p.registerPrefix(TokenIf, p.parseIfExpression)
	p.registerPrefix(TokenFunc, p.parseFunctionLiteral)
	p.registerPrefix(TokenLeftBracket, p.parseArrayLiteral)
	p.registerPrefix(TokenLeftBrace, p.parseMapLiteral)
	p.registerPrefix(TokenNew, p.parseNewExpression)

	// Register infix parse functions
	p.infixParseFns = make(map[TokenType]infixParseFn)
	p.registerInfix(TokenPlus, p.parseInfixExpression)
	p.registerInfix(TokenMinus, p.parseInfixExpression)
	p.registerInfix(TokenAsterisk, p.parseInfixExpression)
	p.registerInfix(TokenSlash, p.parseInfixExpression)
	p.registerInfix(TokenPercent, p.parseInfixExpression)
	p.registerInfix(TokenEqual, p.parseInfixExpression)
	p.registerInfix(TokenNotEqual, p.parseInfixExpression)
	p.registerInfix(TokenLT, p.parseInfixExpression)
	p.registerInfix(TokenLTE, p.parseInfixExpression)
	p.registerInfix(TokenGT, p.parseInfixExpression)
	p.registerInfix(TokenGTE, p.parseInfixExpression)
	p.registerInfix(TokenAnd, p.parseInfixExpression)
	p.registerInfix(TokenOr, p.parseInfixExpression)
	p.registerInfix(TokenPipe, p.parseInfixExpression)
	p.registerInfix(TokenCaret, p.parseInfixExpression)
	p.registerInfix(TokenAmpersand, p.parseInfixExpression)
	p.registerInfix(TokenLeftShift, p.parseInfixExpression)
	p.registerInfix(TokenRightShift, p.parseInfixExpression)
	p.registerInfix(TokenLeftParen, p.parseCallExpression)
	p.registerInfix(TokenLeftBracket, p.parseIndexExpression)
	p.registerInfix(TokenDot, p.parseMemberExpression)
	p.registerInfix(TokenAssign, p.parseAssignmentExpression)
	p.registerInfix(TokenDefine, p.parseAssignmentExpression)
	p.registerInfix(TokenPlusAssign, p.parseAssignmentExpression)
	p.registerInfix(TokenMinusAssign, p.parseAssignmentExpression)
	p.registerInfix(TokenMulAssign, p.parseAssignmentExpression)
	p.registerInfix(TokenDivAssign, p.parseAssignmentExpression)
	p.registerInfix(TokenModAssign, p.parseAssignmentExpression)
	p.registerInfix(TokenAndAssign, p.parseAssignmentExpression)
	p.registerInfix(TokenOrAssign, p.parseAssignmentExpression)
	p.registerInfix(TokenXorAssign, p.parseAssignmentExpression)
	p.registerInfix(TokenLShiftAssign, p.parseAssignmentExpression)
	p.registerInfix(TokenRShiftAssign, p.parseAssignmentExpression)
	p.registerInfix(TokenInc, p.parsePostfixExpression)
	p.registerInfix(TokenDec, p.parsePostfixExpression)
	p.registerInfix(TokenQuestion, p.parseTernaryExpression)

	return p
}

// nextToken advances to the next token
func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.lexer.NextToken()
}

// Errors returns the list of parsing errors
func (p *Parser) Errors() []string {
	return p.errors
}

// addError adds a parsing error with current position
func (p *Parser) addError(msg string) {
	errorMsg := fmt.Sprintf("%s at line %d, column %d", msg, p.curToken.Line, p.curToken.Column)
	p.errors = append(p.errors, errorMsg)
}

// addPeekError adds a parsing error for the next token
func (p *Parser) addPeekError(expected TokenType) {
	msg := fmt.Sprintf("expected next token to be %s, got %s instead", expected, p.peekToken.Type)
	p.addError(msg)
}

// curTokenIs checks if current token is of the given type
func (p *Parser) curTokenIs(t TokenType) bool {
	return p.curToken.Type == t
}

// peekTokenIs checks if next token is of the given type
func (p *Parser) peekTokenIs(t TokenType) bool {
	return p.peekToken.Type == t
}

// expectPeek advances if next token is expected type, returns false otherwise
func (p *Parser) expectPeek(t TokenType) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	}
	p.addPeekError(t)
	return false
}

// curPrecedence returns the precedence of current token
func (p *Parser) curPrecedence() int {
	if p, ok := precedences[p.curToken.Type]; ok {
		return p
	}
	return PrecedenceLowest
}

// peekPrecedence returns the precedence of next token
func (p *Parser) peekPrecedence() int {
	if p, ok := precedences[p.peekToken.Type]; ok {
		return p
	}
	return PrecedenceLowest
}

// registerPrefix registers a prefix parse function
func (p *Parser) registerPrefix(tokenType TokenType, fn prefixParseFn) {
	p.prefixParseFns[tokenType] = fn
}

// registerInfix registers an infix parse function
func (p *Parser) registerInfix(tokenType TokenType, fn infixParseFn) {
	p.infixParseFns[tokenType] = fn
}

// ParseProgram parses the entire input into a Program AST node
func (p *Parser) ParseProgram() *Program {
	program := &Program{}
	program.Statements = []Statement{}

	for p.curToken.Type != TokenEOF {
		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}
		p.nextToken()
	}

	return program
}

// parseStatement parses a single statement
func (p *Parser) parseStatement() Statement {
	switch p.curToken.Type {
	case TokenVar:
		return p.parseVarStatement()
	case TokenLet:
		return p.parseLetStatement()
	case TokenConst:
		return p.parseConstStatement()
	case TokenReturn:
		return p.parseReturnStatement()
	case TokenBreak:
		return p.parseBreakStatement()
	case TokenContinue:
		return p.parseContinueStatement()
	case TokenFallthrough:
		return p.parseFallthroughStatement()
	case TokenFunc:
		return p.parseFunctionDeclaration()
	case TokenIf:
		return p.parseIfStatement()
	case TokenFor:
		return p.parseForStatement()
	case TokenWhile:
		return p.parseWhileStatement()
	case TokenDo:
		return p.parseDoWhileStatement()
	case TokenSwitch:
		return p.parseSwitchStatement()
	case TokenClass:
		return p.parseClassDeclaration()
	case TokenTry:
		return p.parseTryStatement()
	case TokenDefer:
		return p.parseDeferStatement()
	case TokenThrow:
		return p.parseThrowStatement()
	case TokenImport:
		return p.parseImportStatement()
	case TokenExport:
		return p.parseExportStatement()
	case TokenLeftBrace:
		return p.parseBlockStatement()
	default:
		return p.parseExpressionStatement()
	}
}

// parseVarStatement parses a 'var' declaration statement
func (p *Parser) parseVarStatement() *VarStatement {
	stmt := &VarStatement{Token: p.curToken}

	if !p.expectPeek(TokenIdentifier) {
		return nil
	}

	stmt.Name = &Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if p.peekTokenIs(TokenAssign) {
		p.nextToken()
		p.nextToken()
		stmt.Value = p.parseExpression(PrecedenceLowest)
	}

	if p.peekTokenIs(TokenSemicolon) {
		p.nextToken()
	}

	return stmt
}

// parseLetStatement parses a 'let' declaration statement
func (p *Parser) parseLetStatement() *LetStatement {
	stmt := &LetStatement{Token: p.curToken}

	if !p.expectPeek(TokenIdentifier) {
		return nil
	}

	stmt.Name = &Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if p.peekTokenIs(TokenAssign) {
		p.nextToken()
		p.nextToken()
		stmt.Value = p.parseExpression(PrecedenceLowest)
	}

	if p.peekTokenIs(TokenSemicolon) {
		p.nextToken()
	}

	return stmt
}

// parseDefineStatement parses a ':=' short variable declaration statement
func (p *Parser) parseDefineStatement() *DefineStatement {
	stmt := &DefineStatement{Token: p.curToken}

	if !p.expectPeek(TokenIdentifier) {
		return nil
	}

	stmt.Name = &Identifier{Token: p.curToken, Value: p.curToken.Literal}

	// Skip the := token
	p.nextToken()

	// Parse the value expression
	stmt.Value = p.parseExpression(PrecedenceLowest)

	if p.peekTokenIs(TokenSemicolon) {
		p.nextToken()
	}

	return stmt
}

// parseConstStatement parses a 'const' declaration statement
func (p *Parser) parseConstStatement() *ConstStatement {
	stmt := &ConstStatement{Token: p.curToken}

	if !p.expectPeek(TokenIdentifier) {
		return nil
	}

	stmt.Name = &Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(TokenAssign) {
		return nil
	}

	p.nextToken()
	stmt.Value = p.parseExpression(PrecedenceLowest)

	if p.peekTokenIs(TokenSemicolon) {
		p.nextToken()
	}

	return stmt
}

// parseReturnStatement parses a 'return' statement
func (p *Parser) parseReturnStatement() *ReturnStatement {
	stmt := &ReturnStatement{Token: p.curToken}

	p.nextToken()

	if !p.curTokenIs(TokenSemicolon) {
		stmt.ReturnValue = p.parseExpression(PrecedenceLowest)
	}

	if p.peekTokenIs(TokenSemicolon) {
		p.nextToken()
	}

	return stmt
}

// parseBreakStatement parses a 'break' statement
func (p *Parser) parseBreakStatement() *BreakStatement {
	stmt := &BreakStatement{Token: p.curToken}

	if p.peekTokenIs(TokenSemicolon) {
		p.nextToken()
	}

	return stmt
}

// parseContinueStatement parses a 'continue' statement
func (p *Parser) parseContinueStatement() *ContinueStatement {
	stmt := &ContinueStatement{Token: p.curToken}

	if p.peekTokenIs(TokenSemicolon) {
		p.nextToken()
	}

	return stmt
}

// parseFallthroughStatement parses a 'fallthrough' statement
func (p *Parser) parseFallthroughStatement() *FallthroughStatement {
	stmt := &FallthroughStatement{Token: p.curToken}

	if p.peekTokenIs(TokenSemicolon) {
		p.nextToken()
	}

	return stmt
}

// parseExpressionStatement parses an expression statement
func (p *Parser) parseExpressionStatement() *ExpressionStatement {
	stmt := &ExpressionStatement{Token: p.curToken}

	stmt.Expression = p.parseExpression(PrecedenceLowest)

	if p.peekTokenIs(TokenSemicolon) {
		p.nextToken()
	}

	return stmt
}

// parseBlockStatement parses a block of statements enclosed in braces
func (p *Parser) parseBlockStatement() *BlockStatement {
	block := &BlockStatement{Token: p.curToken}
	block.Statements = []Statement{}

	p.nextToken()

	for !p.curTokenIs(TokenRightBrace) && !p.curTokenIs(TokenEOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken()
	}

	return block
}

// parseIfStatement parses an if statement
func (p *Parser) parseIfStatement() *IfStatement {
	stmt := &IfStatement{Token: p.curToken}

	// Support both styles: if (condition) and if condition
	hasParen := p.peekTokenIs(TokenLeftParen)
	if hasParen {
		p.nextToken() // consume '('
		p.nextToken()
	} else {
		p.nextToken() // move to condition
	}

	stmt.Condition = p.parseExpression(PrecedenceLowest)

	if hasParen {
		if !p.expectPeek(TokenRightParen) {
			return nil
		}
	}

	if !p.expectPeek(TokenLeftBrace) {
		return nil
	}

	stmt.Consequence = p.parseBlockStatement()

	if p.peekTokenIs(TokenElse) {
		p.nextToken()

		if p.peekTokenIs(TokenIf) {
			p.nextToken()
			stmt.Alternative = &BlockStatement{
				Token: p.curToken,
				Statements: []Statement{p.parseIfStatement()},
			}
		} else {
			if !p.expectPeek(TokenLeftBrace) {
				return nil
			}
			stmt.Alternative = p.parseBlockStatement()
		}
	}

	return stmt
}

// parseForStatement parses a for loop statement
func (p *Parser) parseForStatement() *ForStatement {
	stmt := &ForStatement{Token: p.curToken}

	// Check for infinite loop: for { ... }
	if p.peekTokenIs(TokenLeftBrace) {
		p.nextToken() // move to '{'
		stmt.Body = p.parseBlockStatement()
		return stmt
	}

	// Move past 'for' token
	p.nextToken()

	// Check for for...in loop: for (key in obj) { ... } or for (key, value in obj) { ... }
	hasParen := p.peekTokenIs(TokenLeftParen)
	if hasParen {
		p.nextToken() // consume '('
	}

	// Look ahead for "in" keyword to detect for...in

	// Save current state
	savedCur := p.curToken
	savedPeek := p.peekToken
	savedErrors := p.errors

	// Try to parse for...in pattern
	if p.curTokenIs(TokenIdentifier) {
		key := &Identifier{Token: p.curToken, Value: p.curToken.Literal}
		p.nextToken()

		// Check for comma (value present)
		var value *Identifier
		if p.curTokenIs(TokenComma) {
			p.nextToken()
			if p.curTokenIs(TokenIdentifier) {
				value = &Identifier{Token: p.curToken, Value: p.curToken.Literal}
				p.nextToken()
			}
		}

		// Check for "in" keyword
		if p.curTokenIs(TokenIn) {
			p.nextToken()
			// Parse the iterate expression
			iterateExpr := p.parseExpression(PrecedenceLowest)
			if iterateExpr != nil {
				// Check for closing parenthesis
				if !hasParen || p.expectPeek(TokenRightParen) {
					// This is a for...in loop
					stmt.IsForIn = true
					stmt.Key = key
					stmt.Value = value
					stmt.Iterate = iterateExpr
					// Parse body
					if !p.expectPeek(TokenLeftBrace) {
						return nil
					}
					stmt.Body = p.parseBlockStatement()
					return stmt
				}
			}
		}
	}

	// Rollback: not a for...in loop
	p.curToken = savedCur
	p.peekToken = savedPeek
	p.errors = savedErrors

	// Restore hasParen check
	hasParen = p.peekTokenIs(TokenLeftParen)

	// Support both styles: for (init; cond; post) and for init; cond; post
	if hasParen {
		p.nextToken() // consume '('
		p.nextToken() // move inside parenthesis
	} else {
		p.nextToken() // move to first clause
	}

	
	// Check if this is a single condition for loop (for cond { ... } or for (cond) { ... })
	isSingleCondition := false

	// Save state to rollback if needed
	prevCurToken := p.curToken
	prevPeekToken := p.peekToken
	prevErrorCount := len(p.errors)

	// Try to parse as single condition
	expr := p.parseExpression(PrecedenceLowest)
	if expr != nil {
		// Check if after expression we have the expected end of loop header
		if (hasParen && p.curTokenIs(TokenRightParen)) || (!hasParen && p.curTokenIs(TokenLeftBrace)) {
			// It's a single condition loop
			stmt.Condition = expr
			if hasParen {
				p.nextToken() // consume ')'
			}
			isSingleCondition = true
		} else {
			// Rollback: not a single condition, it's a 3-part loop
			p.curToken = prevCurToken
			p.peekToken = prevPeekToken
			p.errors = p.errors[:prevErrorCount]
		}
	} else {
		// Rollback: failed to parse as expression
		p.curToken = prevCurToken
		p.peekToken = prevPeekToken
		p.errors = p.errors[:prevErrorCount]
	}

	if !isSingleCondition {
		// Standard 3-part for loop
		// Parse init clause
		if !p.curTokenIs(TokenSemicolon) {
			initStmt := p.parseStatement()
			// Consume semicolon after init clause
			if p.curTokenIs(TokenSemicolon) {
				// Statement left curToken on semicolon, consume it
				stmt.Init = initStmt
				p.nextToken()
			} else if p.peekTokenIs(TokenSemicolon) {
				// Semicolon is next token, consume it
				stmt.Init = initStmt
				p.nextToken()
			} else {
				// No semicolon: check if this is a single condition loop
				// First check if initStmt is an expression statement
				var condExpr Expression
				if initStmt != nil {
					if exprStmt, ok := initStmt.(*ExpressionStatement); ok {
						condExpr = exprStmt.Expression
					}
				} else {
					// initStmt is nil, try to parse as expression
					// Save state
					prevCur := p.curToken
					prevPeek := p.peekToken
					prevErrCount := len(p.errors)
					condExpr = p.parseExpression(PrecedenceLowest)
					// If parsing expression failed, rollback
					if condExpr == nil {
						p.curToken = prevCur
						p.peekToken = prevPeek
						p.errors = p.errors[:prevErrCount]
					}
				}

				if condExpr != nil {
					stmt.Condition = condExpr
					isSingleCondition = true
					// Skip the rest of the 3-part parsing
					if hasParen {
						if !p.expectPeek(TokenRightParen) {
							return nil
						}
					}
				} else {
					p.addError("expected ';' after for loop init")
					return nil
				}
			}
		} else {
			// Empty init clause, consume semicolon
			p.nextToken()
		}

		// Only continue if it's still a 3-part loop
		if !isSingleCondition {
			// Parse condition clause
			if !p.curTokenIs(TokenSemicolon) {
				stmt.Condition = p.parseExpression(PrecedenceLowest)
			}
			// Consume semicolon after condition
			if !p.expectPeek(TokenSemicolon) {
				return nil
			}
			p.nextToken() // move past semicolon to update clause

			// Parse update clause
			if !hasParen || !p.curTokenIs(TokenRightParen) {
				stmt.Update = p.parseStatement()
			}

			if hasParen {
				if !p.expectPeek(TokenRightParen) {
					return nil
				}
			}
		}
	}

	if !p.expectPeek(TokenLeftBrace) {
		return nil
	}

	stmt.Body = p.parseBlockStatement()

	return stmt
}

// parseWhileStatement parses a while loop statement
func (p *Parser) parseWhileStatement() *WhileStatement {
	stmt := &WhileStatement{Token: p.curToken}

	// Support both styles: while (condition) and while condition
	hasParen := p.peekTokenIs(TokenLeftParen)
	if hasParen {
		p.nextToken() // consume '('
		p.nextToken()
	} else {
		p.nextToken() // move to condition
	}

	stmt.Condition = p.parseExpression(PrecedenceLowest)

	if hasParen {
		if !p.expectPeek(TokenRightParen) {
			return nil
		}
	}

	if !p.expectPeek(TokenLeftBrace) {
		return nil
	}

	stmt.Body = p.parseBlockStatement()

	return stmt
}

// parseDoWhileStatement parses a do-while loop statement
func (p *Parser) parseDoWhileStatement() *DoWhileStatement {
	stmt := &DoWhileStatement{Token: p.curToken}

	if !p.expectPeek(TokenLeftBrace) {
		return nil
	}

	stmt.Body = p.parseBlockStatement()

	if !p.expectPeek(TokenWhile) {
		return nil
	}

	if !p.expectPeek(TokenLeftParen) {
		return nil
	}

	p.nextToken()
	stmt.Condition = p.parseExpression(PrecedenceLowest)

	if !p.expectPeek(TokenRightParen) {
		return nil
	}

	if p.peekTokenIs(TokenSemicolon) {
		p.nextToken()
	}

	return stmt
}

// parseSwitchStatement parses a switch statement
func (p *Parser) parseSwitchStatement() *SwitchStatement {
	stmt := &SwitchStatement{Token: p.curToken}

	if !p.expectPeek(TokenLeftParen) {
		return nil
	}

	p.nextToken()
	stmt.Expression = p.parseExpression(PrecedenceLowest)

	if !p.expectPeek(TokenRightParen) {
		return nil
	}

	if !p.expectPeek(TokenLeftBrace) {
		return nil
	}

	p.nextToken()

	for !p.curTokenIs(TokenRightBrace) && !p.curTokenIs(TokenEOF) {
		if p.curTokenIs(TokenCase) {
			caseStmt := p.parseCaseStatement()
			stmt.Cases = append(stmt.Cases, caseStmt)
		} else if p.curTokenIs(TokenDefault) {
			defaultStmt := p.parseDefaultStatement()
			stmt.DefaultCase = defaultStmt
		} else {
			p.addError(fmt.Sprintf("unexpected token %s in switch statement", p.curToken.Type))
			p.nextToken()
		}
	}

	return stmt
}

// parseCaseStatement parses a case clause in a switch statement
func (p *Parser) parseCaseStatement() *CaseStatement {
	stmt := &CaseStatement{Token: p.curToken}
	stmt.Expressions = []Expression{}

	p.nextToken()

	// Parse case expressions (multiple values separated by commas)
	for {
		expr := p.parseExpression(PrecedenceLowest)
		stmt.Expressions = append(stmt.Expressions, expr)

		if p.peekTokenIs(TokenComma) {
			p.nextToken()
			p.nextToken()
		} else {
			break
		}
	}

	if !p.expectPeek(TokenColon) {
		return nil
	}

	p.nextToken()
	stmt.Body = p.parseBlockStatement()

	return stmt
}

// parseDefaultStatement parses a default clause in a switch statement
func (p *Parser) parseDefaultStatement() *DefaultStatement {
	stmt := &DefaultStatement{Token: p.curToken}

	if !p.expectPeek(TokenColon) {
		return nil
	}

	p.nextToken()
	stmt.Body = p.parseBlockStatement()

	return stmt
}

// parseFunctionDeclaration parses a function declaration
func (p *Parser) parseFunctionDeclaration() *FunctionDeclaration {
	decl := &FunctionDeclaration{Token: p.curToken}

	// Check for function name
	if p.peekTokenIs(TokenIdentifier) {
		p.nextToken()
		decl.Name = &Identifier{Token: p.curToken, Value: p.curToken.Literal}
	} else {
		// Anonymous function
		decl.Name = &Identifier{Token: p.curToken, Value: ""}
	}

	if !p.expectPeek(TokenLeftParen) {
		return nil
	}

	decl.Parameters = p.parseFunctionParameters()

	if !p.expectPeek(TokenLeftBrace) {
		return nil
	}

	decl.Body = p.parseBlockStatement()

	return decl
}

// parseFunctionParameters parses function parameters
func (p *Parser) parseFunctionParameters() []*FunctionParameter {
	params := []*FunctionParameter{}

	if p.peekTokenIs(TokenRightParen) {
		p.nextToken()
		return params
	}

	p.nextToken()

	for {
		param := &FunctionParameter{}

		if p.curTokenIs(TokenEllipsis) {
			param.Variadic = true
			p.nextToken()
		}

		if !p.curTokenIs(TokenIdentifier) {
			p.addError(fmt.Sprintf("expected parameter name, got %s", p.curToken.Type))
			return nil
		}

		param.Name = &Identifier{Token: p.curToken, Value: p.curToken.Literal}

		// Check for default value
		if p.peekTokenIs(TokenAssign) {
			p.nextToken()
			p.nextToken()
			param.DefaultValue = p.parseExpression(PrecedenceLowest)
		}

		params = append(params, param)

		if p.peekTokenIs(TokenComma) {
			p.nextToken()
			p.nextToken()
		} else {
			break
		}
	}

	if !p.expectPeek(TokenRightParen) {
		return nil
	}

	return params
}

// parseClassDeclaration parses a class declaration
func (p *Parser) parseClassDeclaration() *ClassDeclaration {
	decl := &ClassDeclaration{Token: p.curToken}

	if !p.expectPeek(TokenIdentifier) {
		return nil
	}

	decl.Name = &Identifier{Token: p.curToken, Value: p.curToken.Literal}

	// Check for superclass
	if p.peekTokenIs(TokenColon) {
		p.nextToken()
		if !p.expectPeek(TokenIdentifier) {
			return nil
		}
		decl.SuperClass = &Identifier{Token: p.curToken, Value: p.curToken.Literal}
	}

	if !p.expectPeek(TokenLeftBrace) {
		return nil
	}

	p.nextToken()

	decl.Methods = []*FunctionLiteral{}

	for !p.curTokenIs(TokenRightBrace) && !p.curTokenIs(TokenEOF) {
		if p.curTokenIs(TokenFunc) {
			p.nextToken()
			method := p.parseFunctionLiteral().(*FunctionLiteral)
			decl.Methods = append(decl.Methods, method)

			// Check if this is the constructor
			if method.Name == "init" {
				decl.Constructor = method
			}
		} else {
			p.addError(fmt.Sprintf("unexpected token %s in class body", p.curToken.Type))
			p.nextToken()
		}
	}

	return decl
}

// parseTryStatement parses a try-catch-finally statement
func (p *Parser) parseTryStatement() *TryStatement {
	stmt := &TryStatement{Token: p.curToken}

	if !p.expectPeek(TokenLeftBrace) {
		return nil
	}

	stmt.TryBlock = p.parseBlockStatement()

	if p.peekTokenIs(TokenCatch) {
		p.nextToken()
		stmt.Catch = p.parseCatchStatement()
	}

	if p.peekTokenIs(TokenFinally) {
		p.nextToken()
		stmt.Finally = p.parseFinallyStatement()
	}

	if stmt.Catch == nil && stmt.Finally == nil {
		p.addError("try statement must have at least one catch or finally block")
		return nil
	}

	return stmt
}

// parseCatchStatement parses a catch clause
func (p *Parser) parseCatchStatement() *CatchStatement {
	stmt := &CatchStatement{Token: p.curToken}

	// Parse optional catch parameter
	if p.peekTokenIs(TokenLeftParen) {
		p.nextToken()
		if !p.expectPeek(TokenIdentifier) {
			return nil
		}
		stmt.Param = &Identifier{Token: p.curToken, Value: p.curToken.Literal}
		if !p.expectPeek(TokenRightParen) {
			return nil
		}
	}

	if !p.expectPeek(TokenLeftBrace) {
		return nil
	}

	stmt.CatchBlock = p.parseBlockStatement()

	return stmt
}

// parseFinallyStatement parses a finally clause
func (p *Parser) parseFinallyStatement() *FinallyStatement {
	stmt := &FinallyStatement{Token: p.curToken}

	if !p.expectPeek(TokenLeftBrace) {
		return nil
	}

	stmt.FinallyBlock = p.parseBlockStatement()

	return stmt
}

// parseDeferStatement parses a defer statement
func (p *Parser) parseDeferStatement() *DeferStatement {
	stmt := &DeferStatement{Token: p.curToken}

	p.nextToken()
	callExpr, ok := p.parseExpression(PrecedenceLowest).(*CallExpression)
	if !ok {
		p.addError("defer statement must be followed by a function call")
		return nil
	}

	stmt.Call = callExpr

	if p.peekTokenIs(TokenSemicolon) {
		p.nextToken()
	}

	return stmt
}

// parseThrowStatement parses a throw statement
func (p *Parser) parseThrowStatement() *ThrowStatement {
	stmt := &ThrowStatement{Token: p.curToken}

	p.nextToken()
	stmt.Value = p.parseExpression(PrecedenceLowest)

	if p.peekTokenIs(TokenSemicolon) {
		p.nextToken()
	}

	return stmt
}

// Expression parsing

// parseExpression parses an expression with the given precedence
func (p *Parser) parseExpression(precedence int) Expression {
	prefix := p.prefixParseFns[p.curToken.Type]
	if prefix == nil {
		p.addError(fmt.Sprintf("no prefix parse function for %s found", p.curToken.Type))
		return nil
	}

	leftExp := prefix()

	for !p.peekTokenIs(TokenSemicolon) && precedence < p.peekPrecedence() {
		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil {
			return leftExp
		}

		p.nextToken()

		leftExp = infix(leftExp)
	}

	return leftExp
}

// parseIdentifier parses an identifier expression
func (p *Parser) parseIdentifier() Expression {
	return &Identifier{
		Token: p.curToken,
		Value: p.curToken.Literal,
	}
}

// parseIntegerLiteral parses an integer literal expression
func (p *Parser) parseIntegerLiteral() Expression {
	lit := &IntLiteral{Token: p.curToken}

	value, err := strconv.ParseInt(p.curToken.Literal, 10, 64)
	if err != nil {
		p.addError(fmt.Sprintf("could not parse %q as integer", p.curToken.Literal))
		return nil
	}

	lit.Value = types.Int(value)
	return lit
}

// parseFloatLiteral parses a float literal expression
func (p *Parser) parseFloatLiteral() Expression {
	lit := &FloatLiteral{Token: p.curToken}

	value, err := strconv.ParseFloat(p.curToken.Literal, 64)
	if err != nil {
		p.addError(fmt.Sprintf("could not parse %q as float", p.curToken.Literal))
		return nil
	}

	lit.Value = types.Float(value)
	return lit
}

// parseStringLiteral parses a string literal expression
func (p *Parser) parseStringLiteral() Expression {
	return &StringLiteral{
		Token: p.curToken,
		Value: types.String(p.curToken.Literal),
	}
}

// parseCharLiteral parses a character literal expression
func (p *Parser) parseCharLiteral() Expression {
	lit := &CharLiteral{Token: p.curToken}
	if len(p.curToken.Literal) == 0 {
		lit.Value = types.Char(0)
	} else {
		runes := []rune(p.curToken.Literal)
		if len(runes) > 0 {
			lit.Value = types.Char(runes[0])
		}
	}
	return lit
}

// parseBooleanLiteral parses a boolean literal expression
func (p *Parser) parseBooleanLiteral() Expression {
	return &BoolLiteral{
		Token: p.curToken,
		Value: types.Bool(p.curTokenIs(TokenTrue)),
	}
}

// parseNullLiteral parses a null literal expression
func (p *Parser) parseNullLiteral() Expression {
	return &NullLiteral{Token: p.curToken}
}

// parsePrefixExpression parses a prefix operator expression
func (p *Parser) parsePrefixExpression() Expression {
	expr := &PrefixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
	}

	p.nextToken()

	expr.Right = p.parseExpression(PrecedencePrefix)

	return expr
}

// parseInfixExpression parses an infix operator expression
func (p *Parser) parseInfixExpression(left Expression) Expression {
	expr := &InfixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
		Left:     left,
	}

	precedence := p.curPrecedence()
	p.nextToken()
	expr.Right = p.parseExpression(precedence)

	return expr
}

// parseGroupedExpression parses an expression enclosed in parentheses
func (p *Parser) parseGroupedExpression() Expression {
	p.nextToken()

	exp := p.parseExpression(PrecedenceLowest)

	if !p.expectPeek(TokenRightParen) {
		return nil
	}

	return exp
}

// parseIfExpression parses an if expression (when used in expression context)
func (p *Parser) parseIfExpression() Expression {
	stmt := p.parseIfStatement()
	return stmt
}

// parseFunctionLiteral parses a function literal expression
func (p *Parser) parseFunctionLiteral() Expression {
	lit := &FunctionLiteral{Token: p.curToken}

	if p.peekTokenIs(TokenIdentifier) {
		p.nextToken()
		lit.Name = p.curToken.Literal
	}

	if !p.expectPeek(TokenLeftParen) {
		return nil
	}

	// Convert FunctionParameter to the type expected by FunctionLiteral
	params := p.parseFunctionParameters()
	lit.Parameters = make([]*FunctionParameter, len(params))
	for i, param := range params {
		lit.Parameters[i] = &FunctionParameter{
			Name:         param.Name,
			DefaultValue: param.DefaultValue,
			Variadic:     param.Variadic,
		}
	}

	if !p.expectPeek(TokenLeftBrace) {
		return nil
	}

	lit.Body = p.parseBlockStatement()

	return lit
}

// parseArrayLiteral parses an array literal expression
func (p *Parser) parseArrayLiteral() Expression {
	array := &ArrayLiteral{Token: p.curToken}
	array.Elements = p.parseExpressionList(TokenRightBracket)
	return array
}

// parseMapLiteral parses a map literal expression
func (p *Parser) parseMapLiteral() Expression {
	mapLit := &MapLiteral{Token: p.curToken}
	mapLit.Pairs = make(map[Expression]Expression)

	for !p.peekTokenIs(TokenRightBrace) && !p.peekTokenIs(TokenEOF) {
		p.nextToken()
		key := p.parseExpression(PrecedenceLowest)

		if !p.expectPeek(TokenColon) {
			return nil
		}

		p.nextToken()
		value := p.parseExpression(PrecedenceLowest)

		mapLit.Pairs[key] = value

		if !p.peekTokenIs(TokenRightBrace) && !p.expectPeek(TokenComma) {
			return nil
		}
	}

	if !p.expectPeek(TokenRightBrace) {
		return nil
	}

	return mapLit
}

// parseNewExpression parses a new operator expression
func (p *Parser) parseNewExpression() Expression {
	expr := &NewExpression{Token: p.curToken}

	p.nextToken()
	expr.Class = p.parseExpression(PrecedenceLowest)

	if !p.expectPeek(TokenLeftParen) {
		return nil
	}

	expr.Args = p.parseExpressionList(TokenRightParen)

	return expr
}

// parseCallExpression parses a function call expression
func (p *Parser) parseCallExpression(function Expression) Expression {
	exp := &CallExpression{
		Token:    p.curToken,
		Function: function,
	}
	exp.Arguments = p.parseExpressionList(TokenRightParen)
	return exp
}

// parseIndexExpression parses an array/index access expression
func (p *Parser) parseIndexExpression(left Expression) Expression {
	exp := &IndexExpression{
		Token: p.curToken,
		Left:  left,
	}

	p.nextToken()
	exp.Index = p.parseExpression(PrecedenceLowest)

	if !p.expectPeek(TokenRightBracket) {
		return nil
	}

	return exp
}

// parseMemberExpression parses a member access expression
func (p *Parser) parseMemberExpression(object Expression) Expression {
	exp := &MemberExpression{
		Token:  p.curToken,
		Object: object,
	}

	if !p.expectPeek(TokenIdentifier) {
		return nil
	}

	exp.Member = &Identifier{
		Token: p.curToken,
		Value: p.curToken.Literal,
	}

	return exp
}

// parseAssignmentExpression parses an assignment expression
func (p *Parser) parseAssignmentExpression(left Expression) Expression {
	expr := &AssignmentExpression{
		Token:    p.curToken,
		Left:     left,
		Operator: p.curToken.Literal,
	}

	precedence := p.curPrecedence()
	p.nextToken()
	// Assignment is right-associative, use precedence - 1 for right-hand side
	expr.Right = p.parseExpression(precedence - 1)

	return expr
}

// parseTernaryExpression parses a ternary conditional expression
func (p *Parser) parseTernaryExpression(condition Expression) Expression {
	expr := &TernaryExpression{
		Token:     p.curToken,
		Condition: condition,
	}

	p.nextToken()
	expr.TrueExpr = p.parseExpression(PrecedenceLowest)

	if !p.expectPeek(TokenColon) {
		return nil
	}

	p.nextToken()
	expr.FalseExpr = p.parseExpression(PrecedenceLowest)

	return expr
}

// parsePostfixExpression parses a postfix increment or decrement expression (i++, i--)
func (p *Parser) parsePostfixExpression(left Expression) Expression {
	expr := &PostfixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
		Left:     left,
	}

	return expr
}

// parseExpressionList parses a list of expressions separated by commas
func (p *Parser) parseExpressionList(end TokenType) []Expression {
	list := []Expression{}

	if p.peekTokenIs(end) {
		p.nextToken()
		return list
	}

	p.nextToken()
	list = append(list, p.parseExpression(PrecedenceLowest))

	for p.peekTokenIs(TokenComma) {
		p.nextToken()
		p.nextToken()
		list = append(list, p.parseExpression(PrecedenceLowest))
	}

	if !p.expectPeek(end) {
		return nil
	}

	return list
}

// parseImportStatement parses an import statement
func (p *Parser) parseImportStatement() *ImportStatement {
	stmt := &ImportStatement{Token: p.curToken}

	// Check for namespace import: import * as name from "path"
	if p.peekTokenIs(TokenAsterisk) {
		p.nextToken() // consume '*'
		if !p.expectPeek(TokenAs) {
			return nil
		}
		if !p.expectPeek(TokenIdentifier) {
			return nil
		}
		stmt.Namespace = &Identifier{Token: p.curToken, Value: p.curToken.Literal}
		if !p.expectPeek(TokenFrom) {
			return nil
		}
		if !p.expectPeek(TokenString) {
			return nil
		}
		stmt.ModulePath = &StringLiteral{
			Token: p.curToken,
			Value: types.String(p.curToken.Literal),
		}
		return stmt
	}

	// Check for named imports: import { a, b as c } from "path"
	if p.peekTokenIs(TokenLeftBrace) {
		p.nextToken() // consume '{'
		stmt.Specs = []*ImportSpec{}

		for !p.peekTokenIs(TokenRightBrace) && !p.peekTokenIs(TokenEOF) {
			p.nextToken()
			spec := &ImportSpec{}
			spec.Name = &Identifier{Token: p.curToken, Value: p.curToken.Literal}

			if p.peekTokenIs(TokenAs) {
				p.nextToken() // consume 'as'
				if !p.expectPeek(TokenIdentifier) {
					return nil
				}
				spec.Alias = &Identifier{Token: p.curToken, Value: p.curToken.Literal}
			}

			stmt.Specs = append(stmt.Specs, spec)

			if p.peekTokenIs(TokenComma) {
				p.nextToken() // consume ','
			}
		}

		if !p.expectPeek(TokenRightBrace) {
			return nil
		}

		if !p.expectPeek(TokenFrom) {
			return nil
		}

		if !p.expectPeek(TokenString) {
			return nil
		}

		stmt.ModulePath = &StringLiteral{
			Token: p.curToken,
			Value: types.String(p.curToken.Literal),
		}

		return stmt
	}

	// Default import or default + named imports: import name, { a, b } from "path"
	if p.peekTokenIs(TokenIdentifier) {
		p.nextToken()
		stmt.DefaultImport = &Identifier{Token: p.curToken, Value: p.curToken.Literal}

		// Check if there are named imports after comma
		if p.peekTokenIs(TokenComma) {
			p.nextToken() // consume ','
			if !p.expectPeek(TokenLeftBrace) {
				return nil
			}
			// Parse named imports
			stmt.Specs = []*ImportSpec{}
			for !p.peekTokenIs(TokenRightBrace) && !p.peekTokenIs(TokenEOF) {
				p.nextToken()
				spec := &ImportSpec{}
				spec.Name = &Identifier{Token: p.curToken, Value: p.curToken.Literal}

				if p.peekTokenIs(TokenAs) {
					p.nextToken() // consume 'as'
					if !p.expectPeek(TokenIdentifier) {
						return nil
					}
					spec.Alias = &Identifier{Token: p.curToken, Value: p.curToken.Literal}
				}

				stmt.Specs = append(stmt.Specs, spec)

				if p.peekTokenIs(TokenComma) {
					p.nextToken() // consume ','
				}
			}

			if !p.expectPeek(TokenRightBrace) {
				return nil
			}
		}

		if !p.expectPeek(TokenFrom) {
			return nil
		}
		if !p.expectPeek(TokenString) {
			return nil
		}
		stmt.ModulePath = &StringLiteral{
			Token: p.curToken,
			Value: types.String(p.curToken.Literal),
		}
		return stmt
	}

	// Bare import: import "path"
	if p.peekTokenIs(TokenString) {
		p.nextToken()
		stmt.ModulePath = &StringLiteral{
			Token: p.curToken,
			Value: types.String(p.curToken.Literal),
		}
		return stmt
	}

	p.addError("invalid import statement syntax")
	return nil
}

// parseExportStatement parses an export statement
func (p *Parser) parseExportStatement() *ExportStatement {
	stmt := &ExportStatement{Token: p.curToken}

	// Advance past 'export' token
	p.nextToken()

	// Check for default export: export default ...
	if p.curTokenIs(TokenDefault) {
		stmt.IsDefault = true
		p.nextToken() // consume 'default'
		// Parse the declaration
		stmt.Declaration = p.parseStatement()
		return stmt
	}

	// Check for named exports: export { a, b as c }
	if p.curTokenIs(TokenLeftBrace) {
		p.nextToken() // consume '{'
		stmt.Specs = []*ExportSpec{}

		for !p.curTokenIs(TokenRightBrace) && !p.curTokenIs(TokenEOF) {
			spec := &ExportSpec{}
			spec.Name = &Identifier{Token: p.curToken, Value: p.curToken.Literal}

			if p.peekTokenIs(TokenAs) {
				p.nextToken() // consume 'as'
				if !p.expectPeek(TokenIdentifier) {
					return nil
				}
				spec.Alias = &Identifier{Token: p.curToken, Value: p.curToken.Literal}
			}

			stmt.Specs = append(stmt.Specs, spec)

			if p.peekTokenIs(TokenComma) {
				p.nextToken() // consume ','
				p.nextToken() // move to next identifier
			} else {
				p.nextToken() // move to '}'
			}
		}

		return stmt
	}

	// Export declaration: export func/let/const/class ...
	stmt.Declaration = p.parseStatement()
	return stmt
}
