package parser

import (
	"fmt"
	"unicode"
	"unicode/utf8"
)

// Lexer converts source code into a stream of tokens
type Lexer struct {
	input   string
	pos     int // current position in input (points to current rune)
	readPos int // current reading position (after current rune)
	ch      rune // current rune being processed
	line    int // current line number
	column  int // current column number
}

// NewLexer creates a new lexer for the given input
func NewLexer(input string) *Lexer {
	l := &Lexer{
		input:  input,
		line:   1,
		column: 0,
	}
	l.readChar()
	return l
}

// readChar reads the next character from input
func (l *Lexer) readChar() {
	if l.readPos >= len(l.input) {
		l.pos = l.readPos // Update pos to reflect current position
		l.ch = 0 // EOF
	} else {
		r, size := utf8.DecodeRuneInString(l.input[l.readPos:])
		l.ch = r
		l.pos = l.readPos
		l.readPos += size
		l.column++
	}
}

// peekChar returns the next character without advancing the position
func (l *Lexer) peekChar() rune {
	if l.readPos >= len(l.input) {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(l.input[l.readPos:])
	return r
}

// NextToken returns the next token from the input
func (l *Lexer) NextToken() Token {
	var tok Token

	l.skipWhitespace()

	// Skip comments
	for l.ch == '/' && (l.peekChar() == '/' || l.peekChar() == '*') {
		if l.peekChar() == '/' {
			l.readLineComment()
		} else {
			l.readBlockComment()
		}
		l.skipWhitespace()
	}

	switch l.ch {
	case '=':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = l.newToken(TokenEqual, string(ch)+string(l.ch))
		} else if l.peekChar() == '>' {
			ch := l.ch
			l.readChar()
			tok = l.newToken(TokenArrow, string(ch)+string(l.ch))
		} else {
			tok = l.newToken(TokenAssign, string(l.ch))
		}
	case '+':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = l.newToken(TokenPlusAssign, string(ch)+string(l.ch))
		} else if l.peekChar() == '+' {
			ch := l.ch
			l.readChar()
			tok = l.newToken(TokenInc, string(ch)+string(l.ch))
		} else {
			tok = l.newToken(TokenPlus, string(l.ch))
		}
	case '-':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = l.newToken(TokenMinusAssign, string(ch)+string(l.ch))
		} else if l.peekChar() == '-' {
			ch := l.ch
			l.readChar()
			tok = l.newToken(TokenDec, string(ch)+string(l.ch))
		} else {
			tok = l.newToken(TokenMinus, string(l.ch))
		}
	case '!':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = l.newToken(TokenNotEqual, string(ch)+string(l.ch))
		} else {
			tok = l.newToken(TokenBang, string(l.ch))
		}
	case '*':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = l.newToken(TokenMulAssign, string(ch)+string(l.ch))
		} else {
			tok = l.newToken(TokenAsterisk, string(l.ch))
		}
	case '/':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = l.newToken(TokenDivAssign, string(ch)+string(l.ch))
		} else if l.peekChar() == '/' {
			// Line comment
			return l.readLineComment()
		} else if l.peekChar() == '*' {
			// Block comment
			return l.readBlockComment()
		} else {
			tok = l.newToken(TokenSlash, string(l.ch))
		}
	case '%':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = l.newToken(TokenModAssign, string(ch)+string(l.ch))
		} else {
			tok = l.newToken(TokenPercent, string(l.ch))
		}
	case '<':
		if l.peekChar() == '<' {
			ch := l.ch
			l.readChar()
			if l.peekChar() == '=' {
				ch2 := l.ch
				l.readChar()
				tok = l.newToken(TokenLShiftAssign, string(ch)+string(ch2)+string(l.ch))
			} else {
				tok = l.newToken(TokenLeftShift, string(ch)+string(l.ch))
			}
		} else if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = l.newToken(TokenLTE, string(ch)+string(l.ch))
		} else {
			tok = l.newToken(TokenLT, string(l.ch))
		}
	case '>':
		if l.peekChar() == '>' {
			ch := l.ch
			l.readChar()
			if l.peekChar() == '=' {
				ch2 := l.ch
				l.readChar()
				tok = l.newToken(TokenRShiftAssign, string(ch)+string(ch2)+string(l.ch))
			} else {
				tok = l.newToken(TokenRightShift, string(ch)+string(l.ch))
			}
		} else if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = l.newToken(TokenGTE, string(ch)+string(l.ch))
		} else {
			tok = l.newToken(TokenGT, string(l.ch))
		}
	case '&':
		if l.peekChar() == '&' {
			ch := l.ch
			l.readChar()
			tok = l.newToken(TokenAnd, string(ch)+string(l.ch))
		} else if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = l.newToken(TokenAndAssign, string(ch)+string(l.ch))
		} else {
			tok = l.newToken(TokenAmpersand, string(l.ch))
		}
	case '|':
		if l.peekChar() == '|' {
			ch := l.ch
			l.readChar()
			tok = l.newToken(TokenOr, string(ch)+string(l.ch))
		} else if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = l.newToken(TokenOrAssign, string(ch)+string(l.ch))
		} else {
			tok = l.newToken(TokenPipe, string(l.ch))
		}
	case '^':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = l.newToken(TokenXorAssign, string(ch)+string(l.ch))
		} else {
			tok = l.newToken(TokenCaret, string(l.ch))
		}
	case '~':
		tok = l.newToken(TokenTilde, string(l.ch))
	case '?':
		tok = l.newToken(TokenQuestion, string(l.ch))
	case ':':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = l.newToken(TokenDefine, string(ch)+string(l.ch))
		} else {
			tok = l.newToken(TokenColon, string(l.ch))
		}
	case ';':
		tok = l.newToken(TokenSemicolon, string(l.ch))
	case '(':
		tok = l.newToken(TokenLeftParen, string(l.ch))
	case ')':
		tok = l.newToken(TokenRightParen, string(l.ch))
	case ',':
		tok = l.newToken(TokenComma, string(l.ch))
	case '.':
		if l.peekChar() == '.' && l.peekCharN(2) == '.' {
			l.readChar()
			l.readChar()
			tok = l.newToken(TokenEllipsis, "...")
		} else {
			tok = l.newToken(TokenDot, string(l.ch))
		}
	case '[':
		tok = l.newToken(TokenLeftBracket, string(l.ch))
	case ']':
		tok = l.newToken(TokenRightBracket, string(l.ch))
	case '{':
		tok = l.newToken(TokenLeftBrace, string(l.ch))
	case '}':
		tok = l.newToken(TokenRightBrace, string(l.ch))
	case '"':
		return l.readString()
	case '`':
		return l.readRawString()
	case '\'':
		return l.readCharLiteral()
	case 0:
		tok.Literal = ""
		tok.Type = TokenEOF
	default:
		if isLetter(l.ch) {
			literal := l.readIdentifier()
			tokType := LookupKeyword(literal)
			return l.newToken(tokType, literal)
		} else if isDigit(l.ch) {
			return l.readNumber()
		} else {
			errMsg := fmt.Sprintf("unexpected character: %q", l.ch)
			return l.newToken(TokenError, errMsg)
		}
	}

	l.readChar()
	return tok
}

// newToken creates a new token with current position
func (l *Lexer) newToken(tokenType TokenType, literal string) Token {
	return Token{
		Type:    tokenType,
		Literal: literal,
		Line:    l.line,
		Column:  l.column - len(literal),
	}
}

// readIdentifier reads an identifier from input
func (l *Lexer) readIdentifier() string {
	start := l.pos
	for isLetter(l.ch) || isDigit(l.ch) || l.ch == '_' {
		l.readChar()
	}
	return l.input[start:l.pos]
}

// readNumber reads a number (int or float) from input
func (l *Lexer) readNumber() Token {
	start := l.pos
	tokType := TokenInt

	// Read integer part
	for isDigit(l.ch) {
		l.readChar()
	}

	// Check for float
	if l.ch == '.' && isDigit(l.peekChar()) {
		tokType = TokenFloat
		l.readChar()
		for isDigit(l.ch) {
			l.readChar()
		}
	}

	// Check for exponent
	if (l.ch == 'e' || l.ch == 'E') && (isDigit(l.peekChar()) || (l.peekChar() == '+' || l.peekChar() == '-') && isDigit(l.peekCharN(2))) {
		tokType = TokenFloat
		l.readChar()
		if l.ch == '+' || l.ch == '-' {
			l.readChar()
		}
		for isDigit(l.ch) {
			l.readChar()
		}
	}

	return l.newToken(tokType, l.input[start:l.pos])
}

// readString reads a quoted string from input
func (l *Lexer) readString() Token {
	var result []rune
	l.readChar() // Skip opening quote

	for l.ch != '"' && l.ch != 0 {
		if l.ch == '\\' {
			l.readChar() // Skip escape character
			switch l.ch {
			case 'n':
				result = append(result, '\n')
			case 'r':
				result = append(result, '\r')
			case 't':
				result = append(result, '\t')
			case '\\':
				result = append(result, '\\')
			case '"':
				result = append(result, '"')
			case '\'':
				result = append(result, '\'')
			default:
				// Unknown escape, keep as is
				result = append(result, '\\', l.ch)
			}
		} else {
			result = append(result, l.ch)
		}
		l.readChar()
	}

	if l.ch == 0 {
		return l.newToken(TokenError, "unterminated string")
	}

	l.readChar() // Skip closing quote
	return l.newToken(TokenString, string(result))
}

// readRawString reads a raw string (backtick quoted) from input
func (l *Lexer) readRawString() Token {
	start := l.pos + 1 // Skip opening backtick
	l.readChar()

	for l.ch != '`' && l.ch != 0 {
		l.readChar()
	}

	if l.ch == 0 {
		return l.newToken(TokenError, "unterminated raw string")
	}

	literal := l.input[start:l.pos]
	l.readChar() // Skip closing backtick
	return l.newToken(TokenRawString, literal)
}

// readCharLiteral reads a character literal from input
func (l *Lexer) readCharLiteral() Token {
	var result rune
	l.readChar() // Skip opening quote

	if l.ch == '\\' {
		l.readChar() // Skip escape character
		switch l.ch {
		case 'n':
			result = '\n'
		case 'r':
			result = '\r'
		case 't':
			result = '\t'
		case '\\':
			result = '\\'
		case '"':
			result = '"'
		case '\'':
			result = '\''
		default:
			// Unknown escape, keep as is
			result = l.ch
		}
	} else {
		result = l.ch
	}

	l.readChar() // Move past the character

	if l.ch != '\'' {
		return l.newToken(TokenError, "unterminated character literal")
	}

	l.readChar() // Skip closing quote
	return l.newToken(TokenChar, string(result))
}

// readLineComment reads a line comment (// ...)
func (l *Lexer) readLineComment() Token {
	start := l.pos
	for l.ch != '\n' && l.ch != 0 {
		l.readChar()
	}
	return l.newToken(TokenComment, l.input[start:l.pos])
}

// readBlockComment reads a block comment (/* ... */)
func (l *Lexer) readBlockComment() Token {
	start := l.pos
	l.readChar() // Skip '/'
	l.readChar() // Skip '*'

	for !(l.ch == '*' && l.peekChar() == '/') && l.ch != 0 {
		if l.ch == '\n' {
			l.line++
			l.column = 0
		}
		l.readChar()
	}

	if l.ch == 0 {
		return l.newToken(TokenError, "unterminated block comment")
	}

	l.readChar() // Skip '*'
	l.readChar() // Skip '/'

	return l.newToken(TokenComment, l.input[start:l.pos])
}

// skipWhitespace skips over whitespace characters
func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		if l.ch == '\n' {
			l.line++
			l.column = 0
		}
		l.readChar()
	}
}

// peekCharN returns the nth character ahead without advancing position
func (l *Lexer) peekCharN(n int) rune {
	pos := l.readPos
	for i := 0; i < n-1; i++ {
		if pos >= len(l.input) {
			return 0
		}
		_, size := utf8.DecodeRuneInString(l.input[pos:])
		pos += size
	}
	r, _ := utf8.DecodeRuneInString(l.input[pos:])
	return r
}

// isLetter checks if a rune is a letter
func isLetter(ch rune) bool {
	return unicode.IsLetter(ch) || ch == '_'
}

// isDigit checks if a rune is a digit
func isDigit(ch rune) bool {
	return unicode.IsDigit(ch)
}
