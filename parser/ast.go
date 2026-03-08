package parser

import "github.com/topxeq/nxlang/types"

// Node is the base interface for all AST nodes
type Node interface {
	// TokenLiteral returns the literal value of the token associated with this node
	TokenLiteral() string
	// Line returns the line number where this node starts
	Line() int
	// Column returns the column number where this node starts
	Column() int
}

// Statement is the interface for all statement nodes
type Statement interface {
	Node
	statementNode()
}

// Expression is the interface for all expression nodes
type Expression interface {
	Node
	expressionNode()
}

// Program represents the root node of the AST
type Program struct {
	Statements []Statement
}

// TokenLiteral implements Node interface
func (p *Program) TokenLiteral() string {
	if len(p.Statements) > 0 {
		return p.Statements[0].TokenLiteral()
	}
	return ""
}

// Line implements Node interface
func (p *Program) Line() int {
	if len(p.Statements) > 0 {
		return p.Statements[0].Line()
	}
	return 0
}

// Column implements Node interface
func (p *Program) Column() int {
	if len(p.Statements) > 0 {
		return p.Statements[0].Column()
	}
	return 0
}

// Identifier represents an identifier (variable name, function name, etc.)
type Identifier struct {
	Token Token
	Value string
}

func (i *Identifier) expressionNode()      {}
func (i *Identifier) TokenLiteral() string { return i.Token.Literal }
func (i *Identifier) Line() int            { return i.Token.Line }
func (i *Identifier) Column() int          { return i.Token.Column }

// Literal expressions

// IntLiteral represents an integer literal
type IntLiteral struct {
	Token Token
	Value types.Int
}

func (il *IntLiteral) expressionNode()      {}
func (il *IntLiteral) TokenLiteral() string { return il.Token.Literal }
func (il *IntLiteral) Line() int            { return il.Token.Line }
func (il *IntLiteral) Column() int          { return il.Token.Column }

// FloatLiteral represents a floating point literal
type FloatLiteral struct {
	Token Token
	Value types.Float
}

func (fl *FloatLiteral) expressionNode()      {}
func (fl *FloatLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FloatLiteral) Line() int            { return fl.Token.Line }
func (fl *FloatLiteral) Column() int          { return fl.Token.Column }

// StringLiteral represents a string literal
type StringLiteral struct {
	Token Token
	Value types.String
}

func (sl *StringLiteral) expressionNode()      {}
func (sl *StringLiteral) TokenLiteral() string { return sl.Token.Literal }
func (sl *StringLiteral) Line() int            { return sl.Token.Line }
func (sl *StringLiteral) Column() int          { return sl.Token.Column }

// CharLiteral represents a character literal
type CharLiteral struct {
	Token Token
	Value types.Char
}

func (cl *CharLiteral) expressionNode()      {}
func (cl *CharLiteral) TokenLiteral() string { return cl.Token.Literal }
func (cl *CharLiteral) Line() int            { return cl.Token.Line }
func (cl *CharLiteral) Column() int          { return cl.Token.Column }

// BoolLiteral represents a boolean literal
type BoolLiteral struct {
	Token Token
	Value types.Bool
}

func (bl *BoolLiteral) expressionNode()      {}
func (bl *BoolLiteral) TokenLiteral() string { return bl.Token.Literal }
func (bl *BoolLiteral) Line() int            { return bl.Token.Line }
func (bl *BoolLiteral) Column() int          { return bl.Token.Column }

// NullLiteral represents a null literal
type NullLiteral struct {
	Token Token
}

func (nl *NullLiteral) expressionNode()      {}
func (nl *NullLiteral) TokenLiteral() string { return nl.Token.Literal }
func (nl *NullLiteral) Line() int            { return nl.Token.Line }
func (nl *NullLiteral) Column() int          { return nl.Token.Column }

// Statements

// ExpressionStatement represents a statement that is just an expression (e.g., function call)
type ExpressionStatement struct {
	Token      Token // The first token of the expression
	Expression Expression
}

func (es *ExpressionStatement) statementNode()       {}
func (es *ExpressionStatement) TokenLiteral() string { return es.Token.Literal }
func (es *ExpressionStatement) Line() int            { return es.Token.Line }
func (es *ExpressionStatement) Column() int          { return es.Token.Column }

// VarStatement represents a variable declaration with 'var' keyword
type VarStatement struct {
	Token Token // 'var' token
	Name  *Identifier
	Value Expression
}

func (vs *VarStatement) statementNode()       {}
func (vs *VarStatement) TokenLiteral() string { return vs.Token.Literal }
func (vs *VarStatement) Line() int            { return vs.Token.Line }
func (vs *VarStatement) Column() int          { return vs.Token.Column }

// LetStatement represents a variable declaration with 'let' keyword
type LetStatement struct {
	Token Token // 'let' token
	Name  *Identifier
	Value Expression
}

func (ls *LetStatement) statementNode()       {}
func (ls *LetStatement) TokenLiteral() string { return ls.Token.Literal }
func (ls *LetStatement) Line() int            { return ls.Token.Line }
func (ls *LetStatement) Column() int          { return ls.Token.Column }

// ConstStatement represents a constant declaration
type ConstStatement struct {
	Token Token // 'const' token
	Name  *Identifier
	Value Expression
}

func (cs *ConstStatement) statementNode()       {}
func (cs *ConstStatement) TokenLiteral() string { return cs.Token.Literal }
func (cs *ConstStatement) Line() int            { return cs.Token.Line }
func (cs *ConstStatement) Column() int          { return cs.Token.Column }

// DefineStatement represents a short variable declaration using ':='
type DefineStatement struct {
	Token Token // ':=' token
	Name  *Identifier
	Value Expression
}

func (ds *DefineStatement) statementNode()       {}
func (ds *DefineStatement) TokenLiteral() string { return ds.Token.Literal }
func (ds *DefineStatement) Line() int            { return ds.Token.Line }
func (ds *DefineStatement) Column() int          { return ds.Token.Column }

// ReturnStatement represents a return statement
type ReturnStatement struct {
	Token       Token // 'return' token
	ReturnValue Expression
}

func (rs *ReturnStatement) statementNode()       {}
func (rs *ReturnStatement) TokenLiteral() string { return rs.Token.Literal }
func (rs *ReturnStatement) Line() int            { return rs.Token.Line }
func (rs *ReturnStatement) Column() int          { return rs.Token.Column }

// BreakStatement represents a break statement
type BreakStatement struct {
	Token Token // 'break' token
}

func (bs *BreakStatement) statementNode()       {}
func (bs *BreakStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BreakStatement) Line() int            { return bs.Token.Line }
func (bs *BreakStatement) Column() int          { return bs.Token.Column }

// ContinueStatement represents a continue statement
type ContinueStatement struct {
	Token Token // 'continue' token
}

func (cs *ContinueStatement) statementNode()       {}
func (cs *ContinueStatement) TokenLiteral() string { return cs.Token.Literal }
func (cs *ContinueStatement) Line() int            { return cs.Token.Line }
func (cs *ContinueStatement) Column() int          { return cs.Token.Column }

// FallthroughStatement represents a fallthrough statement in switch case
type FallthroughStatement struct {
	Token Token // 'fallthrough' token
}

func (fs *FallthroughStatement) statementNode()       {}
func (fs *FallthroughStatement) TokenLiteral() string { return fs.Token.Literal }
func (fs *FallthroughStatement) Line() int            { return fs.Token.Line }
func (fs *FallthroughStatement) Column() int          { return fs.Token.Column }

// BlockStatement represents a block of statements enclosed in braces
type BlockStatement struct {
	Token      Token // '{' token
	Statements []Statement
}

func (bs *BlockStatement) statementNode()       {}
func (bs *BlockStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BlockStatement) Line() int            { return bs.Token.Line }
func (bs *BlockStatement) Column() int          { return bs.Token.Column }

// IfStatement represents an if statement with optional else clause
type IfStatement struct {
	Token       Token // 'if' token
	Condition   Expression
	Consequence *BlockStatement
	Alternative *BlockStatement
}

func (is *IfStatement) statementNode()       {}
func (is *IfStatement) expressionNode()      {}
func (is *IfStatement) TokenLiteral() string { return is.Token.Literal }
func (is *IfStatement) Line() int            { return is.Token.Line }
func (is *IfStatement) Column() int          { return is.Token.Column }

// ForStatement represents a for loop statement
type ForStatement struct {
	Token     Token // 'for' token
	Init      Statement
	Condition Expression
	Update    Statement
	Body      *BlockStatement

	// For...in specific fields
	IsForIn   bool        // Whether this is a for...in loop
	Key       *Identifier // Key variable for for...in
	Value     *Identifier // Value variable for for...in (optional)
	Iterate   Expression  // The expression to iterate over
}

func (fs *ForStatement) statementNode()       {}
func (fs *ForStatement) TokenLiteral() string { return fs.Token.Literal }
func (fs *ForStatement) Line() int            { return fs.Token.Line }
func (fs *ForStatement) Column() int          { return fs.Token.Column }

// WhileStatement represents a while loop statement
type WhileStatement struct {
	Token     Token // 'while' token
	Condition Expression
	Body      *BlockStatement
}

func (ws *WhileStatement) statementNode()       {}
func (ws *WhileStatement) TokenLiteral() string { return ws.Token.Literal }
func (ws *WhileStatement) Line() int            { return ws.Token.Line }
func (ws *WhileStatement) Column() int          { return ws.Token.Column }

// DoWhileStatement represents a do-while loop statement
type DoWhileStatement struct {
	Token     Token // 'do' token
	Body      *BlockStatement
	Condition Expression
}

func (dws *DoWhileStatement) statementNode()       {}
func (dws *DoWhileStatement) TokenLiteral() string { return dws.Token.Literal }
func (dws *DoWhileStatement) Line() int            { return dws.Token.Line }
func (dws *DoWhileStatement) Column() int          { return dws.Token.Column }

// SwitchStatement represents a switch statement
type SwitchStatement struct {
	Token       Token // 'switch' token
	Expression  Expression
	Cases       []*CaseStatement
	DefaultCase *DefaultStatement
}

func (ss *SwitchStatement) statementNode()       {}
func (ss *SwitchStatement) TokenLiteral() string { return ss.Token.Literal }
func (ss *SwitchStatement) Line() int            { return ss.Token.Line }
func (ss *SwitchStatement) Column() int          { return ss.Token.Column }

// CaseStatement represents a case clause in a switch statement
type CaseStatement struct {
	Token       Token // 'case' token
	Expressions []Expression
	Body        *BlockStatement
}

func (cs *CaseStatement) statementNode()       {}
func (cs *CaseStatement) TokenLiteral() string { return cs.Token.Literal }
func (cs *CaseStatement) Line() int            { return cs.Token.Line }
func (cs *CaseStatement) Column() int          { return cs.Token.Column }

// DefaultStatement represents a default clause in a switch statement
type DefaultStatement struct {
	Token Token // 'default' token
	Body  *BlockStatement
}

func (ds *DefaultStatement) statementNode()       {}
func (ds *DefaultStatement) TokenLiteral() string { return ds.Token.Literal }
func (ds *DefaultStatement) Line() int            { return ds.Token.Line }
func (ds *DefaultStatement) Column() int          { return ds.Token.Column }

// Function definitions

// FunctionParameter represents a parameter in a function definition
type FunctionParameter struct {
	Name         *Identifier
	DefaultValue Expression
	Variadic     bool
}

// FunctionLiteral represents a function literal/expression
type FunctionLiteral struct {
	Token      Token // 'func' token
	Name       string
	Parameters []*FunctionParameter
	Body       *BlockStatement
}

func (fl *FunctionLiteral) expressionNode()      {}
func (fl *FunctionLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FunctionLiteral) Line() int            { return fl.Token.Line }
func (fl *FunctionLiteral) Column() int          { return fl.Token.Column }

// FunctionDeclaration represents a top-level function declaration
type FunctionDeclaration struct {
	Token      Token // 'func' token
	Name       *Identifier
	Parameters []*FunctionParameter
	Body       *BlockStatement
}

func (fd *FunctionDeclaration) statementNode()       {}
func (fd *FunctionDeclaration) TokenLiteral() string { return fd.Token.Literal }
func (fd *FunctionDeclaration) Line() int            { return fd.Token.Line }
func (fd *FunctionDeclaration) Column() int          { return fd.Token.Column }

// Expressions

// PrefixExpression represents a prefix operator expression (e.g., !x, -y)
type PrefixExpression struct {
	Token    Token // The operator token
	Operator string
	Right    Expression
}

func (pe *PrefixExpression) expressionNode()      {}
func (pe *PrefixExpression) TokenLiteral() string { return pe.Token.Literal }
func (pe *PrefixExpression) Line() int            { return pe.Token.Line }
func (pe *PrefixExpression) Column() int          { return pe.Token.Column }

// InfixExpression represents an infix operator expression (e.g., x + y, a * b)
type InfixExpression struct {
	Token    Token // The operator token
	Left     Expression
	Operator string
	Right    Expression
}

func (ie *InfixExpression) expressionNode()      {}
func (ie *InfixExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *InfixExpression) Line() int            { return ie.Token.Line }
func (ie *InfixExpression) Column() int          { return ie.Token.Column }

// PostfixExpression represents a postfix operator expression (e.g., i++, i--)
type PostfixExpression struct {
	Token    Token // The operator token
	Left     Expression
	Operator string
}

func (pe *PostfixExpression) expressionNode()      {}
func (pe *PostfixExpression) TokenLiteral() string { return pe.Token.Literal }
func (pe *PostfixExpression) Line() int            { return pe.Token.Line }
func (pe *PostfixExpression) Column() int          { return pe.Token.Column }

// CallExpression represents a function call expression
type CallExpression struct {
	Token     Token // '(' token
	Function  Expression
	Arguments []Expression
}

func (ce *CallExpression) expressionNode()      {}
func (ce *CallExpression) TokenLiteral() string { return ce.Token.Literal }
func (ce *CallExpression) Line() int            { return ce.Token.Line }
func (ce *CallExpression) Column() int          { return ce.Token.Column }

// ArrayLiteral represents an array literal
type ArrayLiteral struct {
	Token    Token // '[' token
	Elements []Expression
}

func (al *ArrayLiteral) expressionNode()      {}
func (al *ArrayLiteral) TokenLiteral() string { return al.Token.Literal }
func (al *ArrayLiteral) Line() int            { return al.Token.Line }
func (al *ArrayLiteral) Column() int          { return al.Token.Column }

// IndexExpression represents an array/index access expression (e.g., arr[0])
type IndexExpression struct {
	Token Token // '[' token
	Left  Expression
	Index Expression
}

func (ie *IndexExpression) expressionNode()      {}
func (ie *IndexExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *IndexExpression) Line() int            { return ie.Token.Line }
func (ie *IndexExpression) Column() int          { return ie.Token.Column }

// MapLiteral represents a map literal
type MapLiteral struct {
	Token Token // '{' token
	Pairs map[Expression]Expression
}

func (ml *MapLiteral) expressionNode()      {}
func (ml *MapLiteral) TokenLiteral() string { return ml.Token.Literal }
func (ml *MapLiteral) Line() int            { return ml.Token.Line }
func (ml *MapLiteral) Column() int          { return ml.Token.Column }

// MemberExpression represents a member access expression (e.g., obj.property)
type MemberExpression struct {
	Token  Token // '.' token
	Object Expression
	Member *Identifier
}

func (me *MemberExpression) expressionNode()      {}
func (me *MemberExpression) TokenLiteral() string { return me.Token.Literal }
func (me *MemberExpression) Line() int            { return me.Token.Line }
func (me *MemberExpression) Column() int          { return me.Token.Column }

// AssignmentExpression represents an assignment expression (e.g., x = 5, y += 3)
type AssignmentExpression struct {
	Token    Token // The assignment operator token
	Left     Expression
	Operator string
	Right    Expression
}

func (ae *AssignmentExpression) expressionNode()      {}
func (ae *AssignmentExpression) TokenLiteral() string { return ae.Token.Literal }
func (ae *AssignmentExpression) Line() int            { return ae.Token.Line }
func (ae *AssignmentExpression) Column() int          { return ae.Token.Column }

// TernaryExpression represents a ternary conditional expression (e.g., condition ? true : false)
type TernaryExpression struct {
	Token     Token // '?' token
	Condition Expression
	TrueExpr  Expression
	FalseExpr Expression
}

func (te *TernaryExpression) expressionNode()      {}
func (te *TernaryExpression) TokenLiteral() string { return te.Token.Literal }
func (te *TernaryExpression) Line() int            { return te.Token.Line }
func (te *TernaryExpression) Column() int          { return te.Token.Column }

// Class definitions

// ClassDeclaration represents a class declaration
type ClassDeclaration struct {
	Token       Token // 'class' token
	Name        *Identifier
	SuperClass  *Identifier
	Constructor *FunctionLiteral
	Methods     []*FunctionLiteral
}

func (cd *ClassDeclaration) statementNode()       {}
func (cd *ClassDeclaration) TokenLiteral() string { return cd.Token.Literal }
func (cd *ClassDeclaration) Line() int            { return cd.Token.Line }
func (cd *ClassDeclaration) Column() int          { return cd.Token.Column }

// NewExpression represents a new operator expression (e.g., new ClassName())
type NewExpression struct {
	Token  Token // 'new' token
	Class  Expression
	Args   []Expression
}

func (ne *NewExpression) expressionNode()      {}
func (ne *NewExpression) TokenLiteral() string { return ne.Token.Literal }
func (ne *NewExpression) Line() int            { return ne.Token.Line }
func (ne *NewExpression) Column() int          { return ne.Token.Column }

// ThisExpression represents a 'this' keyword expression
type ThisExpression struct {
	Token Token // 'this' token
}

func (te *ThisExpression) expressionNode()      {}
func (te *ThisExpression) TokenLiteral() string { return te.Token.Literal }
func (te *ThisExpression) Line() int            { return te.Token.Line }
func (te *ThisExpression) Column() int          { return te.Token.Column }

// SuperExpression represents a 'super' keyword expression
type SuperExpression struct {
	Token Token // 'super' token
}

func (se *SuperExpression) expressionNode()      {}
func (se *SuperExpression) TokenLiteral() string { return se.Token.Literal }
func (se *SuperExpression) Line() int            { return se.Token.Line }
func (se *SuperExpression) Column() int          { return se.Token.Column }

// Exception handling

// TryStatement represents a try-catch-finally statement
type TryStatement struct {
	Token     Token // 'try' token
	TryBlock  *BlockStatement
	Catch     *CatchStatement
	Finally   *FinallyStatement
}

func (ts *TryStatement) statementNode()       {}
func (ts *TryStatement) TokenLiteral() string { return ts.Token.Literal }
func (ts *TryStatement) Line() int            { return ts.Token.Line }
func (ts *TryStatement) Column() int          { return ts.Token.Column }

// CatchStatement represents a catch clause in a try statement
type CatchStatement struct {
	Token    Token // 'catch' token
	Param    *Identifier
	CatchBlock *BlockStatement
}

func (cs *CatchStatement) statementNode()       {}
func (cs *CatchStatement) TokenLiteral() string { return cs.Token.Literal }
func (cs *CatchStatement) Line() int            { return cs.Token.Line }
func (cs *CatchStatement) Column() int          { return cs.Token.Column }

// FinallyStatement represents a finally clause in a try statement
type FinallyStatement struct {
	Token        Token // 'finally' token
	FinallyBlock *BlockStatement
}

func (fs *FinallyStatement) statementNode()       {}
func (fs *FinallyStatement) TokenLiteral() string { return fs.Token.Literal }
func (fs *FinallyStatement) Line() int            { return fs.Token.Line }
func (fs *FinallyStatement) Column() int          { return fs.Token.Column }

// DeferStatement represents a defer statement
type DeferStatement struct {
	Token Token // 'defer' token
	Call  *CallExpression
}

func (ds *DeferStatement) statementNode()       {}
func (ds *DeferStatement) TokenLiteral() string { return ds.Token.Literal }
func (ds *DeferStatement) Line() int            { return ds.Token.Line }
func (ds *DeferStatement) Column() int          { return ds.Token.Column }

// ThrowStatement represents a throw statement
type ThrowStatement struct {
	Token Token // 'throw' token
	Value Expression
}

func (ts *ThrowStatement) statementNode()       {}
func (ts *ThrowStatement) TokenLiteral() string { return ts.Token.Literal }
func (ts *ThrowStatement) Line() int            { return ts.Token.Line }
func (ts *ThrowStatement) Column() int          { return ts.Token.Column }

// ImportSpec represents a single import specifier in an import statement
type ImportSpec struct {
	Name *Identifier // Name of the export from the module
	Alias *Identifier // Optional local alias name
}

// ImportStatement represents an import statement
type ImportStatement struct {
	Token Token // 'import' token
	Specs []*ImportSpec // List of import specifiers
	ModulePath *StringLiteral // Module path for namespace imports
	Namespace *Identifier // Optional namespace for import * as
	DefaultImport *Identifier // Optional default import name
}

func (is *ImportStatement) statementNode()       {}
func (is *ImportStatement) TokenLiteral() string { return is.Token.Literal }
func (is *ImportStatement) Line() int            { return is.Token.Line }
func (is *ImportStatement) Column() int          { return is.Token.Column }

// ExportSpec represents a single export specifier in an export statement
type ExportSpec struct {
	Name *Identifier // Name of the exported item
	Alias *Identifier // Optional alias name
}

// ExportStatement represents an export statement
type ExportStatement struct {
	Token Token // 'export' token
	Specs []*ExportSpec // List of export specifiers
	Declaration Statement // Optional declaration being exported (func, const, etc.)
	IsDefault bool // Whether this is a default export
}

func (es *ExportStatement) statementNode()       {}
func (es *ExportStatement) TokenLiteral() string { return es.Token.Literal }
func (es *ExportStatement) Line() int            { return es.Token.Line }
func (es *ExportStatement) Column() int          { return es.Token.Column }
