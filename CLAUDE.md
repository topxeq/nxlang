# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview
Nxlang (Nx Language/能效语言) is a Go-based scripting language with Go-like syntax, bytecode virtual machine, and cross-platform support. It features:
- Weak typing with automatic type conversion
- Bytecode compilation and execution (`.nx` source files, `.nxb` bytecode files)
- REPL mode and built-in syntax-highlighted editor
- Rich standard library with support for concurrency, HTTP, file I/O, data processing, graphics, and more
- UTF-8 encoding for all strings and files

## Repository Structure
```
/
├── cmd/          # CLI entry points (nx main program)
├── internal/     # Internal packages not exposed to users
│   ├── parser/   # Source code parser and AST builder
│   ├── compiler/ # Bytecode compiler
│   ├── vm/       # Virtual machine for bytecode execution
│   └── core/     # Core type system and object implementations
├── pkg/          # Public Go API for embedding Nxlang in other applications
├── stdlib/       # Standard library implementations
├── examples/     # Example Nxlang scripts
├── tests/        # Test cases and test suites
└── docs/         # Documentation
```

## Common Development Commands
```bash
# Build the nx binary
go build -o nx ./cmd/nx

# Run all tests
go test ./...

# Run a specific test package
go test ./internal/vm -v

# Run a single test case
go test ./internal/vm -run TestArithmeticOperations -v

# Lint the codebase (requires golangci-lint)
golangci-lint run

# Run Nxlang REPL
./nx

# Execute an Nxlang script
./nx path/to/script.nx

# Compile Nxlang script to bytecode
./nx compile path/to/script.nx -o path/to/output.nxb

# Run precompiled bytecode
./nx run path/to/output.nxb
```

## Core Architecture
Nxlang follows a standard compiler-VM architecture:
1. **Parser**: Converts `.nx` source code into Abstract Syntax Tree (AST)
2. **Compiler**: Transforms AST into platform-agnostic bytecode
3. **Virtual Machine**: Executes bytecode with stack-based evaluation
4. **Type System**: All values implement a common `Object` interface for uniform handling
5. **Standard Library**: Built-in functions and types accessible from Nxlang code

## Important Conventions
- **Type Codes**: Use fixed constant values for type codes (not Go `iota`) to ensure cross-version compatibility
- **Error Handling**: Return error objects or strings prefixed with `TXERROR:` for runtime errors
- **Cross-Platform**: Ensure all code works on both Windows and Linux (avoid platform-specific syscalls)
- **Documentation**: All public APIs and core functionality must have English comments
- **Bytecode Compatibility**: Maintain backward compatibility for bytecode format across versions

## Testing Requirements
The test suite must cover all functionality specified in `task.md`, including:
- Basic arithmetic and type conversion
- Control flow statements (if, for, switch)
- Function definitions and calls (including recursion, default parameters, variadic functions)
- Data structures (arrays, maps, custom objects)
- Concurrency features (threads, mutexes)
- Standard library functions
- Error messages with line numbers and code context

## Target Repository Synchronization
All source code must be synchronized to `D:/goprjs/src/github.com/topxeq/nxlang` for GitHub release preparation, including:
- `go.mod` and `go.sum` files
- MIT LICENSE file
- README.md with project description and usage examples
- Complete source code and documentation
