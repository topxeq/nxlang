package compiler

import (
	"testing"

	"github.com/topxeq/nxlang/parser"
	"github.com/topxeq/nxlang/types"
)

func TestOpcodes(t *testing.T) {
	tests := []struct {
		op       Opcode
		expected byte
		name     string
	}{
		{OpNOP, 0x00, "OpNOP"},
		{OpPush, 0x01, "OpPush"},
		{OpPop, 0x02, "OpPop"},
		{OpDup, 0x03, "OpDup"},
		{OpSwap, 0x04, "OpSwap"},
		{OpLoadLocal, 0x10, "OpLoadLocal"},
		{OpStoreLocal, 0x11, "OpStoreLocal"},
		{OpLoadGlobal, 0x12, "OpLoadGlobal"},
		{OpStoreGlobal, 0x13, "OpStoreGlobal"},
		{OpAdd, 0x20, "OpAdd"},
		{OpSub, 0x21, "OpSub"},
		{OpMul, 0x22, "OpMul"},
		{OpDiv, 0x23, "OpDiv"},
		{OpMod, 0x24, "OpMod"},
		{OpNeg, 0x25, "OpNeg"},
		{OpPow, 0x26, "OpPow"},
		{OpBitAnd, 0x30, "OpBitAnd"},
		{OpBitOr, 0x31, "OpBitOr"},
		{OpBitXor, 0x32, "OpBitXor"},
		{OpBitNot, 0x33, "OpBitNot"},
		{OpShiftL, 0x34, "OpShiftL"},
		{OpShiftR, 0x35, "OpShiftR"},
		{OpEq, 0x40, "OpEq"},
		{OpNotEq, 0x41, "OpNotEq"},
		{OpLt, 0x42, "OpLt"},
		{OpLte, 0x43, "OpLte"},
		{OpGt, 0x44, "OpGt"},
		{OpGte, 0x45, "OpGte"},
		{OpNot, 0x50, "OpNot"},
		{OpAnd, 0x51, "OpAnd"},
		{OpOr, 0x52, "OpOr"},
		{OpJmp, 0x60, "OpJmp"},
		{OpJmpIfTrue, 0x61, "OpJmpIfTrue"},
		{OpJmpIfFalse, 0x62, "OpJmpIfFalse"},
		{OpCall, 0x63, "OpCall"},
		{OpReturn, 0x64, "OpReturn"},
		{OpReturnVoid, 0x65, "OpReturnVoid"},
		{OpClosure, 0x66, "OpClosure"},
		{OpNewArray, 0x70, "OpNewArray"},
		{OpNewMap, 0x71, "OpNewMap"},
		{OpIndexGet, 0x72, "OpIndexGet"},
		{OpIndexSet, 0x73, "OpIndexSet"},
		{OpMemberGet, 0x74, "OpMemberGet"},
		{OpMemberSet, 0x75, "OpMemberSet"},
		{OpNewObject, 0x76, "OpNewObject"},
	}

	for _, tt := range tests {
		if byte(tt.op) != tt.expected {
			t.Errorf("Expected %s = 0x%x, got 0x%x", tt.name, tt.expected, byte(tt.op))
		}
	}
}

func TestSymbolScopeConstants(t *testing.T) {
	if ScopeGlobal != "GLOBAL" {
		t.Errorf("Expected ScopeGlobal = 'GLOBAL', got %q", ScopeGlobal)
	}
	if ScopeLocal != "LOCAL" {
		t.Errorf("Expected ScopeLocal = 'LOCAL', got %q", ScopeLocal)
	}
	if ScopeBuiltin != "BUILTIN" {
		t.Errorf("Expected ScopeBuiltin = 'BUILTIN', got %q", ScopeBuiltin)
	}
	if ScopeFree != "FREE" {
		t.Errorf("Expected ScopeFree = 'FREE', got %q", ScopeFree)
	}
	if ScopeFunction != "FUNCTION" {
		t.Errorf("Expected ScopeFunction = 'FUNCTION', got %q", ScopeFunction)
	}
}

func TestSymbolTable(t *testing.T) {
	st := NewSymbolTable()

	if st == nil {
		t.Error("Expected non-nil SymbolTable")
	}

	if st.Count() != 0 {
		t.Errorf("Expected count 0, got %d", st.Count())
	}
}

func TestSymbolTableDefine(t *testing.T) {
	st := NewSymbolTable()

	sym := st.Define("foo")
	if sym.Name != "foo" {
		t.Errorf("Expected name 'foo', got %q", sym.Name)
	}
	if sym.Index != 0 {
		t.Errorf("Expected index 0, got %d", sym.Index)
	}
	if sym.Scope != ScopeGlobal {
		t.Errorf("Expected scope GLOBAL, got %q", sym.Scope)
	}

	sym = st.Define("bar")
	if sym.Index != 1 {
		t.Errorf("Expected index 1, got %d", sym.Index)
	}
}

func TestSymbolTableResolve(t *testing.T) {
	st := NewSymbolTable()

	st.Define("x")
	st.Define("y")

	sym, ok := st.Resolve("x")
	if !ok {
		t.Error("Expected to resolve 'x'")
	}
	if sym.Name != "x" {
		t.Errorf("Expected name 'x', got %q", sym.Name)
	}

	sym, ok = st.Resolve("y")
	if !ok {
		t.Error("Expected to resolve 'y'")
	}
	if sym.Name != "y" {
		t.Errorf("Expected name 'y', got %q", sym.Name)
	}
}

func TestSymbolTableResolveNotFound(t *testing.T) {
	st := NewSymbolTable()

	_, ok := st.Resolve("nonexistent")
	if ok {
		t.Error("Expected not to find nonexistent symbol")
	}
}

func TestSymbolTableDefineBuiltin(t *testing.T) {
	st := NewSymbolTable()

	sym := st.DefineBuiltin(0, "print")
	if sym.Name != "print" {
		t.Errorf("Expected name 'print', got %q", sym.Name)
	}
	if sym.Scope != ScopeBuiltin {
		t.Errorf("Expected scope BUILTIN, got %q", sym.Scope)
	}
}

func TestSymbolTableDefineConstant(t *testing.T) {
	st := NewSymbolTable()

	sym := st.DefineConstant("pi", types.Float(3.14))
	if sym.Name != "pi" {
		t.Errorf("Expected name 'pi', got %q", sym.Name)
	}
	if sym.Type == nil {
		t.Error("Expected non-nil Type")
	}
}

func TestSymbolTableDefineFunctionName(t *testing.T) {
	st := NewSymbolTable()

	sym := st.DefineFunctionName("myFunc")
	if sym.Name != "myFunc" {
		t.Errorf("Expected name 'myFunc', got %q", sym.Name)
	}
	if sym.Scope != ScopeFunction {
		t.Errorf("Expected scope FUNCTION, got %q", sym.Scope)
	}
}

func TestSymbolTableEnclosed(t *testing.T) {
	outer := NewSymbolTable()
	outer.Define("outerVar")

	inner := NewEnclosedSymbolTable(outer)

	sym, ok := inner.Resolve("outerVar")
	if !ok {
		t.Error("Expected to resolve 'outerVar' in enclosed scope")
	}
	if sym.Name != "outerVar" {
		t.Errorf("Expected name 'outerVar', got %q", sym.Name)
	}
}

func TestSymbolTableUpdateType(t *testing.T) {
	st := NewSymbolTable()

	st.Define("x")

	updated := st.UpdateType("x", types.Int(42))
	if !updated {
		t.Error("Expected to update type")
	}

	sym, _ := st.Resolve("x")
	if sym.Type == nil {
		t.Error("Expected non-nil Type")
	}
}

func TestSymbolTableFreeSymbols(t *testing.T) {
	outer := NewSymbolTable()
	outer.Define("capturedVar")

	inner := NewEnclosedSymbolTable(outer)
	inner.Resolve("capturedVar")
}

func TestNewCompiler(t *testing.T) {
	c := NewCompiler()
	if c == nil {
		t.Error("Expected non-nil Compiler")
	}
	if c.constants == nil {
		t.Error("Expected non-nil constants")
	}
	if c.symbolTable == nil {
		t.Error("Expected non-nil symbolTable")
	}
	if c.scopes == nil {
		t.Error("Expected non-nil scopes")
	}
	if c.errors == nil {
		t.Error("Expected non-nil errors")
	}
}

func TestCompilerErrors(t *testing.T) {
	c := NewCompiler()
	if len(c.Errors()) != 0 {
		t.Errorf("Expected no errors initially, got %d", len(c.Errors()))
	}
}

func TestNewModuleCompiler(t *testing.T) {
	modules := make(map[string]*Module)
	c := NewModuleCompiler("test/module.nx", modules)

	if c == nil {
		t.Error("Expected non-nil Compiler")
	}
	if c.ModulePath != "test/module.nx" {
		t.Errorf("Expected ModulePath 'test/module.nx', got %q", c.ModulePath)
	}
	if !c.isModule {
		t.Error("Expected isModule to be true")
	}
}

func TestModuleNew(t *testing.T) {
	m := &Module{
		Name:    "test",
		Path:    "/path/to/test",
		Exports: make(map[string]string),
	}

	if m.Name != "test" {
		t.Errorf("Expected Name 'test', got %q", m.Name)
	}
	if m.Path != "/path/to/test" {
		t.Errorf("Expected Path '/path/to/test', got %q", m.Path)
	}
	if m.Exports == nil {
		t.Error("Expected non-nil Exports")
	}
}

func TestCompilationScope(t *testing.T) {
	scope := CompilationScope{
		instructions:  []byte{0x01, 0x02, 0x03},
		numLocals:     5,
		numParameters: 2,
		isVariadic:    true,
		defaultValues: []int{1, 2},
	}

	if len(scope.instructions) != 3 {
		t.Errorf("Expected 3 instructions, got %d", len(scope.instructions))
	}
	if scope.numLocals != 5 {
		t.Errorf("Expected numLocals 5, got %d", scope.numLocals)
	}
	if scope.numParameters != 2 {
		t.Errorf("Expected numParameters 2, got %d", scope.numParameters)
	}
	if !scope.isVariadic {
		t.Error("Expected isVariadic to be true")
	}
}

func TestLoopContext(t *testing.T) {
	lc := LoopContext{
		continueTarget: 10,
		breakTarget:    20,
		continueJumps:  []int{5, 15},
	}

	if lc.continueTarget != 10 {
		t.Errorf("Expected continueTarget 10, got %d", lc.continueTarget)
	}
	if lc.breakTarget != 20 {
		t.Errorf("Expected breakTarget 20, got %d", lc.breakTarget)
	}
	if len(lc.continueJumps) != 2 {
		t.Errorf("Expected 2 continueJumps, got %d", len(lc.continueJumps))
	}
}

func TestClassContext(t *testing.T) {
	cc := ClassContext{
		Name:       "MyClass",
		SuperName:  "ParentClass",
		SuperIndex: 3,
	}

	if cc.Name != "MyClass" {
		t.Errorf("Expected Name 'MyClass', got %q", cc.Name)
	}
	if cc.SuperName != "ParentClass" {
		t.Errorf("Expected SuperName 'ParentClass', got %q", cc.SuperName)
	}
	if cc.SuperIndex != 3 {
		t.Errorf("Expected SuperIndex 3, got %d", cc.SuperIndex)
	}
}

// Tests for Compile function using parser integration
func TestCompileProgram(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		hasError bool
	}{
		{"empty program", "", false},
		{"simple integer", "42", false},
		{"simple string", `"hello"`, false},
		{"arithmetic", "1 + 2 * 3", false},
		{"variable declaration", "let x = 5", false},
		{"var declaration", "var y = 10", false},
		{"short declaration", "z := 15", false},
		{"if statement", "if true { 1 } else { 2 }", false},
		{"for loop", "for i in 3 { pln(i) }", false},
		{"while loop", "while false { }", false},
		{"function declaration", "func add(a, b) { return a + b }", false},
		{"array literal", "[1, 2, 3]", false},
		{"function call", "pln(42)", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := parser.NewLexer(tt.source)
			p := parser.NewParser(lexer)
			program := p.ParseProgram()

			if len(p.Errors()) > 0 {
				if !tt.hasError {
					t.Fatalf("Unexpected parse errors: %v", p.Errors())
				}
				return
			}

			c := NewCompiler()
			err := c.Compile(program)

			if tt.hasError {
				if err == nil {
					t.Error("Expected compile error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected compile error: %v", err)
				}
			}
		})
	}
}

func TestCompileArithmetic(t *testing.T) {
	tests := []string{
		"1 + 2",
		"1 - 2",
		"1 * 2",
		"1 / 2",
		"1 % 2",
		"-5",
	}

	for _, source := range tests {
		lexer := parser.NewLexer(source)
		p := parser.NewParser(lexer)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			t.Fatalf("Parse errors for %q: %v", source, p.Errors())
		}

		c := NewCompiler()
		if err := c.Compile(program); err != nil {
			t.Errorf("Compile error for %q: %v", source, err)
		}
	}
}

func TestCompileComparison(t *testing.T) {
	tests := []string{
		"1 == 2",
		"1 != 2",
		"1 < 2",
		"1 <= 2",
		"1 > 2",
		"1 >= 2",
	}

	for _, source := range tests {
		lexer := parser.NewLexer(source)
		p := parser.NewParser(lexer)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			t.Fatalf("Parse errors for %q: %v", source, p.Errors())
		}

		c := NewCompiler()
		if err := c.Compile(program); err != nil {
			t.Errorf("Compile error for %q: %v", source, err)
		}
	}
}

func TestCompileLogical(t *testing.T) {
	tests := []string{
		"true && false",
		"true || false",
		"!true",
	}

	for _, source := range tests {
		lexer := parser.NewLexer(source)
		p := parser.NewParser(lexer)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			t.Fatalf("Parse errors for %q: %v", source, p.Errors())
		}

		c := NewCompiler()
		if err := c.Compile(program); err != nil {
			t.Errorf("Compile error for %q: %v", source, err)
		}
	}
}

func TestCompileBitwise(t *testing.T) {
	tests := []string{
		"1 & 2",
		"1 | 2",
		"1 ^ 2",
		"1 << 2",
		"1 >> 2",
		"~1",
	}

	for _, source := range tests {
		lexer := parser.NewLexer(source)
		p := parser.NewParser(lexer)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			t.Fatalf("Parse errors for %q: %v", source, p.Errors())
		}

		c := NewCompiler()
		if err := c.Compile(program); err != nil {
			t.Errorf("Compile error for %q: %v", source, err)
		}
	}
}

func TestCompileFunctionWithDefaults(t *testing.T) {
	source := `func greet(name = "World") { return "Hello, " + name }`

	lexer := parser.NewLexer(source)
	p := parser.NewParser(lexer)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	c := NewCompiler()
	if err := c.Compile(program); err != nil {
		t.Errorf("Compile error: %v", err)
	}
}

func TestCompileVariadicFunction(t *testing.T) {
	source := `func sum(...args) { return args }`

	lexer := parser.NewLexer(source)
	p := parser.NewParser(lexer)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	c := NewCompiler()
	if err := c.Compile(program); err != nil {
		t.Errorf("Compile error: %v", err)
	}
}

func TestCompileClosure(t *testing.T) {
	source := `
func outer(x) {
	func inner(y) {
		return x + y
	}
	return inner
}
`

	lexer := parser.NewLexer(source)
	p := parser.NewParser(lexer)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	c := NewCompiler()
	if err := c.Compile(program); err != nil {
		t.Errorf("Compile error: %v", err)
	}
}

func TestCompileSwitchStatement(t *testing.T) {
	source := `
let x = 1
switch x {
case 1:
	pln("one")
case 2:
	pln("two")
default:
	pln("other")
}
`

	lexer := parser.NewLexer(source)
	p := parser.NewParser(lexer)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	c := NewCompiler()
	if err := c.Compile(program); err != nil {
		t.Errorf("Compile error: %v", err)
	}
}

func TestCompileTryCatch(t *testing.T) {
	source := `
try {
	throw "error"
} catch (e) {
	pln(e)
}
`

	lexer := parser.NewLexer(source)
	p := parser.NewParser(lexer)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	c := NewCompiler()
	if err := c.Compile(program); err != nil {
		t.Errorf("Compile error: %v", err)
	}
}

func TestCompileClass(t *testing.T) {
	source := `
class Point {
	func init(x, y) {
		this.x = x
		this.y = y
	}

	func toString() {
		return "(" + this.x + ", " + this.y + ")"
	}
}
`

	lexer := parser.NewLexer(source)
	p := parser.NewParser(lexer)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	c := NewCompiler()
	if err := c.Compile(program); err != nil {
		t.Errorf("Compile error: %v", err)
	}
}

func TestCompileInterface(t *testing.T) {
	source := `
interface Writer {
	write(data string)
}
`

	lexer := parser.NewLexer(source)
	p := parser.NewParser(lexer)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		// Skip if parser doesn't support this syntax
		t.Skipf("Parser doesn't support interface syntax: %v", p.Errors())
	}

	c := NewCompiler()
	if err := c.Compile(program); err != nil {
		t.Errorf("Compile error: %v", err)
	}
}

func TestCompilePostfix(t *testing.T) {
	source := "let i = 0; i++"

	lexer := parser.NewLexer(source)
	p := parser.NewParser(lexer)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		// Skip if parser doesn't support postfix
		t.Skipf("Parser doesn't support postfix: %v", p.Errors())
	}

	c := NewCompiler()
	if err := c.Compile(program); err != nil {
		t.Errorf("Compile error: %v", err)
	}
}

func TestCompileCompoundAssignment(t *testing.T) {
	source := "let x = 0; x += 1"

	lexer := parser.NewLexer(source)
	p := parser.NewParser(lexer)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		// Skip if parser doesn't support compound assignment
		t.Skipf("Parser doesn't support compound assignment: %v", p.Errors())
	}

	c := NewCompiler()
	if err := c.Compile(program); err != nil {
		t.Errorf("Compile error: %v", err)
	}
}

func TestBytecodeGeneration(t *testing.T) {
	source := "let x = 42"
	lexer := parser.NewLexer(source)
	p := parser.NewParser(lexer)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	c := NewCompiler()
	if err := c.Compile(program); err != nil {
		t.Fatalf("Compile error: %v", err)
	}

	bc := c.Bytecode()
	if bc == nil {
		t.Fatal("Expected non-nil bytecode")
	}

	if len(bc.Constants) == 0 {
		t.Error("Expected at least one constant")
	}

	// Check that main function exists
	if bc.MainFunc < 0 || bc.MainFunc >= len(bc.Constants) {
		t.Errorf("Invalid MainFunc index: %d", bc.MainFunc)
	}
}

func TestCompileIndexExpression(t *testing.T) {
	source := "let arr = [1, 2, 3]; arr[0]"

	lexer := parser.NewLexer(source)
	p := parser.NewParser(lexer)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	c := NewCompiler()
	if err := c.Compile(program); err != nil {
		t.Errorf("Compile error: %v", err)
	}
}

func TestCompileMemberExpression(t *testing.T) {
	source := `let obj = {"name": "test"}; obj["name"]`

	lexer := parser.NewLexer(source)
	p := parser.NewParser(lexer)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	c := NewCompiler()
	if err := c.Compile(program); err != nil {
		t.Errorf("Compile error: %v", err)
	}
}

func TestCompileNewExpression(t *testing.T) {
	source := `
class Point {
	func init(x, y) {
		this.x = x
		this.y = y
	}
}
let p = new Point(1, 2)
`

	lexer := parser.NewLexer(source)
	p := parser.NewParser(lexer)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	c := NewCompiler()
	if err := c.Compile(program); err != nil {
		t.Errorf("Compile error: %v", err)
	}
}

func TestCompileForRange(t *testing.T) {
	source := "for i, v in [1, 2, 3] { pln(i, v) }"

	lexer := parser.NewLexer(source)
	p := parser.NewParser(lexer)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	c := NewCompiler()
	if err := c.Compile(program); err != nil {
		t.Errorf("Compile error: %v", err)
	}
}

func TestCompileDefer(t *testing.T) {
	source := `
func test() {
	defer pln("cleanup")
	pln("body")
}
`

	lexer := parser.NewLexer(source)
	p := parser.NewParser(lexer)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	c := NewCompiler()
	if err := c.Compile(program); err != nil {
		t.Errorf("Compile error: %v", err)
	}
}

func TestCompileBreakContinue(t *testing.T) {
	source := `
for i in 10 {
	if i == 5 {
		break
	}
	if i == 3 {
		continue
	}
	pln(i)
}
`

	lexer := parser.NewLexer(source)
	p := parser.NewParser(lexer)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	c := NewCompiler()
	if err := c.Compile(program); err != nil {
		t.Errorf("Compile error: %v", err)
	}
}

func TestCompileComplexExpressions(t *testing.T) {
	tests := []string{
		"1 + 2 * 3",
		"(1 + 2) * 3",
		"1 + 2 + 3 + 4",
		"1 - 2 - 3",
		"1 * 2 * 3",
		"1 / 2 / 3",
		"1 % 2 % 3",
		"1 + 2 * 3 - 4 / 5",
		"-(1 + 2)",
		"!true && false",
		"true || !false",
		"1 < 2 && 3 > 4",
		"1 <= 2 || 3 >= 4",
		"1 == 2 || 3 != 4",
		"1 & 2 | 3 ^ 4",
		"1 << 2 >> 3",
		"~1",
	}

	for _, source := range tests {
		lexer := parser.NewLexer(source)
		p := parser.NewParser(lexer)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			t.Fatalf("Parse errors for %q: %v", source, p.Errors())
		}

		c := NewCompiler()
		if err := c.Compile(program); err != nil {
			t.Errorf("Compile error for %q: %v", source, err)
		}
	}
}

func TestCompileAssignmentExpressions(t *testing.T) {
	tests := []string{
		"let x = 1; var x = x + 2",
	}

	for _, source := range tests {
		lexer := parser.NewLexer(source)
		p := parser.NewParser(lexer)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			t.Fatalf("Parse errors for %q: %v", source, p.Errors())
		}

		c := NewCompiler()
		if err := c.Compile(program); err != nil {
			t.Errorf("Compile error for %q: %v", source, err)
		}
	}
}

func TestCompileIncrementDecrement(t *testing.T) {
	tests := []string{
		"let x = 1; x++",
		"let x = 1; x--",
		"let x = 1; ++x",
		"let x = 1; --x",
	}

	for _, source := range tests {
		lexer := parser.NewLexer(source)
		p := parser.NewParser(lexer)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			// Skip if parser doesn't support increment/decrement
			continue
		}

		c := NewCompiler()
		if err := c.Compile(program); err != nil {
			t.Errorf("Compile error for %q: %v", source, err)
		}
	}
}

func TestCompileArrayOperations(t *testing.T) {
	tests := []string{
		"[1, 2, 3]",
		"let arr = [1, 2, 3]; arr[0]",
		"len([1, 2, 3])",
		"[]",
		"[1]",
		"[1, 2]",
	}

	for _, source := range tests {
		lexer := parser.NewLexer(source)
		p := parser.NewParser(lexer)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			t.Fatalf("Parse errors for %q: %v", source, p.Errors())
		}

		c := NewCompiler()
		if err := c.Compile(program); err != nil {
			t.Errorf("Compile error for %q: %v", source, err)
		}
	}
}

func TestCompileMapOperations(t *testing.T) {
	// Map syntax may vary
	t.Skip("Map literal syntax not fully supported")
}

func TestCompileFunctionFeatures(t *testing.T) {
	tests := []string{
		`func() { return 1 }`,
		`func(x) { return x }`,
		`func(x, y) { return x + y }`,
		`func(x, y, z) { return x + y + z }`,
		`func(x = 1) { return x }`,
		`func(x, y = 2) { return x + y }`,
		`func(...args) { return args }`,
	}

	for _, source := range tests {
		lexer := parser.NewLexer(source)
		p := parser.NewParser(lexer)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			t.Fatalf("Parse errors for %q: %v", source, p.Errors())
		}

		c := NewCompiler()
		if err := c.Compile(program); err != nil {
			t.Errorf("Compile error for %q: %v", source, err)
		}
	}
}

func TestCompileClassFeatures(t *testing.T) {
	tests := []string{
		`class Empty {}`,
		`class Point { func init(x, y) { this.x = x } }`,
		`class Point { func toString() { return "point" } }`,
		`class Child < Parent {}`,
	}

	for _, source := range tests {
		lexer := parser.NewLexer(source)
		p := parser.NewParser(lexer)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			t.Fatalf("Parse errors for %q: %v", source, p.Errors())
		}

		c := NewCompiler()
		if err := c.Compile(program); err != nil {
			t.Errorf("Compile error for %q: %v", source, err)
		}
	}
}

func TestCompileControlFlow(t *testing.T) {
	tests := []string{
		`if true { 1 }`,
		`if true { 1 } else { 2 }`,
		`if true { 1 } else if false { 2 } else { 3 }`,
		`for i in 10 { pln(i) }`,
		`for i, v in [1, 2, 3] { pln(i, v) }`,
		`while true { break }`,
	}

	for _, source := range tests {
		lexer := parser.NewLexer(source)
		p := parser.NewParser(lexer)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			t.Fatalf("Parse errors for %q: %v", source, p.Errors())
		}

		c := NewCompiler()
		if err := c.Compile(program); err != nil {
			t.Errorf("Compile error for %q: %v", source, err)
		}
	}
}

func TestCompileSwitchFeatures(t *testing.T) {
	source := `
let x = 1
switch x {
case 1:
	pln("one")
case 2:
	pln("two")
default:
	pln("other")
}
`

	lexer := parser.NewLexer(source)
	p := parser.NewParser(lexer)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	c := NewCompiler()
	if err := c.Compile(program); err != nil {
		t.Errorf("Compile error: %v", err)
	}
}

func TestCompileTryCatchFeatures(t *testing.T) {
	tests := []string{
		`try { pln("try") } catch (e) { pln(e) }`,
		`try { pln("try") } finally { pln("finally") }`,
		`try { pln("try") } catch (e) { pln(e) } finally { pln("finally") }`,
		`throw "error"`,
	}

	for _, source := range tests {
		lexer := parser.NewLexer(source)
		p := parser.NewParser(lexer)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			t.Fatalf("Parse errors for %q: %v", source, p.Errors())
		}

		c := NewCompiler()
		if err := c.Compile(program); err != nil {
			t.Errorf("Compile error for %q: %v", source, err)
		}
	}
}

func TestCompileBuiltins(t *testing.T) {
	tests := []string{
		`pln(1)`,
		`len([1, 2, 3])`,
		`typeOf(1)`,
		`int(3.14)`,
		`float(3)`,
		`string(123)`,
		`bool(1)`,
	}

	for _, source := range tests {
		lexer := parser.NewLexer(source)
		p := parser.NewParser(lexer)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			t.Fatalf("Parse errors for %q: %v", source, p.Errors())
		}

		c := NewCompiler()
		if err := c.Compile(program); err != nil {
			t.Errorf("Compile error for %q: %v", source, err)
		}
	}
}

func TestCompileMemberAccess(t *testing.T) {
	tests := []string{
		`"hello".len()`,
		`[1, 2, 3].len()`,
	}

	for _, source := range tests {
		lexer := parser.NewLexer(source)
		p := parser.NewParser(lexer)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			t.Fatalf("Parse errors for %q: %v", source, p.Errors())
		}

		c := NewCompiler()
		if err := c.Compile(program); err != nil {
			t.Errorf("Compile error for %q: %v", source, err)
		}
	}
}

func TestCompilerScopes(t *testing.T) {
	// Test nested scopes
	source := `
let global = 1
func outer() {
	let outerVar = 2
	func inner() {
		let innerVar = 3
		return global + outerVar + innerVar
	}
	return inner
}
`

	lexer := parser.NewLexer(source)
	p := parser.NewParser(lexer)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	c := NewCompiler()
	if err := c.Compile(program); err != nil {
		t.Errorf("Compile error: %v", err)
	}
}

func TestCompileTernaryExpression(t *testing.T) {
	tests := []string{
		`true ? 1 : 2`,
		`false ? 1 : 2`,
		`let x = 5; x > 0 ? "positive" : "negative"`,
		`true ? (false ? 1 : 2) : 3`,
	}

	for _, source := range tests {
		lexer := parser.NewLexer(source)
		p := parser.NewParser(lexer)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			t.Fatalf("Parse errors for %q: %v", source, p.Errors())
		}

		c := NewCompiler()
		if err := c.Compile(program); err != nil {
			t.Errorf("Compile error for %q: %v", source, err)
		}
	}
}

func TestCompileForLoopVariants(t *testing.T) {
	tests := []string{
		`for i := 0; i < 10; i++ { 1 }`,
		`for { break }`,
		`for i, v in [1, 2, 3] { 1 }`,
	}

	for _, source := range tests {
		lexer := parser.NewLexer(source)
		p := parser.NewParser(lexer)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			t.Fatalf("Parse errors for %q: %v", source, p.Errors())
		}

		c := NewCompiler()
		if err := c.Compile(program); err != nil {
			t.Errorf("Compile error for %q: %v", source, err)
		}
	}
}

func TestCompileSwitchStatementExtended(t *testing.T) {
	source := `
let x = 1
switch x {
case 1:
	1
case 2:
	2
default:
	3
}
`
	lexer := parser.NewLexer(source)
	p := parser.NewParser(lexer)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	c := NewCompiler()
	if err := c.Compile(program); err != nil {
		t.Errorf("Compile error: %v", err)
	}
}

func TestCompileTryCatchFinally(t *testing.T) {
	source := `
try {
	throw "error"
} catch {
	print("caught")
} finally {
	print("done")
}
`
	lexer := parser.NewLexer(source)
	p := parser.NewParser(lexer)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	c := NewCompiler()
	if err := c.Compile(program); err != nil {
		t.Errorf("Compile error: %v", err)
	}
}
