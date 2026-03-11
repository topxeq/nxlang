package vm

import (
	"testing"

	"github.com/topxeq/nxlang/compiler"
	"github.com/topxeq/nxlang/parser"
	"github.com/topxeq/nxlang/types"
)

func runSource(t *testing.T, source string) types.Object {
	lexer := parser.NewLexer(source)
	p := parser.NewParser(lexer)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	comp := compiler.NewCompiler()
	if err := comp.Compile(program); err != nil {
		t.Fatalf("Compile error: %v", err)
	}

	bc := comp.Bytecode()
	vm := NewVM(bc)

	if err := vm.Run(); err != nil {
		t.Fatalf("Runtime error: %v", err)
	}

	if vm.Stack().Size() > 0 {
		return vm.Stack().Peek()
	}
	return types.UndefinedValue
}

func TestHelloWorld(t *testing.T) {
	source := `pln("Hello World")`
	result := runSource(t, source)
	if result == nil {
		t.Error("Expected undefined, got nil")
	}
}

func TestArithmeticOperations(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"1 + 2", types.Int(3)},
		{"1 - 2", types.Int(-1)},
		{"2 * 3", types.Int(6)},
		{"6 / 2", types.Int(3)},
		{"7 / 2", types.Float(3.5)},
		{"10 % 3", types.Int(1)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestVariableDeclaration(t *testing.T) {
	source := `
let a = 1
var b = 2
c := 3
a + b + c
`
	result := runSource(t, source)
	if !result.Equals(types.Int(6)) {
		t.Errorf("Expected 6, got %v", result)
	}
}

func TestRecursiveFunction(t *testing.T) {
	source := `
func fib(n) {
    if n <= 1 {
        return n
    }
    return fib(n-1) + fib(n-2)
}
fib(8)
`
	result := runSource(t, source)
	if !result.Equals(types.Int(21)) {
		t.Errorf("Expected 21, got %v", result)
	}
}

func TestForLoop(t *testing.T) {
	source := `
let sum = 0
for i in 5 {
    sum = sum + i
}
sum
`
	result := runSource(t, source)
	if !result.Equals(types.Int(10)) {
		t.Errorf("Expected 10, got %v", result)
	}
}

func TestArrayOperations(t *testing.T) {
	source := `
let arr = [1, 2, 3, 4, 5]
arr[0] + arr[2]
`
	result := runSource(t, source)
	if !result.Equals(types.Int(4)) {
		t.Errorf("Expected 4, got %v", result)
	}
}

func TestMapOperations(t *testing.T) {
	source := `
let m = {"a": 1, "b": 2}
m["a"] + m["b"]
`
	result := runSource(t, source)
	if !result.Equals(types.Int(3)) {
		t.Errorf("Expected 3, got %v", result)
	}
}

func TestTypeFunctions(t *testing.T) {
	tests := []struct {
		source           string
		expectedType     string
		expectedTypeCode uint8
	}{
		{"typeOf(123)", "int", types.TypeInt},
		{"typeOf(3.14)", "float", types.TypeFloat},
		{"typeOf(true)", "bool", types.TypeBool},
		{"typeOf(\"hello\")", "string", types.TypeString},
		{"typeOf('A')", "char", types.TypeChar},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if result.ToStr() != tt.expectedType {
			t.Errorf("Source: %s, Expected type: %s, Got: %s", tt.source, tt.expectedType, result.ToStr())
		}
	}
}

func TestTypeConversion(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"int(3.14)", types.Int(3)},
		{"float(123)", types.Float(123)},
		{"string(123)", types.String("123")},
		{"bool(1)", types.Bool(true)},
		{"bool(0)", types.Bool(false)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestStaticMethods(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"int.parse(\"123\")", types.Int(123)},
		{"float.parse(\"3.14\")", types.Float(3.14)},
		{"string.parse(123)", types.String("123")},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestConstants(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"piC", types.Float(3.141592653589793)},
		{"eC", types.Float(2.718281828459045)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestCompileAndRunByteCode(t *testing.T) {
	source := `
let source1 = "let a = 1
let b = 2
return a + b"
let bc = compile(source1)
runByteCode(bc)
`
	result := runSource(t, source)
	if !result.Equals(types.Int(3)) {
		t.Errorf("Expected 3, got %v", result)
	}
}

func TestDivisionByZero(t *testing.T) {
	source := `
let x = 10
let y = 0
x / y
`

	lexer := parser.NewLexer(source)
	p := parser.NewParser(lexer)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	comp := compiler.NewCompiler()
	if err := comp.Compile(program); err != nil {
		t.Fatalf("Compile error: %v", err)
	}

	bc := comp.Bytecode()
	vm := NewVM(bc)

	err := vm.Run()
	if err == nil {
		t.Error("Expected division by zero error")
	}
}

func TestStringOperations(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"toUpper(\"hello\")", types.String("HELLO")},
		{"toLower(\"HELLO\")", types.String("hello")},
		{"trim(\"  hello  \")", types.String("hello")},
		{"len(\"hello\")", types.Int(5)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestMathFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"abs(-5)", func(r types.Object) bool { return r.Equals(types.Int(5)) }},
		{"sqrt(16)", func(r types.Object) bool { return r.Equals(types.Float(4)) }},
		{"pow(2, 3)", func(r types.Object) bool { return r.Equals(types.Float(8)) }},
		{"floor(3.7)", func(r types.Object) bool { return r.Equals(types.Float(3)) }},
		{"ceil(3.2)", func(r types.Object) bool { return r.Equals(types.Float(4)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}
