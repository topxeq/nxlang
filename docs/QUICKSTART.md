# Nxlang Quick Start Guide

## Installation

```bash
# Build from source
go build -o nx ./cmd/nx

# Or download from releases
```

## Quick Examples

### Hello World
```nx
pln("Hello, World!")
```

### Variables
```nx
let name = "Nxlang"
let version = 1.0
pln("Welcome to", name, "v", version)
```

### Arithmetic
```nx
pln("1 + 2 =", 1 + 2)
pln("10 - 3 =", 10 - 3)
pln("4 * 5 =", 4 * 5)
pln("20 / 4 =", 20 / 4)
pln("17 % 5 =", 17 % 5)
```

### Arrays
```nx
let arr = [1, 2, 3, 4, 5]
pln("Array:", arr)
pln("First element:", arr[0])
pln("Length:", len(arr))

// Append
append(arr, 6, 7)
pln("After append:", arr)

// Range
let nums = range(1, 10, 2)
pln("Range:", nums)  // [1, 3, 5, 7, 9]
```

### Maps
```nx
let person = {"name": "Tom", "age": 16}
pln("Person:", person)
pln("Name:", person["name"])

// Modify
person["age"] = 17
person["city"] = "Beijing"
pln("Updated:", person)
```

### Functions
```nx
// Simple function
func greet(name) {
    return "Hello, " + name
}
pln(greet("World"))

// Default parameters
func add(a, b = 0) {
    return a + b
}
pln(add(5))    // 5
pln(add(5, 3)) // 8

// Variadic
func sum(nums...) {
    total := 0
    for _, n in nums {
        total = total + n
    }
    return total
}
pln(sum(1, 2, 3, 4, 5))  // 15
```

### Classes
```nx
class Calculator {
    func init() {
        this.result = 0
    }
    
    func add(n) {
        this.result = this.result + n
        return this
    }
    
    func subtract(n) {
        this.result = this.result - n
        return this
    }
    
    func getResult() {
        return this.result
    }
}

let calc = Calculator()
calc.add(10).subtract(3)
pln("Result:", calc.getResult())  // 7
```

### Control Flow
```nx
// If statement
let score = 85
if score >= 90 {
    pln("A")
} else if score >= 80 {
    pln("B")
} else {
    pln("C")
}

// For loop
for i in 5 {
    pln("Count:", i)
}

// Switch
let day = 3
switch day {
case 1: pln("Monday")
case 2: pln("Tuesday")
case 3: pln("Wednesday")
default: pln("Other day")
}
```

### Error Handling
```nx
try {
    let result = 10 / 0
} catch (e) {
    pln("Caught error:", e)
} finally {
    pln("Done")
}
```

### Threads
```nx
func worker(id) {
    pln("Worker", id, "started")
    sleep(0.1)
    pln("Worker", id, "done")
}

thread(worker, 1)
thread(worker, 2)
waitForThreads()
pln("All workers finished")
```

### JSON
```nx
// Encode
let data = {"name": "Tom", "age": 16}
let jsonStr = toJson(data)
pln(jsonStr)

// Decode
let parsed = fromJson(jsonStr)
pln("Name:", parsed["name"])
```

### HTTP
```nx
// GET request
let resp = httpGet("https://httpbin.org/get")
pln(resp)

// POST request
let resp = httpPost("https://httpbin.org/post", "Hello")
pln(resp)
```

### File Operations
```nx
// Write file
writeFile("hello.txt", "Hello, World!")

// Read file
let content = readFile("hello.txt")
pln(content)
```

### Debugging
```nx
let x = 10
let y = 20

// Debug print
debug(x, y)

// Show call stack
trace(x)

// Show variables
vars()

// Type info
typeInfo(x)
```

### Performance
```nx
// Start profiler
profilerStart()

// ... your code ...

// Stop and get stats
let stats = profilerStop()
pln("Instructions:", stats["instructions"])

// Memory usage
pln("Memory:", memoryUsage())
```

## Running Scripts

```bash
# Run source file
nx script.nx

# Compile to bytecode
nx compile script.nx -o script.nxb

# Run bytecode
nx run script.nxb
nx script.nxb
```

## REPL Mode

```bash
# Start interactive mode
nx

# Type commands directly
> pln("Hello")
Hello
> 1 + 2
3
> exit
Goodbye!
```
