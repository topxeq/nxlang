# Nxlang (Efficiency Language)

A lightweight, Go-based scripting language with Go-like syntax, bytecode virtual machine, and cross-platform support.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/topxeq/nxlang)](https://goreportcard.com/report/github.com/topxeq/nxlang)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.21-blue.svg)](https://golang.org/dl/)

[中文版本](./README_zh-CN.md)

## ✨ Core Features

- **Familiar Syntax**: Go-like syntax design, low learning curve, seamless adaptation for Go developers
- **Weak Type System**: Automatic type conversion, flexible and efficient coding, reduces redundant type declarations
- **Bytecode Execution**: Compiles to platform-agnostic bytecode (.nxb), runs faster than traditional interpreted languages
- **Built-in REPL**: Interactive command line with syntax highlighting for quick debugging and prototyping
- **Integrated Editor**: Built-in code editor, write and run scripts without additional tools
- **Rich Standard Library**: Built-in support for concurrency, HTTP, file I/O, data processing, graphics, and more
- **Cross-Platform Support**: Runs perfectly on Windows, Linux, macOS with no external dependencies
- **Module System**: Supports module import/export with consistent function references, suitable for large project development
- **Native UTF-8**: Full stack UTF-8 support, strings and files use UTF-8 encoding by default
- **High Performance**: Built on Go, runtime performance far exceeds dynamic languages like Python/JavaScript

## 🚀 Quick Start

### Installation
Download prebuilt binaries from [Releases](https://github.com/topxeq/nxlang/releases) or build from source.

### Run REPL (Interactive Mode)
```bash
nx
```

### Execute a Script
```bash
nx path/to/script.nx
```

### Compile to Bytecode
```bash
nx compile path/to/script.nx -o output.nxb
```

### Run Precompiled Bytecode
```bash
nx run output.nxb
```

## 📝 Example Code
```nx
// Hello World
pln("Hello, Nxlang! 👋")

// Function definition
func factorial(n) {
    if n <= 1 { return 1 }
    return n * factorial(n - 1)
}

pln("Factorial of 10:", factorial(10))

// Module import
import { sqrt, random } from "math"
pln("sqrt(25) =", sqrt(25))
pln("Random number:", random())

// Built-in data structures
var fruits = array("Apple", "Banana", "Cherry")
var person = map("name", "Bob", "age", 28, "city", "Shanghai")

// Static methods on types
let num = int.parse("123")
let pi = float.parse("3.14159")

// Self-growing sequence
var seq1 = seq(1, 2, 3)
seq1[5] = 100  // Auto-grows sequence

// Object reference
var x = 100
var r = ref(x)
setref(r, 200)

// Control flow
for (var i = 0; i < 5; i++) {
    pln("Count:", i)
}
```

## 📦 Standard Library

## 📦 Standard Library

### Math Functions
`abs`, `sqrt`, `sin`, `cos`, `tan`, `floor`, `ceil`, `round`, `pow`, `random`

### String Functions
`len`, `toUpper`, `toLower`, `trim`, `split`, `join`, `contains`, `replace`, `substr`, `startsWith`, `endsWith`

### Collection Functions
`array`, `append`, `map`, `orderedMap`, `stack`, `queue`, `seq`, `keys`, `values`, `delete`, `sortMap`, `reverseMap`

### Type Conversion Functions
`int`, `float`, `bool`, `string`, `byte`, `uint`, `char`, `bytes`, `chars`
- Static methods: `int.parse()`, `float.parse()`, `string.parse()`, `bool.parse()`

### Object Reference Functions
`ref()`, `deref()`, `setref()` - Object reference and dereference support

### Seq Type (Self-growing Sequence)
`seq()` - Create self-growing sequence with auto-expanding index access
- Methods: `Append()`, `Pop()`, `Clear()`, `Resize()`, `Fill()`, `Range()`, `Reverse()`, `Join()`, `Includes()`, `IndexOf()`, `Get()`, `Set()`

### Time Functions
`now()`, `unix()`, `unixMilli()`, `formatTime(timestamp, format)`, `parseTime(timeStr, format)`, `sleep(seconds)`

### JSON Functions
`toJson(value, indent=false)` - Convert value to JSON string

### Concurrency Functions
`thread(func)` - Spawn a new thread running the given function
`mutex()` - Create a mutual exclusion lock
`rwMutex()` - Create a read-write mutex
`waitForThreads` - Wait for all threads to complete

### I/O Functions
`pln(...)` - Print values with newline
`pr(...)` - Print values without newline
`printf(format, ...)` - Print formatted string
`readFile(filename)`, `writeFile(filename, content)` - File I/O

### HTTP Functions
`httpGet(url)`, `httpPost(url, body)`, `httpPostJSON(url, data)`, `httpPut(url, body)`, `httpDelete(url)`, `httpRequest(method, url, body, headers)`
- Returns httpResponse object with fields: `statusCode`, `body`, `headers`

### Graphics/Canvas Functions
`canvas(width, height)`, `clear(canvas, r, g, b, a)`, `drawPoint(canvas, x, y, r, g, b, a)`
`drawLine(canvas, x0, y0, x1, y1, r, g, b, a)`, `drawCircle(canvas, cx, cy, radius, r, g, b, a)`
`fillCircle(canvas, cx, cy, radius, r, g, b, a)`, `drawRectangle(canvas, x, y, w, h, r, g, b, a)`
`fillRectangle(canvas, x, y, w, h, r, g, b, a)`, `getPixel(canvas, x, y)`
`savePNG(canvas, filename)`, `loadPNG(filename)`

### Data/CSV Functions
`readCSV(filename)`, `writeCSV(filename, data, headers)`, `parseCSV(csvString)`, `toCSV(data)`
`openCSV(filename, hasHeader)`, `readCSVRow(reader)`, `readCSVAll(reader)`, `closeCSV(reader)`
`createCSVWriter(filename)`, `writeCSVRow(writer, row)`, `closeCSV(writer)`

### Plugin System
`loadPlugin(path)`, `unloadPlugin(name)`, `callPlugin(pluginName, funcName, args...)`, `listPlugins()`
- Load Go-based plugins and call plugin functions from Nxlang code

## 🏗️ Build from Source
```bash
# Clone repository
git clone https://github.com/topxeq/nxlang.git
cd nxlang

# Build binary
go build -o nx ./cmd/nx

# Install to system (Linux/macOS)
sudo mv nx /usr/local/bin/
```

## 🧪 Running Tests
```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./vm -v
```

## 🏛️ Architecture

Nxlang follows a standard compiler-VM architecture:
1. **Lexer**: Converts source code to token stream
2. **Parser**: Builds Abstract Syntax Tree (AST) from tokens
3. **Compiler**: Transforms AST into platform-agnostic bytecode (.nxb)
4. **Virtual Machine**: Executes bytecode with stack-based evaluation
5. **Standard Library**: Built-in functions and types implemented in Go
6. **Plugin System**: Go-based plugins for extensibility

### Project Structure
```
nxlang/
├── cmd/nx/          # CLI entry point
├── compiler/        # Bytecode compiler
├── parser/          # Source code parser
├── vm/              # Virtual machine
├── types/           # Type system (Object interface)
├── plugin/          # Plugin system
├── graphics/        # Graphics/canvas support
├── data/            # Data processing (CSV, etc.)
├── tests/           # Test suites
│   ├── builtin/     # Built-in function tests
│   ├── features/    # Feature tests (OOP, refs, etc.)
│   ├── stdlib/      # Standard library tests
│   └── integration/ # Integration tests
└── examples/        # Example scripts
```

## 🔗 Links

- [GitHub Repository](https://github.com/topxeq/nxlang)
- [Issue Tracker](https://github.com/topxeq/nxlang/issues)
- [Documentation](docs/)
- [Examples](examples/)

## 📄 License
MIT License - see [LICENSE](LICENSE) for details.

---

Made with ❤️ by TopXeQ
