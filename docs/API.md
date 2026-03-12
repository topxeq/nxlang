# Nxlang API Reference

## Built-in Functions

### Output Functions
| Function | Description | Example |
|----------|-------------|---------|
| `pln(a, ...)` | Print with newline | `pln("Hello")` |
| `pr(a, ...)` | Print without newline | `pr("Hello")` |
| `printf(fmt, ...)` | Print with format | `printf("Hello %s", "World")` |

### Type Functions
| Function | Description | Example |
|----------|-------------|---------|
| `typeOf(x)` | Get type name | `typeOf(123)` → "int" |
| `typeCode(x)` | Get type code | `typeCode(123)` → 5 |
| `typeInfo(x)` | Get detailed type info | `typeInfo(x)` |

### Type Conversion
| Function | Description | Example |
|----------|-------------|---------|
| `int(x)` | Convert to int | `int("123")` |
| `float(x)` | Convert to float | `float("3.14")` |
| `bool(x)` | Convert to bool | `bool(1)` |
| `string(x)` | Convert to string | `string(123)` |
| `byte(x)` | Convert to byte | `byte("A")` |
| `char(x)` | Convert to char | `char(65)` |
| `toJson(x)` | Convert to JSON | `toJson(obj)` |
| `fromJson(s)` | Parse JSON | `fromJson("{}")` |

### Collection Functions
| Function | Description | Example |
|----------|-------------|---------|
| `len(x)` | Get length | `len([1,2,3])` |
| `append(arr, ...)` | Append elements | `append(arr, 4, 5)` |
| `keys(m)` | Get map keys | `keys({"a":1})` |
| `values(m)` | Get map values | `values({"a":1})` |
| `delete(m, k)` | Delete key | `delete(m, "key")` |
| `array(...)` | Create array | `array(1, 2, 3)` |
| `map(...)` | Create map | `map("a", 1, "b", 2)` |
| `range(n)` | Create range | `range(5)` → [0,1,2,3,4] |
| `repeat(s, n)` | Repeat string n times | `repeat("ab", 3)` → "ababab" |

### Array Functions (v1.1.1+)
| Function | Description | Example |
|----------|-------------|---------|
| `includes(arr, val)` | Check if array contains value | `includes([1,2,3], 2)` |
| `find(arr, val)` | Find index of value | `find([1,2,3], 2)` → 1 |
| `slice(arr, start, end)` | Slice array | `slice([1,2,3,4], 1, 3)` → [2,3] |
| `concat(arr1, arr2)` | Concatenate arrays | `concat([1], [2,3])` → [1,2,3] |
| `reverse(arr)` | Reverse array | `reverse([1,2,3])` → [3,2,1] |
| `zip(arr1, arr2)` | Zip two arrays | `zip([1,2], ["a","b"])` → [[1,"a"], [2,"b"]] |
| `zipToMap(keys, values)` | Zip to map | `zipToMap(["a","b"], [1,2])` → {"a":1,"b":2} |
| `flatten(arr)` | Flatten nested array | `flatten([[1,2],[3,4]])` → [1,2,3,4] |
| `unique(arr)` | Unique elements | `unique([1,2,2,3])` → [1,2,3] |
| `rangeOf(start, end, step)` | Range with step | `rangeOf(0, 6, 2)` → [0,2,4] |
| `chunk(arr, size)` | Chunk array | `chunk([1,2,3,4,5], 2)` → [[1,2],[3,4],[5]] |
| `groupBy(arr, keyFn)` | Group by key | `groupBy(arr, fn(x){x.type})` |
| `count(arr, val)` | Count occurrences | `count([1,2,2,3], 2)` → 2 |
| `any(arr)` | Any truthy | `any([false, true, false])` → true |
| `all(arr)` | All truthy | `all([true, true, true])` → true |

### Performance Functions (v1.1.0+)
| Function | Description | Example |
|----------|-------------|---------|
| `fastSum(n)` | O(1) sum 0 to n-1 | `fastSum(1000000)` → 499999500000 |
| `fastRangeSum(start, end)` | O(1) sum of range | `fastRangeSum(0, 100)` → 4950 |
| `fastEach(arr, fn)` | Fast array iteration | `fastEach(arr, fn)` |
| `fastMap(arr, fn)` | Fast array map | `fastMap(arr, fn)` |
| `fastFilter(arr, fn)` | Fast array filter | `fastFilter(arr, fn)` |
| `fastReduce(arr, fn, init)` | Fast reduce | `fastReduce(arr, fn, 0)` |
| `sum(arr)` | Sum of array elements | `sum([1,2,3])` → 6 |
| `avg(arr)` | Average of array | `avg([1,2,3])` → 2 |

### Math Functions
| Function | Description | Example |
|----------|-------------|---------|
| `abs(x)` | Absolute value | `abs(-5)` |
| `min(a, b)` | Minimum of two | `min(3, 7)` → 3 |
| `max(a, b)` | Maximum of two | `max(3, 7)` → 7 |
| `clamp(val, min, max)` | Clamp value | `clamp(5, 0, 10)` → 5 |

### String Functions
| Function | Description | Example |
|----------|-------------|---------|
| `toUpper(s)` | Uppercase | `toUpper("hello")` |
| `toLower(s)` | Lowercase | `toLower("HELLO")` |
| `trim(s)` | Trim whitespace | `trim("  hi  ")` |
| `split(s, sep)` | Split string | `split("a,b", ",")` |
| `join(arr, sep)` | Join array | `join(["a","b"], ",")` |
| `contains(s, sub)` | Check substring | `contains("hello", "ell")` |
| `replace(s, old, new)` | Replace substring | `replace("hi", "i", "ey")` |
| `substr(s, start, len)` | Substring | `substr("hello", 0, 3)` |
| `urlEncode(s)` | URL encode | `urlEncode("hello world")` → "hello+world" |
| `urlDecode(s)` | URL decode | `urlDecode("hello%20world")` → "hello world" |
| `parseJson(s)` | Parse JSON | `parseJson("{\"a\":1}")` → {"a":1} |

### Math Functions
| Function | Description | Example |
|----------|-------------|---------|
| `abs(x)` | Absolute value | `abs(-5)` |
| `sqrt(x)` | Square root | `sqrt(16)` |
| `sin(x)` | Sine | `sin(0)` |
| `cos(x)` | Cosine | `cos(0)` |
| `tan(x)` | Tangent | `tan(0)` |
| `floor(x)` | Floor | `floor(3.7)` |
| `ceil(x)` | Ceiling | `ceil(3.2)` |
| `round(x)` | Round | `round(3.5)` |
| `pow(x, y)` | Power | `pow(2, 3)` |
| `random()` | Random float | `random()` |

### Time Functions
| Function | Description | Example |
|----------|-------------|---------|
| `now()` | Current timestamp | `now()` |
| `unix()` | Unix timestamp | `unix()` |
| `unixMilli()` | Millisecond timestamp | `unixMilli()` |
| `unixNano()` | Nanosecond timestamp | `unixNano()` |
| `formatTime(ts, fmt)` | Format time | `formatTime(now(), "2006-01-02")` |
| `parseTime(s, fmt)` | Parse time string | `parseTime("2024-01-01")` |
| `year(ts)` | Get year | `year(now())` |
| `month(ts)` | Get month | `month(now())` |
| `day(ts)` | Get day | `day(now())` |
| `hour(ts)` | Get hour | `hour(now())` |
| `minute(ts)` | Get minute | `minute(now())` |
| `second(ts)` | Get second | `second(now())` |
| `weekday(ts)` | Get weekday | `weekday(now())` |
| `addDate(ts, y, m)` | Add years/months | `addDate(now(), 1, 0)` |
| `dateDiff(ts1, ts2)` | Days between | `dateDiff(t1, t2)` |

### File Functions
| Function | Description | Example |
|----------|-------------|---------|
| `readFile(path)` | Read file | `readFile("test.txt")` |
| `writeFile(path, content)` | Write file | `writeFile("test.txt", "hi")` |

### Thread/Concurrency
| Function | Description | Example |
|----------|-------------|---------|
| `thread(fn, ...)` | Start thread | `thread(myFunc, arg)` |
| `waitForThreads()` | Wait for threads | `waitForThreads()` |
| `mutex()` | Create mutex | `mutex()` |
| `rwMutex()` | Create read-write mutex | `rwMutex()` |
| `sleep(seconds)` | Sleep | `sleep(0.1)` |

### Debug Functions
| Function | Description | Example |
|----------|-------------|---------|
| `debug(a, ...)` | Debug print | `debug(x, y)` |
| `trace(x)` | Print call stack | `trace(x)` |
| `vars()` | Show variables | `vars()` |
| `breakpoint(x)` | Breakpoint | `breakpoint(x)` |

### Performance Functions
| Function | Description | Example |
|----------|-------------|---------|
| `profilerStart()` | Start profiler | `profilerStart()` |
| `profilerStop()` | Stop profiler | `profilerStop()` |
| `instructionCount()` | Get instruction count | `instructionCount()` |
| `gc()` | Run GC | `gc()` |
| `memoryUsage()` | Memory stats | `memoryUsage()` |
| `version()` | Get version | `version()` |

### System Functions
| Function | Description | Example |
|----------|-------------|---------|
| `exit(code)` | Exit program | `exit(0)` |

## Language Features

### Variables
```nx
let x = 10          // immutable
var y = 20           // mutable
z := x + y          // short declaration
const PI = 3.14     // constant
```

### Functions
```nx
func add(a, b = 0) {
    return a + b
}

// Variadic
func sum(a, b...) {
    result := a
    for _, v in b {
        result = result + v
    }
    return result
}
```

### Classes
```nx
class Person {
    func init(name, age) {
        this.name = name
        this.age = age
    }
    
    func greet() {
        return "Hello, " + this.name
    }
}

let p = Person("Tom", 16)
pln(p.greet())
```

### Control Flow
```nx
// If
if x > 10 {
    pln("big")
} else {
    pln("small")
}

// For
for i in 10 {
    pln(i)
}

// Switch
switch x {
case 1:
    pln("one")
default:
    pln("other")
}
```

### Exception Handling
```nx
try {
    let result = 10 / 0
} catch (e) {
    pln("Error:", e)
} finally {
    pln("Cleanup")
}

// Defer
defer {
    pln("Cleanup")
}
```

### Threading
```nx
func worker(n) {
    pln("Thread", n, "started")
    sleep(0.1)
}

thread(worker, 1)
thread(worker, 2)
waitForThreads()
```
