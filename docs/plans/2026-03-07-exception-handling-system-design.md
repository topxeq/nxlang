# 异常处理系统设计文档

## 1. 概述
本文档描述Nxlang异常处理系统的设计与实现方案，包含try/catch/finally语句、defer延迟执行语句、自定义错误类型和调用栈信息收集四大核心功能。

## 2. 核心设计

### 2.1 架构概述
采用基于栈的异常处理表 + defer链表的设计方案：
- 编译阶段：为每个try块生成异常处理表，记录各块的指令偏移
- 执行阶段：VM维护异常处理栈和defer链表，异常发生时自动查找处理程序

### 2.2 字节码指令扩展
新增以下字节码指令：
| 指令 | 操作数 | 功能描述 |
|------|--------|----------|
| OpEnterTry | try_offset, catch_offset, finally_offset | 进入try块，注册异常处理程序 |
| OpExitTry | 0 | 正常退出try块，注销异常处理程序 |
| OpDefer | func_index, arg_count | 注册延迟执行函数 |

### 2.3 编译器实现
1. **Try语句编译**：
   - 生成OpEnterTry指令，记录try、catch、finally块的位置
   - 编译try块内容，结尾生成OpExitTry指令
   - 编译catch块，将异常对象存储到catch参数中
   - 编译finally块，在try正常退出和catch处理后都执行

2. **Defer语句编译**：
   - 生成OpDefer指令，将defer的函数调用注册到当前函数帧的defer链表

3. **Throw语句编译**：
   - 编译异常表达式，生成OpThrow指令抛出异常

### 2.4 VM实现
1. **帧结构扩展**：
   ```go
   type Frame struct {
       // 现有字段...
       deferStack []*DeferCall // 延迟调用栈
       exceptionHandler *ExceptionHandler // 当前异常处理程序
   }
   ```

2. **异常处理流程**：
   - 异常抛出时，从当前帧开始向上查找异常处理程序
   - 执行所有当前帧的defer函数
   - 跳转到catch块处理异常
   - 最后执行finally块

3. **Defer执行流程**：
   - 函数正常返回或异常抛出时，逆序执行defer链表中的所有调用
   - defer执行时抛出的异常会覆盖原有异常

### 2.5 错误类型扩展
1. **自定义错误支持**：
   - 扩展Error类型，支持错误码、错误消息、调用栈、自定义属性
   - 提供`error(message, code)`内置函数创建自定义错误

2. **调用栈收集**：
   - 异常发生时遍历调用栈，收集每个帧的函数名、行号、指令位置
   - 错误对象包含完整调用栈信息，打印时自动显示

## 3. 实现计划

### 阶段1：字节码与VM基础结构扩展
- 新增异常处理相关字节码指令
- 扩展Frame结构支持defer栈和异常处理程序
- 实现OpThrow指令的异常抛出逻辑

### 阶段2：编译器支持
- 实现TryStatement、CatchStatement、FinallyStatement的编译
- 实现DeferStatement的编译
- 实现ThrowStatement的编译

### 阶段3：VM异常处理逻辑
- 实现异常处理栈的维护逻辑
- 实现defer函数的注册与执行逻辑
- 实现异常的查找与跳转逻辑

### 阶段4：错误类型与调用栈
- 扩展Error类型支持自定义属性和调用栈
- 实现调用栈信息收集功能
- 提供内置函数支持自定义错误创建

### 阶段5：测试与优化
- 编写全面的测试用例覆盖所有异常处理场景
- 优化性能，减少正常执行路径的开销
- 完善错误消息和调试信息

## 4. 示例语法
```nx
// try/catch/finally示例
try {
    let x = 10 / 0
    pln("This line won't execute")
} catch (e) {
    pln("Caught error:", e.message)
    pln("Error code:", e.code)
    pln("Stack trace:", e.stack)
} finally {
    pln("Cleanup logic always executes")
}

// defer示例
func readFile(path) {
    let f = openFile(path)
    defer f.close() // 函数退出时自动关闭文件

    let content = f.readAll()
    return content
}

// 自定义错误示例
throw error("File not found", 404, { path: "/tmp/file.txt" })
```
