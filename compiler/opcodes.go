package compiler

// Opcode represents a bytecode instruction
type Opcode byte

// Bytecode instruction set
const (
	// NOP: No operation
	OpNOP Opcode = 0x00

	// Stack operations
	OpPush      Opcode = 0x01 // Push constant from constant pool: OpPush <const_index>
	OpPop       Opcode = 0x02 // Pop top value from stack
	OpDup       Opcode = 0x03 // Duplicate top value on stack
	OpSwap      Opcode = 0x04 // Swap top two values on stack

	// Load/store operations
	OpLoadLocal  Opcode = 0x10 // Load local variable: OpLoadLocal <local_index>
	OpStoreLocal Opcode = 0x11 // Store local variable: OpStoreLocal <local_index>
	OpLoadGlobal Opcode = 0x12 // Load global variable: OpLoadGlobal <name_index>
	OpStoreGlobal Opcode = 0x13 // Store global variable: OpStoreGlobal <name_index>
	OpLoadUpvalue Opcode = 0x14 // Load upvalue (closure variable): OpLoadUpvalue <upvalue_index>
	OpStoreUpvalue Opcode = 0x15 // Store upvalue: OpStoreUpvalue <upvalue_index>
	OpLoadConst  Opcode = 0x16 // Load constant: OpLoadConst <const_index> (alias for OpPush)

	// Arithmetic operations
	OpAdd    Opcode = 0x20 // Add top two values: b + a
	OpSub    Opcode = 0x21 // Subtract top two values: b - a
	OpMul    Opcode = 0x22 // Multiply top two values: b * a
	OpDiv    Opcode = 0x23 // Divide top two values: b / a
	OpMod    Opcode = 0x24 // Modulo top two values: b % a
	OpNeg    Opcode = 0x25 // Negate top value: -a
	OpPow    Opcode = 0x26 // Power: b ** a

	// Bitwise operations
	OpBitAnd  Opcode = 0x30 // Bitwise AND: b & a
	OpBitOr   Opcode = 0x31 // Bitwise OR: b | a
	OpBitXor  Opcode = 0x32 // Bitwise XOR: b ^ a
	OpBitNot  Opcode = 0x33 // Bitwise NOT: ~a
	OpShiftL  Opcode = 0x34 // Left shift: b << a
	OpShiftR  Opcode = 0x35 // Right shift: b >> a

	// Comparison operations
	OpEq     Opcode = 0x40 // Equal: b == a
	OpNotEq  Opcode = 0x41 // Not equal: b != a
	OpLt     Opcode = 0x42 // Less than: b < a
	OpLte    Opcode = 0x43 // Less than or equal: b <= a
	OpGt     Opcode = 0x44 // Greater than: b > a
	OpGte    Opcode = 0x45 // Greater than or equal: b >= a

	// Logical operations
	OpNot    Opcode = 0x50 // Logical NOT: !a
	OpAnd    Opcode = 0x51 // Logical AND: b && a (short-circuit)
	OpOr     Opcode = 0x52 // Logical OR: b || a (short-circuit)

	// Control flow
	OpJmp          Opcode = 0x60 // Unconditional jump: OpJmp <offset>
	OpJmpIfTrue    Opcode = 0x61 // Jump if top value is true: OpJmpIfTrue <offset>
	OpJmpIfFalse   Opcode = 0x62 // Jump if top value is false: OpJmpIfFalse <offset>
	OpCall         Opcode = 0x63 // Call function: OpCall <arg_count>
	OpReturn       Opcode = 0x64 // Return from function with value
	OpReturnVoid   Opcode = 0x65 // Return from function without value
	OpClosure      Opcode = 0x66 // Create closure: OpClosure <func_index> <upvalue_count> <upvalue1> ... <upvalueN>

	// Data structures
	OpNewArray     Opcode = 0x70 // Create new array: OpNewArray <element_count>
	OpNewMap       Opcode = 0x71 // Create new map
	OpIndexGet     Opcode = 0x72 // Get index: collection[index]
	OpIndexSet     Opcode = 0x73 // Set index: collection[index] = value
	OpMemberGet    Opcode = 0x74 // Get member: object.member
	OpMemberSet    Opcode = 0x75 // Set member: object.member = value
	OpNewObject    Opcode = 0x76 // Create new object instance: OpNewObject <class_index> <arg_count>

	// Built-in functions
	OpPrint        Opcode = 0x80 // Print value to stdout
	OpPrintLine    Opcode = 0x81 // Print value with newline
	OpTypeCode     Opcode = 0x82 // Get type code of value
	OpTypeName     Opcode = 0x83 // Get type name of value
	OpIsError      Opcode = 0x84 // Check if value is an error
	OpThrow        Opcode = 0x85 // Throw an error

	// Exception handling
	OpTry          Opcode = 0x90 // Start of try block: OpTry <catch_offset> <finally_offset>
	OpCatch        Opcode = 0x91 // Start of catch block
	OpFinally      Opcode = 0x92 // Start of finally block
	OpDefer        Opcode = 0x93 // Record deferred function call

	// Module system
	OpImport       Opcode = 0xA0 // Import module: OpImport <path_const_index>
	OpImportMember Opcode = 0xA1 // Import specific member: OpImportMember <module_const_index> <name_const_index>
)

// OpcodeInfo contains metadata about an opcode
type OpcodeInfo struct {
	Name      string
	Operands  int // Number of operand bytes
	StackPop  int // Number of values popped from stack
	StackPush int // Number of values pushed to stack
}

// OpcodeTable maps opcodes to their metadata
var OpcodeTable = map[Opcode]OpcodeInfo{
	OpNOP:          {"NOP", 0, 0, 0},
	OpPush:         {"PUSH", 2, 0, 1}, // 2-byte constant index
	OpPop:          {"POP", 0, 1, 0},
	OpDup:          {"DUP", 0, 1, 2},
	OpSwap:         {"SWAP", 0, 2, 2},

	OpLoadLocal:    {"LOAD_LOCAL", 1, 0, 1}, // 1-byte local index
	OpStoreLocal:   {"STORE_LOCAL", 1, 1, 0},
	OpLoadGlobal:   {"LOAD_GLOBAL", 2, 0, 1}, // 2-byte name index
	OpStoreGlobal:  {"STORE_GLOBAL", 2, 1, 0},
	OpLoadUpvalue:  {"LOAD_UPVALUE", 1, 0, 1}, // 1-byte upvalue index
	OpStoreUpvalue: {"STORE_UPVALUE", 1, 1, 0},
	OpLoadConst:    {"LOAD_CONST", 2, 0, 1},

	OpAdd:          {"ADD", 0, 2, 1},
	OpSub:          {"SUB", 0, 2, 1},
	OpMul:          {"MUL", 0, 2, 1},
	OpDiv:          {"DIV", 0, 2, 1},
	OpMod:          {"MOD", 0, 2, 1},
	OpNeg:          {"NEG", 0, 1, 1},
	OpPow:          {"POW", 0, 2, 1},

	OpBitAnd:       {"BIT_AND", 0, 2, 1},
	OpBitOr:        {"BIT_OR", 0, 2, 1},
	OpBitXor:       {"BIT_XOR", 0, 2, 1},
	OpBitNot:       {"BIT_NOT", 0, 1, 1},
	OpShiftL:       {"SHIFT_L", 0, 2, 1},
	OpShiftR:       {"SHIFT_R", 0, 2, 1},

	OpEq:           {"EQ", 0, 2, 1},
	OpNotEq:        {"NOT_EQ", 0, 2, 1},
	OpLt:           {"LT", 0, 2, 1},
	OpLte:          {"LTE", 0, 2, 1},
	OpGt:           {"GT", 0, 2, 1},
	OpGte:          {"GTE", 0, 2, 1},

	OpNot:          {"NOT", 0, 1, 1},
	OpAnd:          {"AND", 0, 2, 1},
	OpOr:           {"OR", 0, 2, 1},

	OpJmp:          {"JMP", 2, 0, 0}, // 2-byte offset
	OpJmpIfTrue:    {"JMP_IF_TRUE", 2, 1, 0},
	OpJmpIfFalse:   {"JMP_IF_FALSE", 2, 1, 0},
	OpCall:         {"CALL", 1, 0, 1}, // 1-byte arg count, pops args + function
	OpReturn:       {"RETURN", 0, 1, 0},
	OpReturnVoid:   {"RETURN_VOID", 0, 0, 0},
	OpClosure:      {"CLOSURE", 3, 0, 1}, // 2-byte func index + 1-byte upvalue count + N upvalue specs

	OpNewArray:     {"NEW_ARRAY", 2, 0, 1}, // 2-byte element count
	OpNewMap:       {"NEW_MAP", 0, 0, 1},
	OpIndexGet:     {"INDEX_GET", 0, 2, 1}, // pops index and collection, pushes value
	OpIndexSet:     {"INDEX_SET", 0, 3, 0}, // pops value, index and collection
	OpMemberGet:    {"MEMBER_GET", 2, 1, 1}, // pops object, pushes value, 2-byte name index
	OpMemberSet:    {"MEMBER_SET", 2, 2, 0}, // pops value and object, 2-byte name index
	OpNewObject:    {"NEW_OBJECT", 3, 0, 1}, // 2-byte class index + 1-byte arg count

	OpPrint:        {"PRINT", 0, 1, 0},
	OpPrintLine:    {"PRINT_LINE", 0, 1, 0},
	OpTypeCode:     {"TYPE_CODE", 0, 1, 1},
	OpTypeName:     {"TYPE_NAME", 0, 1, 1},
	OpIsError:      {"IS_ERROR", 0, 1, 1},
	OpThrow:        {"THROW", 0, 1, 0},

	// Exception handling
	OpTry:          {"TRY", 4, 0, 0}, // 2-byte catch offset + 2-byte finally offset
	OpCatch:        {"CATCH", 0, 0, 1}, // Pushes the caught exception to stack
	OpFinally:      {"FINALLY", 0, 0, 0},
	OpDefer:        {"DEFER", 1, 0, 0}, // 1-byte argument count, pops args + function from stack and records as defer

	// Module system
	OpImport:       {"IMPORT", 2, 0, 1}, // 2-byte path constant index, pushes module object to stack
	OpImportMember: {"IMPORT_MEMBER", 2, 1, 1}, // 2-byte name constant index, pops module, pushes member to stack
}
