# Nxlang Improvements Design

Date: 2026-03-11

## Overview

This design document outlines improvements to the Nxlang scripting language implementation based on task.md requirements. The improvements follow an incremental approach to ensure stability and testability.

## Current State Analysis

### Working Features
- Basic arithmetic operations
- Variable declarations (let, var, :=)
- Recursive function calls
- For loops (for i in N, for i,v in array/map)
- Arrays and Maps
- Type functions (typeOf, typeCode, typeName)
- Primitive types: int, float, bool, string, char, byte, uint
- Constants (const keyword, piC, eC)
- compile() and runByteCode() functions
- String operations (toUpper, toLower, trim, split, etc.)
- Static methods (int.parse, float.parse)
- HTTP functions
- Plugin system

### Missing/Problematic Features
1. **error() builtin function** - Not registered in VM
2. **toJson() function** - Referenced but not implemented
3. **fromJson() function** - Marked as TODO
4. **class instantiation** - Error: "cannot call non-function type class"
5. **Directory structure** - Missing internal/, stdlib/, pkg/
6. **Go test files** - No test files in any package

## Design

### Task 1: Add Go Unit Tests

#### Test Files Structure
```
parser/parser_test.go     - Lexer and parser tests
compiler/compiler_test.go - Compiler tests
vm/vm_test.go             - Virtual machine tests
types/convert_test.go     - Type conversion tests
```

#### Test Cases (based on task.md)
1. Hello World
2. Arithmetic operations (+, -, *, /)
3. Variable declarations
4. Recursive function calls (fibonacci)
5. Control flow (if/for)
6. Arrays and Maps
7. Error handling (division by zero, undefined variable)
8. Type conversions

### Task 2: Implement Missing Builtin Functions

#### error(msg) Function
- Location: vm/vm.go registerBuiltins()
- Creates Error object with message
- Returns types.NewError(msg, 0, 0, "")

#### toJson(obj, opts...) Function
- Location: vm/vm.go registerBuiltins()
- Converts object to JSON string
- Options: "-sort" (sort keys), "-indent" (pretty print)
- Supports: maps, arrays, primitives

#### fromJson(str) Function
- Location: vm/vm.go registerBuiltins()
- Parses JSON string to Nxlang objects
- Returns map/array/primitive

### Task 3: Fix Class Definition

#### Problem
Class instantiation fails with "cannot call non-function type class"

#### Investigation Areas
1. Compiler: How class expressions are compiled
2. VM: How OpNewObject opcode handles class types
3. Parser: How class definitions are parsed

#### Solution Approach
1. Verify Class type is properly registered
2. Check OpNewObject handling in VM
3. Ensure class constant is properly created

### Task 4: Complete Directory Structure

#### New Directories
```
internal/  - Internal packages (optional reorganization)
stdlib/    - Standard library modules
pkg/       - Public API for embedding
```

#### Considerations
- Keep existing structure working
- Add new directories incrementally
- Update imports if reorganizing

### Task 5: Create Sync Script

#### Target Directory
/root/goprjs/src/github.com/topxeq/nxlang

#### Files to Sync
- All .go source files
- go.mod, go.sum
- LICENSE
- README.md, README_zh-CN.md
- docs/
- examples/

#### Script Location
scripts/sync_to_github.sh

## Implementation Order

1. Create Go unit tests (foundation for validation)
2. Implement error(), toJson(), fromJson() functions
3. Fix class instantiation issue
4. Create missing directories
5. Create and test sync script

## Success Criteria

1. All tests pass: `go test ./...`
2. All task.md test cases work correctly
3. Code synced to target directory
4. No compilation errors
5. All builtin functions work as specified