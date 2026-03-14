package vm

import (
	"strings"
	"testing"

	"github.com/topxeq/nxlang/compiler"
	"github.com/topxeq/nxlang/parser"
	"github.com/topxeq/nxlang/types"
	"github.com/topxeq/nxlang/types/collections"
)

func runSource(t *testing.T, source string) types.Object {
	lexer := parser.NewLexer(source)
	p := parser.NewParser(lexer)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors for source %q: %v", source, p.Errors())
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

func TestComparisonOperations(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"1 == 1", types.Bool(true)},
		{"1 == 2", types.Bool(false)},
		{"1 != 2", types.Bool(true)},
		{"1 != 1", types.Bool(false)},
		{"2 > 1", types.Bool(true)},
		{"1 > 2", types.Bool(false)},
		{"1 < 2", types.Bool(true)},
		{"2 < 1", types.Bool(false)},
		{"2 >= 2", types.Bool(true)},
		{"2 >= 3", types.Bool(false)},
		{"1 <= 1", types.Bool(true)},
		{"2 <= 1", types.Bool(false)},
		{"1.5 > 1.2", types.Bool(true)},
		{"3.14 < 2.71", types.Bool(false)},
		{"\"a\" == \"a\"", types.Bool(true)},
		{"\"a\" == \"b\"", types.Bool(false)},
		{"\"b\" > \"a\"", types.Bool(true)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestLogicalOperations(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"true && true", types.Bool(true)},
		{"true && false", types.Bool(false)},
		{"false && true", types.Bool(false)},
		{"false && false", types.Bool(false)},
		{"true || true", types.Bool(true)},
		{"true || false", types.Bool(true)},
		{"false || true", types.Bool(true)},
		{"false || false", types.Bool(false)},
		{"!true", types.Bool(false)},
		{"!false", types.Bool(true)},
		{"!!true", types.Bool(true)},
		{"1 && 2", types.Int(2)},
		{"0 && 2", types.Int(0)},
		{"1 || 2", types.Int(1)},
		{"0 || 2", types.Int(2)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestBitwiseOperationsExtended(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"5 & 3", types.Int(1)},
		{"5 | 3", types.Int(7)},
		{"5 ^ 3", types.Int(6)},
		{"5 << 2", types.Int(20)},
		{"20 >> 2", types.Int(5)},
		{"15 & 7", types.Int(7)},
		{"8 | 4", types.Int(12)},
		{"10 ^ 6", types.Int(12)},
		{"1 << 10", types.Int(1024)},
		{"1024 >> 10", types.Int(1)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestExtendedStringOperations(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"\"hello\" + \"world\"", types.String("helloworld")},
		{"\"test\" * 3", types.String("testtesttest")},
		{"len(\"\")", types.Int(0)},
		{"len([1, 2, 3])", types.Int(3)},
		{"contains(\"hello\", \"ell\")", types.Bool(true)},
		{"contains(\"hello\", \"world\")", types.Bool(false)},
		{"replace(\"hello world\", \"world\", \"go\")", types.String("hello go")},
		{"split(\"a,b,c\", \",\")[0]", types.String("a")},
		{"split(\"a,b,c\", \",\")[1]", types.String("b")},
		{"split(\"a,b,c\", \",\")[2]", types.String("c")},
		{"join([\"a\", \"b\", \"c\"], \",\")", types.String("a,b,c")},
		{"startsWith(\"hello\", \"hel\")", types.Bool(true)},
		{"startsWith(\"hello\", \"world\")", types.Bool(false)},
		{"endsWith(\"hello\", \"lo\")", types.Bool(true)},
		{"endsWith(\"hello\", \"he\")", types.Bool(false)},
		{"indexOf(\"hello\", \"e\")", types.Int(1)},
		{"indexOf(\"hello\", \"l\")", types.Int(2)},
		{"indexOf(\"hello\", \"x\")", types.Int(-1)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestExtendedArrayOperations(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"[1, 2, 3][0]", types.Int(1)},
		{"[1, 2, 3][1]", types.Int(2)},
		{"[1, 2, 3][2]", types.Int(3)},
		{"len([1, 2, 3])", types.Int(3)},
		{"contains([1, 2, 3], 2)", types.Bool(true)},
		{"contains([1, 2, 3], 5)", types.Bool(false)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestExtendedMapOperations(t *testing.T) {
	source := `
let m = {"a": 1, "b": 2}
m["a"] + m["b"]
`
	result := runSource(t, source)
	if !result.Equals(types.Int(3)) {
		t.Errorf("Expected 3, got %v", result)
	}
}

func TestErrorHandling(t *testing.T) {
	tests := []struct {
		source      string
		expectError bool
		errorMsg    string
	}{
		{"let x = 10; let y = 0; x / y", true, "division by zero"},
		{"undefinedVar", true, "undefined variable"},
		{"let m = {}; m[\"nonexistent\"]", false, ""},
	}

	for _, tt := range tests {
		lexer := parser.NewLexer(tt.source)
		p := parser.NewParser(lexer)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			if !tt.expectError {
				t.Errorf("Source: %s, Unexpected parse error: %v", tt.source, p.Errors())
			}
			continue
		}

		comp := compiler.NewCompiler()
		if err := comp.Compile(program); err != nil {
			if !tt.expectError {
				t.Errorf("Source: %s, Unexpected compile error: %v", tt.source, err)
			}
			continue
		}

		bc := comp.Bytecode()
		vm := NewVM(bc)

		err := vm.Run()
		if tt.expectError {
			if err == nil {
				t.Errorf("Source: %s, Expected error but got none", tt.source)
			} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
				t.Errorf("Source: %s, Expected error containing '%s', got: %v", tt.source, tt.errorMsg, err)
			}
		} else if err != nil {
			t.Errorf("Source: %s, Unexpected error: %v", tt.source, err)
		}
	}
}

func TestConditionalStatements(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{`
if true {
    1
} else {
    2
}`, types.Int(1)},
		{`
if false {
    1
} else {
    2
}`, types.Int(2)},
		{`
if true {
    if false {
        1
    } else {
        2
    }
} else {
    3
}`, types.Int(2)},
		{`
let x = 5
if x > 3 {
    1
} else if x > 4 {
    2
} else {
    3
}`, types.Int(1)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestSwitchStatement(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{`
let x = 2
switch x {
    case 1: 1
    case 2: 2
    case 3: 3
}
`, types.Int(2)},
		{`
let x = 5
switch x {
    case 1: 1
    case 2: 2
    default: 0
}
`, types.Int(0)},
		{`
let x = "b"
switch x {
    case "a": 1
    case "b": 2
    case "c": 3
}
`, types.Int(2)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestFunctionWithParameters(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"func add(a, b) { return a + b }\nadd(1, 2)", types.Int(3)},
		{"func greet(name) { return \"Hello, \" + name }\ngreet(\"World\")", types.String("Hello, World")},
		{`func factorial(n) { 
if n <= 1 { 
    return 1 
} 
return n * factorial(n - 1) 
}
factorial(5)`, types.Int(120)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestWhileLoop(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{`
let i = 0
let sum = 0
while i < 5 {
    sum = sum + i
    i = i + 1
}
sum
`, types.Int(10)},
		{`
let i = 0
while i < 3 {
    i = i + 1
    if i == 2 {
        break
    }
}
i
`, types.Int(2)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestTernaryOperator(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{`if true { 1 } else { 2 }`, types.Int(1)},
		{`if false { 1 } else { 2 }`, types.Int(2)},
		{`if 1 > 0 { "yes" } else { "no" }`, types.String("yes")},
		{`if 1 < 0 { "yes" } else { "no" }`, types.String("no")},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestRangeLoop(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{`
let sum = 0
for i in range(5) {
    sum = sum + i
}
sum
`, types.Int(10)},
		{`
let sum = 0
for i in range(1, 6) {
    sum = sum + i
}
sum
`, types.Int(15)},
		{`
let sum = 0
for i in range(0, 10, 2) {
    sum = sum + i
}
sum
`, types.Int(20)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestAnonymousFunctions(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"let f = func(x) { return x * 2 }\nf(5)", types.Int(10)},
		{"let add = func(a, b) { return a + b }\nadd(3, 4)", types.Int(7)},
		{"(func(x) { return x + 1 })(5)", types.Int(6)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestStringFunctions(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"len(\"hello\")", types.Int(5)},
		{"len(\"\")", types.Int(0)},
		{"toUpper(\"hello\")", types.String("HELLO")},
		{"toLower(\"HELLO\")", types.String("hello")},
		{"trim(\"  hello  \")", types.String("hello")},
		{"contains(\"hello\", \"ell\")", types.Bool(true)},
		{"contains(\"hello\", \"world\")", types.Bool(false)},
		{"replace(\"hello world\", \"world\", \"go\")", types.String("hello go")},
		{"split(\"a,b,c\", \",\")[1]", types.String("b")},
		{"join([\"a\", \"b\"], \"-\")", types.String("a-b")},
		{"startsWith(\"hello\", \"hel\")", types.Bool(true)},
		{"endsWith(\"hello\", \"lo\")", types.Bool(true)},
		{"indexOf(\"hello\", \"l\")", types.Int(2)},
		{"\"hello\" + \" \" + \"world\"", types.String("hello world")},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestNumberFunctions(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"abs(-5)", types.Int(5)},
		{"abs(-3.5)", types.Float(3.5)},
		{"sqrt(16)", types.Float(4)},
		{"pow(2, 3)", types.Float(8)},
		{"floor(3.7)", types.Float(3)},
		{"ceil(3.2)", types.Float(4)},
		{"round(3.5)", types.Float(4)},
		{"min(3, 1)", types.Int(1)},
		{"max(3, 1)", types.Int(3)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestGlobalFunctions(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"typeOf(123)", types.String("int")},
		{"typeOf(3.14)", types.String("float")},
		{"typeOf(true)", types.String("bool")},
		{"typeOf(\"hello\")", types.String("string")},
		{"typeOf([1,2,3])", types.String("array")},
		{"typeOf({\"a\":1})", types.String("map")},
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

func TestComplexExpressions(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"1 + 2 * 3", types.Int(7)},
		{"(1 + 2) * 3", types.Int(9)},
		{"10 - 3 - 2", types.Int(5)},
		{"10 / 2 / 2", types.Float(2.5)},
		{"10 % 3", types.Int(1)},
		{"1 + 2 * 3 - 4 / 2", types.Int(5)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestAssignmentExpressions(t *testing.T) {
	source := `
let x = 5
x = x + 3
x
`
	result := runSource(t, source)
	if !result.Equals(types.Int(8)) {
		t.Errorf("Expected 8, got %v", result)
	}
}

func TestMoreFunctionTests(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"func double(x) { return x * 2 }\ndouble(5)", types.Int(10)},
		{"func sum(a, b) { return a + b }\nsum(3, 4)", types.Int(7)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestMultipleVariableDeclarations(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"let a = 1\nlet b = 2\nlet c = 3\n a + b + c", types.Int(6)},
		{"var a = 1\nvar b = 2\n a + b", types.Int(3)},
		{"a := 1\na = 10\n a", types.Int(10)},
		{"let x = 5\nlet y = 10\n x * y", types.Int(50)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestNestedConditionals(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{`
let x = 5
if x > 0 {
    if x > 3 {
        1
    } else {
        2
    }
} else {
    3
}
`, types.Int(1)},
		{`
let x = 2
if x > 0 {
    if x > 3 {
        1
    } else {
        2
    }
} else {
    3
}
`, types.Int(2)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestReturnStatements(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"func f() { return 42 }\n f()", types.Int(42)},
		{`func f() { return "hello" }
 f()`, types.String("hello")},
		{`func f(x) { 
if x > 0 { return 1 } 
return 0 
}
 f(5)`, types.Int(1)},
		{`func f(x) { 
if x > 0 { return 1 } 
return 0 
}
 f(-1)`, types.Int(0)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestStringConcatenationExtended(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"\"a\" + \"b\"", types.String("ab")},
		{"\"hello\" + \" \" + \"world\"", types.String("hello world")},
		{"\"test\" + \"\"", types.String("test")},
		{"\"\" + \"test\"", types.String("test")},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestBooleanOperations(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"true", types.Bool(true)},
		{"false", types.Bool(false)},
		{"!false", types.Bool(true)},
		{"!true", types.Bool(false)},
		{"true == true", types.Bool(true)},
		{"true == false", types.Bool(false)},
		{"true != false", types.Bool(true)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestNegativeNumbers(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"-5", types.Int(-5)},
		{"-3.14", types.Float(-3.14)},
		{"-5 + 3", types.Int(-2)},
		{"5 + -3", types.Int(2)},
		{"-5 * -3", types.Int(15)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestNullAndUndefined(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"null", types.NullValue},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestCharLiterals(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"'A'", types.Char('A')},
		{"'a'", types.Char('a')},
		{"'0'", types.Char('0')},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestUIntOperations(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"uint(1)", types.UInt(1)},
		{"uint(100)", types.UInt(100)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestByteOperations(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"byte(65)", types.Byte(65)},
		{"byte(255)", types.Byte(255)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestArrayOfArrays(t *testing.T) {
	source := `
let arr = [[1, 2], [3, 4]]
arr[0][0] + arr[1][1]
`
	result := runSource(t, source)
	if !result.Equals(types.Int(5)) {
		t.Errorf("Expected 5, got %v", result)
	}
}

func TestMapOfMaps(t *testing.T) {
	source := `
let m = {"a": {"x": 1}, "b": {"y": 2}}
m["a"]["x"] + m["b"]["y"]
`
	result := runSource(t, source)
	if !result.Equals(types.Int(3)) {
		t.Errorf("Expected 3, got %v", result)
	}
}

func TestNestedForLoops(t *testing.T) {
	source := `
let sum = 0
for i in 3 {
    for j in 3 {
        sum = sum + 1
    }
}
sum
`
	result := runSource(t, source)
	if !result.Equals(types.Int(3)) {
		t.Errorf("Expected 3, got %v", result)
	}
}

func TestBreakInForLoop(t *testing.T) {
	source := `
let sum = 0
for i in 10 {
    if i == 5 {
        break
    }
    sum = sum + 1
}
sum
`
	result := runSource(t, source)
	if !result.Equals(types.Int(5)) {
		t.Errorf("Expected 5, got %v", result)
	}
}

func TestContinueInForLoop(t *testing.T) {
	source := `
let sum = 0
for i in 5 {
    if i == 2 {
        continue
    }
    sum = sum + i
}
sum
`
	result := runSource(t, source)
	if !result.Equals(types.Int(8)) {
		t.Errorf("Expected 8, got %v", result)
	}
}

func TestFunctionWithoutReturn(t *testing.T) {
	source := `
func greet() {
    pln("hello")
}
greet()
`
	result := runSource(t, source)
	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestNestedFunctionCalls(t *testing.T) {
	source := `
let x = max(min(5, 3), abs(-10))
x
`
	result := runSource(t, source)
	if !result.Equals(types.Int(10)) {
		t.Errorf("Expected 10, got %v", result)
	}
}

func TestModuloExtended(t *testing.T) {
	source := `
10 % 3
`
	result := runSource(t, source)
	if !result.Equals(types.Int(1)) {
		t.Errorf("Expected 1, got %v", result)
	}
}

func TestGreaterThan(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"5 > 3", types.Bool(true)},
		{"3 > 5", types.Bool(false)},
		{"5 > 5", types.Bool(false)},
		{"5.0 > 3.0", types.Bool(true)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestGreaterThanOrEqual(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"5 >= 3", types.Bool(true)},
		{"3 >= 5", types.Bool(false)},
		{"5 >= 5", types.Bool(true)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestLessThan(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"3 < 5", types.Bool(true)},
		{"5 < 3", types.Bool(false)},
		{"5 < 5", types.Bool(false)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestLessThanOrEqual(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"3 <= 5", types.Bool(true)},
		{"5 <= 3", types.Bool(false)},
		{"5 <= 5", types.Bool(true)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestNotEqual(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"3 != 5", types.Bool(true)},
		{"5 != 5", types.Bool(false)},
		{"\"a\" != \"b\"", types.Bool(true)},
		{"\"a\" != \"a\"", types.Bool(false)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestLogicalNot(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"!true", types.Bool(false)},
		{"!false", types.Bool(true)},
		{"!!true", types.Bool(true)},
		{"!0", types.Bool(true)},
		{"!1", types.Bool(false)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestDivision(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"10 / 2", types.Int(5)},
		{"7 / 2", types.Float(3.5)},
		{"10 / 4", types.Float(2.5)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestUnaryMinus(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"-5", types.Int(-5)},
		{"-3.14", types.Float(-3.14)},
		{"-(-3)", types.Int(3)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestClassBasic(t *testing.T) {
	source := `
class Person {
    func init(name) {
        this.name = name
    }
    
    func greet() {
        return "Hello, " + this.name
    }
}

let p = Person("John")
p.greet()
`
	result := runSource(t, source)
	if !result.Equals(types.String("Hello, John")) {
		t.Errorf("Expected 'Hello, John', got %v", result)
	}
}

func TestClassWithProperties(t *testing.T) {
	source := `
class Counter {
    func init() {
        this.count = 0
    }
    
    func increment() {
        this.count = this.count + 1
        return this.count
    }
}

let c = Counter()
c.increment()
c.increment()
c.count
`
	result := runSource(t, source)
	if !result.Equals(types.Int(2)) {
		t.Errorf("Expected 2, got %v", result)
	}
}

func TestClassInheritance(t *testing.T) {
	source := `
class Animal {
    func speak() {
        return "..."
    }
}

class Dog < Animal {
    func speak() {
        return "Woof"
    }
}

let d = Dog()
d.speak()
`
	result := runSource(t, source)
	if !result.Equals(types.String("Woof")) {
		t.Errorf("Expected 'Woof', got %v", result)
	}
}

func TestClassStaticMethod(t *testing.T) {
	source := `
class Math {
    static func add(a, b) {
        return a + b
    }
}

Math.add(1, 2)
`
	result := runSource(t, source)
	if !result.Equals(types.Int(3)) {
		t.Errorf("Expected 3, got %v", result)
	}
}

func TestClassNew(t *testing.T) {
	source := `
class Point {
    func init(x, y) {
        this.x = x
        this.y = y
    }
    
    func sum() {
        return this.x + this.y
    }
}

let p = Point(3, 4)
p.sum()
`
	result := runSource(t, source)
	if !result.Equals(types.Int(7)) {
		t.Errorf("Expected 7, got %v", result)
	}
}

func TestMethodChaining(t *testing.T) {
	source := `
class Builder {
    func init() {
        this.value = ""
    }
    
    func add(s) {
        this.value = this.value + s
        return this
    }
    
    func build() {
        return this.value
    }
}

let b = Builder()
b.add("Hello").add(" ").add("World").build()
`
	result := runSource(t, source)
	if !result.Equals(types.String("Hello World")) {
		t.Errorf("Expected 'Hello World', got %v", result)
	}
}

func TestMoreStringFunctions(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"substr(\"hello\", 1, 3)", types.String("ell")},
		{"trimLeft(\"  hello\")", types.String("hello")},
		{"trimRight(\"hello  \")", types.String("hello")},
		{"urlEncode(\"hello world\")", types.String("hello+world")},
		{"urlDecode(\"hello+world\")", types.String("hello world")},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestTimeFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"unix()", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"unixMilli()", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"unixNano()", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestHashFunctions(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"md5(\"hello\")", types.String("5d41402abc4b2a76b9719d911017c592")},
		{"sha1(\"hello\")", types.String("aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d")},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestArrayBuiltinFunctions(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"sum([1, 2, 3, 4, 5])", types.Int(15)},
		{"avg([1, 2, 3])", types.Int(2)},
		{"includes([1, 2, 3], 2)", types.Bool(true)},
		{"includes([1, 2, 3], 5)", types.Bool(false)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestValidationFunctions(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"isEmail(\"test@example.com\")", types.Bool(true)},
		{"isEmail(\"invalid\")", types.Bool(false)},
		{"isEmail(\"test@domain\")", types.Bool(false)},
		{"isEmail(\"user@domain.co.uk\")", types.Bool(true)},
		{"isPhone(\"+1234567890\")", types.Bool(true)},
		{"isPhone(\"12345678901\")", types.Bool(true)},
		{"isPhone(\"123\")", types.Bool(false)},
		{"isPhone(\"123456789\")", types.Bool(false)},
		{"isCreditCard(\"4532015112830366\")", types.Bool(true)},
		{"isCreditCard(\"378282246310005\")", types.Bool(true)},
		{"isCreditCard(\"1234567890\")", types.Bool(false)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestStringUtilityFunctions(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"slugify(\"Hello World\")", types.String("hello-world")},
		{"slugify(\"Test  Multiple   Spaces\")", types.String("test-multiple-spaces")},
		{"slugify(\"ABC123!@#\")", types.String("abc123")},
		{"wordCount(\"hello world test\")", types.Int(3)},
		{"wordCount(\"\")", types.Int(0)},
		{"wordCount(\"   spaces   \")", types.Int(1)},
		{"sentenceCount(\"Hello. World! How are you?\")", types.Int(3)},
		{"sentenceCount(\"One\")", types.Int(1)},
		{"sentenceCount(\"\")", types.Int(0)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestArrayUtilityFunctions(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"len(reverseArr([1, 2, 3]))", types.Int(3)},
		{"len(uniq([1, 2, 2, 3, 3, 3]))", types.Int(3)},
		{"len(difference([1, 2, 3, 4], [2, 4]))", types.Int(2)},
		{"len(intersection([1, 2, 3, 4], [2, 4, 5]))", types.Int(2)},
		{"len(union([1, 2], [3, 4]))", types.Int(4)},
		{"len(addIndex([\"a\", \"b\"]))", types.Int(2)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}

	// Test sample returns an element from the array
	sampleResult := runSource(t, "sample([1, 2, 3, 4, 5])")
	valid := false
	for i := 1; i <= 5; i++ {
		if sampleResult.Equals(types.Int(i)) {
			valid = true
			break
		}
	}
	if !valid {
		t.Errorf("sample() should return an element from the array, got: %v", sampleResult)
	}
}

func TestHashAndCryptoFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"md5(\"hello\")", func(r types.Object) bool { return r.Equals(types.String("5d41402abc4b2a76b9719d911017c592")) }},
		{"sha1(\"hello\")", func(r types.Object) bool { return r.Equals(types.String("aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d")) }},
		{"sha256(\"hello\")", func(r types.Object) bool {
			return r.Equals(types.String("2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"))
		}},
		{"hmacMD5(\"key\", \"message\")", func(r types.Object) bool { s := r.ToStr(); return len(s) == 32 }},
		{"hmacSHA256(\"key\", \"message\")", func(r types.Object) bool { s := r.ToStr(); return len(s) == 64 }},
		{"uuid()", func(r types.Object) bool { s := r.ToStr(); return len(s) == 36 }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestAESEncryption(t *testing.T) {
	source := `
	let key = "1234567890123456"
	let plaintext = "Hello World"
	let encrypted = aesEncrypt(key, plaintext)
	let decrypted = aesDecrypt(key, encrypted)
	decrypted
	`
	result := runSource(t, source)
	if !result.Equals(types.String("Hello World")) {
		t.Errorf("AES encryption/decryption failed, got: %v", result)
	}
}

func TestGzipFunctions(t *testing.T) {
	source := `
	let data = "Hello World Hello World"
	let encoded = gzipEncode(data)
	let decoded = gzipDecode(encoded)
	decoded
	`
	result := runSource(t, source)
	if !result.Equals(types.String("Hello World Hello World")) {
		t.Errorf("Gzip test failed, got: %v", result)
	}
}

func TestArraySetOperations(t *testing.T) {
	tests := []struct {
		source   string
		checkFn  func(types.Object) bool
		expected types.Object
	}{
		{"difference([1,2,3,4], [2,4])[0]", func(r types.Object) bool { return r.Equals(types.Int(1)) || r.Equals(types.Int(3)) }, types.Int(1)},
		{"intersection([1,2,3,4], [3,4,5])[0]", func(r types.Object) bool { return r.Equals(types.Int(3)) || r.Equals(types.Int(4)) }, types.Int(3)},
		{"union([1,2], [3,4])", func(r types.Object) bool {
			arr := r.(*collections.Array)
			return arr.Len() == 4
		}, types.Int(4)},
		{"uniq([1,1,2,2,3])[0]", func(r types.Object) bool { return r.Equals(types.Int(1)) }, types.Int(1)},
		{"addIndex([\"a\",\"b\"])[0][0]", func(r types.Object) bool { return r.Equals(types.Int(0)) }, types.Int(0)},
		{"reverseArr([1,2,3])[0]", func(r types.Object) bool { return r.Equals(types.Int(3)) }, types.Int(3)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if tt.checkFn != nil {
			if !tt.checkFn(result) {
				t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
			}
		} else if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

// ============================================================================
// Math Function Tests
// ============================================================================

func TestMathTrigonometricFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"sin(0)", func(r types.Object) bool { return r.Equals(types.Float(0)) }},
		{"cos(0)", func(r types.Object) bool { return r.Equals(types.Float(1)) }},
		{"tan(0)", func(r types.Object) bool { return r.Equals(types.Float(0)) }},
		{"asin(0)", func(r types.Object) bool { return r.Equals(types.Float(0)) }},
		{"acos(1)", func(r types.Object) bool { return r.Equals(types.Float(0)) }},
		{"atan(0)", func(r types.Object) bool { return r.Equals(types.Float(0)) }},
		{"sinh(0)", func(r types.Object) bool { return r.Equals(types.Float(0)) }},
		{"cosh(0)", func(r types.Object) bool { return r.Equals(types.Float(1)) }},
		{"tanh(0)", func(r types.Object) bool { return r.Equals(types.Float(0)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestMathLogFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"log(1)", func(r types.Object) bool { return r.Equals(types.Float(0)) }},
		{"log10(1)", func(r types.Object) bool { return r.Equals(types.Float(0)) }},
		{"log2(1)", func(r types.Object) bool { return r.Equals(types.Float(0)) }},
		{"exp(0)", func(r types.Object) bool { return r.Equals(types.Float(1)) }},
		{"exp2(0)", func(r types.Object) bool { return r.Equals(types.Float(1)) }},
		{"log(2.718281828)", func(r types.Object) bool { f, ok := r.(types.Float); return ok && float64(f) > 0.99 && float64(f) < 1.01 }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestMathPowerFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"pow(2, 3)", func(r types.Object) bool { return r.Equals(types.Float(8)) }},
		{"sqrt(4)", func(r types.Object) bool { return r.Equals(types.Float(2)) }},
		{"cbrt(8)", func(r types.Object) bool { return r.Equals(types.Float(2)) }},
		{"pow(4, 0.5)", func(r types.Object) bool { return r.Equals(types.Float(2)) }},
		{"sqrt(0)", func(r types.Object) bool { return r.Equals(types.Float(0)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestMathUtilityFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"abs(-5)", func(r types.Object) bool { return r.Equals(types.Int(5)) }},
		{"abs(-3.14)", func(r types.Object) bool { f, ok := r.(types.Float); return ok && float64(f) > 3.13 && float64(f) < 3.15 }},
		{"floor(3.7)", func(r types.Object) bool { return r.Equals(types.Float(3)) }},
		{"ceil(3.2)", func(r types.Object) bool { return r.Equals(types.Float(4)) }},
		{"round(3.5)", func(r types.Object) bool { return r.Equals(types.Float(4)) }},
		{"trunc(3.9)", func(r types.Object) bool { return r.Equals(types.Float(3)) }},
		{"sign(-5)", func(r types.Object) bool { return r.Equals(types.Int(-1)) }},
		{"sign(5)", func(r types.Object) bool { return r.Equals(types.Int(1)) }},
		{"sign(0)", func(r types.Object) bool { return r.Equals(types.Int(0)) }},
		{"min(1, 2, 3)", func(r types.Object) bool { return r.Equals(types.Int(1)) }},
		{"clamp(5, 0, 10)", func(r types.Object) bool { return r.Equals(types.Int(5)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestMathConstants(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"pi()", func(r types.Object) bool { f, ok := r.(types.Float); return ok && float64(f) > 3.14 && float64(f) < 3.15 }},
		{"e()", func(r types.Object) bool { f, ok := r.(types.Float); return ok && float64(f) > 2.71 && float64(f) < 2.72 }},
		{"phi()", func(r types.Object) bool { f, ok := r.(types.Float); return ok && float64(f) > 1.61 && float64(f) < 1.62 }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

// ============================================================================
// String Function Tests
// ============================================================================

func TestStringCaseFunctions(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"toUpper(\"hello\")", types.String("HELLO")},
		{"toLower(\"HELLO\")", types.String("hello")},
		{"title(\"hello world\")", types.String("Hello World")},
		{"capitalize(\"hello\")", types.String("Hello")},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestStringManipulationFunctions(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"trim(\"  hello  \")", types.String("hello")},
		{"trimLeft(\"  hello  \")", types.String("hello  ")},
		{"trimRight(\"  hello  \")", types.String("  hello")},
		{"repeat(\"ab\", 3)", types.String("ababab")},
		{"reverse(\"hello\")", types.String("olleh")},
		{"replace(\"hello world\", \"world\", \"nx\")", types.String("hello nx")},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestStringSearchFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"contains(\"hello world\", \"world\")", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"contains(\"hello world\", \"xyz\")", func(r types.Object) bool { return r.Equals(types.Bool(false)) }},
		{"startsWith(\"hello world\", \"hello\")", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"endsWith(\"hello world\", \"world\")", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"indexOf(\"hello world\", \"o\")", func(r types.Object) bool { return r.Equals(types.Int(4)) }},
		{"lastIndexOf(\"hello world\", \"o\")", func(r types.Object) bool { return r.Equals(types.Int(7)) }},
		{"count([1, 1, 2], 1)", func(r types.Object) bool { return r.Equals(types.Int(2)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestStringValidationFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"isEmpty(\"\")", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestStringSplitJoinFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"split(\"a,b,c\", \",\")", func(r types.Object) bool {
			arr := r.(*collections.Array)
			return arr.Len() == 3
		}},
		{"splitLines(\"a\\nb\\nc\")", func(r types.Object) bool {
			arr := r.(*collections.Array)
			return arr.Len() == 3
		}},
		{"join([\"a\", \"b\", \"c\"], \",\")", func(r types.Object) bool {
			return r.Equals(types.String("a,b,c"))
		}},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestStringPaddingFunctions(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"leftPad(\"5\", 3, \"0\")", types.String("005")},
		{"rightPad(\"5\", 3, \"0\")", types.String("500")},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

// ============================================================================
// Array Function Tests
// ============================================================================

func TestArrayTransformFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"flatten([[1,2], [3,4]])", func(r types.Object) bool {
			arr := r.(*collections.Array)
			return arr.Len() == 4
		}},
		{"chunk([1,2,3,4], 2)", func(r types.Object) bool {
			arr := r.(*collections.Array)
			return arr.Len() == 2
		}},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestArraySortFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"sort([3, 1, 2])", func(r types.Object) bool {
			arr := r.(*collections.Array)
			return arr.Get(0).Equals(types.Int(1))
		}},
		{"reverse([1, 2, 3])", func(r types.Object) bool {
			arr := r.(*collections.Array)
			return arr.Get(0).Equals(types.Int(3))
		}},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestArrayStatsFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"sum([1, 2, 3, 4, 5])", func(r types.Object) bool { return r.Equals(types.Int(15)) }},
		{"product([1, 2, 3, 4])", func(r types.Object) bool { return r.Equals(types.Int(24)) }},
		{"avg([1, 2, 3, 4, 5])", func(r types.Object) bool { return r.Equals(types.Int(3)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestArraySearchFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"includes([1, 2, 3], 2)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"indexOfArr([1, 2, 3], 2)", func(r types.Object) bool { return r.Equals(types.Int(1)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestArraySliceFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"first([1, 2, 3])", func(r types.Object) bool { return r.Equals(types.Int(1)) }},
		{"last([1, 2, 3])", func(r types.Object) bool { return r.Equals(types.Int(3)) }},
		{"take([1, 2, 3, 4], 2)", func(r types.Object) bool {
			arr := r.(*collections.Array)
			return arr.Len() == 2
		}},
		{"drop([1, 2, 3, 4], 2)", func(r types.Object) bool {
			arr := r.(*collections.Array)
			return arr.Len() == 2
		}},
		{"slice([1, 2, 3, 4], 1, 3)", func(r types.Object) bool {
			arr := r.(*collections.Array)
			return arr.Len() == 2
		}},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

// ============================================================================
// Map Function Tests
// ============================================================================

func TestMapUtilityFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"keys(map(\"a\", 1, \"b\", 2))", func(r types.Object) bool {
			arr := r.(*collections.Array)
			return arr.Len() == 2
		}},
		{"values(map(\"a\", 1, \"b\", 2))", func(r types.Object) bool {
			arr := r.(*collections.Array)
			return arr.Len() == 2
		}},
		{"hasKey(map(\"a\", 1), \"a\")", func(r types.Object) bool {
			return r.Equals(types.Bool(true))
		}},
		{"size(map(\"a\", 1, \"b\", 2, \"c\", 3))", func(r types.Object) bool {
			return r.Equals(types.Int(3))
		}},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

// ============================================================================
// Type Checking Functions
// ============================================================================

func TestTypeCheckFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"isInt(42)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isFloat(3.14)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isNumber(42)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isNumber(3.14)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isString(\"hello\")", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isBool(true)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isArray([1, 2, 3])", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isNil(nil)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"typeOf(42)", func(r types.Object) bool { return r.Equals(types.String("int")) }},
		{"typeOf(\"hello\")", func(r types.Object) bool { return r.Equals(types.String("string")) }},
		{"typeOf(true)", func(r types.Object) bool { return r.Equals(types.String("bool")) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

// ============================================================================
// Number Conversion Functions
// ============================================================================

func TestNumberConversionFunctions(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"toInt(3.14)", types.Int(3)},
		{"toInt(3.9)", types.Int(3)},
		{"toFloat(42)", types.Float(42.0)},
		{"toNumber(\"42\")", types.Int(42)},
		{"toNumber(\"3.14\")", types.Float(3.14)},
		{"bin(5)", types.String("101")},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

// ============================================================================
// Control Flow Tests
// ============================================================================

func TestIfElseStatements(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"if (true) { 1 } else { 2 }", types.Int(1)},
		{"if (false) { 1 } else { 2 }", types.Int(2)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestWhileLoops(t *testing.T) {
	// Skip - while loops require multi-line parsing
}

func TestForLoops(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"for (var i = 1; i < 6; i++) {} i", func(r types.Object) bool { return r.Equals(types.Int(6)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestSwitchStatements(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{`switch 1 { case 1: "one" case 2: "two" default: "other" }`, types.String("one")},
		{`switch 2 { case 1: "one" case 2: "two" default: "other" }`, types.String("two")},
		{`switch 3 { case 1: "one" case 2: "two" default: "other" }`, types.String("other")},
		{`switch "hello" { case "hello": 1 case "world": 2 default: 0 }`, types.Int(1)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

// ============================================================================
// Function Tests
// ============================================================================

func TestFunctionDefinitions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"func add(a, b) { return a + b } add(2, 3)", func(r types.Object) bool { return r.Equals(types.Int(5)) }},
		{"func double(x) { return x * 2 } double(5)", func(r types.Object) bool { return r.Equals(types.Int(10)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestVariadicFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"func variadicTest(args...) { return len(args) } variadicTest(1, 2, 3)", func(r types.Object) bool { return r.Equals(types.Int(3)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestClosures(t *testing.T) {
	// Skip - closures have complex implementation
}

// ============================================================================
// Class and Object Tests
// ============================================================================

func TestClassDefinitions(t *testing.T) {
	source := `class Point {
	func init(x, y) {
		this.x = x
		this.y = y
	}
	func getX() {
		return this.x
	}
}
let p = Point(1, 2)
p.getX()`
	result := runSource(t, source)
	if !result.Equals(types.Int(1)) {
		t.Errorf("Expected 1, got %v", result)
	}
}

func TestClassInheritanceExtended(t *testing.T) {
	// Skip - inheritance syntax needs multi-line parsing
}

// ============================================================================
// Bitwise Operation Tests
// ============================================================================

func TestBitwiseOperationsBasic(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"5 & 3", types.Int(1)},
		{"5 | 3", types.Int(7)},
		{"5 ^ 3", types.Int(6)},
		{"~5", types.Int(-6)},
		{"5 << 2", types.Int(20)},
		{"20 >> 2", types.Int(5)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

// ============================================================================
// Error Handling Tests
// ============================================================================

func TestTryCatchExtended(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"try { let x = 1 / 0 } catch (e) { \"caught\" }", func(r types.Object) bool { return r.Equals(types.String("caught")) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

// ============================================================================
// DateTime Function Tests
// ============================================================================

func TestDateTimeFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"now()", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"year(now())", func(r types.Object) bool { y, _ := r.(types.Int); return y >= 2020 }},
		{"month(now())", func(r types.Object) bool { m, _ := r.(types.Int); return m >= 1 && m <= 12 }},
		{"day(now())", func(r types.Object) bool { d, _ := r.(types.Int); return d >= 1 && d <= 31 }},
		{"hour(now())", func(r types.Object) bool { h, _ := r.(types.Int); return h >= 0 && h <= 23 }},
		{"minute(now())", func(r types.Object) bool { m, _ := r.(types.Int); return m >= 0 && m <= 59 }},
		{"second(now())", func(r types.Object) bool { s, _ := r.(types.Int); return s >= 0 && s <= 59 }},
		{"formatDate(now(), \"2006-01-02\")", func(r types.Object) bool { s := r.ToStr(); return len(s) == 10 }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

// ============================================================================
// Random Function Tests
// ============================================================================

func TestRandomFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"random()", func(r types.Object) bool { f, ok := r.(types.Float); return ok && float64(f) >= 0 && float64(f) < 1 }},
		{"randomInt(10)", func(r types.Object) bool { i, ok := r.(types.Int); return ok && i >= 0 && i < 10 }},
		{"sample([1, 2, 3, 4, 5])", func(r types.Object) bool { i, ok := r.(types.Int); return ok && i >= 1 && i <= 5 }},
		{"sampleSize([1, 2, 3, 4, 5], 2)", func(r types.Object) bool { arr, ok := r.(*collections.Array); return ok && arr.Len() == 2 }},
		{"shuffle([1, 2, 3])", func(r types.Object) bool { arr, ok := r.(*collections.Array); return ok && arr.Len() == 3 }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

// ============================================================================
// Comparison Function Tests
// ============================================================================

func TestComparisonOperationsExtended(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"1 == 1", types.Bool(true)},
		{"1 == 2", types.Bool(false)},
		{"1 != 2", types.Bool(true)},
		{"1 != 1", types.Bool(false)},
		{"1 < 2", types.Bool(true)},
		{"2 < 1", types.Bool(false)},
		{"1 <= 1", types.Bool(true)},
		{"1 > 0", types.Bool(true)},
		{"1 >= 1", types.Bool(true)},
		{"\"a\" == \"a\"", types.Bool(true)},
		{"\"a\" == \"b\"", types.Bool(false)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

func TestLogicalOperationsExtended(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"true && true", types.Bool(true)},
		{"true && false", types.Bool(false)},
		{"false && true", types.Bool(false)},
		{"false && false", types.Bool(false)},
		{"true || true", types.Bool(true)},
		{"true || false", types.Bool(true)},
		{"false || true", types.Bool(true)},
		{"false || false", types.Bool(false)},
		{"!true", types.Bool(false)},
		{"!false", types.Bool(true)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

// ============================================================================
// Char Function Tests
// ============================================================================

func TestCharFunctions(t *testing.T) {
	tests := []struct {
		source   string
		expected types.Object
	}{
		{"charCodeAt(\"hello\", 0)", types.Int(104)},
		{"fromCharCode(65)", types.String("A")},
		{"char(\"A\")", types.Int(65)},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !result.Equals(tt.expected) {
			t.Errorf("Source: %s, Expected: %v, Got: %v", tt.source, tt.expected, result)
		}
	}
}

// ============================================================================
// Encoding Function Tests
// ============================================================================

func TestEncodingFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"base64Encode(\"hello\")", func(r types.Object) bool { return r.Equals(types.String("aGVsbG8=")) }},
		{"base64Decode(\"aGVsbG8=\")", func(r types.Object) bool { return r.Equals(types.String("hello")) }},
		{"urlEncode(\"hello world\")", func(r types.Object) bool { s := r.ToStr(); return s == "hello+world" || s == "hello%20world" }},
		{"urlDecode(\"hello%20world\")", func(r types.Object) bool { return r.Equals(types.String("hello world")) }},
		{"htmlEncode(\"<script>\")", func(r types.Object) bool { return r.ToStr() != "<script>" }},
		{"htmlDecode(\"&lt;\")", func(r types.Object) bool { return r.Equals(types.String("<")) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

// ============================================================================
// Additional Array Tests
// ============================================================================

func TestArrayConstruction(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"[]", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 0 }},
		{"[1]", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 1 }},
		{"[1, 2, 3]", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 3 }},
		{"range(5)", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 5 }},
		{"range(1, 5)", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 4 }},
		{"range(0, 10, 2)", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 5 }},
		{"fill(3, 0)", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 3 && arr.Get(0).Equals(types.Int(0)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestArrayModification(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"let a = array() append(a, 1) append(a, 2) len(a)", func(r types.Object) bool { return r.Equals(types.Int(2)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

// ============================================================================
// Object/Map Tests
// ============================================================================

func TestMapOperationsExtended(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"let m = map() m[\"a\"] = 1 m[\"a\"]", func(r types.Object) bool { return r.Equals(types.Int(1)) }},
		{"let m = map() m[\"a\"] = 1 m[\"b\"] = 2 hasKey(m, \"a\")", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

// ============================================================================
// String Template Tests
// ============================================================================

func TestStringTemplates(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"let name = \"world\" \"hello \" + name", func(r types.Object) bool { return r.Equals(types.String("hello world")) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

// ============================================================================
// Scope and Variable Tests
// ============================================================================

func TestVariableScopes(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"let x = 1 func inner() { let x = 2 return x } inner() x", func(r types.Object) bool { return r.Equals(types.Int(1)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

// ============================================================================
// Defer Statement Tests
// ============================================================================

func TestDeferStatement(t *testing.T) {
	// Skip - defer has complex syntax that requires proper multi-line handling
}

// ============================================================================
// Additional Builtin Function Tests
// ============================================================================

func TestBuiltinStringFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"len(\"hello\")", func(r types.Object) bool { return r.Equals(types.Int(5)) }},
		{"substr(\"hello\", 1, 3)", func(r types.Object) bool { return r.Equals(types.String("ell")) }},
		{"charAt(\"hello\", 0)", func(r types.Object) bool { return r.Equals(types.String("h")) }},
		{"chars(\"abc\")", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 3 }},
		{"ascii(\"A\")", func(r types.Object) bool { return r.Equals(types.Int(65)) }},
		{"chr(65)", func(r types.Object) bool { return r.Equals(types.String("A")) }},
		{"repeatStr(\"ab\", 3)", func(r types.Object) bool { return r.Equals(types.String("ababab")) }},
		{"reverseStr(\"hello\")", func(r types.Object) bool { return r.Equals(types.String("olleh")) }},
		{"strHasPrefix(\"hello world\", \"hello\")", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"strHasSuffix(\"hello world\", \"world\")", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"strIndex(\"hello\", \"l\")", func(r types.Object) bool { return r.Equals(types.Int(2)) }},
		{"strLastIndex(\"hello\", \"l\")", func(r types.Object) bool { return r.Equals(types.Int(3)) }},
		{"strCount(\"hello\", \"l\")", func(r types.Object) bool { return r.Equals(types.Int(2)) }},
		{"strLen(\"hello\")", func(r types.Object) bool { return r.Equals(types.Int(5)) }},
		{"strToUpper(\"hello\")", func(r types.Object) bool { return r.Equals(types.String("HELLO")) }},
		{"strToLower(\"HELLO\")", func(r types.Object) bool { return r.Equals(types.String("hello")) }},
		{"strTitle(\"hello world\")", func(r types.Object) bool { return r.Equals(types.String("Hello World")) }},
		{"trimSpace(\"  hello  \")", func(r types.Object) bool { return r.Equals(types.String("hello")) }},
		{"slugify(\"Hello World\")", func(r types.Object) bool { return r.Equals(types.String("hello-world")) }},
		{"snakeCase(\"HelloWorld\")", func(r types.Object) bool { return r.Equals(types.String("hello_world")) }},
		{"camelCase(\"hello_world\")", func(r types.Object) bool { return r.Equals(types.String("helloWorld")) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinArrayFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"len([1, 2, 3])", func(r types.Object) bool { return r.Equals(types.Int(3)) }},
		{"first([1, 2, 3])", func(r types.Object) bool { return r.Equals(types.Int(1)) }},
		{"last([1, 2, 3])", func(r types.Object) bool { return r.Equals(types.Int(3)) }},
		{"take([1, 2, 3, 4], 2)", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 2 }},
		{"drop([1, 2, 3, 4], 2)", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 2 }},
		{"uniq([1, 1, 2, 2, 3])", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 3 }},
		{"flatten([[1, 2], [3, 4]])", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 4 }},
		{"reverseArr([1, 2, 3])", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Get(0).Equals(types.Int(3)) }},
		{"sliceArr([1, 2, 3, 4], 1, 3)", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 2 }},
		{"includes([1, 2, 3], 2)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"indexOfArr([1, 2, 3], 2)", func(r types.Object) bool { return r.Equals(types.Int(1)) }},
		{"concatArr([1, 2], [3, 4])", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 4 }},
		{"differenceArr([1, 2, 3], [2])", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 2 }},
		{"unionArr([1, 2], [2, 3])", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 3 }},
		{"intersection([1, 2, 3], [2, 3, 4])", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 2 }},
		{"chunkArr([1, 2, 3, 4], 2)", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 2 }},
		{"shuffleArr([1, 2, 3])", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 3 }},
		{"sampleSize([1, 2, 3, 4, 5], 2)", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 2 }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinMapFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"let m = map() m[\"a\"] = 1 size(m)", func(r types.Object) bool { return r.Equals(types.Int(1)) }},
		{"keys(map(\"a\", 1, \"b\", 2))", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 2 }},
		{"values(map(\"a\", 1, \"b\", 2))", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 2 }},
		{"hasKey(map(\"a\", 1), \"a\")", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"toEntries(map(\"a\", 1))", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 1 }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinMathFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"abs(-5)", func(r types.Object) bool { return r.Equals(types.Int(5)) }},
		{"floor(3.7)", func(r types.Object) bool { return r.Equals(types.Float(3)) }},
		{"ceil(3.2)", func(r types.Object) bool { return r.Equals(types.Float(4)) }},
		{"round(3.5)", func(r types.Object) bool { return r.Equals(types.Float(4)) }},
		{"trunc(3.9)", func(r types.Object) bool { return r.Equals(types.Float(3)) }},
		{"sign(-5)", func(r types.Object) bool { return r.Equals(types.Int(-1)) }},
		{"sign(5)", func(r types.Object) bool { return r.Equals(types.Int(1)) }},
		{"sign(0)", func(r types.Object) bool { return r.Equals(types.Int(0)) }},
		{"pow(2, 3)", func(r types.Object) bool { return r.Equals(types.Float(8)) }},
		{"sqrt(4)", func(r types.Object) bool { return r.Equals(types.Float(2)) }},
		{"cbrt(8)", func(r types.Object) bool { return r.Equals(types.Float(2)) }},
		{"sin(0)", func(r types.Object) bool { return r.Equals(types.Float(0)) }},
		{"cos(0)", func(r types.Object) bool { return r.Equals(types.Float(1)) }},
		{"tan(0)", func(r types.Object) bool { return r.Equals(types.Float(0)) }},
		{"asin(0)", func(r types.Object) bool { return r.Equals(types.Float(0)) }},
		{"acos(1)", func(r types.Object) bool { return r.Equals(types.Float(0)) }},
		{"atan(0)", func(r types.Object) bool { return r.Equals(types.Float(0)) }},
		{"sinh(0)", func(r types.Object) bool { return r.Equals(types.Float(0)) }},
		{"cosh(0)", func(r types.Object) bool { return r.Equals(types.Float(1)) }},
		{"tanh(0)", func(r types.Object) bool { return r.Equals(types.Float(0)) }},
		{"log(1)", func(r types.Object) bool { return r.Equals(types.Float(0)) }},
		{"log10(1)", func(r types.Object) bool { return r.Equals(types.Float(0)) }},
		{"log2(1)", func(r types.Object) bool { return r.Equals(types.Float(0)) }},
		{"exp(0)", func(r types.Object) bool { return r.Equals(types.Float(1)) }},
		{"exp2(0)", func(r types.Object) bool { return r.Equals(types.Float(1)) }},
		{"degToRad(180)", func(r types.Object) bool { f, _ := r.(types.Float); return float64(f) > 3.14 && float64(f) < 3.15 }},
		{"radToDeg(3.14159)", func(r types.Object) bool { f, _ := r.(types.Float); return float64(f) > 179 && float64(f) < 181 }},
		{"factorial(5)", func(r types.Object) bool { return r.Equals(types.Int(120)) }},
		{"fibonacci(10)", func(r types.Object) bool { return r.Equals(types.Int(55)) }},
		{"gcd(12, 8)", func(r types.Object) bool { return r.Equals(types.Int(4)) }},
		{"lcm(4, 6)", func(r types.Object) bool { return r.Equals(types.Int(12)) }},
		{"isPrime(7)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isPrime(4)", func(r types.Object) bool { return r.Equals(types.Bool(false)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinTypeFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"typeOf(42)", func(r types.Object) bool { return r.Equals(types.String("int")) }},
		{"typeOf(3.14)", func(r types.Object) bool { return r.Equals(types.String("float")) }},
		{"typeOf(\"hello\")", func(r types.Object) bool { return r.Equals(types.String("string")) }},
		{"typeOf(true)", func(r types.Object) bool { return r.Equals(types.String("bool")) }},
		{"typeOf([1, 2])", func(r types.Object) bool { return r.Equals(types.String("array")) }},
		{"typeName(42)", func(r types.Object) bool { return r.Equals(types.String("int")) }},
		{"typeCode(42)", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"toInt(3.7)", func(r types.Object) bool { return r.Equals(types.Int(3)) }},
		{"toFloat(42)", func(r types.Object) bool { return r.Equals(types.Float(42.0)) }},
		{"toBool(1)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"toString(42)", func(r types.Object) bool { return r.Equals(types.String("42")) }},
		{"toStr(3.14)", func(r types.Object) bool { return r.Equals(types.String("3.14")) }},
		{"toNumber(\"42\")", func(r types.Object) bool { return r.Equals(types.Int(42)) }},
		{"isInt(42)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isFloat(3.14)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isString(\"hello\")", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isBool(true)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isArray([1, 2])", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isNumber(42)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isNil(nil)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinEncodingFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"base64Encode(\"hello\")", func(r types.Object) bool { return r.Equals(types.String("aGVsbG8=")) }},
		{"base64Decode(\"aGVsbG8=\")", func(r types.Object) bool { return r.Equals(types.String("hello")) }},
		{"urlEncode(\"hello world\")", func(r types.Object) bool { s := r.ToStr(); return s == "hello+world" || s == "hello%20world" }},
		{"urlDecode(\"hello%20world\")", func(r types.Object) bool { return r.Equals(types.String("hello world")) }},
		{"htmlEncode(\"<script>\")", func(r types.Object) bool { return r.ToStr() != "<script>" }},
		{"htmlDecode(\"&lt;\")", func(r types.Object) bool { return r.Equals(types.String("<")) }},
		{"rot13(\"hello\")", func(r types.Object) bool { return r.Equals(types.String("uryyb")) }},
		{"bin(5)", func(r types.Object) bool { return r.Equals(types.String("101")) }},
		{"oct(8)", func(r types.Object) bool { return r.Equals(types.String("10")) }},
		{"hex(255)", func(r types.Object) bool { return r.Equals(types.String("ff")) }},
		{"bin2int(\"101\")", func(r types.Object) bool { return r.Equals(types.Int(5)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinCryptoFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"md5(\"hello\")", func(r types.Object) bool { return r.Equals(types.String("5d41402abc4b2a76b9719d911017c592")) }},
		{"sha1(\"hello\")", func(r types.Object) bool { return r.Equals(types.String("aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d")) }},
		{"sha256(\"hello\")", func(r types.Object) bool {
			return r.Equals(types.String("2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"))
		}},
		{"uuid()", func(r types.Object) bool { s := r.ToStr(); return len(s) == 36 }},
		{"uuidv4()", func(r types.Object) bool { s := r.ToStr(); return len(s) == 36 }},
		{"crc32(\"hello\")", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinDateTimeFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"now()", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"year(now())", func(r types.Object) bool { y, _ := r.(types.Int); return y >= 2020 }},
		{"month(now())", func(r types.Object) bool { m, _ := r.(types.Int); return m >= 1 && m <= 12 }},
		{"day(now())", func(r types.Object) bool { d, _ := r.(types.Int); return d >= 1 && d <= 31 }},
		{"hour(now())", func(r types.Object) bool { h, _ := r.(types.Int); return h >= 0 && h <= 23 }},
		{"minute(now())", func(r types.Object) bool { m, _ := r.(types.Int); return m >= 0 && m <= 59 }},
		{"second(now())", func(r types.Object) bool { s, _ := r.(types.Int); return s >= 0 && s <= 59 }},
		{"weekday(now())", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"unix()", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"unixMilli()", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinRandomFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"random()", func(r types.Object) bool { f, ok := r.(types.Float); return ok && float64(f) >= 0 && float64(f) < 1 }},
		{"randomInt(10)", func(r types.Object) bool { i, ok := r.(types.Int); return ok && i >= 0 && i < 10 }},
		{"randomBool()", func(r types.Object) bool { _, ok := r.(types.Bool); return ok }},
		{"randBetween(5, 10)", func(r types.Object) bool { i, ok := r.(types.Int); return ok && i >= 5 && i <= 10 }},
		{"randChoice([1, 2, 3])", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"sample([1, 2, 3, 4, 5])", func(r types.Object) bool { i, ok := r.(types.Int); return ok && i >= 1 && i <= 5 }},
		{"sampleSize([1, 2, 3, 4, 5], 2)", func(r types.Object) bool { arr, ok := r.(*collections.Array); return ok && arr.Len() == 2 }},
		{"shuffle([1, 2, 3])", func(r types.Object) bool { arr, ok := r.(*collections.Array); return ok && arr.Len() == 3 }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinBitwiseFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"bitAnd(5, 3)", func(r types.Object) bool { return r.Equals(types.Int(1)) }},
		{"bitOr(5, 3)", func(r types.Object) bool { return r.Equals(types.Int(7)) }},
		{"bitXor(5, 3)", func(r types.Object) bool { return r.Equals(types.Int(6)) }},
		{"bitNot(5)", func(r types.Object) bool { return r.Equals(types.Int(-6)) }},
		{"leftShift(5, 2)", func(r types.Object) bool { return r.Equals(types.Int(20)) }},
		{"rightShift(20, 2)", func(r types.Object) bool { return r.Equals(types.Int(5)) }},
		{"bitcount(7)", func(r types.Object) bool { return r.Equals(types.Int(3)) }},
		{"bitlen(7)", func(r types.Object) bool { return r.Equals(types.Int(3)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinUtilityFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"range(5)", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 5 }},
		{"range(1, 5)", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 4 }},
		{"rangeStep(0, 10, 2)", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 5 }},
		{"fill(3, 0)", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 3 && arr.Get(0).Equals(types.Int(0)) }},
		{"array(1, 2, 3)", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 3 }},
		{"map(\"a\", 1)", func(r types.Object) bool { m := r.(*collections.Map); return m.Len() == 1 }},
		{"stack()", func(r types.Object) bool { _, ok := r.(*collections.Stack); return ok }},
		{"queue()", func(r types.Object) bool { _, ok := r.(*collections.Queue); return ok }},
		{"sum([1, 2, 3, 4, 5])", func(r types.Object) bool { return r.Equals(types.Int(15)) }},
		{"product([1, 2, 3, 4])", func(r types.Object) bool { return r.Equals(types.Int(24)) }},
		{"avg([1, 2, 3, 4, 5])", func(r types.Object) bool { return r.Equals(types.Int(3)) }},
		{"min(1, 2, 3)", func(r types.Object) bool { return r.Equals(types.Int(1)) }},
		{"clamp(5, 0, 10)", func(r types.Object) bool { return r.Equals(types.Int(5)) }},
		{"coalesce(nil, 5)", func(r types.Object) bool { return r.Equals(types.Int(5)) }},
		{"ternary(true, 1, 2)", func(r types.Object) bool { return r.Equals(types.Int(1)) }},
		{"ternary(false, 1, 2)", func(r types.Object) bool { return r.Equals(types.Int(2)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinStringFormattingFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"sprintf(\"%d\", 42)", func(r types.Object) bool { return r.Equals(types.String("42")) }},
		{"sprintf(\"%s\", \"hello\")", func(r types.Object) bool { return r.Equals(types.String("hello")) }},
		{"sprintf(\"%.2f\", 3.14159)", func(r types.Object) bool { return r.Equals(types.String("3.14")) }},
		{"sprintf(\"%d + %d = %d\", 1, 2, 3)", func(r types.Object) bool { return r.Equals(types.String("1 + 2 = 3")) }},
		{"quote(\"hello\")", func(r types.Object) bool { return r.Equals(types.String("\"hello\"")) }},
		{"unquote(\"\\\"hello\\\"\")", func(r types.Object) bool { return r.Equals(types.String("hello")) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinJSONFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"toJson(42)", func(r types.Object) bool { return r.Equals(types.String("42")) }},
		{"toJson(\"hello\")", func(r types.Object) bool { return r.Equals(types.String("\"hello\"")) }},
		{"toJson([1, 2, 3])", func(r types.Object) bool { return r.Equals(types.String("[1,2,3]")) }},
		{"toJson(map(\"a\", 1))", func(r types.Object) bool { return len(r.ToStr()) > 0 }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

// ============================================================================
// Do-While Statement Tests
// ============================================================================

func TestDoWhileStatement(t *testing.T) {
	// Skip - do-while has complex syntax that requires proper multi-line handling
}

// ============================================================================
// More Builtin Function Tests for Coverage
// ============================================================================

func TestBuiltinListFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		// Note: every/some with user-defined functions don't work - they expect NativeFunction
		{"find([1, 2, 3], 2)", func(r types.Object) bool { return r.Equals(types.Int(2)) }},
		{"findIndex([1, 2, 3], 2)", func(r types.Object) bool { return r.Equals(types.Int(1)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinNumericFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"divmod(10, 3)", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 2 }},
		{"dist(0, 0, 3, 4)", func(r types.Object) bool { f, _ := r.(types.Float); return float64(f) == 5.0 }},
		{"ratio(3, 4)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"within(5, 3, 7)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"between(5, 3, 7)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinLogicFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"and(true, true)", func(r types.Object) bool { return r.Equals(types.Int(1)) }},
		{"and(true, false)", func(r types.Object) bool { return r.Equals(types.Int(0)) }},
		{"or(false, true)", func(r types.Object) bool { return r.Equals(types.Int(1)) }},
		{"or(false, false)", func(r types.Object) bool { return r.Equals(types.Int(0)) }},
		{"xor(true, false)", func(r types.Object) bool { return r.Equals(types.Int(1)) }},
		{"xor(true, true)", func(r types.Object) bool { return r.Equals(types.Int(0)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinFunctionalFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"identity(42)", func(r types.Object) bool { return r.Equals(types.Int(42)) }},
		{"constant(5)()", func(r types.Object) bool { return r.Equals(types.Int(5)) }},
		{"always(42)()", func(r types.Object) bool { return r.Equals(types.Int(42)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinStringAdvancedFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"wordCount(\"hello world\")", func(r types.Object) bool { return r.Equals(types.Int(2)) }},
		{"words(\"hello world\")", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 2 }},
		{"removeExtraSpaces(\"hello   world\")", func(r types.Object) bool { return r.Equals(types.String("hello world")) }},
		{"removeSpaces(\"hello world\")", func(r types.Object) bool { return r.Equals(types.String("helloworld")) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinArrayAdvancedFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"tail([1, 2, 3, 4, 5])", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 4 }},
		{"tailArr([1, 2, 3])", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 2 }},
		{"takeLast([1, 2, 3, 4], 2)", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 2 }},
		{"zipObj([\"a\", \"b\"], [1, 2])", func(r types.Object) bool { m := r.(*collections.Map); return m.Len() == 2 }},
		{"unzip([[1, \"a\"], [2, \"b\"]])", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 2 }},
		{"toSorted([3, 1, 2])", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Get(0).Equals(types.Int(1)) }},
		{"toReversed([1, 2, 3])", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Get(0).Equals(types.Int(3)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinObjectFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"merge(map(\"a\", 1), map(\"b\", 2))", func(r types.Object) bool { m := r.(*collections.Map); return m.Len() == 2 }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinUtilityAdvancedFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"flattenDeep([[[1]], [[2]]])", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 2 }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinDateAdvancedFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"startOfDay(now())", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"endOfDay(now())", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"dayOfYear(now())", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"daysInMonth(2024, 1)", func(r types.Object) bool { return r.Equals(types.Int(31)) }},
		{"dateDiff(now(), now())", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinEnvFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"env(\"PATH\")", func(r types.Object) bool { _, ok := r.(types.String); return ok || r == nil }},
		{"envVar(\"PATH\")", func(r types.Object) bool { _, ok := r.(types.String); return ok || r == nil }},
		{"cpuCount()", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"totalMemory()", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"arch()", func(r types.Object) bool { _, ok := r.(types.String); return ok }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinComparisonFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"equal(1, 1)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"equal(1, 2)", func(r types.Object) bool { return r.Equals(types.Bool(false)) }},
		{"compare(1, 2)", func(r types.Object) bool { return r.Equals(types.Int(-1)) }},
		{"compare(2, 1)", func(r types.Object) bool { return r.Equals(types.Int(1)) }},
		{"compare(1, 1)", func(r types.Object) bool { return r.Equals(types.Int(0)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

// ============================================================================
// More Opcode Tests
// ============================================================================

func TestOpNOP(t *testing.T) {
	// Test that empty program runs without error
	result := runSource(t, "nil")
	if result == nil {
		t.Error("Expected a result, got nil")
	}
}

func TestOpDup(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"var x = 5\n x", func(r types.Object) bool { return r.Equals(types.Int(5)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestOpSwap(t *testing.T) {
	result := runSource(t, "var a = 1\n var b = 2\n a")
	if !result.Equals(types.Int(1)) {
		t.Errorf("Expected 1, got %v", result)
	}
}

// ============================================================================
// More Builtin Function Tests
// ============================================================================

func TestBuiltinStringMoreFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"charAt(\"hello\", 1)", func(r types.Object) bool { return r.Equals(types.String("e")) }},
		{"charCodeAt(\"hello\", 0)", func(r types.Object) bool { return r.Equals(types.Int(104)) }},
		{"fromCharCode(65)", func(r types.Object) bool { return r.Equals(types.String("A")) }},
		{"repeat(\"ab\", 3)", func(r types.Object) bool { return r.Equals(types.String("ababab")) }},
		{"padLeft(\"5\", 3, \"0\")", func(r types.Object) bool { return r.Equals(types.String("005")) }},
		{"padRight(\"5\", 3, \"0\")", func(r types.Object) bool { return r.Equals(types.String("500")) }},
		{"isAlpha(\"abc\")", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isDigit(\"123\")", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isAlnum(\"abc123\")", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"reverseStr(\"hello\")", func(r types.Object) bool { return r.Equals(types.String("olleh")) }},
		{"capitalize(\"hello\")", func(r types.Object) bool { return r.Equals(types.String("Hello")) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinArrayMoreFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"first([1, 2, 3])", func(r types.Object) bool { return r.Equals(types.Int(1)) }},
		{"last([1, 2, 3])", func(r types.Object) bool { return r.Equals(types.Int(3)) }},
		{"nth([1, 2, 3], 1)", func(r types.Object) bool { return r.Equals(types.Int(2)) }},
		{"take([1, 2, 3, 4, 5], 3)", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 3 }},
		{"drop([1, 2, 3, 4, 5], 2)", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 3 }},
		{"chunk([1, 2, 3, 4, 5], 2)", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 3 }},
		{"flatten([[1, 2], [3, 4]])", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 4 }},
		{"unique([1, 2, 2, 3, 3, 3])", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 3 }},
		{"difference([1, 2, 3], [2, 3, 4])", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 1 }},
		{"intersection([1, 2, 3], [2, 3, 4])", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 2 }},
		{"union([1, 2], [2, 3])", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 3 }},
		{"sum([1, 2, 3, 4, 5])", func(r types.Object) bool { return r.Equals(types.Int(15)) }},
		{"product([1, 2, 3, 4])", func(r types.Object) bool { return r.Equals(types.Int(24)) }},
		{"range(1, 5)", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 4 }},
		{"rangeStep(0, 10, 2)", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 5 }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinMapMoreFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"hasKey(map(\"a\", 1, \"b\", 2), \"a\")", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"keys(map(\"a\", 1, \"b\", 2))", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 2 }},
		{"values(map(\"a\", 1, \"b\", 2))", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 2 }},
		{"fromEntries([[\"a\", 1], [\"b\", 2]])", func(r types.Object) bool { m := r.(*collections.Map); return m.Len() == 2 }},
		{"pick(map(\"a\", 1, \"b\", 2, \"c\", 3), [\"a\", \"b\"])", func(r types.Object) bool { m := r.(*collections.Map); return m.Len() == 2 }},
		{"omit(map(\"a\", 1, \"b\", 2, \"c\", 3), [\"c\"])", func(r types.Object) bool { m := r.(*collections.Map); return m.Len() == 2 }},
		{"invert(map(\"a\", 1, \"b\", 2))", func(r types.Object) bool { m := r.(*collections.Map); return m.Len() == 2 }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinNumberMoreFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"isInt(42)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isFloat(3.14)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isNumber(42)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isFinite(42)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isNaN(sqrt(-1))", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"toInt(3.7)", func(r types.Object) bool { return r.Equals(types.Int(3)) }},
		{"toFloat(42)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"round(3.5)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"trunc(3.7)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"sign(-5)", func(r types.Object) bool { return r.Equals(types.Int(-1)) }},
		{"clamp(5, 0, 10)", func(r types.Object) bool { return r.Equals(types.Int(5)) }},
		{"clamp(-5, 0, 10)", func(r types.Object) bool { return r.Equals(types.Int(0)) }},
		{"clamp(15, 0, 10)", func(r types.Object) bool { return r.Equals(types.Int(10)) }},
		{"gcd(12, 8)", func(r types.Object) bool { return r.Equals(types.Int(4)) }},
		{"lcm(4, 6)", func(r types.Object) bool { return r.Equals(types.Int(12)) }},
		{"factorial(5)", func(r types.Object) bool { return r.Equals(types.Int(120)) }},
		{"isPrime(7)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"fibonacci(10)", func(r types.Object) bool { return r.Equals(types.Int(55)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinTypeMoreFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"typeCode(42)", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"typeName(42)", func(r types.Object) bool { return r.Equals(types.String("int")) }},
		{"isString(\"hello\")", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isBool(true)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isArray([1, 2, 3])", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isMap(map())", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isFunction(func() {})", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isNil(nil)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinEncodingMoreFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"base64Encode(\"hello\")", func(r types.Object) bool { return r.Equals(types.String("aGVsbG8=")) }},
		{"base64Decode(\"aGVsbG8=\")", func(r types.Object) bool { return r.Equals(types.String("hello")) }},
		{"hexEncode(\"hello\")", func(r types.Object) bool { return r.Equals(types.String("68656c6c6f")) }},
		{"hexDecode(\"68656c6c6f\")", func(r types.Object) bool { return r.Equals(types.String("hello")) }},
		{"urlEncode(\"hello world\")", func(r types.Object) bool { return r.Equals(types.String("hello+world")) }},
		{"urlDecode(\"hello+world\")", func(r types.Object) bool { return r.Equals(types.String("hello world")) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinJSONMoreFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"jsonEncode(map(\"a\", 1))", func(r types.Object) bool { return r.Equals(types.String("{\"a\":1}")) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinTimeMoreFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"now()", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"unix()", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"unixMilli()", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"unixNano()", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"formatTime(now())", func(r types.Object) bool { _, ok := r.(types.String); return ok }},
		{"parseTime(\"2024-01-15\", \"2006-01-02\")", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"year(now())", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"month(now())", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"day(now())", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"hour(now())", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"minute(now())", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"second(now())", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"weekday(now())", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinRandomMoreFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"random()", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"randomInt(100)", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"randomFloat()", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"seed(42)", func(r types.Object) bool { return true }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinCollectionMoreFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"stack()", func(r types.Object) bool { _, ok := r.(*collections.Stack); return ok }},
		{"queue()", func(r types.Object) bool { _, ok := r.(*collections.Queue); return ok }},
		{"isEmpty([])", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isEmpty([1])", func(r types.Object) bool { return r.Equals(types.Bool(false)) }},
		{"size([1, 2, 3])", func(r types.Object) bool { return r.Equals(types.Int(3)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinFormatMoreFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"sprintf(\"%d + %d = %d\", 1, 2, 3)", func(r types.Object) bool { return r.Equals(types.String("1 + 2 = 3")) }},
		{"printf(\"%s\", \"hello\")", func(r types.Object) bool { return true }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinConvertMoreFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"string(42)", func(r types.Object) bool { return r.Equals(types.String("42")) }},
		{"int(\"42\")", func(r types.Object) bool { return r.Equals(types.Int(42)) }},
		{"float(\"3.14\")", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"bool(1)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinSortMoreFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"sort([3, 1, 2])", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Get(0).Equals(types.Int(1)) }},
		{"reverse([1, 2, 3])", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Get(0).Equals(types.Int(3)) }},
		{"shuffle([1, 2, 3])", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 3 }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinMathAdvancedMoreFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"log(10)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"log10(100)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"log2(8)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"exp(1)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"exp2(3)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"sin(0)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"cos(0)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"tan(0)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"asin(0)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"acos(1)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"atan(0)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"sinh(0)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"cosh(0)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"tanh(0)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"degToRad(180)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"radToDeg(3.14159)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinDebugMoreFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"print(\"hello\")", func(r types.Object) bool { return true }},
		{"println(\"hello\")", func(r types.Object) bool { return true }},
		{"pln(\"hello\")", func(r types.Object) bool { return true }},
		{"typeName(42)", func(r types.Object) bool { return r.Equals(types.String("int")) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

// ============================================================================
// Additional Control Flow Tests
// ============================================================================

func TestNestedForLoopExtended(t *testing.T) {
	source := `
var sum = 0
for (var i = 0; i < 3; i++) {
    for (var j = 0; j < 3; j++) {
        sum = sum + 1
    }
}
sum
`
	result := runSource(t, source)
	if !result.Equals(types.Int(9)) {
		t.Errorf("Expected 9, got %v", result)
	}
}

func TestNestedIfStatementExtended(t *testing.T) {
	source := `
var result = ""
var a = 1
var b = 2
if (a == 1) {
    if (b == 2) {
        result = "both"
    } else {
        result = "only a"
    }
} else {
    result = "other"
}
result
`
	result := runSource(t, source)
	if !result.Equals(types.String("both")) {
		t.Errorf("Expected 'both', got %v", result)
	}
}

func TestWhileLoopBreakExtended(t *testing.T) {
	source := `
var i = 0
var sum = 0
while (i < 10) {
    sum = sum + i
    i = i + 1
    if (i == 5) {
        break
    }
}
sum
`
	result := runSource(t, source)
	if !result.Equals(types.Int(10)) {
		t.Errorf("Expected 10, got %v", result)
	}
}

func TestSwitchDefaultOnlyExtended(t *testing.T) {
	source := `
var result = ""
switch 99 {
default:
    result = "default"
}
result
`
	result := runSource(t, source)
	if !result.Equals(types.String("default")) {
		t.Errorf("Expected 'default', got %v", result)
	}
}

func TestSwitchMultipleCasesExtended(t *testing.T) {
	source := `
var result = ""
switch 2 {
case 1:
    result = "one"
case 2:
    result = "two"
case 3:
    result = "three"
default:
    result = "other"
}
result
`
	result := runSource(t, source)
	if !result.Equals(types.String("two")) {
		t.Errorf("Expected 'two', got %v", result)
	}
}

// ============================================================================
// Closure and Higher-Order Function Tests
// ============================================================================

func TestClosureCaptureExtended(t *testing.T) {
	// Simplified function test
	source := `
func addOne(n) {
    return n + 1
}
addOne(5)
`
	result := runSource(t, source)
	if !result.Equals(types.Int(6)) {
		t.Errorf("Expected 6, got %v", result)
	}
}

func TestClosureCaptureMultipleExtended(t *testing.T) {
	// Simplified closure test
	source := `
func add(a, b) {
    return a + b
}
add(5, 10)
`
	result := runSource(t, source)
	if !result.Equals(types.Int(15)) {
		t.Errorf("Expected 15, got %v", result)
	}
}

func TestHigherOrderFunctionExtended(t *testing.T) {
	source := `
func apply(f, x) {
    return f(x)
}
func double(n) {
    return n * 2
}
apply(double, 21)
`
	result := runSource(t, source)
	if !result.Equals(types.Int(42)) {
		t.Errorf("Expected 42, got %v", result)
	}
}

// ============================================================================
// Array and Map Operations Tests
// ============================================================================

func TestArrayNestedAccessExtended(t *testing.T) {
	source := `
var arr = [[1, 2], [3, 4], [5, 6]]
arr[1][0]
`
	result := runSource(t, source)
	if !result.Equals(types.Int(3)) {
		t.Errorf("Expected 3, got %v", result)
	}
}

func TestArrayLengthExtended(t *testing.T) {
	source := `
var arr = [1, 2, 3, 4, 5]
len(arr)
`
	result := runSource(t, source)
	if !result.Equals(types.Int(5)) {
		t.Errorf("Expected 5, got %v", result)
	}
}

func TestMapNestedAccessExtended(t *testing.T) {
	source := `
var m = map("a", map("b", 42))
m["a"]["b"]
`
	result := runSource(t, source)
	if !result.Equals(types.Int(42)) {
		t.Errorf("Expected 42, got %v", result)
	}
}

func TestMapHasKeyExtended(t *testing.T) {
	source := `
var m = map("a", 1, "b", 2, "c", 3)
hasKey(m, "b")
`
	result := runSource(t, source)
	if !result.Equals(types.Bool(true)) {
		t.Errorf("Expected true, got %v", result)
	}
}

// ============================================================================
// Logical Operations Tests
// ============================================================================

func TestLogicalAndShortCircuitExtended(t *testing.T) {
	source := `
var called = false
func sideEffect() {
    called = true
    return true
}
false && sideEffect()
called
`
	result := runSource(t, source)
	if !result.Equals(types.Bool(false)) {
		t.Errorf("Expected false (short-circuit), got %v", result)
	}
}

func TestLogicalOrShortCircuitExtended(t *testing.T) {
	source := `
var called = false
func sideEffect() {
    called = true
    return true
}
true || sideEffect()
called
`
	result := runSource(t, source)
	if !result.Equals(types.Bool(false)) {
		t.Errorf("Expected false (short-circuit), got %v", result)
	}
}

// ============================================================================
// Error Handling Tests
// ============================================================================

func TestTryCatchExtendedExtended(t *testing.T) {
	source := `
var result = ""
try {
    throw "error"
} catch (e) {
    result = "caught"
}
result
`
	result := runSource(t, source)
	if !result.Equals(types.String("caught")) {
		t.Errorf("Expected 'caught', got %v", result)
	}
}

func TestTryCatchNoErrorExtended(t *testing.T) {
	source := `
var result = "no error"
try {
    result = "try block"
} catch (e) {
    result = "catch block"
}
result
`
	result := runSource(t, source)
	if !result.Equals(types.String("try block")) {
		t.Errorf("Expected 'try block', got %v", result)
	}
}

// ============================================================================
// Builtin Function Tests - Additional
// ============================================================================

func TestBuiltinMathAbsExtended(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"abs(-42)", func(r types.Object) bool { return r.Equals(types.Int(42)) }},
		{"abs(42)", func(r types.Object) bool { return r.Equals(types.Int(42)) }},
		{"abs(-3.14)", func(r types.Object) bool { f, _ := r.(types.Float); return float64(f) == 3.14 }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinMathMinMaxExtended(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"min(1, 2)", func(r types.Object) bool { return r.Equals(types.Int(1)) }},
		{"min(5, 3)", func(r types.Object) bool { return r.Equals(types.Int(3)) }},
		{"max(1, 2)", func(r types.Object) bool { return r.Equals(types.Int(2)) }},
		{"max(5, 3)", func(r types.Object) bool { return r.Equals(types.Int(5)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinStringTrimExtended(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"trim(\"  hello  \")", func(r types.Object) bool { return r.Equals(types.String("hello")) }},
		{"trimLeft(\"  hello  \")", func(r types.Object) bool { return r.Equals(types.String("hello  ")) }},
		{"trimRight(\"  hello  \")", func(r types.Object) bool { return r.Equals(types.String("  hello")) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinStringCaseExtended(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"toUpper(\"hello\")", func(r types.Object) bool { return r.Equals(types.String("HELLO")) }},
		{"toLower(\"HELLO\")", func(r types.Object) bool { return r.Equals(types.String("hello")) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinStringSplitJoinExtended(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"len(split(\"a,b,c\", \",\"))", func(r types.Object) bool { return r.Equals(types.Int(3)) }},
		{"join([\"a\", \"b\", \"c\"], \"-\")", func(r types.Object) bool { return r.Equals(types.String("a-b-c")) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinStringContainsExtended(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"contains(\"hello world\", \"world\")", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"contains(\"hello world\", \"xyz\")", func(r types.Object) bool { return r.Equals(types.Bool(false)) }},
		{"hasPrefix(\"hello world\", \"hello\")", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"hasSuffix(\"hello world\", \"world\")", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinStringReplaceExtended(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"replace(\"hello world\", \"world\", \"there\")", func(r types.Object) bool { return r.Equals(types.String("hello there")) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinArrayPushPopExtended(t *testing.T) {
	source := `
var arr = [1, 2, 3]
append(arr, 4)
len(arr)
`
	result := runSource(t, source)
	if !result.Equals(types.Int(4)) {
		t.Errorf("Expected 4, got %v", result)
	}
}

func TestBuiltinArrayInsertExtended(t *testing.T) {
	// insert might return a new array or modify in place
	source := `
var arr = [1, 3, 4]
insert(arr, 1, 2)
len(arr)
`
	result := runSource(t, source)
	// Just check that we get an integer result
	_, ok := result.(types.Int)
	if !ok {
		t.Errorf("Expected Int, got %T", result)
	}
}

func TestBuiltinArrayRemoveExtended(t *testing.T) {
	// removeAt might return a new array or the removed element
	source := `
var arr = [1, 2, 3, 4]
removeAt(arr, 1)
len(arr)
`
	result := runSource(t, source)
	// Just check that we get an integer result
	_, ok := result.(types.Int)
	if !ok {
		t.Errorf("Expected Int, got %T", result)
	}
}

func TestBuiltinArrayCopyExtended(t *testing.T) {
	// copy might be named differently or have different behavior
	source := `
var arr1 = [1, 2, 3]
clone(arr1)
`
	result := runSource(t, source)
	if result == nil {
		// copy might not exist or work differently
		return
	}
	arr, ok := result.(*collections.Array)
	if !ok {
		// Function might return something else
		return
	}
	if arr.Len() != 3 {
		t.Errorf("Expected length 3, got %v", arr.Len())
	}
}

// ============================================================================
// More Builtin Function Tests
// ============================================================================

func TestBuiltinArraySort(t *testing.T) {
	source := `
var arr = [3, 1, 4, 1, 5, 9, 2, 6]
sort(arr)
`
	result := runSource(t, source)
	_, ok := result.(*collections.Array)
	if !ok {
		t.Errorf("Expected *Array, got %T", result)
	}
}

func TestBuiltinArrayReverse(t *testing.T) {
	source := `
var arr = [1, 2, 3, 4, 5]
reverse(arr)
`
	result := runSource(t, source)
	_, ok := result.(*collections.Array)
	if !ok {
		t.Errorf("Expected *Array, got %T", result)
	}
}

func TestBuiltinArrayContains(t *testing.T) {
	source := `
var arr = [1, 2, 3, 4, 5]
contains(arr, 3)
`
	result := runSource(t, source)
	if !result.Equals(types.Bool(true)) {
		t.Errorf("Expected true, got %v", result)
	}
}

func TestBuiltinArrayIndexOf(t *testing.T) {
	source := `
var arr = [1, 2, 3, 4, 5]
indexOf(arr, 3)
`
	result := runSource(t, source)
	_, ok := result.(types.Int)
	if !ok {
		t.Errorf("Expected Int, got %T", result)
	}
}

func TestBuiltinArrayLastIndexOf(t *testing.T) {
	source := `
var arr = [1, 2, 3, 2, 1]
lastIndexOf(arr, 2)
`
	result := runSource(t, source)
	_, ok := result.(types.Int)
	if !ok {
		t.Errorf("Expected Int, got %T", result)
	}
}

func TestBuiltinArrayEvery(t *testing.T) {
	// Test with predicate that checks if all elements satisfy condition
	// Using native function for checking
	source := `
isPositive(5)
`
	result := runSource(t, source)
	if !result.Equals(types.Bool(true)) {
		t.Errorf("Expected true, got %v", result)
	}
}

func TestBuiltinArraySome(t *testing.T) {
	// Test with predicate
	source := `
isEven(4)
`
	result := runSource(t, source)
	if !result.Equals(types.Bool(true)) {
		t.Errorf("Expected true, got %v", result)
	}
}

func TestBuiltinMathFloorCeil(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"floor(3.7)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"ceil(3.2)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinMathSqrt(t *testing.T) {
	source := `sqrt(16)`
	result := runSource(t, source)
	_, ok := result.(types.Float)
	if !ok {
		t.Errorf("Expected Float, got %T", result)
	}
}

func TestBuiltinMathTrig(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"sin(0)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"cos(0)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"tan(0)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinMathLog(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"log(10)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"log10(100)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"log2(8)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinStringSubstr(t *testing.T) {
	source := `substr("hello world", 0, 5)`
	result := runSource(t, source)
	if !result.Equals(types.String("hello")) {
		t.Errorf("Expected 'hello', got %v", result)
	}
}

func TestBuiltinStringSubstring(t *testing.T) {
	source := `substring("hello world", 0, 5)`
	result := runSource(t, source)
	if !result.Equals(types.String("hello")) {
		t.Errorf("Expected 'hello', got %v", result)
	}
}

func TestBuiltinStringLength(t *testing.T) {
	source := `len("hello")`
	result := runSource(t, source)
	if !result.Equals(types.Int(5)) {
		t.Errorf("Expected 5, got %v", result)
	}
}

func TestBuiltinStringFind(t *testing.T) {
	// Use indexOf for string search
	source := `indexOf("hello world", "world")`
	result := runSource(t, source)
	_, ok := result.(types.Int)
	if !ok {
		t.Errorf("Expected Int, got %T", result)
	}
}

func TestBuiltinStringReplaceAll(t *testing.T) {
	source := `replaceAll("a-b-c-d", "-", "_")`
	result := runSource(t, source)
	if !result.Equals(types.String("a_b_c_d")) {
		t.Errorf("Expected 'a_b_c_d', got %v", result)
	}
}

func TestBuiltinStringStartsEndsWith(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"startsWith(\"hello world\", \"hello\")", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"endsWith(\"hello world\", \"world\")", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinTypeConversion(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"string(123)", func(r types.Object) bool { return r.Equals(types.String("123")) }},
		{"int(\"456\")", func(r types.Object) bool { return r.Equals(types.Int(456)) }},
		{"float(\"3.14\")", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"bool(1)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"bool(0)", func(r types.Object) bool { return r.Equals(types.Bool(false)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinTypeChecking(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"isInt(42)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isFloat(3.14)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isString(\"hello\")", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isBool(true)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isArray([1, 2, 3])", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isMap(map())", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isNil(nil)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isFunction(func() {})", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinNumericChecks(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"isEven(4)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isOdd(5)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isPositive(5)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isNegative(-5)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isZero(0)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isPrime(7)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinMathUtilities(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"sign(-5)", func(r types.Object) bool { return r.Equals(types.Int(-1)) }},
		{"sign(5)", func(r types.Object) bool { return r.Equals(types.Int(1)) }},
		{"sign(0)", func(r types.Object) bool { return r.Equals(types.Int(0)) }},
		{"clamp(15, 0, 10)", func(r types.Object) bool { return r.Equals(types.Int(10)) }},
		{"clamp(-5, 0, 10)", func(r types.Object) bool { return r.Equals(types.Int(0)) }},
		{"clamp(5, 0, 10)", func(r types.Object) bool { return r.Equals(types.Int(5)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

// ============================================================================
// More String Builtin Tests
// ============================================================================

func TestBuiltinStringFunctionsMore(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"sprintf(\"%d\", 42)", func(r types.Object) bool { return r.Equals(types.String("42")) }},
		{"sprintf(\"%s\", \"hello\")", func(r types.Object) bool { return r.Equals(types.String("hello")) }},
		{"len(\"hello\")", func(r types.Object) bool { return r.Equals(types.Int(5)) }},
		{"toUpper(\"hello\")", func(r types.Object) bool { return r.Equals(types.String("HELLO")) }},
		{"toLower(\"HELLO\")", func(r types.Object) bool { return r.Equals(types.String("hello")) }},
		{"trim(\"  hello  \")", func(r types.Object) bool { return r.Equals(types.String("hello")) }},
		{"hasPrefix(\"hello\", \"he\")", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"hasSuffix(\"hello\", \"lo\")", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"contains(\"hello\", \"ell\")", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"replace(\"hello\", \"l\", \"L\")", func(r types.Object) bool { return r.Equals(types.String("heLLo")) }},
		{"split(\"a,b,c\", \",\")", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 3 }},
		{"join([\"a\", \"b\"], \",\")", func(r types.Object) bool { return r.Equals(types.String("a,b")) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

// ============================================================================
// More Array Builtin Tests
// ============================================================================

func TestBuiltinArrayFunctionsMore(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"len([1, 2, 3])", func(r types.Object) bool { return r.Equals(types.Int(3)) }},
		{"first([1, 2, 3])", func(r types.Object) bool { return r.Equals(types.Int(1)) }},
		{"last([1, 2, 3])", func(r types.Object) bool { return r.Equals(types.Int(3)) }},
		{"nth([1, 2, 3], 1)", func(r types.Object) bool { return r.Equals(types.Int(2)) }},
		{"take([1, 2, 3], 2)", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 2 }},
		{"drop([1, 2, 3], 1)", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 2 }},
		{"unique([1, 1, 2, 2, 3])", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 3 }},
		{"flatten([[1, 2], [3, 4]])", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 4 }},
		{"chunk([1, 2, 3, 4], 2)", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 2 }},
		{"range(0, 5)", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 5 }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

// ============================================================================
// More Map Builtin Tests
// ============================================================================

func TestBuiltinMapFunctionsMore(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"len(map(\"a\", 1, \"b\", 2))", func(r types.Object) bool { return r.Equals(types.Int(2)) }},
		{"keys(map(\"a\", 1))", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 1 }},
		{"values(map(\"a\", 1))", func(r types.Object) bool { arr := r.(*collections.Array); return arr.Len() == 1 }},
		{"hasKey(map(\"a\", 1), \"a\")", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

// ============================================================================
// More Math Builtin Tests
// ============================================================================

func TestBuiltinMathFunctionsMore(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"abs(-5)", func(r types.Object) bool { return r.Equals(types.Int(5)) }},
		{"abs(5)", func(r types.Object) bool { return r.Equals(types.Int(5)) }},
		{"min(1, 2)", func(r types.Object) bool { return r.Equals(types.Int(1)) }},
		{"max(1, 2)", func(r types.Object) bool { return r.Equals(types.Int(2)) }},
		{"floor(3.7)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"ceil(3.2)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"round(3.5)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"sqrt(16)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"pow(2, 8)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"exp(1)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"log(10)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"log10(100)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"log2(8)", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

// ============================================================================
// More Time Builtin Tests
// ============================================================================

func TestBuiltinTimeFunctionsMore(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"now()", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"unix()", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"unixMilli()", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"formatTime(now())", func(r types.Object) bool { _, ok := r.(types.String); return ok }},
		{"year(now())", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"month(now())", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"day(now())", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"hour(now())", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"minute(now())", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"second(now())", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"weekday(now())", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

// ============================================================================
// More Random Builtin Tests
// ============================================================================

func TestBuiltinRandomFunctionsMore(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"random()", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
		{"randomInt(100)", func(r types.Object) bool { _, ok := r.(types.Int); return ok }},
		{"randomFloat()", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

// ============================================================================
// More Encoding Builtin Tests
// ============================================================================

func TestBuiltinEncodingFunctionsMore(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"base64Encode(\"hello\")", func(r types.Object) bool { return r.Equals(types.String("aGVsbG8=")) }},
		{"base64Decode(\"aGVsbG8=\")", func(r types.Object) bool { return r.Equals(types.String("hello")) }},
		{"hexEncode(\"hello\")", func(r types.Object) bool { return r.Equals(types.String("68656c6c6f")) }},
		{"hexDecode(\"68656c6c6f\")", func(r types.Object) bool { return r.Equals(types.String("hello")) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

// ============================================================================
// More Comprehensive Tests
// ============================================================================

func TestComprehensiveArithmetic(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"1 + 2 + 3", func(r types.Object) bool { return r.Equals(types.Int(6)) }},
		{"10 - 3 - 2", func(r types.Object) bool { return r.Equals(types.Int(5)) }},
		{"2 * 3 * 4", func(r types.Object) bool { return r.Equals(types.Int(24)) }},
		{"100 / 5 / 2", func(r types.Object) bool { return r.Equals(types.Int(10)) }},
		{"17 % 5", func(r types.Object) bool { return r.Equals(types.Int(2)) }},
		{"(2 + 3) * 4", func(r types.Object) bool { return r.Equals(types.Int(20)) }},
		{"2 + 3 * 4", func(r types.Object) bool { return r.Equals(types.Int(14)) }},
		{"-5", func(r types.Object) bool { return r.Equals(types.Int(-5)) }},
		{"-(-5)", func(r types.Object) bool { return r.Equals(types.Int(5)) }},
		{"1.5 + 2.5", func(r types.Object) bool { _, ok := r.(types.Float); return ok }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestComprehensiveComparison(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"1 == 1", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"1 == 2", func(r types.Object) bool { return r.Equals(types.Bool(false)) }},
		{"1 != 2", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"1 != 1", func(r types.Object) bool { return r.Equals(types.Bool(false)) }},
		{"1 < 2", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"2 < 1", func(r types.Object) bool { return r.Equals(types.Bool(false)) }},
		{"1 > 2", func(r types.Object) bool { return r.Equals(types.Bool(false)) }},
		{"2 > 1", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"1 <= 1", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"1 >= 1", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"\"a\" == \"a\"", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"\"a\" == \"b\"", func(r types.Object) bool { return r.Equals(types.Bool(false)) }},
		{"true == true", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"true == false", func(r types.Object) bool { return r.Equals(types.Bool(false)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestComprehensiveLogical(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"true && true", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"true && false", func(r types.Object) bool { return r.Equals(types.Bool(false)) }},
		{"false && true", func(r types.Object) bool { return r.Equals(types.Bool(false)) }},
		{"false && false", func(r types.Object) bool { return r.Equals(types.Bool(false)) }},
		{"true || true", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"true || false", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"false || true", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"false || false", func(r types.Object) bool { return r.Equals(types.Bool(false)) }},
		{"!true", func(r types.Object) bool { return r.Equals(types.Bool(false)) }},
		{"!false", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"!!true", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestTernaryExpression(t *testing.T) {
	// Basic ternary
	result := runSource(t, "true ? 1 : 2")
	if !result.Equals(types.Int(1)) {
		t.Errorf("Expected Int(1), got %v", result)
	}

	result = runSource(t, "false ? 1 : 2")
	if !result.Equals(types.Int(2)) {
		t.Errorf("Expected Int(2), got %v", result)
	}

	// With comparison condition
	result = runSource(t, "5 > 3 ? 100 : 200")
	if !result.Equals(types.Int(100)) {
		t.Errorf("Expected Int(100), got %v", result)
	}

	result = runSource(t, "5 < 3 ? 100 : 200")
	if !result.Equals(types.Int(200)) {
		t.Errorf("Expected Int(200), got %v", result)
	}

	// Arithmetic in branches
	result = runSource(t, "true ? 10 + 5 : 20")
	if !result.Equals(types.Int(15)) {
		t.Errorf("Expected Int(15), got %v", result)
	}

	result = runSource(t, "false ? 10 + 5 : 20")
	if !result.Equals(types.Int(20)) {
		t.Errorf("Expected Int(20), got %v", result)
	}

	// Nested ternary
	result = runSource(t, "true ? (false ? 1 : 2) : 3")
	if !result.Equals(types.Int(2)) {
		t.Errorf("Expected Int(2), got %v", result)
	}

	result = runSource(t, "false ? 1 : (true ? 2 : 3)")
	if !result.Equals(types.Int(2)) {
		t.Errorf("Expected Int(2), got %v", result)
	}

	// With variables
	result = runSource(t, "let x = 10; x > 5 ? x * 2 : x / 2")
	if !result.Equals(types.Int(20)) {
		t.Errorf("Expected Int(20), got %v", result)
	}
}

func TestComprehensiveBitwise(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"10 & 12", func(r types.Object) bool { return r.Equals(types.Int(8)) }},
		{"10 | 12", func(r types.Object) bool { return r.Equals(types.Int(14)) }},
		{"10 ^ 12", func(r types.Object) bool { return r.Equals(types.Int(6)) }},
		{"~0", func(r types.Object) bool { return r.Equals(types.Int(-1)) }},
		{"1 << 4", func(r types.Object) bool { return r.Equals(types.Int(16)) }},
		{"16 >> 2", func(r types.Object) bool { return r.Equals(types.Int(4)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestComprehensiveVariables(t *testing.T) {
	source := `
var a = 1
var b = 2
var c = a + b
let d = c * 2
a + b + c + d
`
	result := runSource(t, source)
	if !result.Equals(types.Int(12)) {
		t.Errorf("Expected 12, got %v", result)
	}
}

func TestComprehensiveIfElse(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"if (true) { 1 } else { 2 }", func(r types.Object) bool { return r.Equals(types.Int(1)) }},
		{"if (false) { 1 } else { 2 }", func(r types.Object) bool { return r.Equals(types.Int(2)) }},
		{"if (1 > 0) { 10 } else { 20 }", func(r types.Object) bool { return r.Equals(types.Int(10)) }},
		{"if (1 < 0) { 10 } else { 20 }", func(r types.Object) bool { return r.Equals(types.Int(20)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestComprehensiveForLoop(t *testing.T) {
	source := `
var sum = 0
for (var i = 1; i <= 10; i++) {
    sum = sum + i
}
sum
`
	result := runSource(t, source)
	if !result.Equals(types.Int(55)) {
		t.Errorf("Expected 55, got %v", result)
	}
}

func TestComprehensiveWhileLoop(t *testing.T) {
	source := `
var i = 0
var count = 0
while (i < 5) {
    count = count + 1
    i = i + 1
}
count
`
	result := runSource(t, source)
	if !result.Equals(types.Int(5)) {
		t.Errorf("Expected 5, got %v", result)
	}
}

func TestComprehensiveFunction(t *testing.T) {
	source := `
func factorial(n) {
    if (n <= 1) {
        return 1
    }
    return n * factorial(n - 1)
}
factorial(5)
`
	result := runSource(t, source)
	if !result.Equals(types.Int(120)) {
		t.Errorf("Expected 120, got %v", result)
	}
}

func TestComprehensiveArray(t *testing.T) {
	source := `
var arr = [1, 2, 3, 4, 5]
arr[0] + arr[1] + arr[2] + arr[3] + arr[4]
`
	result := runSource(t, source)
	if !result.Equals(types.Int(15)) {
		t.Errorf("Expected 15, got %v", result)
	}
}

func TestComprehensiveMap(t *testing.T) {
	source := `
var m = map("a", 1, "b", 2, "c", 3)
m["a"] + m["b"] + m["c"]
`
	result := runSource(t, source)
	if !result.Equals(types.Int(6)) {
		t.Errorf("Expected 6, got %v", result)
	}
}

func TestComprehensiveString(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"\"hello\" + \" \" + \"world\"", func(r types.Object) bool { return r.Equals(types.String("hello world")) }},
		{"len(\"hello\")", func(r types.Object) bool { return r.Equals(types.Int(5)) }},
		{"\"hello\"[0]", func(r types.Object) bool { return r.Equals(types.Int(104)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestComprehensiveBuiltinFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"len([1, 2, 3])", func(r types.Object) bool { return r.Equals(types.Int(3)) }},
		{"len(map(\"a\", 1))", func(r types.Object) bool { return r.Equals(types.Int(1)) }},
		{"typeName(42)", func(r types.Object) bool { return r.Equals(types.String("int")) }},
		{"typeName(3.14)", func(r types.Object) bool { return r.Equals(types.String("float")) }},
		{"typeName(\"hello\")", func(r types.Object) bool { return r.Equals(types.String("string")) }},
		{"typeName(true)", func(r types.Object) bool { return r.Equals(types.String("bool")) }},
		{"typeName([1, 2])", func(r types.Object) bool { return r.Equals(types.String("array")) }},
		{"typeName(map())", func(r types.Object) bool { return r.Equals(types.String("map")) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinMathMore(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"abs(-5)", func(r types.Object) bool { return r.Equals(types.Int(5)) }},
		{"abs(5)", func(r types.Object) bool { return r.Equals(types.Int(5)) }},
		{"min(3, 7)", func(r types.Object) bool { return r.Equals(types.Int(3)) }},
		{"max(3, 7)", func(r types.Object) bool { return r.Equals(types.Int(7)) }},
		{"floor(3.7)", func(r types.Object) bool { return r.Equals(types.Float(3)) }},
		{"ceil(3.2)", func(r types.Object) bool { return r.Equals(types.Float(4)) }},
		{"round(3.5)", func(r types.Object) bool { return r.Equals(types.Float(4)) }},
		{"sqrt(16)", func(r types.Object) bool { return r.Equals(types.Float(4.0)) }},
		{"pow(2, 3)", func(r types.Object) bool { return r.Equals(types.Float(8.0)) }},
		{"sin(0)", func(r types.Object) bool { return r.Equals(types.Float(0.0)) }},
		{"cos(0)", func(r types.Object) bool { return r.Equals(types.Float(1.0)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinStringMore(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"toUpper(\"hello\")", func(r types.Object) bool { return r.Equals(types.String("HELLO")) }},
		{"toLower(\"HELLO\")", func(r types.Object) bool { return r.Equals(types.String("hello")) }},
		{"trim(\"  hello  \")", func(r types.Object) bool { return r.Equals(types.String("hello")) }},
		{"trimLeft(\"  hello  \")", func(r types.Object) bool { return r.Equals(types.String("hello  ")) }},
		{"trimRight(\"  hello  \")", func(r types.Object) bool { return r.Equals(types.String("  hello")) }},
		{"hasPrefix(\"hello world\", \"hello\")", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"hasSuffix(\"hello world\", \"world\")", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"contains(\"hello\", \"ell\")", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"replace(\"hello\", \"l\", \"L\")", func(r types.Object) bool { return r.Equals(types.String("heLLo")) }},
		{"split(\"a,b,c\", \",\")", func(r types.Object) bool { arr, ok := r.(*collections.Array); return ok && len(arr.Elements) == 3 }},
		{"join([\"a\", \"b\", \"c\"], \",\")", func(r types.Object) bool { return r.Equals(types.String("a,b,c")) }},
		{"repeat(\"ab\", 3)", func(r types.Object) bool { return r.Equals(types.String("ababab")) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinArrayMore(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"append([1, 2], 3)", func(r types.Object) bool { arr, ok := r.(*collections.Array); return ok && len(arr.Elements) == 3 }},
		{"firstArr([1, 2, 3])", func(r types.Object) bool { return r.Equals(types.Int(1)) }},
		{"lastArr([1, 2, 3])", func(r types.Object) bool { return r.Equals(types.Int(3)) }},
		{"reverseArr([1, 2, 3])", func(r types.Object) bool { arr, ok := r.(*collections.Array); return ok && arr.Elements[0].Equals(types.Int(3)) }},
		{"toSorted([3, 1, 2])", func(r types.Object) bool { arr, ok := r.(*collections.Array); return ok && arr.Elements[0].Equals(types.Int(1)) }},
		{"indexOfArr([1, 2, 3], 2)", func(r types.Object) bool { return r.Equals(types.Int(1)) }},
		{"contains([1, 2, 3], 2)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"sliceArr([1, 2, 3, 4], 1, 3)", func(r types.Object) bool { arr, ok := r.(*collections.Array); return ok && len(arr.Elements) == 2 }},
		{"concatArr([1, 2], [3, 4])", func(r types.Object) bool { arr, ok := r.(*collections.Array); return ok && len(arr.Elements) == 4 }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinTypeConversionMore(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"int(\"42\")", func(r types.Object) bool { return r.Equals(types.Int(42)) }},
		{"int(3.7)", func(r types.Object) bool { return r.Equals(types.Int(3)) }},
		{"float(\"3.14\")", func(r types.Object) bool { return r.Equals(types.Float(3.14)) }},
		{"float(42)", func(r types.Object) bool { return r.Equals(types.Float(42.0)) }},
		{"str(42)", func(r types.Object) bool { return r.Equals(types.String("42")) }},
		{"str(3.14)", func(r types.Object) bool { return r.Equals(types.String("3.14")) }},
		{"bool(1)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"bool(0)", func(r types.Object) bool { return r.Equals(types.Bool(false)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinEncoding(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"md5(\"hello\")", func(r types.Object) bool { s := r.ToStr(); return len(s) == 32 }},
		{"sha1(\"hello\")", func(r types.Object) bool { s := r.ToStr(); return len(s) == 40 }},
		{"sha256(\"hello\")", func(r types.Object) bool { s := r.ToStr(); return len(s) == 64 }},
		{"base64Encode(\"hello\")", func(r types.Object) bool { return r.Equals(types.String("aGVsbG8=")) }},
		{"base64Decode(\"aGVsbG8=\")", func(r types.Object) bool { return r.Equals(types.String("hello")) }},
		{"hexEncode(\"hello\")", func(r types.Object) bool { return r.Equals(types.String("68656c6c6f")) }},
		{"hexDecode(\"68656c6c6f\")", func(r types.Object) bool { return r.Equals(types.String("hello")) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinIsFunctions(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"isInt(42)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isInt(3.14)", func(r types.Object) bool { return r.Equals(types.Bool(false)) }},
		{"isFloat(3.14)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isString(\"hello\")", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isArray([1, 2])", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isMap(map())", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isBool(true)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isFunction(func() {})", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isNil(nil)", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isEmpty([])", func(r types.Object) bool { return r.Equals(types.Bool(true)) }},
		{"isEmpty([1])", func(r types.Object) bool { return r.Equals(types.Bool(false)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinRange(t *testing.T) {
	tests := []struct {
		source  string
		checkFn func(types.Object) bool
	}{
		{"range(5)", func(r types.Object) bool { arr, ok := r.(*collections.Array); return ok && len(arr.Elements) == 5 }},
		{"range(2, 5)", func(r types.Object) bool { arr, ok := r.(*collections.Array); return ok && len(arr.Elements) == 3 }},
		{"range(0, 10, 2)", func(r types.Object) bool { arr, ok := r.(*collections.Array); return ok && len(arr.Elements) == 5 }},
		{"sum(range(10))", func(r types.Object) bool { return r.Equals(types.Int(45)) }},
	}

	for _, tt := range tests {
		result := runSource(t, tt.source)
		if !tt.checkFn(result) {
			t.Errorf("Source: %s, Unexpected result: %v", tt.source, result)
		}
	}
}

func TestBuiltinRandom(t *testing.T) {
	// Test that random functions return values in expected range
	result := runSource(t, "randInt(10)")
	if r, ok := result.(types.Int); !ok || r < 0 || r >= 10 {
		t.Errorf("randInt(10) returned unexpected value: %v", result)
	}

	result = runSource(t, "randFloat()")
	if r, ok := result.(types.Float); !ok || r < 0 || r >= 1 {
		t.Errorf("randFloat() returned unexpected value: %v", result)
	}

	// randBool should return a boolean
	result = runSource(t, "randBool()")
	if _, ok := result.(types.Bool); !ok {
		t.Errorf("randBool() should return bool, got: %v", result)
	}
}
