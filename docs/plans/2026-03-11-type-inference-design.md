# Type Inference Design

## Goal
Add compile-time type inference for local variables to enable fast integer opcodes in more cases.

## Current State
- Fast opcodes (OpAddInt, etc.) only work when both operands are integer literals
- Symbol already has unused `Type types.Object` field

## Design

### Approach
Track variable types during compilation. When a variable is assigned, record its type. When the variable is used in operations, check its inferred type.

### Implementation

1. **Type tracking in SymbolTable**
   - Use existing `Symbol.Type` field to store inferred type
   - Update type when variable is assigned

2. **Inference rules**
   - Integer literal → `types.Int`
   - Float literal → `types.Float`
   - String literal → `types.String`
   - `a + b` where both have same type → same type
   - `a + b` where types differ → `nil` (fallback to generic opcode)

3. **Compiler changes**
   - Add helper to get inferred type of symbol
   - In infix expression: check if both operands have integer type
   - Emit fast opcode when types match integers

### Example
```nx
let x = 1       // x: Int
let y = 2       // y: Int  
x + y           // Uses OpAddInt (fast)
```

### Backward Compatibility
- Unknown types fall back to generic opcodes
- Existing bytecode unchanged

### Testing
- Unit tests for type inference
- Benchmark comparison
