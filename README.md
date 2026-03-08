# Nxlang (能效语言)

Nxlang is a Go-based scripting language with Go-like syntax, bytecode virtual machine, and cross-platform support.

## Features

- **Go-like syntax** with simplified constraints and weak typing
- **Bytecode compilation**: Compile `.nx` source files to portable `.nxb` bytecode
- **Fast execution**: Stack-based virtual machine optimized for performance
- **Cross-platform**: Runs on Windows and Linux with zero dependencies
- **Rich standard library** with support for:
  - Strings, collections (arrays, maps, stacks, queues)
  - Math, time, JSON serialization
  - Concurrency with threads and synchronization primitives (mutex, rwmutex)
- **REPL mode** for interactive development
- **Built-in syntax-highlighted editor** (coming soon)
- **UTF-8 support** for all strings and source files

## Installation

```bash
# Build from source
git clone https://github.com/topxeq/nxlang.git
cd nxlang
go build -o nx ./cmd/nx

# Add to PATH (optional)
mv nx /usr/local/bin/
```

## Quick Start

### Run a script
```bash
nx run examples/hello.nx
```

### Start REPL
```bash
nx repl
> pln("Hello Nxlang!")
Hello Nxlang!
> x = 10 + 20
> pln(x)
30
```

### Compile to bytecode
```bash
nx compile myscript.nx -o myscript.nxb
nx run myscript.nxb
```

## Example Usage

```nx
// Variables
var name = "Nxlang"
var version = 1.0
var isGreat = true

// Functions
func add(a, b) {
    return a + b
}

pln("10 + 20 = ", add(10, 20))

// Arrays
var arr = array(1, 2, 3, 4, 5)
append(arr, 6)
pln("Array: ", arr, " Length: ", len(arr))

// Maps
var user = map(
    "name", "Alice",
    "age", 30,
    "email", "alice@example.com"
)
pln("User: ", user)
pln("Keys: ", keys(user))

// Control flow
for var i = 0; i < 5; i++ {
    pln("Count: ", i)
}

// Standard library
pln("Square root of 25: ", sqrt(25))
pln("Uppercase: ", toUpper("hello world"))
pln("Current time: ", formatTime())
```

## Standard Library

### Math Functions
`abs`, `sqrt`, `sin`, `cos`, `tan`, `floor`, `ceil`, `round`, `pow`, `random`

### String Functions
`len`, `toUpper`, `toLower`, `trim`, `split`, `join`, `contains`, `replace`, `substr`

### Collection Functions
`array`, `append`, `map`, `orderedMap`, `stack`, `queue`, `keys`, `values`

### Time Functions
`now`, `unix`, `unixMilli`, `formatTime`, `sleep`

### JSON Functions
`toJson(value, indent=false)` - Convert value to JSON string

### Concurrency Functions
`thread(func)` - Spawn a new thread running the given function
`mutex()` - Create a mutual exclusion lock
`rwMutex()` - Create a read-write mutex

### I/O Functions
`pln(...)` - Print values with newline
`pr(...)` - Print values without newline
`printf(format, ...)` - Print formatted string

## Architecture

Nxlang follows a standard compiler-VM architecture:
1. **Lexer**: Converts source code to token stream
2. **Parser**: Builds Abstract Syntax Tree (AST) from tokens
3. **Compiler**: Transforms AST into platform-agnostic bytecode
4. **Virtual Machine**: Executes bytecode with stack-based evaluation
5. **Standard Library**: Built-in functions and types implemented in Go

## License

MIT License - see LICENSE file for details
