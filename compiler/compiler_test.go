package compiler

import (
	"testing"

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
