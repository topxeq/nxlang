package vm

import (
	"strings"
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

func TestBitwiseOperations(t *testing.T) {
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

func TestStringConcatenation(t *testing.T) {
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

func TestModulo(t *testing.T) {
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
