package parser

// TokenType represents the type of lexical token
type TokenType int

// Token types
const (
	// Special tokens
	TokenEOF TokenType = iota
	TokenError
	TokenComment

	// Literals
	TokenIdentifier // main, x, y
	TokenInt        // 12345
	TokenFloat      // 123.45
	TokenString     // "hello world"
	TokenChar       // 'a'
	TokenRawString  // `raw string`

	// Operators
	TokenPlus       // +
	TokenMinus      // -
	TokenAsterisk   // *
	TokenSlash      // /
	TokenPercent    // %
	TokenAmpersand  // &
	TokenPipe       // |
	TokenCaret      // ^
	TokenTilde      // ~
	TokenLeftShift  // <<
	TokenRightShift // >>
	TokenAnd        // &&
	TokenOr         // ||
	TokenBang       // !
	TokenQuestion   // ?
	TokenColon      // :
	TokenAssign     // =

	// Comparison operators
	TokenEqual    // ==
	TokenNotEqual // !=
	TokenLT       // <
	TokenLTE      // <=
	TokenGT       // >
	TokenGTE      // >=

	// Assignment operators
	TokenPlusAssign    // +=
	TokenMinusAssign   // -=
	TokenMulAssign     // *=
	TokenDivAssign     // /=
	TokenModAssign     // %=
	TokenAndAssign     // &=
	TokenOrAssign      // |=
	TokenXorAssign     // ^=
	TokenLShiftAssign  // <<=
	TokenRShiftAssign  // >>=
	TokenDefine        // :=
	TokenInc           // ++
	TokenDec           // --

	// Delimiters
	TokenLeftParen    // (
	TokenRightParen   // )
	TokenLeftBracket  // [
	TokenRightBracket // ]
	TokenLeftBrace    // {
	TokenRightBrace   // }
	TokenComma        // ,
	TokenDot          // .
	TokenSemicolon    // ;
	TokenEllipsis     // ...
	TokenArrow        // =>

	// Keywords
	keywordStart
	TokenVar
	TokenLet
	TokenConst
	TokenFunc
	TokenClass
	TokenIf
	TokenElse
	TokenFor
	TokenWhile
	TokenDo
	TokenSwitch
	TokenCase
	TokenDefault
	TokenBreak
	TokenContinue
	TokenFallthrough
	TokenIn
	TokenReturn
	TokenThrow
	TokenTry
	TokenCatch
	TokenFinally
	TokenDefer
	TokenImport
	TokenExport
	TokenAs
	TokenFrom
	TokenThis
	TokenSuper
	TokenNew
	TokenNil
	TokenTrue
	TokenFalse
	keywordEnd
)

// Token represents a lexical token with type, value, and position information
type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Column  int
}

var keywords = map[string]TokenType{
	"var":      TokenVar,
	"let":      TokenLet,
	"const":    TokenConst,
	"func":     TokenFunc,
	"class":    TokenClass,
	"if":       TokenIf,
	"else":     TokenElse,
	"for":      TokenFor,
	"while":    TokenWhile,
	"do":       TokenDo,
	"switch":   TokenSwitch,
	"case":     TokenCase,
	"default":      TokenDefault,
	"break":        TokenBreak,
	"continue":     TokenContinue,
	"fallthrough":  TokenFallthrough,
	"in":           TokenIn,
	"return":       TokenReturn,
	"throw":    TokenThrow,
	"try":      TokenTry,
	"catch":    TokenCatch,
	"finally":  TokenFinally,
	"defer":    TokenDefer,
	"import":   TokenImport,
	"export":   TokenExport,
	"as":       TokenAs,
	"from":     TokenFrom,
	"this":     TokenThis,
	"super":    TokenSuper,
	"new":      TokenNew,
	"nil":      TokenNil,
	"true":     TokenTrue,
	"false":    TokenFalse,
}

// LookupKeyword checks if an identifier is a keyword
func LookupKeyword(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return TokenIdentifier
}

// IsKeyword returns true if the token type is a keyword
func (t TokenType) IsKeyword() bool {
	return t > keywordStart && t < keywordEnd
}

// String returns the string representation of the token type
func (t TokenType) String() string {
	switch t {
	case TokenEOF:
		return "EOF"
	case TokenError:
		return "ERROR"
	case TokenComment:
		return "COMMENT"
	case TokenIdentifier:
		return "IDENTIFIER"
	case TokenInt:
		return "INT"
	case TokenFloat:
		return "FLOAT"
	case TokenString:
		return "STRING"
	case TokenChar:
		return "CHAR"
	case TokenRawString:
		return "RAW_STRING"
	case TokenPlus:
		return "+"
	case TokenMinus:
		return "-"
	case TokenAsterisk:
		return "*"
	case TokenSlash:
		return "/"
	case TokenPercent:
		return "%"
	case TokenAmpersand:
		return "&"
	case TokenPipe:
		return "|"
	case TokenCaret:
		return "^"
	case TokenTilde:
		return "~"
	case TokenLeftShift:
		return "<<"
	case TokenRightShift:
		return ">>"
	case TokenAnd:
		return "&&"
	case TokenOr:
		return "||"
	case TokenBang:
		return "!"
	case TokenQuestion:
		return "?"
	case TokenColon:
		return ":"
	case TokenAssign:
		return "="
	case TokenEqual:
		return "=="
	case TokenNotEqual:
		return "!="
	case TokenLT:
		return "<"
	case TokenLTE:
		return "<="
	case TokenGT:
		return ">"
	case TokenGTE:
		return ">="
	case TokenPlusAssign:
		return "+="
	case TokenMinusAssign:
		return "-="
	case TokenMulAssign:
		return "*="
	case TokenDivAssign:
		return "/="
	case TokenModAssign:
		return "%="
	case TokenAndAssign:
		return "&="
	case TokenOrAssign:
		return "|="
	case TokenXorAssign:
		return "^="
	case TokenLShiftAssign:
		return "<<="
	case TokenRShiftAssign:
		return ">>="
	case TokenDefine:
		return ":="
	case TokenLeftParen:
		return "("
	case TokenRightParen:
		return ")"
	case TokenLeftBracket:
		return "["
	case TokenRightBracket:
		return "]"
	case TokenLeftBrace:
		return "{"
	case TokenRightBrace:
		return "}"
	case TokenComma:
		return ","
	case TokenDot:
		return "."
	case TokenSemicolon:
		return ";"
	case TokenEllipsis:
		return "..."
	case TokenArrow:
		return "=>"
	case TokenVar:
		return "var"
	case TokenLet:
		return "let"
	case TokenConst:
		return "const"
	case TokenFunc:
		return "func"
	case TokenClass:
		return "class"
	case TokenIf:
		return "if"
	case TokenElse:
		return "else"
	case TokenFor:
		return "for"
	case TokenWhile:
		return "while"
	case TokenDo:
		return "do"
	case TokenSwitch:
		return "switch"
	case TokenCase:
		return "case"
	case TokenDefault:
		return "default"
	case TokenBreak:
		return "break"
	case TokenContinue:
		return "continue"
	case TokenFallthrough:
		return "fallthrough"
	case TokenIn:
		return "in"
	case TokenReturn:
		return "return"
	case TokenThrow:
		return "throw"
	case TokenTry:
		return "try"
	case TokenCatch:
		return "catch"
	case TokenFinally:
		return "finally"
	case TokenDefer:
		return "defer"
	case TokenImport:
		return "import"
	case TokenExport:
		return "export"
	case TokenAs:
		return "as"
	case TokenFrom:
		return "from"
	case TokenThis:
		return "this"
	case TokenSuper:
		return "super"
	case TokenNew:
		return "new"
	case TokenNil:
		return "nil"
	case TokenTrue:
		return "true"
	case TokenFalse:
		return "false"
	default:
		return "UNKNOWN"
	}
}
