# 任务整体简介

用Go语言编写一个名为Nx语言的脚本语言，脚本语言主程序名称即为nx，脚本语言的正式英文名称为Nxlang，中文名称为Nx语言或能效语言。

# 说明与要求

- 本任务的工作目录是`/mnt1/aiprjs/nx`，所有中间文件、临时文件、结果文件都在这个目录下创建，可以创建子目录；唯一的例外是，生成的Nxlang的代码要同步到 `/root/goprjs/src/github.com/topxeq/nxlang` 目录下，做好上传github的仓库的准备；
- Nxlang准备发布到github上，仓库地址为：https://github.com/topxeq/nxlang，要为上传代码到该仓库做各种准备，例如编写README.md文件、添加LICENSE文件、添加go.mod文件、添加go.sum文件等。所有文档和代码内注释均用英文编写；
- Nxlang编写代码时要添加足够的注释，并符合Go语言一般的注释规范，包括函数说明、变量说明、类型说明、结构体说明、枚举说明等。
- Nxlang要求能够跨平台运行，目前先支持Windows和Linux平台。
- Nxlang的代码运行时实际上是先编译为字节码然后由虚拟机运行，字节码编译后在各个平台上是一致的、不变的，在不同平台上的虚拟机中可以直接运行，并可以接受输入零至多个参数，返回结果都是字符串。除非发生大的结构变化，一般情况下，更早版本的字节码在新的Nxlang版本也可以正常执行。
- 注意，Nxlang中提到的字节码不是指可执行程序，而是Nxlang私有格式的，但在可平台都能一样执行并且效果一致的二进制数据；
- Nxlang的脚本（代码）文件一般以`.nx`为扩展名，Nxlang的字节码文件一般以`.nxb`为扩展名。
- Nxlang内部的字符串和文件等编码均统一使用UTF-8编码。
- Nxlang的主程序要求支持启动一个命令行界面的代码编辑器，支持语法高亮，高亮方案可选用于Go语言相同；
- Nxlang的主程序可直接运行指定的Nxlang代码文件；
- Nxlang主程序支持启动REPL，命令行如果没有指定明确的执行代码目标，则启动REPL
- Nxlang是弱类型语言，声明变量时无需指定类型；
- Nxlang的整体语法接近于Go语言，但取消了一些限制；
- Nxlang中变量的声明与赋值采用类似Go语言的写法，支持下述写法：`var a`然后`a = 1`，或者直接写`var a = 1.2`，也支持`let a = false`的写法，也支持`a := "abc"`的写法。var和let关键字在Nxlang中等效；
- Nxlang内部通过支持Object接口，数据类型都实现这个接口，这样实现让脚本代码中的所有数据类型的统一；
- Nxlang目前先支持undefined、bool、byte、char（基于Go语言中的rune类型）、int、uint、bigInt、float、bigFloat、string、time、array（基于Go语言中的slice，是动态数组）、map, bytes, chars（基于Go语言中的[]rune）、stringBuilder、bytesBuffer、func、error、byteCode、mutex（互斥锁）、rwMutex（读写锁）、reader、writer、file这些对象；
- Nxlang还需要支持image（图像）、orderedMap（保证顺序的map）、stack（堆栈）、queue（队列）、seq（自增长序列）等对象；
- Nxlang还需要支持httpReq（HTTP请求）对象，用于发送HTTP请求；还需要支持httpResp（HTTP响应）对象，用于处理HTTP响应；还需要支持mux（多路复用）对象，用于处理多个HTTP请求和响应；例如：读取HTTP请求、处理HTTP请求、返回HTTP响应等；mux对象支持添加、删除、查询、处理等方法；
- Nxlang还需要支持csv、excel对象，用于处理csv、excel文件；例如：读取csv文件、写入excel文件、处理excel文件等；
- Nxlang还需要支持canvas（画布）对象，用于绘制图像、处理图像等；例如：绘制图像、处理图像等；
- Nxlang还需要支持font（字体）对象，用于处理字体、处理字体等；例如：从文件加载字体、获取字体信息、处理字体等；
- Nxlang还需要支持对象引用和解引用，通过对象引用，可以实现对传递给函数的参数进行修改等操作；由于Nxlang中，所有数据类型也都是对象，因此，通过对象引用，可以实现对所有数据类型进行修改等操作；用Go语言实现对象引用和解引用时，很容易引发循环导入的问题，要一开始就规划避免这种情况；
- Nxlang还需要支持any类型，用于表示任意类型，用于存储任何数据类型；
- Nxlang中所有对象都支持一些通用的方法，如toStr、typeCode、typeName等，分别用于将对象转换为字符串、获取对象的类型、类型名称等。这是通过Go语言实现中实现Object接口，数据类型都支持这些方法来实现的。如果某些通用方法某个对象不支持，则该方法将返回error对象并通过error对象的说明中说明错误信息。
- Nxlang的对象除了支持通用方法，一般还有自己特殊的方法函数和成员变量，例如：int支持floor、ceil、round等方法；mux对象支持setHandleFunc等方法，csv、excel对象支持read、write、close等方法；
- Nxlang的所有对象都支持通过同名的内置函数来新建对象，并可以传递必要的参数来赋以初值；例如：`let a = byte(18)`、`b := bool(true)`、`var time1 = time("2026-01-02 15:04:05")`等；
- Nxlang支持对象类的静态方法和静态成员，例如：`int.parse("123")`等；静态方法都是以类型或对象名称为前缀的；
- Nxlang允许用户编写自定义函数；
- Nxlang允许为每个对象（严谨的说是对象类）增加方法函数和成员变量；对象方法函数中，this关键字表示当前方法所属的对象实例。
- Nxlang允许自定义对象类，用户可以为每个对象类添加方法函数和成员变量，支持初始化方法（类似别的语言构造函数，将在对象创建时调用，用于初始化对象的成员变量等操作）。定义对象的语法类似：
```
class MyClass {
    func method1() {
        pln("method1")
    }
    member1 := 100
}
```
- Go语言的对象系统暂不考虑支持继承、多继承、多态等对象系统特征。

- Nxlang支持函数的定义与调用，函数的定义与Go语言中的写法类似，支持参数、返回值、异常处理等，但不能指定返回值的类型，也不支持多返回值（多返回值可以用返回数组的方法来支持），只能返回一个值，如果没有返回值则默认返回undefined。函数的调用与Go语言中的写法类似，支持参数、异常处理等。
- Nxlang支持`func func1(param1, param2...) {...}`的写法定义函数，也支持变量式的函数定义，例如：
```
let func1 = func(param1, param2...) {
    return param1 + param2
}
```
- Nxlang支持函数递归调用，例如斐波那契数列的计算如下：
```
func fib(n) int {
    if n <= 1 {
        return n
    }
    return fib(n-1) + fib(n-2)
}
```

- Nxlang支持函数的参数默认值，例如：
```
func func3(param1 = 0, param2 = 0) {
    return param1 + param2
}
```

- Nxlang支持函数参数传递可变个数的参数，例如：
```
func func4(param1, param2...) {
    var sum = param1
    for _, param in param2 {
        sum += param
    }

    return sum
}
```

- 向包含可变个数参数的函数传递参数时，写法类似：
```
func func5(param1, param2...) {
    var sum = param1
    for _, param in param2 {
        sum += param
    }

    return sum
}

array1 := [1, 2, 3, 4, 5]
func5(1, array1...)

```

- Nxlang支持for循环，与Go语言中的写法类似，支持for、foreach、for...in等循环语法。支持break、continue等控制流语句。
- Nxlang支持自增/自减 ++, -- 操作符，与Go语言中的写法类似，支持在循环中使用，也可以在普通语句中使用。
- for 循环除支持 for (init; condition; step) { } 格式，也支持对 for condition { } 格式的支持：
- for 也支持for in循环，例如：
```
for i, v in 10 {
    pln(i)
}
```

或
```
for i in 10 {
    pln(i)
}
```

或：
```
for i, v in [1, 2, 3, 4, 5] {
    pln(i)
}
```
或
```
map1 := {"a": 1, "b": "2", "c": false}
for k, v in map1 {
    pln(k, v)
}
```


- Nxlang支持switch语句，与Go语言中的写法类似，支持switch、case、default、break、fallthrough等控制流语句。
- Nxlang支持if、else、else if等控制流语句。
- Nxlang中实现所有Go语言中的运算符
- Nxlang中支持在运算中不同类型的变量或数值进行运算，原则是总是向左边的数字类型转换，例如：字符串加数字时，结果是字符串，数字加字符串时，如果字符串可以被转换为数字，则转换为数字计算，否则出错；
- Nxlang中支持基于try...catch...finally式的异常处理，也支持defer方式的异常处理；Nxlang中，原则上一般不出发异常，更多的是通过函数返回error对象“TXERROR:”开头的表示错误的字符串来表示异常。除非一些逻辑上无法处理的情况，才会抛出异常，可以用try...catch...finally来处理异常。
- Nxlang中通过thread内置函数支持并发（多线程）处理，使用方法类似： `thread(threadFunc1, param1, param2...)`，这将启动一个新线程，执行`threadFunc1`函数，参数为`param1, param2...`。
- Nxlang中的字符串支持Go语言中的几种写法，即用双引号括起来的写法，和用反引号扩起来的写法，反引号写法支持多行字符串，不支持单引号扩起来，单引号括起来的是char类型，表示一个Unicode字符。
- Nxlang支持插件机制，允许开发者用Go语言开发自己的插件，这些插件可以加载进Nxlang的虚拟机中，由Nxlang的代码调用；
- Nxlang支持在代码内部执行Nxlang的代码，两种方式，一种是再启动一个Nxlang的虚拟机，传入执行的参数，并获得执行的结果，另一种是直接在当前虚拟机中执行。
- Nxlang中出现内部错误和异常时，应该能够提示行号和相应代码相关的信息，便于用户定位和调试问题。错误信息应包含：错误类型、错误描述、发生错误的行号、以及错误行的代码内容。例如：`Error: division by zero at line 5: "x = 1 / 0"`
- Nxlang的代码可以接受外部传递进来的参数，参考下面的说明例子实现：

```
sourceT := `
param vargs...

for _, v in vargs {
    pln(i, v)
}

return vargs[2]

`

byteCodeT := compile(sourceT)

rs := byteCodeT.run("abc", 123.5, true, {"name": "Tom", "age": 16})

pl("rs: %v", rs)

```

- Nxlang的代码同时支持固定参数和可变参数，例如：

```go
sourceT := `
param (v1, v2, vargs...)

pln("input:", v1, v2, vargs...)

sum := v1 + v2

for i, v in vargs {
	sum += v
}

return sum
`

result := runCode(sourceT, 1, 2, 3, 4, 5)

pl("result: %v", result)

```

- Nxlang中，一般的情况下，返回错误应该是error对象，某些特殊的情况下必须返回字符串时，才会返回`TXERROR:`开头的表示出错的字符串；
- Nxlang的变量、函数名、对象名等命名法建议采用驼峰法结合最后一位大写字母表示数据类型，也可以用下划线分隔单词的命名方法
- Nxlang提供一些预定义全局变量，均以大写的G结尾，如argsG
- Nxlang支持用const关键字定义常量，例如：`const a = 10`、`const b = "hello"`等；
- Nxlang提供一些预定义全局常量，均以大写的C结尾，如piC、eC等；
- Nxlang的代码支持注释，注释风格与Go语言完全一致；
- 编写中的文档与代码中的注释均要求使用英语书写；
-  Nxlang提供大量内置函数，内置函数先支持：prf（等效于Go语言中的fmt.Printf）、pln（等效于Go语言中的fmt.Println）、pl（类似Go语言中的fmt.Printf但最后加上一个换行符“\n”）


# Nxlang的主要内置函数

- prf（等效于Go语言中的fmt.Printf）：用于格式化输出，与Go语言中的fmt.Printf类似，支持格式化字符串和参数。
- pln（等效于Go语言中的fmt.Println）：用于输出，与Go语言中的fmt.Println类似，最后会自动添加一个换行符“\n”。
- pl（类似Go语言中的fmt.Printf但最后加上一个换行符“\n”）：与prf类似，只是不自动添加换行符。
- typeCode（）：用于获取对象的类型，返回一个整数表示类型码，这个整数表示对象的类型，基本固定（内部不使用iota实现，而是直接用常量定义，否则容易出现Nxlang不同版本中类型码不一致的问题）。
- typeName（）：用于获取对象的类型名称，返回一个字符串表示类型的名称。
- isErr：判断一个对象是否是Nxlang中的错误，返回一个bool值。如果对象是Nxlang中的error类型，或者对象是Nxlang中的string类型并且以`TXERROR:`开头，isErr函数都将返回true，否则返回false。
- sleep（）：用于暂停所处线程代码的执行，单位为秒，可以是浮点数，也支持整数。
```
sleep(1.2) // 表示暂停1.2秒，可以是浮点数，也支持整数
```

- toJson（）：用于将对象转换为JSON字符串。支持所有支持的类型。可以使用`-sort`参数对对象进行排序，以生成更可读的JSON字符串，也支持使用`-indent`参数添加缩进，以生成更可读的JSON字符串。例如:
```
toJson({"name": "Tom", "age": 16}, "-sort")
```
输出：`{"age": 16, "name": "Tom"}`
```
- runCode（）：用于在当前虚拟机中执行Nxlang的代码，支持传入参数和获取结果。例如：
```
runCode(`pl('Hello World: %v-%v-%v')`, "Tom", "Tom", "Tom")
```
输出：`Hello World`
```

- compile：编译一段Nxlang代码为字节码；Go语言实现compile函数的时候，容易产生循环引用（import）的问题，设计之处就要想办法避免；
- runByteCode：运行字节码，可以传入参数，返回执行结果
- addMethod：为对象类添加方法函数，参数为对象类名、方法名、方法函数。
- addMember：为对象类添加成员变量，参数为对象类名、成员名、成员值。

## 常用内置函数

### 数学函数
- abs：返回数值的绝对值。
- sqrt：返回数值的平方根。
- sin：返回角度的正弦值（弧度制）。
- cos：返回角度的余弦值（弧度制）。
- tan：返回角度的正切值（弧度制）。
- exp：返回e的指定次幂。
- log：返回自然对数。
- log10：返回常用对数（以10为底）。
- pow：返回基数的指定次幂。

### 字符串函数
- len：返回字符串、数组或对象的长度。
- string：将任意类型转换为字符串。
- toUpper：将字符串转换为大写。
- toLower：将字符串转换为小写。
- trim：去除字符串首尾空白字符。
- split：按指定分隔符分割字符串，返回数组。

### 类型转换函数
- int：将值转换为整数。
- float：将值转换为浮点数。
- bool：将值转换为布尔值。
- byte：将值转换为字节（0-255）。
- char：将值转换为字符（UTF-8编码）。

### 数组函数
- array：创建新数组。
- append：向数组追加元素，返回新数组。

### 映射函数
- map：创建新映射。

### 对象函数
- object：创建新对象。

### 时间函数
- now：返回当前时间。
- unix：返回Unix时间戳（10位）。
- unixMilli：返回Unix时间戳（13位）。
- formatTime：格式化时间为字符串。
- parseTime：解析字符串为时间。

### 字节和字符函数
- bytes：将字符串转换为字节数组。
- chars：将字符串转换为字符数组。
- stringFromBytes：将字节数组转换为字符串。
- stringFromChars：将字符数组转换为字符串。

### 其他函数
- typeOf：返回值的类型名称。
- error：创建错误对象。
- thread：启动新线程执行函数。

# 测试用Nxlang代码

要求测试并确保下述代码可以运行：

- Hello World

```
pln("Hello World")
```

- 四则运算

```
pln(1 + 2)
pln(1 - 2)
pln(1 * 2)
pln(1 / 2)

```

- 变量声明与赋值

```
let a = 1
b := 2.3
let c
c = a + b
pln(c)
```

- 递归函数调用

```
func fib(n) {
    if n <= 1 {
        return n
    }

    return fib(n-1) + fib(n-2)
}

pln(fib(8))
```

- 并发（多线程）处理

```
func threadFunc1(startNum1) {
    for true {
        pln("Thread 1:", startNum1)
        sleep(0.1)
        startNum1++

        if startNum1 > 100 {
            return
        }
    }
}

func threadFunc2(startNum1) {
    for true {
        pln("Thread 2", startNum1)
        sleep(0.3)
        startNum1 += 10

        if startNum1 > 100 {
            return
        }
    }
}

thread(threadFunc1, 3)
thread(threadFunc2, 0)
```

# 其他测试用例

- 测试生成字节码，并直接运行字节码，传入参数并获取结果，例如：
```
let a = 1
var b
b = 2.3
c := a + b
pln(c)

return c
```

- 测试错误提示（除零错误）
```
let x = 10
let y = 0
let z = x / y
```
预期输出类似：`Runtime Error: division by zero at line 3: "let z = x / y"`

- 测试错误提示（未定义变量）
```
pln(undefinedVar)
```
预期输出类似：`Runtime Error: undefined variable 'undefinedVar' at line 1: "pln(undefinedVar)"`

- 测试错误提示（运算式类型无法转换）
```
let a = "hello"
let b = 123 + a
```
预期输出类似：`Runtime Error: string "hello" cannot be converted to number: "let b = 123 + a"`

- 测试错误提示（函数参数错误）
```
func add(a, b) {
    return a + b
}
pln(add(1))
```
预期输出类似：`Runtime Error: function 'add' expects 2 arguments, got 1 at line 5: "pln(add(1))"`

- 测试错误提示（数组越界）
```
let arr = [1, 2, 3]
pln(arr[10])
```
预期输出类似：`Runtime Error: index out of bounds: 10 (array length: 3) at line 2: "pln(arr[10])"`

- 测试字节码的生成与运行·测试用例，例如将下面的代码生成字节码文件，然后运行该文件，预期输出为`3`：
```
let a = 1
let b = 2
let c = a + b
pln(c)
```
预期输出：：`3`

- 测试在Go语言中作为库调用并传入参数，获得执行结果：
```go
package main

import (
    "fmt"
    "github.com/topxeq/nxlang"
)

func main() {
    source := `
param a, b, c

result := a + b + c
pln("a + b + c =", result)

return result
    `
    
    bytecode, err := nxlang.Compile([]byte(source), nxlang.DefaultCompilerOptions)
    if err != nil {
        panic(err)
    }
    
    vm := nxlang.NewVM(bytecode)
    result, err := vm.Run(nil, 1, 2, 3)
    if err != nil {
        panic(err)
    }
    
    fmt.Println("Result:", result)
}
```
预期输出：
```
a + b + c = 6
Result: 6
```

- 测试在Go语言中作为库调用并传入参数，参数中既包括固定参数，也包含可变参数，并执行获得执行结果：
```go
package main

import (
    "fmt"
    "github.com/topxeq/nxlang"
)

func main() {
    source := `
param (a, b, vargs...)

pln("a:", a)
pln("b:", b)
pln("vargs:", vargs)

sum := a + b
for i, v in vargs {
    sum = sum + v
}

return sum
    `
    
    bytecode, err := nxlang.Compile([]byte(source), nxlang.DefaultCompilerOptions)
    if err != nil {
        panic(err)
    }
    
    vm := nxlang.NewVM(bytecode)
    result, err := vm.Run(nil, 1, 2, 3, 4, 5)
    if err != nil {
        panic(err)
    }
    
    fmt.Println("Result:", result)
}
```
预期输出：
```
a:1
b:2
vargs:[3 4 5]
Result: 15
```

- 测试isErr函数：
```
pln("isErr('hello'):", isErr("hello"))
pln("isErr('TXERROR: something'):", isErr("TXERROR: something"))

let err1 = error("test error")
pln("isErr(error('test error')):", isErr(err1))

pln("isErr(123):", isErr(123))
pln("isErr(true):", isErr(true))
pln("isErr([1, 2, 3]):", isErr([1, 2, 3]))
pln("isErr({'a': 1}):", isErr({"a": 1}))

pln("isErr(undefined):", isErr(undefined))
```
预期输出：
```
isErr('hello'):false
isErr('TXERROR: something'):true
isErr(error('test error')):true
isErr(123):false
isErr(true):false
isErr([1, 2, 3]):false
isErr({'a': 1}):false
isErr(undefined):true
```

- 测试char类型（单引号括起来的Unicode字符）：
```
let c1 = 'A'
pln("c1:", c1)

let c2 = '中'
pln("c2:", c2)

let c3 = '\n'
pln("c3 (newline):", c3)

let c4 = '\t'
pln("c4 (tab):", c4)

let c5 = '\\'
pln("c5 (backslash):", c5)

pln("typeOf('A'):", typeOf('A'))
pln("typeOf(\"A\"):", typeOf("A"))
```
预期输出：
```
c1:65
c2:20013
c3 (newline):10
c4 (tab):9
c5 (backslash):92
typeOf('A'):char
typeOf("A"):string
```

- 测试 uint 类型：
```
// 测试uint类型的功能
pln("=== 测试 uint 类型 ===")

// 测试1: 基本uint转换
pln("测试1: 基本uint转换")
pln("uint(123):", uint(123))
pln("uint(3.14):", uint(3.14))
pln("uint(true):", uint(true))
pln("uint(false):", uint(false))
pln("uint('456'):", uint("456"))
pln("uint(byte(65)):", uint(byte(65)))

// 测试2: 类型检查
pln("\n测试2: 类型检查")
let u1 = uint(123)
pln("typeOf(uint(123)):", typeOf(u1))
pln("typeCode(uint(123)):", typeCode(u1))
pln("typeName(uint(123)):", typeName(u1))

// 测试3: 负数转换（应该报错）
pln("\n测试3: 负数转换")
pln("uint(-123):", uint(-123))
pln("uint(-3.14):", uint(-3.14))

// 测试4: 其他类型转换为uint
pln("\n测试4: 其他类型转换为uint")
pln("uint('0'):", uint("0"))
pln("uint('1000'):", uint("1000"))

// 测试5: uint转换为其他类型
pln("\n测试5: uint转换为其他类型")
let u2 = uint(255)
pln("int(u2):", int(u2))
pln("float(u2):", float(u2))
pln("bool(u2):", bool(u2))
pln("string(u2):", string(u2))
```
预期输出：
```
=== 测试 uint 类型 ===
测试1: 基本uint转换
uint(123):123
uint(3.14):3
uint(true):1
uint(false):0
uint('456'):456
uint(byte(65)):65

测试2: 类型检查
typeOf(uint(123)):uint
typeCode(uint(123)):uint
typeName(uint(123)):unsigned integer

测试3: 负数转换
uint(-123):TXERROR: int value cannot be negative for uint
uint(-3.14):TXERROR: float value cannot be negative for uint

测试4: 其他类型转换为uint
uint('0'):0
uint('1000'):1000

测试5: uint转换为其他类型
int(u2):255
float(u2):255
bool(u2):true
string(u2):255
```

- 测试 typeOf 和 typeCode 函数：
```
// 测试所有类型的typeOf和typeCode函数输出
pln("=== 测试 typeOf 和 typeCode 函数 ===")

// 测试布尔类型
pln("布尔类型:")
pln("typeOf(true):", typeOf(true))
pln("typeCode(true):", typeCode(true))

// 测试整数类型
pln("\n整数类型:")
pln("typeOf(123):", typeOf(123))
pln("typeCode(123):", typeCode(123))

// 测试浮点数类型
pln("\n浮点数类型:")
pln("typeOf(3.14):", typeOf(3.14))
pln("typeCode(3.14):", typeCode(3.14))

// 测试字符串类型
pln("\n字符串类型:")
pln("typeOf('hello'):", typeOf("hello"))
pln("typeCode('hello'):", typeCode("hello"))

// 测试字符类型
pln("\n字符类型:")
pln("typeOf('A'):", typeOf('A'))
pln("typeCode('A'):", typeCode('A'))

// 测试字节类型
pln("\n字节类型:")
pln("typeOf(byte(65)):", typeOf(byte(65)))
pln("typeCode(byte(65)):", typeCode(byte(65)))

// 测试数组类型
pln("\n数组类型:")
pln("typeOf([1, 2, 3]):", typeOf([1, 2, 3]))
pln("typeCode([1, 2, 3]):", typeCode([1, 2, 3]))

// 测试对象类型
pln("\n对象类型:")
pln("typeOf({a: 1}):", typeOf({a: 1}))
pln("typeCode({a: 1}):", typeCode({a: 1}))

// 测试函数类型
pln("\n函数类型:")
func funcTest(x) { return x * 2 }
pln("typeOf(funcTest):", typeOf(funcTest))
pln("typeCode(funcTest):", typeCode(funcTest))

// 测试未定义类型
pln("\n未定义类型:")
let undefinedVar
pln("typeOf(undefinedVar):", typeOf(undefinedVar))
pln("typeCode(undefinedVar):", typeCode(undefinedVar))

// 测试空值类型
pln("\n空值类型:")
let nullVar = null
pln("typeOf(nullVar):", typeOf(nullVar))
pln("typeCode(nullVar):", typeCode(nullVar))
```
预期输出：
```
=== 测试 typeOf 和 typeCode 函数 ===
布尔类型:
typeOf(true):bool
typeCode(true):bool

整数类型:
typeOf(123):int
typeCode(123):int

浮点数类型:
typeOf(3.14):float
typeCode(3.14):float

字符串类型:
typeOf('hello'):string
typeCode('hello'):string

字符类型:
typeOf('A'):char
typeCode('A'):char

字节类型:
typeOf(byte(65)):byte
typeCode(byte(65)):byte

数组类型:
typeOf([1, 2, 3]):array
typeCode([1, 2, 3]):array

对象类型:
typeOf({a: 1}):map
typeCode({a: 1}):map

函数类型:
typeOf(funcTest):func
typeCode(funcTest):func

未定义类型:
typeOf(undefinedVar):undefined
typeCode(undefinedVar):undefined

空值类型:
typeOf(nullVar):undefined
typeCode(nullVar):undefined
```

- 测试各种数据类型转换：
```
pln("=== int() 转换 ===")
pln("int(3.14):", int(3.14))
pln("int(\"123\"):", int("123"))
pln("int(true):", int(true))
pln("int(false):", int(false))

pln("=== float() 转换 ===")
pln("float(123):", float(123))
pln("float(\"3.14\"):", float("3.14"))
pln("float(true):", float(true))

pln("=== bool() 转换 ===")
pln("bool(0):", bool(0))
pln("bool(1):", bool(1))
pln("bool(\"\"):", bool(""))
pln("bool(\"hello\"):", bool("hello"))
pln("bool([]):", bool([]))
pln("bool([1,2]):", bool([1,2]))

pln("=== byte() 转换 ===")
pln("byte(65):", byte(65))
pln("byte(255):", byte(255))
pln("byte('A'):", byte('A'))
pln("byte(\"A\"):", byte("A"))

pln("=== string() 转换 ===")
pln("string(123):", string(123))
pln("string(3.14):", string(3.14))
pln("string(true):", string(true))
pln("string('A'):", string('A'))
pln("string([1,2,3]):", string([1,2,3]))

pln("=== toStr() 转换 ===")
pln("toStr(123):", toStr(123))
pln("toStr(3.14):", toStr(3.14))
pln("toStr(true):", toStr(true))
pln("toStr('A'):", toStr('A'))

pln("=== bytes() 转换 ===")
pln("bytes(\"Hello\"):", bytes("Hello"))

pln("=== chars() 转换 ===")
pln("chars(\"Hello\"):", chars("Hello"))

pln("=== typeCode() 和 typeName() ===")
pln("typeCode(123):", typeCode(123))
pln("typeName(123):", typeName(123))
pln("typeCode(3.14):", typeCode(3.14))
pln("typeName(3.14):", typeName(3.14))
pln("typeCode(\"hello\"):", typeCode("hello"))
pln("typeName(\"hello\"):", typeName("hello"))
pln("typeCode('A'):", typeCode('A'))
pln("typeName('A'):", typeName('A'))
pln("typeCode(true):", typeCode(true))
pln("typeName(true):", typeName(true))
pln("typeCode([1,2,3]):", typeCode([1,2,3]))
pln("typeName([1,2,3]):", typeName([1,2,3]))
pln("typeCode({\"a\":1}):", typeCode({"a":1}))
pln("typeName({\"a\":1}):", typeName({"a":1}))
```
预期输出：
```
=== int() 转换 ===
int(3.14):3
int("123"):123
int(true):1
int(false):0
=== float() 转换 ===
float(123):123
float("3.14"):3.14
float(true):1
=== bool() 转换 ===
bool(0):false
bool(1):true
bool(""):false
bool("hello"):true
bool([]):false
bool([1,2]):true
=== byte() 转换 ===
byte(65):65
byte(255):255
byte('A'):65
byte("A"):65
=== string() 转换 ===
string(123):123
string(3.14):3.14
string(true):true
string('A'):A
string([1,2,3]):[1 2 3]
=== toStr() 转换 ===
toStr(123):123
toStr(3.14):3.14
toStr(true):true
toStr('A'):A
=== bytes() 转换 ===
bytes("Hello"):[72 101 108 108 111]
=== chars() 转换 ===
chars("Hello"):[72 101 108 108 111]
=== typeCode() 和 typeName() ===
typeCode(123):int
typeName(123):integer
typeCode(3.14):float
typeName(3.14):float
typeCode("hello"):string
typeName("hello"):string
typeCode('A'):char
typeName('A'):character
typeCode(true):bool
typeName(true):boolean
typeCode([1,2,3]):array
typeName([1,2,3]):array
typeCode({"a":1}):map
typeName({"a":1}):map
```

- 测试无法转换的情况（返回error对象）：
```
pln("=== 无法转换的情况 ===")

pln("int(\"abc\"):", int("abc"))
pln("isErr(int(\"abc\")):", isErr(int("abc")))

pln("float(\"abc\"):", float("abc"))
pln("isErr(float(\"abc\")):", isErr(float("abc")))

pln("byte(300):", byte(300))
pln("isErr(byte(300)):", isErr(byte(300)))

pln("byte(-1):", byte(-1))
pln("isErr(byte(-1)):", isErr(byte(-1)))

pln("byte(\"\"):", byte(""))
pln("isErr(byte(\"\"):", isErr(byte("")))

pln("byte([1,2,3]):", byte([1,2,3]))
pln("isErr(byte([1,2,3])):", isErr(byte([1,2,3])))

pln("int([1,2,3]):", int([1,2,3]))
pln("isErr(int([1,2,3])):", isErr(int([1,2,3])))

pln("float({\"a\":1}):", float({"a":1}))
pln("isErr(float({\"a\":1})):", isErr(float({"a":1})))
```
预期输出：
```
=== 无法转换的情况 ===
int("abc"):TXERROR: cannot convert string to int
isErr(int("abc")):true
float("abc"):TXERROR: cannot convert string to float
isErr(float("abc")):true
byte(300):TXERROR: int value out of byte range
isErr(byte(300)):true
byte(-1):TXERROR: int value out of byte range
isErr(byte(-1)):true
byte(""):TXERROR: cannot convert empty string to byte
isErr(byte("")):true
byte([1,2,3]):TXERROR: cannot convert to byte
isErr(byte([1,2,3])):true
int([1,2,3]):TXERROR: cannot convert to int
isErr(int([1,2,3])):true
float({"a":1}):TXERROR: cannot convert to float
isErr(float({"a":1})):true
```

- 测试表达式计算时左侧优先的类型转换：
```
pln("=== 左侧优先的类型转换 ===")

pln("int + float:", 1 + 2.5)
pln("float + int:", 2.5 + 1)

pln("int + string:", 123 + "456")
pln("string + int:", "abc" + 123)

pln("float + string:", 3.14 + "2.5")
pln("string + float:", "test" + 3.14)

pln("string + bool:", "value:" + true)
pln("string + array:", "arr:" + [1, 2, 3])
pln("string + map:", "map:" + {"a": 1})

pln("typeof(1 + 2.5):", typeof(1 + 2.5))
pln("typeof(2.5 + 1):", typeof(2.5 + 1))
pln("typeof(123 + \"456\"):", typeof(123 + "456"))
pln("typeof(\"abc\" + 123):", typeof("abc" + 123))
```
预期输出：
```
=== 左侧优先的类型转换 ===
int + float:3
float + int:3.5
int + string:579
string + int:abc123
float + string:5.64
string + float:test3.14
string + bool:value:true
string + array:arr:[1 2 3]
string + map:map:map[a:1]
typeof(1 + 2.5):number
typeof(2.5 + 1):number
typeof(123 + "456"):number
typeof("abc" + 123):string
```

- 测试表达式计算时无法转换的情况（返回error对象）：
```
pln("=== 无法转换的情况 ===")

let r1 = 1 + "abc"
pln("1 + \"abc\":", r1)
pln("isErr(1 + \"abc\"):", isErr(r1))

let r2 = 3.14 + "xyz"
pln("3.14 + \"xyz\":", r2)
pln("isErr(3.14 + \"xyz\"):", isErr(r2))

let r3 = 1 + [1,2,3]
pln("1 + [1,2,3]:", r3)
pln("isErr(1 + [1,2,3]):", isErr(r3))

let r4 = 2.5 + {"a":1}
pln("2.5 + {\"a\":1}:", r4)
pln("isErr(2.5 + {\"a\":1}):", isErr(r4))
```
预期输出：
```
=== 无法转换的情况 ===
1 + "abc":cannot convert string to int for addition
isErr(1 + "abc"):true
3.14 + "xyz":cannot convert string to float for addition
isErr(3.14 + "xyz"):true
1 + [1,2,3]:unsupported type for int addition
isErr(1 + [1,2,3]):true
2.5 + {"a":1}:unsupported type for float addition
isErr(2.5 + {"a":1}):true
```

- 测试复杂表达式运算（包含数值、变量、函数、多步骤计算）：
```
pln("=== 复杂表达式运算 ===")

let a = 10
let b = 20
let c = 3

pln("a + b * c:", a + b * c)
pln("(a + b) * c:", (a + b) * c)
pln("a * b - c:", a * b - c)
pln("a + b / c:", a + b / c)

func square(x) {
    return x * x
}

func add(x, y) {
    return x + y
}

pln("square(5):", square(5))
pln("add(3, 4):", add(3, 4))
pln("square(a) + b:", square(a) + b)
pln("add(a, b) * c:", add(a, b) * c)

let d = square(add(a, b))
pln("d = square(add(a, b)):", d)

let e = (a + b) * (c - 1) / 2
pln("e = (a + b) * (c - 1) / 2:", e)

func factorial(n) {
    if n <= 1 {
        return 1
    }
    return n * factorial(n - 1)
}

pln("factorial(5):", factorial(5))
pln("factorial(3) + square(4):", factorial(3) + square(4))

let arr = [1, 2, 3, 4, 5]
let sum = 0
for i, v in arr {
    sum = sum + v
}
pln("sum of arr:", sum)

pln("typeof(square):", typeof(square))
pln("typeof(factorial(5)):", typeof(factorial(5)))
```
预期输出：
```
=== 复杂表达式运算 ===
a + b * c:70
(a + b) * c:90
a * b - c:197
a + b / c:16
square(5):25
add(3, 4):7
square(a) + b:120
add(a, b) * c:90
d = square(add(a, b)):900
e = (a + b) * (c - 1) / 2:30
factorial(5):120
factorial(3) + square(4):22
sum of arr:15
typeof(square):function
typeof(factorial(5)):number
```

- 测试compile和runByteCode函数：
```
let source1 = "let a = 1
let b = 2
return a + b"

let bc = compile(source1)
pln("Bytecode compiled")

let result = runByteCode(bc)
pln("Result:", result)

let source2 = "param x, y
return x * y"

let bc2 = compile(source2)
let result2 = runByteCode(bc2, 3, 4)
pln("Result2:", result2)
```
预期输出：
```
Bytecode compiled
Result:3
Result2:12
```

- 测试静态方法：
```
// 测试int.parse静态方法
pln("=== 测试静态方法 ===")
let num1 = int.parse("123")
pln("int.parse(\"123\"):", num1)
pln("typeOf(int.parse(\"123\")):", typeOf(num1))

// 测试float.parse静态方法
let num2 = float.parse("3.14")
pln("float.parse(\"3.14\"):", num2)
pln("typeOf(float.parse(\"3.14\")):", typeOf(num2))

// 测试无法解析的情况
let err1 = int.parse("abc")
pln("int.parse(\"abc\"):", err1)
pln("isErr(int.parse(\"abc\")):", isErr(err1))

let err2 = float.parse("xyz")
pln("float.parse(\"xyz\"):", err2)
pln("isErr(float.parse(\"xyz\")):", isErr(err2))
```
预期输出：
```
=== 测试静态方法 ===
int.parse("123"):123
typeOf(int.parse("123")):int
float.parse("3.14"):3.14
typeOf(float.parse("3.14")):float
int.parse("abc"):TXERROR: cannot convert string to int
isErr(int.parse("abc")):true
float.parse("xyz"):TXERROR: cannot convert string to float
isErr(float.parse("xyz")):true
```

- 测试常量：
```
// 测试const关键字定义常量
pln("=== 测试常量 ===")
const PI = 3.14159
const NAME = "Nxlang"
const COUNT = 100

pln("PI:", PI)
pln("NAME:", NAME)
pln("COUNT:", COUNT)

// 测试预定义常量
pln("\n=== 测试预定义常量 ===")
pln("piC:", piC)
pln("eC:", eC)
```
预期输出：
```
=== 测试常量 ===
PI:3.14159
NAME:Nxlang
COUNT:100

=== 测试预定义常量 ===
piC:3.141592653589793
eC:2.718281828459045
```

- 测试数组操作：
```
// 测试数组基本操作
pln("=== 测试数组操作 ===")

// 创建数组
let arr1 = [1, 2, 3, 4, 5]
pln("arr1:", arr1)

// 访问数组元素
pln("arr1[0]:", arr1[0])
pln("arr1[2]:", arr1[2])

// 修改数组元素
arr1[1] = 10
pln("arr1 after modification:", arr1)

// 数组长度
pln("length of arr1:", len(arr1))

// 向数组追加元素
let arr2 = append(arr1, 6, 7, 8)
pln("arr2 after append:", arr2)

// 数组遍历
pln("\nArray traversal:")
for i, v in arr2 {
    pln("index ", i, ": ", v)
}

// 空数组
let emptyArr = []
pln("\nempty array:", emptyArr)
pln("length of empty array:", len(emptyArr))
```
预期输出：
```
=== 测试数组操作 ===
arr1:[1 2 3 4 5]
arr1[0]:1
arr1[2]:3
arr1 after modification:[1 10 3 4 5]
length of arr1:5
arr2 after append:[1 10 3 4 5 6 7 8]

Array traversal:
index 0: 1
index 1: 10
index 2: 3
index 3: 4
index 4: 5
index 5: 6
index 6: 7
index 7: 8

empty array:[]
length of empty array:0
```

- 测试映射（map）操作：
```
// 测试映射基本操作
pln("=== 测试映射操作 ===")

// 创建映射
let map1 = {"name": "Tom", "age": 16, "active": true}
pln("map1:", map1)

// 访问映射元素
pln("map1['name']:", map1["name"])
pln("map1['age']:", map1["age"])

// 修改映射元素
map1["age"] = 17
pln("map1 after modification:", map1)

// 添加新元素
map1["city"] = "New York"
pln("map1 after adding city:", map1)

// 映射遍历
pln("\nMap traversal:")
for k, v in map1 {
    pln("key:", k, ", value:", v)
}

// 空映射
let emptyMap = {}
pln("\nempty map:", emptyMap)
```
预期输出：
```
=== 测试映射操作 ===
map1:map[name:Tom age:16 active:true]
map1['name']:Tom
map1['age']:16
map1 after modification:map[name:Tom age:17 active:true]
map1 after adding city:map[name:Tom age:17 active:true city:New York]

Map traversal:
key:name, value:Tom
key:age, value:17
key:active, value:true
key:city, value:New York

empty map:map[]
```

- 测试字符串操作：
```
// 测试字符串基本操作
pln("=== 测试字符串操作 ===")

let str1 = "Hello, World!"
pln("str1:", str1)

// 字符串长度
pln("length of str1:", len(str1))

// 字符串转换
pln("toUpper(str1):", toUpper(str1))
pln("toLower(str1):", toLower(str1))

// 字符串 trim
let str2 = "   Hello   "
pln("str2:", str2)
pln("trim(str2):", trim(str2))

// 字符串分割
let str3 = "a,b,c,d"
let parts = split(str3, ",")
pln("split('a,b,c,d', ','):", parts)

// 字符串拼接
let str4 = "Hello" + " " + "World"
pln("str4:", str4)

// 多行字符串
let multiStr = `
This is a
multi-line
string
`
pln("multi-line string:", multiStr)
```
预期输出：
```
=== 测试字符串操作 ===
str1:Hello, World!
length of str1:13
toUpper(str1):HELLO, WORLD!
toLower(str1):hello, world!
str2:   Hello   
trim(str2):Hello
split('a,b,c,d', ','):[a b c d]
str4:Hello World
multi-line string:

This is a
multi-line
string

```

- 测试时间函数：
```
// 测试时间函数
pln("=== 测试时间函数 ===")

// 当前时间
let nowTime = now()
pln("now():", nowTime)

// Unix时间戳
let unixTime = unix()
pln("unix():", unixTime)

let unixMilliTime = unixMilli()
pln("unixMilli():", unixMilliTime)

// 格式化时间
let formattedTime = formatTime(nowTime, "2006-01-02 15:04:05")
pln("formatted time:", formattedTime)

// 解析时间
let parsedTime = parseTime("2026-01-01 12:00:00", "2006-01-02 15:04:05")
pln("parsed time:", parsedTime)
```
预期输出：
```
=== 测试时间函数 ===
now():2026-03-06 12:00:00 +0000 UTC
unix():1770331200
unixMilli():1770331200000
formatted time:2026-03-06 12:00:00
parsed time:2026-01-01 12:00:00 +0000 UTC
```

- 测试线程同步（使用mutex）：
```
// 测试线程同步
pln("=== 测试线程同步 ===")

let counter = 0
let mutex = mutex()

func increment() {
    for i in 100 {
        mutex.lock()
        counter++
        mutex.unlock()
        sleep(0.001)
    }
}

// 启动多个线程
thread(increment)
thread(increment)
thread(increment)

// 等待线程完成
sleep(1)

pln("Final counter value:", counter)
```
预期输出：
```
=== 测试线程同步 ===
Final counter value:300
```

- 测试异常处理（try...catch...finally）：
```
// 测试异常处理
pln("=== 测试异常处理 ===")

try {
    let x = 10 / 0
    pln("This line should not execute")
} catch (e) {
    pln("Caught exception:", e)
} finally {
    pln("Finally block executed")
}

// 测试defer
pln("\n=== 测试defer ===")

defer {
    pln("Deferred statement executed")
}

pln("Main code executed")
```
预期输出：
```
=== 测试异常处理 ===
Caught exception:division by zero
Finally block executed

=== 测试defer ===
Main code executed
Deferred statement executed
```

- 测试对象类定义和使用：
```
// 测试对象类定义和使用
pln("=== 测试对象类定义 ===")

class Person {
    func init(name, age) {
        this.name = name
        this.age = age
    }
    
    func greet() {
        return "Hello, my name is " + this.name + " and I'm " + this.age + " years old"
    }
    
    func setAge(newAge) {
        this.age = newAge
    }
}

// 创建对象实例
let person1 = Person("Tom", 16)
pln("person1.greet():", person1.greet())

// 修改对象属性
person1.setAge(17)
pln("person1.greet() after age change:", person1.greet())

// 访问对象属性
pln("person1.name:", person1.name)
pln("person1.age:", person1.age)
```
预期输出：
```
=== 测试对象类定义 ===
person1.greet():Hello, my name is Tom and I'm 16 years old
person1.greet() after age change:Hello, my name is Tom and I'm 17 years old
person1.name:Tom
person1.age:17
```

- 测试添加方法和成员变量：
```
// 测试添加方法和成员变量
pln("=== 测试添加方法和成员变量 ===")

// 为int类型添加方法
addMethod("int", "square", func() {
    return this * this
})

// 为int类型添加成员变量
addMember("int", "description", "integer type")

// 测试添加的方法和成员
let num = 5
pln("num.square():", num.square())
pln("num.description:", num.description)
```
预期输出：
```
=== 测试添加方法和成员变量 ===
num.square():25
num.description:integer type
```

- 测试JSON转换：
```
// 测试JSON转换
pln("=== 测试JSON转换 ===")

let data = {"name": "Tom", "age": 16, "active": true, "scores": [85, 90, 95]}

// 转换为JSON
let jsonStr = toJson(data)
pln("JSON string:", jsonStr)

// 转换为JSON并排序
let sortedJson = toJson(data, "-sort")
pln("Sorted JSON:", sortedJson)

// 转换为JSON并添加缩进
let indentedJson = toJson(data, "-indent")
pln("Indented JSON:", indentedJson)
```
预期输出：
```
=== 测试JSON转换 ===
JSON string:{"name":"Tom","age":16,"active":true,"scores":[85,90,95]}
Sorted JSON:{"active":true,"age":16,"name":"Tom","scores":[85,90,95]}
Indented JSON:{
  "name": "Tom",
  "age": 16,
  "active": true,
  "scores": [
    85,
    90,
    95
  ]
}
```
