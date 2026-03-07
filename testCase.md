# Nxlang 测试用例文档

## 测试环境
- 版本：Nxlang 1.0.0
- 系统：Windows/Linux
- 测试日期：2026-03-07

---

## 1. 基础变量声明测试

| 测试编号 | 测试描述 | 测试代码 | 预期输出 | 测试结果 |
|----------|----------|----------|----------|----------|
| TC001 | var声明变量 | `var a = 10<br>pln(a)` | 10 | ☐ 未测试 |
| TC002 | let声明变量 | `let b = 20.5<br>pln(b)` | 20.5 | ☐ 未测试 |
| TC003 | const声明常量 | `const c = "hello"<br>pln(c)` | hello | ☐ 未测试 |
| TC004 | 多变量声明 | `var x = 1, y = 2, z = 3<br>pln(x + y + z)` | 6 | ☐ 未测试 |

---

## 2. 基础数据类型测试

| 测试编号 | 测试描述 | 测试代码 | 预期输出 | 测试结果 |
|----------|----------|----------|----------|----------|
| TC011 | 整数类型 | `var i = 12345<br>pln(typeName(i))<br>pln(i)` | int<br>12345 | ☐ 未测试 |
| TC012 | 浮点数类型 | `var f = 123.456<br>pln(typeName(f))<br>pln(f)` | float<br>123.456 | ☐ 未测试 |
| TC013 | 布尔类型 | `var t = true<br>var f = false<br>pln(t, " ", f)` | true false | ☐ 未测试 |
| TC014 | 字符串类型 | `var s = "Hello 世界"<br>pln(typeName(s))<br>pln(len(s))` | string<br>8 | ☐ 未测试 |
| TC015 | null/undefined | `var n = nil<br>pln(typeName(n))` | null | ☐ 未测试 |

---

## 3. 运算符测试

| 测试编号 | 测试描述 | 测试代码 | 预期输出 | 测试结果 |
|----------|----------|----------|----------|----------|
| TC021 | 算术运算 | `var a = 10, b = 3<br>pln(a + b)<br>pln(a - b)<br>pln(a * b)<br>pln(a / b)<br>pln(a % b)` | 13<br>7<br>30<br>3.3333333333333335<br>1 | ☐ 未测试 |
| TC022 | 比较运算 | `var a = 10, b = 20<br>pln(a == b)<br>pln(a != b)<br>pln(a < b)<br>pln(a > b)<br>pln(a <= 10)<br>pln(a >= 10)` | false<br>true<br>true<br>false<br>true<br>true | ☐ 未测试 |
| TC023 | 逻辑运算 | `var t = true, f = false<br>pln(t && f)<br>pln(t || f)<br>pln(!t)` | false<br>true<br>false | ☐ 未测试 |
| TC024 | 位运算 | `var a = 60, b = 13<br>pln(a & b)<br>pln(a | b)<br>pln(a ^ b)<br>pln(a << 2)<br>pln(a >> 2)` | 12<br>61<br>49<br>240<br>15 | ☐ 未测试 |
| TC025 | 负数运算 | `var a = 10<br>pln(-a)` | -10 | ☐ 未测试 |

---

## 4. 字符串操作测试

| 测试编号 | 测试描述 | 测试代码 | 预期输出 | 测试结果 |
|----------|----------|----------|----------|----------|
| TC031 | 字符串拼接 | `var s1 = "Hello", s2 = "World"<br>pln(s1 + " " + s2)` | Hello World | ☐ 未测试 |
| TC032 | 字符串重复 | `var s = "abc"<br>pln(s * 3)` | abcabcabc | ☐ 未测试 |
| TC033 | 转义字符 | `pln("Line1\nLine2\n\tIndented")` | Line1<br>Line2<br>    Indented | ☐ 未测试 |
| TC034 | 索引访问 | `var s = "Hello"<br>pln(s[0])<br>pln(s[4])` | H<br>o | ☐ 未测试 |
| TC035 | 字符串函数 | `var s = "  Hello World  "<br>pln(toUpper(s))<br>pln(toLower(s))<br>pln(trim(s))<br>pln(contains(s, "World"))` | HELLO WORLD<br>hello world<br>Hello World<br>true | ☐ 未测试 |
| TC036 | 分割与合并 | `var s = "a,b,c,d"<br>var arr = split(s, ",")<br>pln(len(arr))<br>pln(join(arr, "|"))` | 4<br>a|b|c|d | ☐ 未测试 |

---

## 5. 控制流语句测试

| 测试编号 | 测试描述 | 测试代码 | 预期输出 | 测试结果 |
|----------|----------|----------|----------|----------|
| TC041 | if语句 | `var a = 10<br>if (a > 5) { pln("大于5") } else { pln("小于等于5") }` | 大于5 | ☐ 未测试 |
| TC042 | 多重if判断 | `var score = 85<br>if (score >= 90) { pln("A") } else if (score >= 80) { pln("B") } else { pln("C") }` | B | ☐ 未测试 |
| TC043 | for循环 | `var sum = 0<br>for (var i = 1; i <= 5; i++) { sum += i }<br>pln(sum)` | 15 | ☐ 未测试 |
| TC044 | break语句 | `var i = 0<br>for (true) { i++; if (i == 5) break; }<br>pln(i)` | 5 | ☐ 未测试 |
| TC045 | continue语句 | `var sum = 0<br>for (var i = 1; i <= 10; i++) { if (i % 2 == 0) continue; sum += i }<br>pln(sum)` | 25 | ☐ 未测试 |

---

## 6. 函数测试

| 测试编号 | 测试描述 | 测试代码 | 预期输出 | 测试结果 |
|----------|----------|----------|----------|----------|
| TC051 | 无参函数 | `func hello() { pln("Hello") }<br>hello()` | Hello | ☐ 未测试 |
| TC052 | 带参函数 | `func add(a, b) { return a + b }<br>pln(add(10, 20))` | 30 | ☐ 未测试 |
| TC053 | 多返回值 | `func swap(a, b) { return b, a }<br>var x = 1, y = 2<br>var newX, newY = swap(x, y)` | TODO | ☐ 未测试 |
| TC054 | 递归函数 | `func fib(n) { if (n <= 1) return n; return fib(n-1) + fib(n-2) }<br>pln(fib(10))` | 55 | ☐ 未测试 |
| TC055 | 嵌套调用 | `func square(x) { return x * x }<br>func sumOfSquares(a, b) { return square(a) + square(b) }<br>pln(sumOfSquares(3, 4))` | 25 | ☐ 未测试 |

---

## 7. 集合类型测试

| 测试编号 | 测试描述 | 测试代码 | 预期输出 | 测试结果 |
|----------|----------|----------|----------|----------|
| TC061 | 数组创建 | `var arr = array(1, 2, 3, 4, 5)<br>pln(len(arr))<br>pln(arr)` | 5<br>[1, 2, 3, 4, 5] | ☐ 未测试 |
| TC062 | 数组操作 | `var arr = array()<br>append(arr, 1)<br>append(arr, 2)<br>pln(arr[0])<br>arr[1] = 100<br>pln(arr)` | 1<br>[1, 100] | ☐ 未测试 |
| TC063 | Map创建 | `var m = map("name", "张三", "age", 25)<br>pln(m["name"])<br>pln(m["age"])` | 张三<br>25 | ☐ 未测试 |
| TC064 | Map操作 | `var m = map()<br>m["a"] = 1<br>m["b"] = 2<br>pln(keys(m))<br>pln(values(m))` | [a, b]<br>[1, 2] | ☐ 未测试 |
| TC065 | 有序Map | `var om = orderedMap("b", 2, "a", 1, "c", 3)<br>pln(keys(om))` | [b, a, c] | ☐ 未测试 |
| TC066 | Stack操作 | `var s = stack()<br>s.push(1)<br>s.push(2)<br>pln(s.pop())<br>pln(s.pop())` | 2<br>1 | ☐ 未测试 |
| TC067 | Queue操作 | `var q = queue()<br>q.enqueue(1)<br>q.enqueue(2)<br>pln(q.dequeue())<br>pln(q.dequeue())` | 1<br>2 | ☐ 未测试 |

---

## 8. 标准库函数测试

| 测试编号 | 测试描述 | 测试代码 | 预期输出 | 测试结果 |
|----------|----------|----------|----------|----------|
| TC071 | 数学函数 | `pln(abs(-10))<br>pln(sqrt(25))<br>pln(pow(2, 3))<br>pln(floor(3.7))<br>pln(ceil(3.2))` | 10<br>5<br>8<br>3<br>4 | ☐ 未测试 |
| TC072 | 时间函数 | `pln("Unix时间戳: " + unix())<br>pln("当前时间: " + formatTime())` | 输出当前时间戳和格式化时间 | ☐ 未测试 |
| TC073 | JSON函数 | `var obj = map("name", "test", "age", 20)<br>var jsonStr = toJson(obj, true)<br>pln(jsonStr)` | 格式化的JSON字符串 | ☐ 未测试 |
| TC074 | 类型函数 | `var x = 10<br>pln(typeName(x))<br>pln(typeCode(x))` | int<br>5 | ☐ 未测试 |
| TC075 | 随机数 | `var r = random()<br>pln(r >= 0 && r < 1)` | true | ☐ 未测试 |
| TC076 | sleep函数 | `var start = unixMilli()<br>sleep(100)<br>var end = unixMilli()<br>pln(end - start >= 100)` | true | ☐ 未测试 |

---

## 9. 错误边界测试

| 测试编号 | 测试描述 | 测试代码 | 预期输出 | 测试结果 |
|----------|----------|----------|----------|----------|
| TC081 | 除零错误 | `var a = 10 / 0` | 运行时错误：division by zero | ☐ 未测试 |
| TC082 | 数组越界访问 | `var arr = array(1, 2, 3)<br>pln(arr[100])` | undefined | ☐ 未测试 |
| TC083 | 调用非函数 | `var x = 10<br>x()` | 运行时错误：cannot call non-function type int | ☐ 未测试 |
| TC084 | 参数不足 | `func add(a, b) { return a + b }<br>add(10)` | 运行时错误：expected 2 arguments, got 1 | ☐ 未测试 |
| TC085 | 栈溢出保护 | `func recurse() { recurse() }<br>recurse()` | 运行时错误：stack overflow | ☐ 未测试 |

---

## 测试总结
| 测试用例总数 | 通过 | 失败 | 通过率 |
|--------------|------|------|--------|
| 45 | - | - | - |
