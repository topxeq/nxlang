# Nxlang (能效语言)

轻量级Go生态脚本语言，类Go语法，字节码虚拟机，跨平台运行，专为高效开发和性能平衡设计。

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/topxeq/nxlang)](https://goreportcard.com/report/github.com/topxeq/nxlang)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.21-blue.svg)](https://golang.org/dl/)

[English Version](./README.md)

## ✨ 核心特性

- **熟悉的语法**: 类Go语法设计，学习成本低，无缝适配Go开发者习惯
- **弱类型系统**: 自动类型转换，编写灵活高效，减少冗余类型声明
- **字节码执行**: 编译为平台无关字节码(.nxb)，运行效率高于传统解释型语言
- **内置REPL**: 交互式命令行，支持语法高亮，快速调试和原型开发
- **集成编辑器**: 内置代码编辑器，无需额外工具即可编写运行脚本
- **丰富标准库**: 内置并发、HTTP、文件IO、数据处理、图形等常用功能
- **跨平台支持**: 完美运行于Windows、Linux、macOS，无外部依赖
- **模块系统**: 支持模块导入导出，函数引用一致性保证，适合大型项目开发
- **UTF-8原生**: 全栈UTF-8支持，字符串和文件默认使用UTF-8编码
- **高性能**: 基于Go实现，运行性能远超Python/JavaScript等动态语言

## 🚀 快速开始

### 安装
从 [Releases](https://github.com/topxeq/nxlang/releases) 下载预编译二进制文件，或者从源码编译。

### 运行REPL（交互式环境）
```bash
nx
```

### 执行脚本
```bash
nx path/to/script.nx
```

### 编译为字节码
```bash
nx compile path/to/script.nx -o output.nxb
```

### 运行预编译字节码
```bash
nx run output.nxb
```

## 📝 示例代码
```nx
// Hello World
pln("Hello, Nxlang! 👋")

// 函数定义
func factorial(n) {
    if n <= 1 { return 1 }
    return n * factorial(n - 1)
}

pln("10的阶乘:", factorial(10))

// 模块导入
import { sqrt, random } from "math"
pln("sqrt(25) =", sqrt(25))
pln("随机数:", random())

// 内置数据结构
var fruits = array("苹果", "香蕉", "樱桃")
var person = map("name", "Bob", "age", 28, "city", "上海")

// 控制流
for (var i = 0; i < 5; i++) {
    pln("计数:", i)
}
```

## 📦 标准库

### 数学函数
`abs`, `sqrt`, `sin`, `cos`, `tan`, `floor`, `ceil`, `round`, `pow`, `random`

### 字符串函数
`len`, `toUpper`, `toLower`, `trim`, `split`, `join`, `contains`, `replace`, `substr`

### 集合函数
`array`, `append`, `map`, `orderedMap`, `stack`, `queue`, `keys`, `values`

### 时间函数
`now`, `unix`, `unixMilli`, `formatTime`, `sleep`

### JSON函数
`toJson(value, indent=false)` - 将值转换为JSON字符串

### 并发函数
`thread(func)` - 启动新线程运行给定函数
`mutex()` - 创建互斥锁
`rwMutex()` - 创建读写锁

### I/O函数
`pln(...)` - 打印值并换行
`pr(...)` - 打印值不换行
`printf(format, ...)` - 格式化打印

## 🏗️ 从源码编译
```bash
# 克隆仓库
git clone https://github.com/topxeq/nxlang.git
cd nxlang

# 编译二进制
go build -o nx ./cmd/nx

# 安装到系统（Linux/macOS）
sudo mv nx /usr/local/bin/
```

## 🧪 运行测试
```bash
# 运行所有测试
go test ./...

# 运行指定包测试
go test ./vm -v
```

## 🏛️ 架构设计
Nxlang采用标准的编译器-VM架构：
1. **词法分析器**: 将源代码转换为token流
2. **语法分析器**: 从token构建抽象语法树(AST)
3. **编译器**: 将AST转换为平台无关字节码
4. **虚拟机**: 基于栈执行字节码
5. **标准库**: Go实现的内置函数和类型

## 📄 许可证
MIT License - 查看 [LICENSE](LICENSE) 文件获取详细信息。

---

Made with ❤️ by the Nxlang Team
